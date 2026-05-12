package core

import (
	stderrors "errors"
	"sort"

	"github.com/awesome-goose/goose/modules/sql"
	"gorm.io/gorm"
)

// Layer0 implements the low-level core primitives:
//   - AppendTransactionRows
//   - GetAccount, GetTransactionRow, GetGroup, ListGroups, ListTransactionRows, ListAccounts
//
// All mutating calls accept a *gorm.DB that must already be inside a tx;
// Layer1 owns the transaction boundary so it can also lock idempotency.
type Layer0 struct {
	db *sql.Db `inject:""`
}

// AppendOpts is the input set for AppendTransactionRows.
type AppendOpts struct {
	Rows []RowInput
}

// AppendTransactionRows atomically appends one or more transaction rows + entries
// and updates the cached balances on each account involved.
//
// The caller MUST pass a *gorm.DB that is already inside a DB transaction.
// This function additionally performs SELECT FOR UPDATE on every account in
// id order, the per-row precondition lock on the latest row of each group,
// and the prepaid overdraw check.
func (l *Layer0) AppendTransactionRows(tx *gorm.DB, opts AppendOpts) ([]AppendResult, *LedgerError) {
	if len(opts.Rows) == 0 {
		return nil, ErrBadRequest(ErrInvalidRequest, "no rows provided", nil)
	}

	// 1. Per-row shape validation.
	for i, row := range opts.Rows {
		if err := validateRow(row); err != nil {
			err.Details = MergeDetails(err.Details, map[string]any{"row": i})
			return nil, err
		}
	}

	// 2. Collect account ids referenced (primary + entries) so we lock once,
	//    in id order, before any insert. Spans across all rows in this batch.
	accountIDSet := map[int64]struct{}{}
	for _, row := range opts.Rows {
		accountIDSet[row.PrimaryAccountID] = struct{}{}
		for _, e := range row.Entries {
			accountIDSet[e.AccountID] = struct{}{}
		}
	}
	accountIDs := make([]int64, 0, len(accountIDSet))
	for id := range accountIDSet {
		accountIDs = append(accountIDs, id)
	}
	sort.Slice(accountIDs, func(i, j int) bool { return accountIDs[i] < accountIDs[j] })

	// 3. Lock + load accounts.
	accounts, err := lockAccounts(tx, accountIDs)
	if err != nil {
		return nil, err
	}

	// 4. Validate currency match per row & per-entry account state.
	for i, row := range opts.Rows {
		primaryAcct, ok := accounts[row.PrimaryAccountID]
		if !ok {
			return nil, ErrNotFoundf(ErrAccountNotFound,
				"primary account not found", map[string]any{"row": i, "account_id": row.PrimaryAccountID})
		}
		if primaryAcct.Currency != row.Currency {
			return nil, ErrBadRequest(ErrCurrencyMismatch,
				"primary account currency does not match row currency", map[string]any{"row": i})
		}
		for _, e := range row.Entries {
			acct, ok := accounts[e.AccountID]
			if !ok {
				return nil, ErrNotFoundf(ErrAccountNotFound,
					"entry account not found", map[string]any{"row": i, "account_id": e.AccountID})
			}
			if acct.Currency != row.Currency {
				return nil, ErrBadRequest(ErrCurrencyMismatch,
					"entry account currency does not match row currency",
					map[string]any{"row": i, "account_id": e.AccountID})
			}
			if acct.Status != AccountActive {
				return nil, ErrConflictf(ErrAccountFrozen,
					"account is not active", map[string]any{"account_id": e.AccountID, "status": string(acct.Status)})
			}
		}
	}

	// 5. Group precondition: if prior_status set, verify latest row of group;
	//    if NULL, ensure no rows exist for the group.
	for i, row := range opts.Rows {
		if err := checkGroupPrecondition(tx, row); err != nil {
			err.Details = MergeDetails(err.Details, map[string]any{"row": i})
			return nil, err
		}
	}

	// 6. Compute deltas across the batch (by account, summed) and verify
	//    no prepaid account ends up negative.
	aggregate := map[int64]*AccountDelta{}
	for _, row := range opts.Rows {
		deltas, derr := ComputeRowDeltas(row)
		if derr != nil {
			return nil, derr
		}
		for _, d := range deltas {
			agg, ok := aggregate[d.AccountID]
			if !ok {
				agg = &AccountDelta{AccountID: d.AccountID}
				aggregate[d.AccountID] = agg
			}
			agg.BalanceDiff += d.BalanceDiff
			agg.AvailableDiff += d.AvailableDiff
		}
	}
	for id, agg := range aggregate {
		acct := accounts[id]
		if acct.Class == ClassPrepaid {
			if acct.Balance+agg.BalanceDiff < 0 || acct.AvailableBalance+agg.AvailableDiff < 0 {
				return nil, ErrConflictf(ErrInsufficientFunds,
					"prepaid account would overdraw", map[string]any{
						"account_id":     id,
						"available":      acct.AvailableBalance,
						"available_diff": agg.AvailableDiff,
						"balance":        acct.Balance,
						"balance_diff":   agg.BalanceDiff,
					})
			}
		}
	}

	// 7. Insert rows + entries.
	results := make([]AppendResult, 0, len(opts.Rows))
	for _, row := range opts.Rows {
		txID, txCreatedAt, ierr := insertTransactionRow(tx, row)
		if ierr != nil {
			return nil, ierr
		}
		entries, ierr := insertEntries(tx, txID, row.Entries)
		if ierr != nil {
			return nil, ierr
		}

		rowView := TransactionRow{
			ID:               txID,
			GroupID:          row.GroupID,
			Status:           row.Status,
			PriorStatus:      row.PriorStatus,
			Type:             row.Type,
			PrimaryAccountID: row.PrimaryAccountID,
			Currency:         row.Currency,
			Tags:             row.Tags,
			IdempotencyKey:   row.IdempotencyKey,
			FxPairGroupID:    row.FxPairGroupID,
			Entries:          entries,
		}
		if t, ok := ToTime(txCreatedAt); ok {
			rowView.CreatedAt = t
		}
		results = append(results, AppendResult{Row: rowView, Entries: entries})
	}

	// 8. Apply aggregated cache update.
	for _, agg := range aggregate {
		if uerr := updateAccountCache(tx, agg.AccountID, agg.BalanceDiff, agg.AvailableDiff); uerr != nil {
			return nil, uerr
		}
	}

	return results, nil
}

// ─── reads (non-transactional) ────────────────────────────────────────────────

func (l *Layer0) GetAccount(id int64) (*Account, *LedgerError) {
	return getAccount(l.db.DB, id)
}

func (l *Layer0) ListAccounts(walletID string, currency, class, status *string, tagsFilter Tags, limit, offset int) ([]Account, *LedgerError) {
	q := l.db.DB.Table(`"WalletAccounts"`).Where(`"wallet_id" = ?`, walletID)
	if currency != nil {
		q = q.Where(`"currency" = ?`, *currency)
	}
	if class != nil {
		q = q.Where(`"class" = ?`, *class)
	}
	if status != nil {
		q = q.Where(`"status" = ?`, *status)
	}
	for k, v := range tagsFilter {
		q = q.Where(`"tags" ->> ? = ?`, k, v)
	}
	q = q.Order(`"id" DESC`)
	if limit > 0 {
		q = q.Limit(limit)
	}
	if offset > 0 {
		q = q.Offset(offset)
	}
	var rows []AccountRow
	if err := q.Scan(&rows).Error; err != nil {
		return nil, ErrInternalf("list accounts failed: " + err.Error())
	}
	out := make([]Account, len(rows))
	for i, r := range rows {
		out[i] = r.ToAccount()
	}
	return out, nil
}

func (l *Layer0) GetTransactionRow(id int64) (*TransactionRow, *LedgerError) {
	row, err := getTransactionRow(l.db.DB, id)
	if err != nil {
		return nil, err
	}
	entries, err := getEntriesForTx(l.db.DB, id)
	if err != nil {
		return nil, err
	}
	row.Entries = entries
	return row, nil
}

func (l *Layer0) GetGroup(groupID string) (*GroupView, *LedgerError) {
	var rows []txRow
	if err := l.db.DB.Table(`"WalletTransactions"`).
		Where(`"group_id" = ?`, groupID).
		Order(`"created_at" ASC, "id" ASC`).
		Scan(&rows).Error; err != nil {
		return nil, ErrInternalf("group lookup failed: " + err.Error())
	}
	if len(rows) == 0 {
		return nil, ErrNotFoundf(ErrGroupNotFound, "group not found", map[string]any{"group_id": groupID})
	}
	out := make([]TransactionRow, 0, len(rows))
	for _, r := range rows {
		t := r.toRow()
		entries, err := getEntriesForTx(l.db.DB, t.ID)
		if err != nil {
			return nil, err
		}
		t.Entries = entries
		out = append(out, t)
	}
	return &GroupView{
		GroupID:      groupID,
		LatestStatus: out[len(out)-1].Status,
		Rows:         out,
	}, nil
}

// ─── helpers ──────────────────────────────────────────────────────────────────

type AccountRow struct {
	ID               int64  `gorm:"column:id"`
	WalletID         string `gorm:"column:wallet_id"`
	Currency         string `gorm:"column:currency"`
	Class            string `gorm:"column:class"`
	Status           string `gorm:"column:status"`
	Balance          int64  `gorm:"column:balance"`
	AvailableBalance int64  `gorm:"column:available_balance"`
	Tags             []byte `gorm:"column:tags"`
	Metadata         []byte `gorm:"column:metadata"`
	CreatedAt        any    `gorm:"column:created_at"`
}

func (r AccountRow) ToAccount() Account {
	a := Account{
		ID:               r.ID,
		WalletID:         r.WalletID,
		Currency:         r.Currency,
		Class:            AccountClass(r.Class),
		Status:           AccountStatus(r.Status),
		Balance:          r.Balance,
		AvailableBalance: r.AvailableBalance,
	}
	a.Tags = ParseTags(r.Tags)
	a.Metadata = r.Metadata
	if t, ok := ToTime(r.CreatedAt); ok {
		a.CreatedAt = t
	}
	return a
}

type txRow struct {
	ID               int64   `gorm:"column:id"`
	GroupID          string  `gorm:"column:group_id"`
	Status           string  `gorm:"column:status"`
	PriorStatus      *string `gorm:"column:prior_status"`
	Type             string  `gorm:"column:type"`
	PrimaryAccountID int64   `gorm:"column:primary_account_id"`
	Currency         string  `gorm:"column:currency"`
	Tags             []byte  `gorm:"column:tags"`
	IdempotencyKey   *string `gorm:"column:idempotency_key"`
	FxPairGroupID    *string `gorm:"column:fx_pair_group_id"`
	CreatedAt        any     `gorm:"column:created_at"`
}

func (r txRow) toRow() TransactionRow {
	t := TransactionRow{
		ID:               r.ID,
		GroupID:          r.GroupID,
		Status:           TxStatus(r.Status),
		Type:             EntryType(r.Type),
		PrimaryAccountID: r.PrimaryAccountID,
		Currency:         r.Currency,
		IdempotencyKey:   r.IdempotencyKey,
		FxPairGroupID:    r.FxPairGroupID,
		Tags:             ParseTags(r.Tags),
	}
	if r.PriorStatus != nil {
		ps := TxStatus(*r.PriorStatus)
		t.PriorStatus = &ps
	}
	if ct, ok := ToTime(r.CreatedAt); ok {
		t.CreatedAt = ct
	}
	return t
}

type entryRow struct {
	ID            int64  `gorm:"column:id"`
	TransactionID int64  `gorm:"column:transaction_id"`
	AccountID     int64  `gorm:"column:account_id"`
	Type          string `gorm:"column:type"`
	Amount        int64  `gorm:"column:amount"`
	CreatedAt     any    `gorm:"column:created_at"`
}

func (r entryRow) toEntry() Entry {
	e := Entry{
		ID:            r.ID,
		TransactionID: r.TransactionID,
		AccountID:     r.AccountID,
		Type:          EntryType(r.Type),
		Amount:        r.Amount,
	}
	if t, ok := ToTime(r.CreatedAt); ok {
		e.CreatedAt = t
	}
	return e
}

func validateRow(row RowInput) *LedgerError {
	if row.GroupID == "" {
		return ErrBadRequest(ErrInvalidRequest, "group_id is required", nil)
	}
	if row.Type != TypeDebit && row.Type != TypeCredit {
		return ErrBadRequest(ErrInvalidRequest, "type must be DEBIT or CREDIT", nil)
	}
	if row.Currency == "" {
		return ErrBadRequest(ErrInvalidRequest, "currency is required", nil)
	}
	if row.PrimaryAccountID == 0 {
		return ErrBadRequest(ErrInvalidRequest, "primary_account_id is required", nil)
	}
	if len(row.Entries) == 0 {
		return ErrBadRequest(ErrInvalidRequest, "entries must not be empty", nil)
	}
	if !isValidTransition(row.Status, row.PriorStatus) {
		return ErrBadRequest(ErrInvalidStatusTransition,
			"invalid (status, prior_status) combination", map[string]any{
				"status":       string(row.Status),
				"prior_status": priorStatusStr(row.PriorStatus),
			})
	}
	primaryHits := 0
	var debitTotal, creditTotal int64
	for _, e := range row.Entries {
		if e.Amount <= 0 {
			return ErrBadRequest(ErrInvalidAmount, "entry amount must be > 0", map[string]any{"account_id": e.AccountID})
		}
		if e.Type != TypeDebit && e.Type != TypeCredit {
			return ErrBadRequest(ErrInvalidRequest, "entry type must be DEBIT or CREDIT", nil)
		}
		if e.AccountID == row.PrimaryAccountID {
			primaryHits++
			if e.Type != row.Type {
				return ErrBadRequest(ErrPrimaryTypeMismatch,
					"primary entry type must match row.type", map[string]any{"row_type": string(row.Type), "entry_type": string(e.Type)})
			}
		}
		if e.Type == TypeDebit {
			debitTotal += e.Amount
		} else {
			creditTotal += e.Amount
		}
	}
	if primaryHits != 1 {
		return ErrBadRequest(ErrPrimaryEntryMissing,
			"exactly one entry must be on primary_account_id", map[string]any{"hits": primaryHits})
	}
	if debitTotal != creditTotal {
		return ErrBadRequest(ErrUnbalancedRow,
			"debit total must equal credit total", map[string]any{"debit": debitTotal, "credit": creditTotal})
	}
	return nil
}

func priorStatusStr(p *TxStatus) string {
	if p == nil {
		return ""
	}
	return string(*p)
}

func isValidTransition(status TxStatus, prior *TxStatus) bool {
	switch status {
	case StatusLien:
		return prior == nil
	case StatusExecuted:
		return prior == nil || *prior == StatusLien
	case StatusReversed:
		return prior != nil && (*prior == StatusLien || *prior == StatusExecuted)
	}
	return false
}

func lockAccounts(tx *gorm.DB, ids []int64) (map[int64]Account, *LedgerError) {
	if len(ids) == 0 {
		return map[int64]Account{}, nil
	}
	var rows []AccountRow
	err := tx.Raw(
		`SELECT id, wallet_id, currency, class, status, balance, available_balance, tags, metadata, created_at
		 FROM "WalletAccounts"
		 WHERE id IN ?
		 ORDER BY id ASC
		 FOR UPDATE`, ids,
	).Scan(&rows).Error
	if err != nil {
		return nil, ErrInternalf("lock accounts failed: " + err.Error())
	}
	if len(rows) != len(ids) {
		return nil, ErrNotFoundf(ErrAccountNotFound, "one or more accounts not found", map[string]any{
			"requested": len(ids), "found": len(rows),
		})
	}
	out := make(map[int64]Account, len(rows))
	for _, r := range rows {
		out[r.ID] = r.ToAccount()
	}
	return out, nil
}

func checkGroupPrecondition(tx *gorm.DB, row RowInput) *LedgerError {
	type latestRow struct {
		ID     int64  `gorm:"column:id"`
		Status string `gorm:"column:status"`
	}
	var latest latestRow
	err := tx.Raw(
		`SELECT id, status FROM "WalletTransactions"
		 WHERE group_id = ?
		 ORDER BY created_at DESC, id DESC
		 LIMIT 1
		 FOR UPDATE`, row.GroupID,
	).Scan(&latest).Error
	if err != nil && !stderrors.Is(err, gorm.ErrRecordNotFound) {
		return ErrInternalf("group lookup failed: " + err.Error())
	}

	noRows := latest.ID == 0
	if row.PriorStatus == nil {
		if !noRows {
			return ErrConflictf(ErrGroupPreconditionFailed,
				"group already has rows; expected new group", map[string]any{"group_id": row.GroupID})
		}
		return nil
	}

	if noRows {
		return ErrNotFoundf(ErrGroupNotFound, "group does not exist", map[string]any{"group_id": row.GroupID})
	}
	if TxStatus(latest.Status) == StatusReversed {
		return ErrConflictf(ErrGroupAlreadyReversed,
			"group is REVERSED and cannot accept new rows", map[string]any{"group_id": row.GroupID})
	}
	if latest.Status != string(*row.PriorStatus) {
		return ErrConflictf(ErrGroupPreconditionFailed,
			"latest row status does not match expected prior_status", map[string]any{
				"group_id": row.GroupID,
				"expected": string(*row.PriorStatus),
				"actual":   latest.Status,
			})
	}
	return nil
}

func insertTransactionRow(tx *gorm.DB, row RowInput) (int64, any, *LedgerError) {
	type result struct {
		ID        int64 `gorm:"column:id"`
		CreatedAt any   `gorm:"column:created_at"`
	}
	var r result
	err := tx.Raw(
		`INSERT INTO "WalletTransactions"
			(group_id, status, prior_status, type, primary_account_id, currency, tags, idempotency_key, fx_pair_group_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?::jsonb, ?, ?)
		 RETURNING id, created_at`,
		row.GroupID, string(row.Status), nullableStatus(row.PriorStatus), string(row.Type),
		row.PrimaryAccountID, row.Currency, JsonOrEmpty(row.Tags),
		stringPtrToAny(row.IdempotencyKey), stringPtrToAny(row.FxPairGroupID),
	).Scan(&r).Error
	if err != nil {
		return 0, nil, ErrInternalf("insert transaction row failed: " + err.Error())
	}
	if r.ID == 0 {
		return 0, nil, ErrInternalf("insert transaction row returned no id")
	}
	return r.ID, r.CreatedAt, nil
}

func insertEntries(tx *gorm.DB, txID int64, inputs []EntryInput) ([]Entry, *LedgerError) {
	if len(inputs) == 0 {
		return nil, nil
	}
	out := make([]Entry, 0, len(inputs))
	for _, e := range inputs {
		var r entryRow
		err := tx.Raw(
			`INSERT INTO "WalletEntries" (transaction_id, account_id, type, amount)
			 VALUES (?, ?, ?, ?)
			 RETURNING id, transaction_id, account_id, type, amount, created_at`,
			txID, e.AccountID, string(e.Type), e.Amount,
		).Scan(&r).Error
		if err != nil {
			return nil, ErrInternalf("insert entry failed: " + err.Error())
		}
		out = append(out, r.toEntry())
	}
	return out, nil
}

func updateAccountCache(tx *gorm.DB, accountID int64, balanceDiff, availableDiff int64) *LedgerError {
	if balanceDiff == 0 && availableDiff == 0 {
		return nil
	}
	res := tx.Exec(
		`UPDATE "WalletAccounts"
		    SET balance = balance + ?, available_balance = available_balance + ?
		  WHERE id = ?`,
		balanceDiff, availableDiff, accountID,
	)
	if res.Error != nil {
		return ErrInternalf("cache update failed: " + res.Error.Error())
	}
	return nil
}

func getAccount(db *gorm.DB, id int64) (*Account, *LedgerError) {
	var r AccountRow
	err := db.Table(`"WalletAccounts"`).Where(`"id" = ?`, id).Scan(&r).Error
	if err != nil {
		return nil, ErrInternalf("account lookup failed: " + err.Error())
	}
	if r.ID == 0 {
		return nil, ErrNotFoundf(ErrAccountNotFound, "account not found", map[string]any{"id": id})
	}
	a := r.ToAccount()
	return &a, nil
}

func getTransactionRow(db *gorm.DB, id int64) (*TransactionRow, *LedgerError) {
	var r txRow
	err := db.Table(`"WalletTransactions"`).Where(`"id" = ?`, id).Scan(&r).Error
	if err != nil {
		return nil, ErrInternalf("transaction lookup failed: " + err.Error())
	}
	if r.ID == 0 {
		return nil, ErrNotFoundf(ErrGroupNotFound, "transaction not found", map[string]any{"id": id})
	}
	t := r.toRow()
	return &t, nil
}

func getEntriesForTx(db *gorm.DB, txID int64) ([]Entry, *LedgerError) {
	var rows []entryRow
	err := db.Table(`"WalletEntries"`).Where(`"transaction_id" = ?`, txID).
		Order(`"id" ASC`).Scan(&rows).Error
	if err != nil {
		return nil, ErrInternalf("entries lookup failed: " + err.Error())
	}
	out := make([]Entry, len(rows))
	for i, r := range rows {
		out[i] = r.toEntry()
	}
	return out, nil
}

func nullableStatus(s *TxStatus) any {
	if s == nil {
		return nil
	}
	return string(*s)
}

func stringPtrToAny(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}

func MergeDetails(a, b map[string]any) map[string]any {
	if a == nil {
		a = map[string]any{}
	}
	for k, v := range b {
		a[k] = v
	}
	return a
}
