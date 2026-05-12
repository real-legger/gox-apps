package core

// Delta describes how a single entry mutates the cached balance and
// available_balance fields of its account.
type Delta struct {
	Balance   int // -1, 0, +1
	Available int // -1, 0, +1
}

// deltaKey describes the row context used to look up entry deltas.
type deltaKey struct {
	rowType     EntryType
	role        string // "primary" | "secondary"
	status      TxStatus
	priorStatus string // "" if NULL
}

// deltaTable mirrors PLAN.md §11 Balance Computation Model.
//
//	row.type | row.status, row.prior_status        | Primary balance | Primary available | Secondary balance | Secondary available
//	---------+-------------------------------------+-----------------+-------------------+-------------------+--------------------
//	DEBIT    | LIEN, NULL                          | ---             | -e.amount         | +e.amount         | ---
//	DEBIT    | EXECUTED, LIEN                      | -e.amount       | ---               | ---               | +e.amount
//	DEBIT    | EXECUTED, NULL (skip-lien)          | -e.amount       | -e.amount         | +e.amount         | +e.amount
//	DEBIT    | REVERSED, LIEN                      | ---             | +e.amount         | -e.amount         | ---
//	DEBIT    | REVERSED, EXECUTED                  | +e.amount       | +e.amount         | -e.amount         | -e.amount
//	CREDIT   | LIEN, NULL                          | +e.amount       | ---               | ---               | -e.amount
//	CREDIT   | EXECUTED, LIEN                      | ---             | +e.amount         | -e.amount         | ---
//	CREDIT   | EXECUTED, NULL (skip-lien)          | +e.amount       | +e.amount         | -e.amount         | -e.amount
//	CREDIT   | REVERSED, LIEN                      | -e.amount       | ---               | ---               | +e.amount
//	CREDIT   | REVERSED, EXECUTED                  | -e.amount       | -e.amount         | +e.amount         | +e.amount
var deltaTable = map[deltaKey]Delta{
	{TypeDebit, "primary", StatusLien, ""}:               {0, -1},
	{TypeDebit, "secondary", StatusLien, ""}:             {+1, 0},
	{TypeDebit, "primary", StatusExecuted, "LIEN"}:       {-1, 0},
	{TypeDebit, "secondary", StatusExecuted, "LIEN"}:     {0, +1},
	{TypeDebit, "primary", StatusExecuted, ""}:           {-1, -1},
	{TypeDebit, "secondary", StatusExecuted, ""}:         {+1, +1},
	{TypeDebit, "primary", StatusReversed, "LIEN"}:       {0, +1},
	{TypeDebit, "secondary", StatusReversed, "LIEN"}:     {-1, 0},
	{TypeDebit, "primary", StatusReversed, "EXECUTED"}:   {+1, +1},
	{TypeDebit, "secondary", StatusReversed, "EXECUTED"}: {-1, -1},

	{TypeCredit, "primary", StatusLien, ""}:               {+1, 0},
	{TypeCredit, "secondary", StatusLien, ""}:             {0, -1},
	{TypeCredit, "primary", StatusExecuted, "LIEN"}:       {0, +1},
	{TypeCredit, "secondary", StatusExecuted, "LIEN"}:     {-1, 0},
	{TypeCredit, "primary", StatusExecuted, ""}:           {+1, +1},
	{TypeCredit, "secondary", StatusExecuted, ""}:         {-1, -1},
	{TypeCredit, "primary", StatusReversed, "LIEN"}:       {-1, 0},
	{TypeCredit, "secondary", StatusReversed, "LIEN"}:     {0, +1},
	{TypeCredit, "primary", StatusReversed, "EXECUTED"}:   {-1, -1},
	{TypeCredit, "secondary", StatusReversed, "EXECUTED"}: {+1, +1},
}

// LookupDelta returns the cache delta for a single entry given the row's
// type, the entry's role on the row (primary/secondary), and the row's
// status pair. Returns ok=false if the combination is not in the table.
func LookupDelta(rowType EntryType, role string, status TxStatus, prior *TxStatus) (Delta, bool) {
	priorStr := ""
	if prior != nil {
		priorStr = string(*prior)
	}
	d, ok := deltaTable[deltaKey{rowType, role, status, priorStr}]
	return d, ok
}

// AccountDelta is the aggregated cache change to apply to one account.
type AccountDelta struct {
	AccountID     int64
	BalanceDiff   int64
	AvailableDiff int64
}

// ComputeRowDeltas walks the entries on a single row and returns the
// aggregated deltas keyed by account_id. Entries on the same account
// (primary + secondary, multi-leg) are summed.
func ComputeRowDeltas(row RowInput) ([]AccountDelta, *LedgerError) {
	byAccount := map[int64]*AccountDelta{}
	for _, e := range row.Entries {
		role := "secondary"
		if e.AccountID == row.PrimaryAccountID {
			role = "primary"
		}
		d, ok := LookupDelta(row.Type, role, row.Status, row.PriorStatus)
		if !ok {
			return nil, ErrBadRequest(ErrInvalidStatusTransition,
				"no balance delta for (type, status, prior_status)", map[string]any{
					"type":   string(row.Type),
					"status": string(row.Status),
				})
		}
		acct, exists := byAccount[e.AccountID]
		if !exists {
			acct = &AccountDelta{AccountID: e.AccountID}
			byAccount[e.AccountID] = acct
		}
		acct.BalanceDiff += int64(d.Balance) * e.Amount
		acct.AvailableDiff += int64(d.Available) * e.Amount
	}

	out := make([]AccountDelta, 0, len(byAccount))
	for _, d := range byAccount {
		out = append(out, *d)
	}
	return out, nil
}
