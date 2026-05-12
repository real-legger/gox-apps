package core

import (
	"encoding/json"
	"sort"
	"strconv"
	"time"

	"github.com/awesome-goose/goose/modules/sql"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Layer1 implements the high-level wallet operations described in PLAN.md §17.2.
// Every mutating call is idempotent (when an idempotency_key is provided),
// fully transactional, and writes only via Layer0.AppendTransactionRows.
type Layer1 struct {
	db     *sql.Db `inject:""`
	layer0 *Layer0 `inject:""`
}

// ── DTOs (Layer 1) ───────────────────────────────────────────────────────────

type LienOpts struct {
	WalletID       string  `json:"wallet_id"`
	FromAccountID  int64   `json:"from_account_id"`
	ToAccountID    int64   `json:"to_account_id"`
	Amount         int64   `json:"amount"`
	Currency       string  `json:"currency"`
	Tags           Tags    `json:"tags,omitempty"`
	IdempotencyKey *string `json:"idempotency_key,omitempty"`
}

type ExecuteOpts struct {
	WalletID       string  `json:"wallet_id"`
	GroupID        string  `json:"group_id"`
	IdempotencyKey *string `json:"idempotency_key,omitempty"`
}

type ReverseOpts ExecuteOpts

type ExecuteDirectOpts = LienOpts

type ConvertOpts struct {
	WalletID       string  `json:"wallet_id"`
	FromAccountID  int64   `json:"from_account_id"`
	ToAccountID    int64   `json:"to_account_id"`
	Amount         int64   `json:"amount"`
	Tags           Tags    `json:"tags,omitempty"`
	IdempotencyKey *string `json:"idempotency_key,omitempty"`
}

type LienResult struct {
	GroupID       string         `json:"group_id"`
	TransactionID int64          `json:"transaction_id"`
	Status        TxStatus       `json:"status"`
	Entries       []Entry        `json:"entries"`
	AccountsAfter []Account      `json:"accounts_after"`
	Row           TransactionRow `json:"row"`
}

type ExecuteResult = LienResult
type ReverseResult = LienResult
type ExecuteDirectResult = LienResult

type ConvertResult struct {
	SourceGroupID string         `json:"source_group_id"`
	TargetGroupID string         `json:"target_group_id"`
	FxPairGroupID string         `json:"fx_pair_group_id"`
	SourceAmount  int64          `json:"source_amount"`
	TargetAmount  int64          `json:"target_amount"`
	Rate          string         `json:"rate"`
	SourceRow     TransactionRow `json:"source_row"`
	TargetRow     TransactionRow `json:"target_row"`
	AccountsAfter []Account      `json:"accounts_after"`
}

// ── Lien ──────────────────────────────────────────────────────────────────────

func (l *Layer1) Lien(opts LienOpts) (json.RawMessage, *LedgerError) {
	return l.runIdempotent(opts.WalletID, opts.IdempotencyKey, opts, func(tx *gorm.DB) (any, *LedgerError) {
		return l.lienInTx(tx, opts, false)
	})
}

func (l *Layer1) ExecuteDirect(opts ExecuteDirectOpts) (json.RawMessage, *LedgerError) {
	return l.runIdempotent(opts.WalletID, opts.IdempotencyKey, opts, func(tx *gorm.DB) (any, *LedgerError) {
		return l.lienInTx(tx, opts, true)
	})
}

// lienInTx materializes the row + entries (including fees & core postings)
// and appends one row. When skipLien=true status=EXECUTED, prior=NULL; when
// false status=LIEN, prior=NULL.
func (l *Layer1) lienInTx(tx *gorm.DB, opts LienOpts, skipLien bool) (LienResult, *LedgerError) {
	if opts.Amount <= 0 {
		return LienResult{}, ErrBadRequest(ErrInvalidAmount, "amount must be > 0", nil)
	}
	if opts.FromAccountID == opts.ToAccountID {
		return LienResult{}, ErrBadRequest(ErrInvalidRequest, "from and to accounts must differ", nil)
	}

	// Resolve fees and core postings against tags ∪ from-account tags ∪ to-account tags.
	fromAcct, lerr := getAccount(tx, opts.FromAccountID)
	if lerr != nil {
		return LienResult{}, lerr
	}
	toAcct, lerr := getAccount(tx, opts.ToAccountID)
	if lerr != nil {
		return LienResult{}, lerr
	}
	if fromAcct.WalletID != opts.WalletID {
		return LienResult{}, ErrBadRequest(ErrInvalidRequest, "from_account does not belong to wallet", nil)
	}
	if fromAcct.Currency != opts.Currency || toAcct.Currency != opts.Currency {
		return LienResult{}, ErrBadRequest(ErrCurrencyMismatch, "account currency does not match request currency", nil)
	}

	tagSet := opts.Tags.Merge(fromAcct.Tags).Merge(toAcct.Tags)

	fees, lerr := ResolveAccumulatingFees(tx, "fee", tagSet, opts.Amount, false)
	if lerr != nil {
		return LienResult{}, lerr
	}
	taxes, lerr := ResolveAccumulatingFees(tx, "tax", tagSet, opts.Amount, false)
	if lerr != nil {
		return LienResult{}, lerr
	}
	postings, lerr := ResolveLedgerPostings(tx, tagSet)
	if lerr != nil {
		return LienResult{}, lerr
	}

	totalFees := int64(0)
	for _, f := range fees {
		totalFees += f.Amount
	}
	for _, t := range taxes {
		totalFees += t.Amount
	}

	// Compose row.
	row := RowInput{
		GroupID:          uuid.New().String(),
		Type:             TypeDebit,
		PrimaryAccountID: opts.FromAccountID,
		Currency:         opts.Currency,
		Tags:             opts.Tags,
		IdempotencyKey:   opts.IdempotencyKey,
	}
	if skipLien {
		row.Status = StatusExecuted
	} else {
		row.Status = StatusLien
	}

	totalDebit := opts.Amount + totalFees
	row.Entries = []EntryInput{
		{AccountID: opts.FromAccountID, Type: TypeDebit, Amount: totalDebit},
		{AccountID: opts.ToAccountID, Type: TypeCredit, Amount: opts.Amount},
	}
	for _, f := range fees {
		row.Entries = append(row.Entries, EntryInput{AccountID: f.AccountID, Type: TypeCredit, Amount: f.Amount})
	}
	for _, t := range taxes {
		row.Entries = append(row.Entries, EntryInput{AccountID: t.AccountID, Type: TypeCredit, Amount: t.Amount})
	}
	for _, p := range postings {
		row.Entries = append(row.Entries, EntryInput{AccountID: p.AccountID, Type: p.Type, Amount: opts.Amount})
	}

	results, aerr := l.layer0.AppendTransactionRows(tx, AppendOpts{Rows: []RowInput{row}})
	if aerr != nil {
		return LienResult{}, aerr
	}
	r := results[0]

	accountsAfter, aerr := loadAccountsAfter(tx, collectAccountIDs(r.Row))
	if aerr != nil {
		return LienResult{}, aerr
	}

	return LienResult{
		GroupID:       r.Row.GroupID,
		TransactionID: r.Row.ID,
		Status:        r.Row.Status,
		Entries:       r.Entries,
		AccountsAfter: accountsAfter,
		Row:           r.Row,
	}, nil
}

// ── Execute (lien → executed) ─────────────────────────────────────────────────

func (l *Layer1) Execute(opts ExecuteOpts) (json.RawMessage, *LedgerError) {
	return l.runIdempotent(opts.WalletID, opts.IdempotencyKey, opts, func(tx *gorm.DB) (any, *LedgerError) {
		return l.transitionGroup(tx, opts.GroupID, StatusLien, StatusExecuted, opts.IdempotencyKey)
	})
}

// ── Reverse ───────────────────────────────────────────────────────────────────

func (l *Layer1) Reverse(opts ReverseOpts) (json.RawMessage, *LedgerError) {
	return l.runIdempotent(opts.WalletID, opts.IdempotencyKey, opts, func(tx *gorm.DB) (any, *LedgerError) {
		return l.reverseInTx(tx, opts.GroupID, opts.IdempotencyKey)
	})
}

func (l *Layer1) reverseInTx(tx *gorm.DB, groupID string, idemKey *string) (LienResult, *LedgerError) {
	latestRow, latestEntries, lerr := loadLatestRowWithEntries(tx, groupID)
	if lerr != nil {
		return LienResult{}, lerr
	}
	if latestRow.Status == StatusReversed {
		return LienResult{}, ErrConflictf(ErrGroupAlreadyReversed, "group is already REVERSED", map[string]any{"group_id": groupID})
	}

	prior := latestRow.Status
	row := RowInput{
		GroupID:          groupID,
		Status:           StatusReversed,
		PriorStatus:      &prior,
		Type:             latestRow.Type,
		PrimaryAccountID: latestRow.PrimaryAccountID,
		Currency:         latestRow.Currency,
		Tags:             latestRow.Tags,
		IdempotencyKey:   idemKey,
	}
	row.Entries = make([]EntryInput, 0, len(latestEntries))
	for _, e := range latestEntries {
		row.Entries = append(row.Entries, EntryInput{
			AccountID: e.AccountID,
			Type:      e.Type.Flip(),
			Amount:    e.Amount,
		})
	}

	results, aerr := l.layer0.AppendTransactionRows(tx, AppendOpts{Rows: []RowInput{row}})
	if aerr != nil {
		return LienResult{}, aerr
	}
	r := results[0]

	accountsAfter, aerr := loadAccountsAfter(tx, collectAccountIDs(r.Row))
	if aerr != nil {
		return LienResult{}, aerr
	}
	return LienResult{
		GroupID:       r.Row.GroupID,
		TransactionID: r.Row.ID,
		Status:        r.Row.Status,
		Entries:       r.Entries,
		AccountsAfter: accountsAfter,
		Row:           r.Row,
	}, nil
}

// ── transitionGroup: shared LIEN → EXECUTED helper ───────────────────────────

func (l *Layer1) transitionGroup(tx *gorm.DB, groupID string, expected, target TxStatus, idemKey *string) (LienResult, *LedgerError) {
	latestRow, latestEntries, lerr := loadLatestRowWithEntries(tx, groupID)
	if lerr != nil {
		return LienResult{}, lerr
	}
	if latestRow.Status != expected {
		return LienResult{}, ErrConflictf(ErrInvalidStatusTransition,
			"group is not in expected status", map[string]any{
				"group_id": groupID, "expected": string(expected), "actual": string(latestRow.Status),
			})
	}

	prior := latestRow.Status
	row := RowInput{
		GroupID:          groupID,
		Status:           target,
		PriorStatus:      &prior,
		Type:             latestRow.Type,
		PrimaryAccountID: latestRow.PrimaryAccountID,
		Currency:         latestRow.Currency,
		Tags:             latestRow.Tags,
		IdempotencyKey:   idemKey,
	}
	row.Entries = make([]EntryInput, 0, len(latestEntries))
	for _, e := range latestEntries {
		row.Entries = append(row.Entries, EntryInput{
			AccountID: e.AccountID,
			Type:      e.Type,
			Amount:    e.Amount,
		})
	}

	results, aerr := l.layer0.AppendTransactionRows(tx, AppendOpts{Rows: []RowInput{row}})
	if aerr != nil {
		return LienResult{}, aerr
	}
	r := results[0]

	accountsAfter, aerr := loadAccountsAfter(tx, collectAccountIDs(r.Row))
	if aerr != nil {
		return LienResult{}, aerr
	}
	return LienResult{
		GroupID:       r.Row.GroupID,
		TransactionID: r.Row.ID,
		Status:        r.Row.Status,
		Entries:       r.Entries,
		AccountsAfter: accountsAfter,
		Row:           r.Row,
	}, nil
}

// ── Convert (FX) ─────────────────────────────────────────────────────────────

func (l *Layer1) Convert(opts ConvertOpts) (json.RawMessage, *LedgerError) {
	return l.runIdempotent(opts.WalletID, opts.IdempotencyKey, opts, func(tx *gorm.DB) (any, *LedgerError) {
		return l.convertInTx(tx, opts)
	})
}

func (l *Layer1) convertInTx(tx *gorm.DB, opts ConvertOpts) (ConvertResult, *LedgerError) {
	if opts.Amount <= 0 {
		return ConvertResult{}, ErrBadRequest(ErrInvalidAmount, "amount must be > 0", nil)
	}
	from, lerr := getAccount(tx, opts.FromAccountID)
	if lerr != nil {
		return ConvertResult{}, lerr
	}
	to, lerr := getAccount(tx, opts.ToAccountID)
	if lerr != nil {
		return ConvertResult{}, lerr
	}
	if from.Currency == to.Currency {
		return ConvertResult{}, ErrBadRequest(ErrSameCurrencyConvert, "convert requires distinct currencies", nil)
	}
	if from.WalletID != opts.WalletID {
		return ConvertResult{}, ErrBadRequest(ErrInvalidRequest, "from_account does not belong to wallet", nil)
	}

	rate, rateID, lerr := lookupRate(tx, from.Currency, to.Currency)
	if lerr != nil {
		return ConvertResult{}, lerr
	}
	targetAmount := applyRate(opts.Amount, rate)
	if targetAmount <= 0 {
		return ConvertResult{}, ErrBadRequest(ErrInvalidAmount, "target amount rounded to zero", map[string]any{
			"source_amount": opts.Amount, "rate": rate,
		})
	}

	// Resolve FX intermediate accounts via lookup config (category="fx_target").
	fxTags := opts.Tags.Merge(Tags{"from_currency": from.Currency, "to_currency": to.Currency})
	rawPayload, lerr := ResolveLookup(tx, "fx_target", fxTags)
	if lerr != nil {
		return ConvertResult{}, lerr
	}
	var fx FxTargetPayload
	if jerr := json.Unmarshal(rawPayload, &fx); jerr != nil {
		return ConvertResult{}, ErrInternalf("invalid fx_target payload")
	}

	srcGroupID := uuid.New().String()
	tgtGroupID := uuid.New().String()
	pairID := srcGroupID

	srcRow := RowInput{
		GroupID:          srcGroupID,
		Status:           StatusExecuted,
		Type:             TypeDebit,
		PrimaryAccountID: opts.FromAccountID,
		Currency:         from.Currency,
		Tags:             cloneWith(opts.Tags, Tags{"product": "convert", "rate": rate, "rate_id": strconv.FormatInt(rateID, 10)}),
		IdempotencyKey:   opts.IdempotencyKey,
		FxPairGroupID:    &pairID,
		Entries: []EntryInput{
			{AccountID: opts.FromAccountID, Type: TypeDebit, Amount: opts.Amount},
			{AccountID: fx.FromAccountID, Type: TypeCredit, Amount: opts.Amount},
		},
	}
	tgtRow := RowInput{
		GroupID:          tgtGroupID,
		Status:           StatusExecuted,
		Type:             TypeCredit,
		PrimaryAccountID: opts.ToAccountID,
		Currency:         to.Currency,
		Tags:             cloneWith(opts.Tags, Tags{"product": "convert", "rate": rate, "rate_id": strconv.FormatInt(rateID, 10)}),
		IdempotencyKey:   opts.IdempotencyKey,
		FxPairGroupID:    &pairID,
		Entries: []EntryInput{
			{AccountID: fx.ToAccountID, Type: TypeDebit, Amount: targetAmount},
			{AccountID: opts.ToAccountID, Type: TypeCredit, Amount: targetAmount},
		},
	}

	results, aerr := l.layer0.AppendTransactionRows(tx, AppendOpts{Rows: []RowInput{srcRow, tgtRow}})
	if aerr != nil {
		// Differentiate FX intermediate insufficiency from generic insufficient funds.
		if aerr.Code == ErrInsufficientFunds {
			if id, ok := aerr.Details["account_id"]; ok {
				if idInt, _ := id.(int64); idInt == fx.ToAccountID {
					return ConvertResult{}, ErrConflictf(ErrFxIntermediateInsufficient,
						"FX intermediate has no liquidity for the pair", aerr.Details)
				}
			}
		}
		return ConvertResult{}, aerr
	}

	allIDs := append(collectAccountIDs(results[0].Row), collectAccountIDs(results[1].Row)...)
	accountsAfter, aerr := loadAccountsAfter(tx, dedupInt64(allIDs))
	if aerr != nil {
		return ConvertResult{}, aerr
	}

	return ConvertResult{
		SourceGroupID: srcGroupID,
		TargetGroupID: tgtGroupID,
		FxPairGroupID: pairID,
		SourceAmount:  opts.Amount,
		TargetAmount:  targetAmount,
		Rate:          rate,
		SourceRow:     results[0].Row,
		TargetRow:     results[1].Row,
		AccountsAfter: accountsAfter,
	}, nil
}

// ── idempotent runner ────────────────────────────────────────────────────────

// runIdempotent serializes the operation:
//
//  1. Compute request hash.
//  2. Open a DB transaction.
//  3. SELECT … FOR UPDATE on the idempotency row (if a key is provided).
//     - same key + same hash → return cached response (replay).
//     - same key + different hash → IDEMPOTENCY_CONFLICT.
//  4. Run the wrapped function (it owns the writes).
//  5. INSERT idempotency row.
//  6. Commit.
func (l *Layer1) runIdempotent(walletID string, key *string, request any, fn func(tx *gorm.DB) (any, *LedgerError)) (json.RawMessage, *LedgerError) {
	var hash []byte
	if key != nil {
		h, err := CanonicalHash(request)
		if err != nil {
			return nil, ErrInternalf("hash request failed: " + err.Error())
		}
		hash = h
	}

	var out json.RawMessage
	var lerr *LedgerError
	txErr := l.db.DB.Transaction(func(tx *gorm.DB) error {
		if key != nil {
			cached, cerr := lookupIdempotency(tx, *key, walletID, hash)
			if cerr != nil {
				lerr = cerr
				return cerr
			}
			if cached != nil {
				out = cached
				return nil
			}
		}

		result, fnErr := fn(tx)
		if fnErr != nil {
			lerr = fnErr
			return fnErr
		}

		raw, jerr := json.Marshal(result)
		if jerr != nil {
			lerr = ErrInternalf("marshal response failed: " + jerr.Error())
			return lerr
		}
		out = raw

		if key != nil {
			if cerr := saveIdempotency(tx, *key, walletID, hash, raw); cerr != nil {
				lerr = cerr
				return cerr
			}
		}
		return nil
	})
	if txErr != nil {
		if lerr != nil {
			return nil, lerr
		}
		return nil, ErrInternalf("transaction failed: " + txErr.Error())
	}
	return out, nil
}

// lookupIdempotency returns the cached response when (key, hash, wallet) match,
// or *LedgerError for any conflict, or (nil, nil) when the key is unused.
func lookupIdempotency(tx *gorm.DB, key, walletID string, hash []byte) (json.RawMessage, *LedgerError) {
	type idemRow struct {
		Key         string `gorm:"column:key"`
		WalletID    string `gorm:"column:wallet_id"`
		RequestHash []byte `gorm:"column:request_hash"`
		Response    []byte `gorm:"column:response"`
		ExpiresAt   any    `gorm:"column:expires_at"`
		CreatedAt   any    `gorm:"column:created_at"`
	}
	var r idemRow
	err := tx.Raw(
		`SELECT key, wallet_id, request_hash, response, expires_at, created_at
		   FROM "WalletIdempotency"
		  WHERE key = ?
		  FOR UPDATE`, key,
	).Scan(&r).Error
	if err != nil {
		return nil, ErrInternalf("idempotency lookup failed: " + err.Error())
	}
	if r.Key == "" {
		return nil, nil
	}
	if r.WalletID != walletID {
		return nil, ErrConflictf(ErrIdempotencyConflict,
			"idempotency key already used by a different wallet", map[string]any{"key": key})
	}
	if !equalBytes(r.RequestHash, hash) {
		return nil, ErrConflictf(ErrIdempotencyConflict,
			"idempotency key already used with a different request body", map[string]any{"key": key})
	}
	return json.RawMessage(r.Response), nil
}

func saveIdempotency(tx *gorm.DB, key, walletID string, hash, response []byte) *LedgerError {
	expiresAt := time.Now().UTC().Add(24 * time.Hour)
	res := tx.Exec(
		`INSERT INTO "WalletIdempotency" (key, wallet_id, request_hash, response, expires_at)
		 VALUES (?, ?, ?, ?::jsonb, ?)
		 ON CONFLICT (key) DO NOTHING`,
		key, walletID, hash, string(response), expiresAt,
	)
	if res.Error != nil {
		return ErrInternalf("idempotency insert failed: " + res.Error.Error())
	}
	return nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

func loadLatestRowWithEntries(tx *gorm.DB, groupID string) (TransactionRow, []Entry, *LedgerError) {
	var r txRow
	err := tx.Raw(
		`SELECT id, group_id, status, prior_status, type, primary_account_id, currency, tags,
		        idempotency_key, fx_pair_group_id, created_at
		   FROM "WalletTransactions"
		  WHERE group_id = ?
		  ORDER BY created_at DESC, id DESC
		  LIMIT 1
		  FOR UPDATE`, groupID,
	).Scan(&r).Error
	if err != nil {
		return TransactionRow{}, nil, ErrInternalf("latest row lookup failed: " + err.Error())
	}
	if r.ID == 0 {
		return TransactionRow{}, nil, ErrNotFoundf(ErrGroupNotFound, "group not found", map[string]any{"group_id": groupID})
	}
	row := r.toRow()
	entries, lerr := getEntriesForTx(tx, row.ID)
	if lerr != nil {
		return TransactionRow{}, nil, lerr
	}
	return row, entries, nil
}

func loadAccountsAfter(tx *gorm.DB, ids []int64) ([]Account, *LedgerError) {
	if len(ids) == 0 {
		return nil, nil
	}
	var rows []AccountRow
	err := tx.Table(`"WalletAccounts"`).
		Where(`"id" IN ?`, ids).
		Order(`"id" ASC`).
		Scan(&rows).Error
	if err != nil {
		return nil, ErrInternalf("load accounts after failed: " + err.Error())
	}
	out := make([]Account, len(rows))
	for i, r := range rows {
		out[i] = r.ToAccount()
	}
	return out, nil
}

func collectAccountIDs(row TransactionRow) []int64 {
	set := map[int64]struct{}{row.PrimaryAccountID: {}}
	for _, e := range row.Entries {
		set[e.AccountID] = struct{}{}
	}
	out := make([]int64, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func dedupInt64(in []int64) []int64 {
	set := map[int64]struct{}{}
	for _, x := range in {
		set[x] = struct{}{}
	}
	out := make([]int64, 0, len(set))
	for x := range set {
		out = append(out, x)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func cloneWith(base Tags, extra Tags) Tags {
	out := make(Tags, len(base)+len(extra))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ── rates ────────────────────────────────────────────────────────────────────

func lookupRate(tx *gorm.DB, from, to string) (string, int64, *LedgerError) {
	type rateRow struct {
		ID   int64  `gorm:"column:id"`
		Rate string `gorm:"column:rate"`
	}
	var r rateRow
	err := tx.Raw(
		`SELECT id, rate::text AS rate
		   FROM "WalletRates"
		  WHERE from_currency = ? AND to_currency = ?
		    AND valid_from <= NOW() AND (valid_to IS NULL OR valid_to > NOW())
		  ORDER BY valid_from DESC
		  LIMIT 1`, from, to,
	).Scan(&r).Error
	if err != nil {
		return "", 0, ErrInternalf("rate lookup failed: " + err.Error())
	}
	if r.ID == 0 {
		return "", 0, ErrNotFoundf(ErrRateNotFound, "no active rate for the pair", map[string]any{
			"from": from, "to": to,
		})
	}
	return r.Rate, r.ID, nil
}

// applyRate computes floor(amount * rate) using a fixed-point multiply. The
// rate string is parsed to a 12-decimal fixed-point integer to match the
// NUMERIC(30,12) precision used in the schema.
func applyRate(amount int64, rate string) int64 {
	const scale int64 = 1_000_000_000_000 // 1e12
	whole, frac := splitDot(rate)
	w, _ := parseInt64(whole)
	f, fracLen := parseInt64(frac)
	for fracLen < 12 {
		f *= 10
		fracLen++
	}
	for fracLen > 12 {
		f /= 10
		fracLen--
	}
	rateMicro := w*scale + f
	// floor(amount * rateMicro / scale)
	prod := amount * rateMicro
	if prod < 0 {
		// Overflow; fall back to simple division. The caller should have validated
		// magnitudes upstream.
		return 0
	}
	return prod / scale
}
