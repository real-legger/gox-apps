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

// TestComputeRowDeltasReversedWithoutPrior verifies that a REVERSED row with
// no PriorStatus has no entry in the delta table and is rejected.
func TestComputeRowDeltasReversedWithoutPrior(t *testing.T) {
	row := core.RowInput{
		GroupID:          "g",
		Status:           core.StatusReversed,
		PriorStatus:      nil,
		Type:             core.TypeDebit,
		PrimaryAccountID: 42,
		Currency:         "NGN",
		Entries: []core.EntryInput{
			{AccountID: 42, Type: core.TypeCredit, Amount: 100},
			{AccountID: 99, Type: core.TypeDebit, Amount: 100},
		},
	}
	_, err := core.ComputeRowDeltas(row)
	if err == nil {
		t.Fatalf("expected error for REVERSED with nil PriorStatus")
	}
	if err.Code != core.ErrInvalidStatusTransition {
		t.Fatalf("got code %q, want %q", err.Code, core.ErrInvalidStatusTransition)
	}
}

// TestComputeRowDeltasLienWithPrior verifies that LIEN is only valid as an
// initial row (PriorStatus must be nil). PriorStatus=EXECUTED is rejected.
func TestComputeRowDeltasLienWithPrior(t *testing.T) {
	exec := core.StatusExecuted
	row := core.RowInput{
		GroupID:          "g",
		Status:           core.StatusLien,
		PriorStatus:      &exec,
		Type:             core.TypeDebit,
		PrimaryAccountID: 42,
		Currency:         "NGN",
		Entries: []core.EntryInput{
			{AccountID: 42, Type: core.TypeDebit, Amount: 100},
			{AccountID: 99, Type: core.TypeCredit, Amount: 100},
		},
	}
	_, err := core.ComputeRowDeltas(row)
	if err == nil {
		t.Fatalf("expected error for LIEN with prior=EXECUTED")
	}
	if err.Code != core.ErrInvalidStatusTransition {
		t.Fatalf("got code %q, want %q", err.Code, core.ErrInvalidStatusTransition)
	}
}

// TestComputeRowDeltasReversedFromReversed verifies that REVERSED→REVERSED has
// no delta-table entry (a group is reversed exactly once).
func TestComputeRowDeltasReversedFromReversed(t *testing.T) {
	rev := core.StatusReversed
	row := core.RowInput{
		GroupID:          "g",
		Status:           core.StatusReversed,
		PriorStatus:      &rev,
		Type:             core.TypeDebit,
		PrimaryAccountID: 42,
		Currency:         "NGN",
		Entries: []core.EntryInput{
			{AccountID: 42, Type: core.TypeCredit, Amount: 100},
			{AccountID: 99, Type: core.TypeDebit, Amount: 100},
		},
	}
	_, err := core.ComputeRowDeltas(row)
	if err == nil {
		t.Fatalf("expected error for REVERSED with prior=REVERSED")
	}
	if err.Code != core.ErrInvalidStatusTransition {
		t.Fatalf("got code %q, want %q", err.Code, core.ErrInvalidStatusTransition)
	}
}

// TestComputeRowDeltasMultiEntrySameAccount verifies entries that touch the
// same account multiple times have their deltas summed under one account_id.
func TestComputeRowDeltasMultiEntrySameAccount(t *testing.T) {
	row := core.RowInput{
		GroupID:          "g",
		Status:           core.StatusExecuted,
		Type:             core.TypeDebit,
		PrimaryAccountID: 42,
		Currency:         "NGN",
		Entries: []core.EntryInput{
			// Primary debit twice on account 42, single credit leg on 99.
			{AccountID: 42, Type: core.TypeDebit, Amount: 400},
			{AccountID: 42, Type: core.TypeDebit, Amount: 600},
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
	if len(got) != 2 {
		t.Fatalf("expected 2 distinct accounts, got %d", len(got))
	}
	// EXECUTED skip-lien DEBIT primary = -1 on both balance and available; aggregated over 400+600.
	if got[42].BalanceDiff != -1000 || got[42].AvailableDiff != -1000 {
		t.Fatalf("primary aggregated: %+v", got[42])
	}
	if got[99].BalanceDiff != 1000 || got[99].AvailableDiff != 1000 {
		t.Fatalf("secondary: %+v", got[99])
	}
}

// TestLookupDeltaInvalidInputs walks combinations of (rowType, role, status,
// prior) that are NOT in the §11 table and asserts ok=false.
func TestLookupDeltaInvalidInputs(t *testing.T) {
	lien := core.StatusLien
	exec := core.StatusExecuted
	rev := core.StatusReversed

	cases := []struct {
		name    string
		rowType core.EntryType
		role    string
		status  core.TxStatus
		prior   *core.TxStatus
	}{
		{"LIEN with prior=LIEN", core.TypeDebit, "primary", lien, &lien},
		{"LIEN with prior=EXECUTED", core.TypeCredit, "primary", lien, &exec},
		{"EXECUTED with prior=REVERSED", core.TypeDebit, "primary", exec, &rev},
		{"REVERSED with prior=REVERSED", core.TypeDebit, "primary", rev, &rev},
		{"REVERSED with prior=nil", core.TypeCredit, "secondary", rev, nil},
		{"unknown role", core.TypeDebit, "tertiary", lien, nil},
		{"unknown rowType", core.EntryType("MIXED"), "primary", lien, nil},
		{"unknown status", core.TypeDebit, "primary", core.TxStatus("PENDING"), nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, ok := core.LookupDelta(c.rowType, c.role, c.status, c.prior); ok {
				t.Fatalf("expected ok=false for %s", c.name)
			}
		})
	}
}
