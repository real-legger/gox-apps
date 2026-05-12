package tests

import (
	"testing"

	"github.com/real-legger/gox-apps/libs/wallet/pkg/core"
)

// TestBalanceDeltaTable walks every row in PLAN.md §11 and verifies the
// (balance, available) deltas the core code produces match the table.
func TestBalanceDeltaTable(t *testing.T) {
	lien := core.StatusLien
	exec := core.StatusExecuted
	rev := core.StatusReversed

	cases := []struct {
		name      string
		rowType   core.EntryType
		role      string
		status    core.TxStatus
		prior     *core.TxStatus
		wantBal   int
		wantAvail int
	}{
		{"DEBIT LIEN primary", core.TypeDebit, "primary", lien, nil, 0, -1},
		{"DEBIT LIEN secondary", core.TypeDebit, "secondary", lien, nil, +1, 0},
		{"DEBIT EXECUTED-from-LIEN primary", core.TypeDebit, "primary", exec, &lien, -1, 0},
		{"DEBIT EXECUTED-from-LIEN secondary", core.TypeDebit, "secondary", exec, &lien, 0, +1},
		{"DEBIT EXECUTED skip-lien primary", core.TypeDebit, "primary", exec, nil, -1, -1},
		{"DEBIT EXECUTED skip-lien secondary", core.TypeDebit, "secondary", exec, nil, +1, +1},
		{"DEBIT REVERSED-from-LIEN primary", core.TypeDebit, "primary", rev, &lien, 0, +1},
		{"DEBIT REVERSED-from-LIEN secondary", core.TypeDebit, "secondary", rev, &lien, -1, 0},
		{"DEBIT REVERSED-from-EXECUTED primary", core.TypeDebit, "primary", rev, &exec, +1, +1},
		{"DEBIT REVERSED-from-EXECUTED secondary", core.TypeDebit, "secondary", rev, &exec, -1, -1},

		{"CREDIT LIEN primary", core.TypeCredit, "primary", lien, nil, +1, 0},
		{"CREDIT LIEN secondary", core.TypeCredit, "secondary", lien, nil, 0, -1},
		{"CREDIT EXECUTED-from-LIEN primary", core.TypeCredit, "primary", exec, &lien, 0, +1},
		{"CREDIT EXECUTED-from-LIEN secondary", core.TypeCredit, "secondary", exec, &lien, -1, 0},
		{"CREDIT EXECUTED skip-lien primary", core.TypeCredit, "primary", exec, nil, +1, +1},
		{"CREDIT EXECUTED skip-lien secondary", core.TypeCredit, "secondary", exec, nil, -1, -1},
		{"CREDIT REVERSED-from-LIEN primary", core.TypeCredit, "primary", rev, &lien, -1, 0},
		{"CREDIT REVERSED-from-LIEN secondary", core.TypeCredit, "secondary", rev, &lien, 0, +1},
		{"CREDIT REVERSED-from-EXECUTED primary", core.TypeCredit, "primary", rev, &exec, -1, -1},
		{"CREDIT REVERSED-from-EXECUTED secondary", core.TypeCredit, "secondary", rev, &exec, +1, +1},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d, ok := core.LookupDelta(c.rowType, c.role, c.status, c.prior)
			if !ok {
				t.Fatalf("LookupDelta missing entry for %s", c.name)
			}
			if d.Balance != c.wantBal || d.Available != c.wantAvail {
				t.Fatalf("got (bal=%d,avail=%d), want (bal=%d,avail=%d)",
					d.Balance, d.Available, c.wantBal, c.wantAvail)
			}
		})
	}
}

// TestComputeRowDeltasLien checks the worked example §18.1 (LIEN row):
// from=42 DEBIT 1000, to=99 CREDIT 1000.
//
//	42 (primary, DEBIT row, LIEN/NULL):       balance ---,  avail -1000
//	99 (secondary, DEBIT row, LIEN/NULL):     balance +1000, avail ---
func TestComputeRowDeltasLien(t *testing.T) {
	row := core.RowInput{
		GroupID:          "g",
		Status:           core.StatusLien,
		Type:             core.TypeDebit,
		PrimaryAccountID: 42,
		Currency:         "NGN",
		Entries: []core.EntryInput{
			{AccountID: 42, Type: core.TypeDebit, Amount: 1000},
			{AccountID: 99, Type: core.TypeCredit, Amount: 1000},
		},
	}
	deltas, err := core.ComputeRowDeltas(row)
	if err != nil {
		t.Fatalf("ComputeRowDeltas: %v", err)
	}
	got := map[int64]core.AccountDelta{}
	for _, d := range deltas {
		got[d.AccountID] = d
	}
	if got[42].BalanceDiff != 0 || got[42].AvailableDiff != -1000 {
		t.Fatalf("primary delta: %+v", got[42])
	}
	if got[99].BalanceDiff != 1000 || got[99].AvailableDiff != 0 {
		t.Fatalf("secondary delta: %+v", got[99])
	}
}

// TestComputeRowDeltasReverseExecuted verifies §18.3 reversal of an executed
// DEBIT with a fee leg: from=42 −1010, to=99 −1000, fee=7 −10.
func TestComputeRowDeltasReverseExecuted(t *testing.T) {
	exec := core.StatusExecuted
	row := core.RowInput{
		GroupID:          "g",
		Status:           core.StatusReversed,
		PriorStatus:      &exec,
		Type:             core.TypeDebit,
		PrimaryAccountID: 42,
		Currency:         "NGN",
		Entries: []core.EntryInput{
			// Reversal flips the entry types (CREDIT-on-from, DEBIT-on-{to,fee}).
			{AccountID: 42, Type: core.TypeCredit, Amount: 1010},
			{AccountID: 99, Type: core.TypeDebit, Amount: 1000},
			{AccountID: 7, Type: core.TypeDebit, Amount: 10},
		},
	}
	deltas, err := core.ComputeRowDeltas(row)
	if err != nil {
		t.Fatalf("ComputeRowDeltas: %v", err)
	}
	got := map[int64]core.AccountDelta{}
	for _, d := range deltas {
		got[d.AccountID] = d
	}
	// Primary 42 (DEBIT row, REVERSED, prior EXECUTED): balance +e, avail +e.
	if got[42].BalanceDiff != 1010 || got[42].AvailableDiff != 1010 {
		t.Fatalf("primary: %+v", got[42])
	}
	// Secondary (DEBIT row, REVERSED, prior EXECUTED): balance -e, avail -e.
	if got[99].BalanceDiff != -1000 || got[99].AvailableDiff != -1000 {
		t.Fatalf("counterparty: %+v", got[99])
	}
	if got[7].BalanceDiff != -10 || got[7].AvailableDiff != -10 {
		t.Fatalf("fee account: %+v", got[7])
	}
}
