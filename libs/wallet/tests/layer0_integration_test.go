package tests

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/real-legger/gox-apps/libs/wallet/pkg/core"
	"gorm.io/gorm"
)

// errRollback is returned from inside Transaction() to force a rollback while
// keeping the in-test assertions intact. Layer0 tests don't need to commit:
// they exercise validation and side effects within a single tx.
var errRollback = errors.New("test rollback")

// runInTx runs fn against a fresh transaction and rolls it back unconditionally.
// fn returns the LedgerError from AppendTransactionRows so the caller can assert.
func runInTx(t *testing.T, db *gorm.DB, fn func(tx *gorm.DB)) {
	t.Helper()
	_ = db.Transaction(func(tx *gorm.DB) error {
		fn(tx)
		return errRollback
	})
}

func TestAppendRows_EmptyBatch(t *testing.T) {
	db := openTestDB(t)
	resetDB(t, db)
	_, l0 := newLayer1(db)

	runInTx(t, db, func(tx *gorm.DB) {
		_, err := l0.AppendTransactionRows(tx, core.AppendOpts{Rows: nil})
		if err == nil || err.Code != core.ErrInvalidRequest {
			t.Fatalf("expected ErrInvalidRequest, got %v", err)
		}
	})
}

func TestAppendRows_AccountNotFound(t *testing.T) {
	db := openTestDB(t)
	resetDB(t, db)
	seedCurrency(t, db, "NGN", 2)
	walletID := seedWallet(t, db)
	from := seedAccount(t, db, walletID, "NGN", "liability", accountOpts{availableBalance: 1000, balance: 1000})
	_, l0 := newLayer1(db)

	runInTx(t, db, func(tx *gorm.DB) {
		_, err := l0.AppendTransactionRows(tx, core.AppendOpts{Rows: []core.RowInput{{
			GroupID:          uuid.NewString(),
			Status:           core.StatusExecuted,
			Type:             core.TypeDebit,
			PrimaryAccountID: from,
			Currency:         "NGN",
			Entries: []core.EntryInput{
				{AccountID: from, Type: core.TypeDebit, Amount: 100},
				{AccountID: 999999, Type: core.TypeCredit, Amount: 100},
			},
		}}})
		if err == nil || err.Code != core.ErrAccountNotFound {
			t.Fatalf("expected ErrAccountNotFound, got %v", err)
		}
	})
}

func TestAppendRows_CurrencyMismatch(t *testing.T) {
	db := openTestDB(t)
	resetDB(t, db)
	seedCurrency(t, db, "NGN", 2)
	seedCurrency(t, db, "USD", 2)
	walletID := seedWallet(t, db)
	from := seedAccount(t, db, walletID, "NGN", "liability", accountOpts{balance: 1000, availableBalance: 1000})
	to := seedAccount(t, db, walletID, "USD", "liability", accountOpts{})
	_, l0 := newLayer1(db)

	runInTx(t, db, func(tx *gorm.DB) {
		_, err := l0.AppendTransactionRows(tx, core.AppendOpts{Rows: []core.RowInput{{
			GroupID:          uuid.NewString(),
			Status:           core.StatusExecuted,
			Type:             core.TypeDebit,
			PrimaryAccountID: from,
			Currency:         "NGN",
			Entries: []core.EntryInput{
				{AccountID: from, Type: core.TypeDebit, Amount: 100},
				{AccountID: to, Type: core.TypeCredit, Amount: 100},
			},
		}}})
		if err == nil || err.Code != core.ErrCurrencyMismatch {
			t.Fatalf("expected ErrCurrencyMismatch, got %v", err)
		}
	})
}

func TestAppendRows_AccountFrozen(t *testing.T) {
	db := openTestDB(t)
	resetDB(t, db)
	seedCurrency(t, db, "NGN", 2)
	walletID := seedWallet(t, db)
	from := seedAccount(t, db, walletID, "NGN", "liability", accountOpts{balance: 1000, availableBalance: 1000, status: "frozen"})
	to := seedAccount(t, db, walletID, "NGN", "liability", accountOpts{})
	_, l0 := newLayer1(db)

	runInTx(t, db, func(tx *gorm.DB) {
		_, err := l0.AppendTransactionRows(tx, core.AppendOpts{Rows: []core.RowInput{{
			GroupID:          uuid.NewString(),
			Status:           core.StatusExecuted,
			Type:             core.TypeDebit,
			PrimaryAccountID: from,
			Currency:         "NGN",
			Entries: []core.EntryInput{
				{AccountID: from, Type: core.TypeDebit, Amount: 100},
				{AccountID: to, Type: core.TypeCredit, Amount: 100},
			},
		}}})
		if err == nil || err.Code != core.ErrAccountFrozen {
			t.Fatalf("expected ErrAccountFrozen, got %v", err)
		}
	})
}

func TestAppendRows_AccountClosed(t *testing.T) {
	db := openTestDB(t)
	resetDB(t, db)
	seedCurrency(t, db, "NGN", 2)
	walletID := seedWallet(t, db)
	from := seedAccount(t, db, walletID, "NGN", "liability", accountOpts{balance: 1000, availableBalance: 1000})
	closed := seedAccount(t, db, walletID, "NGN", "liability", accountOpts{status: "closed"})
	_, l0 := newLayer1(db)

	runInTx(t, db, func(tx *gorm.DB) {
		_, err := l0.AppendTransactionRows(tx, core.AppendOpts{Rows: []core.RowInput{{
			GroupID:          uuid.NewString(),
			Status:           core.StatusExecuted,
			Type:             core.TypeDebit,
			PrimaryAccountID: from,
			Currency:         "NGN",
			Entries: []core.EntryInput{
				{AccountID: from, Type: core.TypeDebit, Amount: 100},
				{AccountID: closed, Type: core.TypeCredit, Amount: 100},
			},
		}}})
		// Closed status takes the same code as frozen: "account is not active".
		if err == nil || err.Code != core.ErrAccountFrozen {
			t.Fatalf("expected ErrAccountFrozen for closed account, got %v", err)
		}
	})
}

func TestAppendRows_PrepaidOverdrawBoundary(t *testing.T) {
	db := openTestDB(t)
	seedCurrency(t, db, "NGN", 2)

	// At the boundary: prepaid with available=100, debit 100 → 0 (allowed).
	t.Run("exact_zero_allowed", func(t *testing.T) {
		resetDB(t, db)
		seedCurrency(t, db, "NGN", 2)
		walletID := seedWallet(t, db)
		prepaid := seedAccount(t, db, walletID, "NGN", "prepaid", accountOpts{balance: 100, availableBalance: 100})
		sink := seedAccount(t, db, walletID, "NGN", "liability", accountOpts{})
		_, l0 := newLayer1(db)

		runInTx(t, db, func(tx *gorm.DB) {
			_, err := l0.AppendTransactionRows(tx, core.AppendOpts{Rows: []core.RowInput{{
				GroupID:          uuid.NewString(),
				Status:           core.StatusExecuted,
				Type:             core.TypeDebit,
				PrimaryAccountID: prepaid,
				Currency:         "NGN",
				Entries: []core.EntryInput{
					{AccountID: prepaid, Type: core.TypeDebit, Amount: 100},
					{AccountID: sink, Type: core.TypeCredit, Amount: 100},
				},
			}}})
			if err != nil {
				t.Fatalf("debit-to-zero should succeed on prepaid, got %v", err)
			}
		})
	})

	// One over: available=100, debit 101 → -1 (rejected).
	t.Run("over_by_one_rejected", func(t *testing.T) {
		resetDB(t, db)
		seedCurrency(t, db, "NGN", 2)
		walletID := seedWallet(t, db)
		prepaid := seedAccount(t, db, walletID, "NGN", "prepaid", accountOpts{balance: 100, availableBalance: 100})
		sink := seedAccount(t, db, walletID, "NGN", "liability", accountOpts{})
		_, l0 := newLayer1(db)

		runInTx(t, db, func(tx *gorm.DB) {
			_, err := l0.AppendTransactionRows(tx, core.AppendOpts{Rows: []core.RowInput{{
				GroupID:          uuid.NewString(),
				Status:           core.StatusExecuted,
				Type:             core.TypeDebit,
				PrimaryAccountID: prepaid,
				Currency:         "NGN",
				Entries: []core.EntryInput{
					{AccountID: prepaid, Type: core.TypeDebit, Amount: 101},
					{AccountID: sink, Type: core.TypeCredit, Amount: 101},
				},
			}}})
			if err == nil || err.Code != core.ErrInsufficientFunds {
				t.Fatalf("expected ErrInsufficientFunds for overdraw, got %v", err)
			}
		})
	})
}

func TestAppendRows_LiabilityCanGoNegative(t *testing.T) {
	db := openTestDB(t)
	resetDB(t, db)
	seedCurrency(t, db, "NGN", 2)
	walletID := seedWallet(t, db)
	// Liability with zero balance; debiting it pushes it negative — allowed.
	liab := seedAccount(t, db, walletID, "NGN", "liability", accountOpts{})
	sink := seedAccount(t, db, walletID, "NGN", "asset", accountOpts{})
	_, l0 := newLayer1(db)

	runInTx(t, db, func(tx *gorm.DB) {
		_, err := l0.AppendTransactionRows(tx, core.AppendOpts{Rows: []core.RowInput{{
			GroupID:          uuid.NewString(),
			Status:           core.StatusExecuted,
			Type:             core.TypeDebit,
			PrimaryAccountID: liab,
			Currency:         "NGN",
			Entries: []core.EntryInput{
				{AccountID: liab, Type: core.TypeDebit, Amount: 500},
				{AccountID: sink, Type: core.TypeCredit, Amount: 500},
			},
		}}})
		if err != nil {
			t.Fatalf("non-prepaid should allow negative balance, got %v", err)
		}
	})
}

func TestAppendRows_GroupNotFound_WithPrior(t *testing.T) {
	db := openTestDB(t)
	resetDB(t, db)
	seedCurrency(t, db, "NGN", 2)
	walletID := seedWallet(t, db)
	from := seedAccount(t, db, walletID, "NGN", "liability", accountOpts{balance: 1000, availableBalance: 1000})
	to := seedAccount(t, db, walletID, "NGN", "liability", accountOpts{})
	_, l0 := newLayer1(db)

	lien := core.StatusLien
	runInTx(t, db, func(tx *gorm.DB) {
		_, err := l0.AppendTransactionRows(tx, core.AppendOpts{Rows: []core.RowInput{{
			GroupID:          uuid.NewString(), // brand-new id that doesn't exist
			Status:           core.StatusExecuted,
			PriorStatus:      &lien,
			Type:             core.TypeDebit,
			PrimaryAccountID: from,
			Currency:         "NGN",
			Entries: []core.EntryInput{
				{AccountID: from, Type: core.TypeDebit, Amount: 100},
				{AccountID: to, Type: core.TypeCredit, Amount: 100},
			},
		}}})
		if err == nil || err.Code != core.ErrGroupNotFound {
			t.Fatalf("expected ErrGroupNotFound, got %v", err)
		}
	})
}

func TestAppendRows_GroupPreconditionLatestMismatch(t *testing.T) {
	db := openTestDB(t)
	resetDB(t, db)
	seedCurrency(t, db, "NGN", 2)
	walletID := seedWallet(t, db)
	from := seedAccount(t, db, walletID, "NGN", "liability", accountOpts{balance: 1000, availableBalance: 1000})
	to := seedAccount(t, db, walletID, "NGN", "liability", accountOpts{})
	_, l0 := newLayer1(db)
	groupID := uuid.NewString()

	// Seed an EXECUTED row first.
	runInTx(t, db, func(tx *gorm.DB) {
		_, err := l0.AppendTransactionRows(tx, core.AppendOpts{Rows: []core.RowInput{{
			GroupID:          groupID,
			Status:           core.StatusExecuted,
			Type:             core.TypeDebit,
			PrimaryAccountID: from,
			Currency:         "NGN",
			Entries: []core.EntryInput{
				{AccountID: from, Type: core.TypeDebit, Amount: 100},
				{AccountID: to, Type: core.TypeCredit, Amount: 100},
			},
		}}})
		if err != nil {
			t.Fatalf("seed: %v", err)
		}
		// commit by NOT returning errRollback; but we need to share state.
		// To share state between tx blocks we exit cleanly here.
	})

	// Because runInTx always rolls back, the seed row was rolled back too.
	// We need a different pattern for this test: do both inserts in the same tx.
	resetDB(t, db)
	seedCurrency(t, db, "NGN", 2)
	walletID = seedWallet(t, db)
	from = seedAccount(t, db, walletID, "NGN", "liability", accountOpts{balance: 1000, availableBalance: 1000})
	to = seedAccount(t, db, walletID, "NGN", "liability", accountOpts{})
	groupID = uuid.NewString()
	lien := core.StatusLien

	runInTx(t, db, func(tx *gorm.DB) {
		// First: write an EXECUTED row (no prior).
		_, err := l0.AppendTransactionRows(tx, core.AppendOpts{Rows: []core.RowInput{{
			GroupID:          groupID,
			Status:           core.StatusExecuted,
			Type:             core.TypeDebit,
			PrimaryAccountID: from,
			Currency:         "NGN",
			Entries: []core.EntryInput{
				{AccountID: from, Type: core.TypeDebit, Amount: 100},
				{AccountID: to, Type: core.TypeCredit, Amount: 100},
			},
		}}})
		if err != nil {
			t.Fatalf("seed in-tx: %v", err)
		}
		// Second: try to append with prior=LIEN; actual is EXECUTED → mismatch.
		_, err = l0.AppendTransactionRows(tx, core.AppendOpts{Rows: []core.RowInput{{
			GroupID:          groupID,
			Status:           core.StatusReversed,
			PriorStatus:      &lien,
			Type:             core.TypeDebit,
			PrimaryAccountID: from,
			Currency:         "NGN",
			Entries: []core.EntryInput{
				{AccountID: from, Type: core.TypeCredit, Amount: 100},
				{AccountID: to, Type: core.TypeDebit, Amount: 100},
			},
		}}})
		if err == nil || err.Code != core.ErrGroupPreconditionFailed {
			t.Fatalf("expected ErrGroupPreconditionFailed, got %v", err)
		}
	})
}

func TestAppendRows_GroupAlreadyReversed(t *testing.T) {
	db := openTestDB(t)
	resetDB(t, db)
	seedCurrency(t, db, "NGN", 2)
	walletID := seedWallet(t, db)
	from := seedAccount(t, db, walletID, "NGN", "liability", accountOpts{balance: 1000, availableBalance: 1000})
	to := seedAccount(t, db, walletID, "NGN", "liability", accountOpts{})
	_, l0 := newLayer1(db)
	groupID := uuid.NewString()
	exec := core.StatusExecuted

	runInTx(t, db, func(tx *gorm.DB) {
		// 1) Insert EXECUTED.
		_, err := l0.AppendTransactionRows(tx, core.AppendOpts{Rows: []core.RowInput{{
			GroupID:          groupID,
			Status:           core.StatusExecuted,
			Type:             core.TypeDebit,
			PrimaryAccountID: from,
			Currency:         "NGN",
			Entries: []core.EntryInput{
				{AccountID: from, Type: core.TypeDebit, Amount: 100},
				{AccountID: to, Type: core.TypeCredit, Amount: 100},
			},
		}}})
		if err != nil {
			t.Fatalf("seed EXECUTED: %v", err)
		}
		// 2) Reverse it.
		_, err = l0.AppendTransactionRows(tx, core.AppendOpts{Rows: []core.RowInput{{
			GroupID:          groupID,
			Status:           core.StatusReversed,
			PriorStatus:      &exec,
			Type:             core.TypeDebit,
			PrimaryAccountID: from,
			Currency:         "NGN",
			Entries: []core.EntryInput{
				{AccountID: from, Type: core.TypeCredit, Amount: 100},
				{AccountID: to, Type: core.TypeDebit, Amount: 100},
			},
		}}})
		if err != nil {
			t.Fatalf("seed REVERSED: %v", err)
		}
		// 3) Try to append again with prior=EXECUTED → must reject as already reversed.
		_, err = l0.AppendTransactionRows(tx, core.AppendOpts{Rows: []core.RowInput{{
			GroupID:          groupID,
			Status:           core.StatusReversed,
			PriorStatus:      &exec,
			Type:             core.TypeDebit,
			PrimaryAccountID: from,
			Currency:         "NGN",
			Entries: []core.EntryInput{
				{AccountID: from, Type: core.TypeCredit, Amount: 100},
				{AccountID: to, Type: core.TypeDebit, Amount: 100},
			},
		}}})
		if err == nil || err.Code != core.ErrGroupAlreadyReversed {
			t.Fatalf("expected ErrGroupAlreadyReversed, got %v", err)
		}
	})
}

func TestAppendRows_ZeroAmount(t *testing.T) {
	db := openTestDB(t)
	resetDB(t, db)
	seedCurrency(t, db, "NGN", 2)
	walletID := seedWallet(t, db)
	from := seedAccount(t, db, walletID, "NGN", "liability", accountOpts{balance: 1000, availableBalance: 1000})
	to := seedAccount(t, db, walletID, "NGN", "liability", accountOpts{})
	_, l0 := newLayer1(db)

	runInTx(t, db, func(tx *gorm.DB) {
		_, err := l0.AppendTransactionRows(tx, core.AppendOpts{Rows: []core.RowInput{{
			GroupID:          uuid.NewString(),
			Status:           core.StatusExecuted,
			Type:             core.TypeDebit,
			PrimaryAccountID: from,
			Currency:         "NGN",
			Entries: []core.EntryInput{
				{AccountID: from, Type: core.TypeDebit, Amount: 0},
				{AccountID: to, Type: core.TypeCredit, Amount: 0},
			},
		}}})
		if err == nil || err.Code != core.ErrInvalidAmount {
			t.Fatalf("expected ErrInvalidAmount, got %v", err)
		}
	})
}

func TestAppendRows_NegativeAmount(t *testing.T) {
	db := openTestDB(t)
	resetDB(t, db)
	seedCurrency(t, db, "NGN", 2)
	walletID := seedWallet(t, db)
	from := seedAccount(t, db, walletID, "NGN", "liability", accountOpts{balance: 1000, availableBalance: 1000})
	to := seedAccount(t, db, walletID, "NGN", "liability", accountOpts{})
	_, l0 := newLayer1(db)

	runInTx(t, db, func(tx *gorm.DB) {
		_, err := l0.AppendTransactionRows(tx, core.AppendOpts{Rows: []core.RowInput{{
			GroupID:          uuid.NewString(),
			Status:           core.StatusExecuted,
			Type:             core.TypeDebit,
			PrimaryAccountID: from,
			Currency:         "NGN",
			Entries: []core.EntryInput{
				{AccountID: from, Type: core.TypeDebit, Amount: -50},
				{AccountID: to, Type: core.TypeCredit, Amount: -50},
			},
		}}})
		if err == nil || err.Code != core.ErrInvalidAmount {
			t.Fatalf("expected ErrInvalidAmount, got %v", err)
		}
	})
}

func TestAppendRows_PrimaryEntryMissing(t *testing.T) {
	db := openTestDB(t)
	resetDB(t, db)
	seedCurrency(t, db, "NGN", 2)
	walletID := seedWallet(t, db)
	from := seedAccount(t, db, walletID, "NGN", "liability", accountOpts{balance: 1000, availableBalance: 1000})
	a := seedAccount(t, db, walletID, "NGN", "liability", accountOpts{})
	b := seedAccount(t, db, walletID, "NGN", "liability", accountOpts{})
	_, l0 := newLayer1(db)

	runInTx(t, db, func(tx *gorm.DB) {
		_, err := l0.AppendTransactionRows(tx, core.AppendOpts{Rows: []core.RowInput{{
			GroupID:          uuid.NewString(),
			Status:           core.StatusExecuted,
			Type:             core.TypeDebit,
			PrimaryAccountID: from, // declared primary…
			Currency:         "NGN",
			Entries: []core.EntryInput{
				// …but neither entry is on `from`. validateRow rejects.
				{AccountID: a, Type: core.TypeDebit, Amount: 100},
				{AccountID: b, Type: core.TypeCredit, Amount: 100},
			},
		}}})
		if err == nil || err.Code != core.ErrPrimaryEntryMissing {
			t.Fatalf("expected ErrPrimaryEntryMissing, got %v", err)
		}
	})
}

func TestAppendRows_PrimaryTypeMismatch(t *testing.T) {
	db := openTestDB(t)
	resetDB(t, db)
	seedCurrency(t, db, "NGN", 2)
	walletID := seedWallet(t, db)
	from := seedAccount(t, db, walletID, "NGN", "liability", accountOpts{balance: 1000, availableBalance: 1000})
	to := seedAccount(t, db, walletID, "NGN", "liability", accountOpts{})
	_, l0 := newLayer1(db)

	runInTx(t, db, func(tx *gorm.DB) {
		_, err := l0.AppendTransactionRows(tx, core.AppendOpts{Rows: []core.RowInput{{
			GroupID:          uuid.NewString(),
			Status:           core.StatusExecuted,
			Type:             core.TypeDebit, // row is DEBIT
			PrimaryAccountID: from,
			Currency:         "NGN",
			Entries: []core.EntryInput{
				// …but primary entry's type is CREDIT.
				{AccountID: from, Type: core.TypeCredit, Amount: 100},
				{AccountID: to, Type: core.TypeDebit, Amount: 100},
			},
		}}})
		if err == nil || err.Code != core.ErrPrimaryTypeMismatch {
			t.Fatalf("expected ErrPrimaryTypeMismatch, got %v", err)
		}
	})
}

func TestAppendRows_Unbalanced(t *testing.T) {
	db := openTestDB(t)
	resetDB(t, db)
	seedCurrency(t, db, "NGN", 2)
	walletID := seedWallet(t, db)
	from := seedAccount(t, db, walletID, "NGN", "liability", accountOpts{balance: 1000, availableBalance: 1000})
	to := seedAccount(t, db, walletID, "NGN", "liability", accountOpts{})
	_, l0 := newLayer1(db)

	runInTx(t, db, func(tx *gorm.DB) {
		_, err := l0.AppendTransactionRows(tx, core.AppendOpts{Rows: []core.RowInput{{
			GroupID:          uuid.NewString(),
			Status:           core.StatusExecuted,
			Type:             core.TypeDebit,
			PrimaryAccountID: from,
			Currency:         "NGN",
			Entries: []core.EntryInput{
				{AccountID: from, Type: core.TypeDebit, Amount: 100},
				{AccountID: to, Type: core.TypeCredit, Amount: 99}, // off by one
			},
		}}})
		if err == nil || err.Code != core.ErrUnbalancedRow {
			t.Fatalf("expected ErrUnbalancedRow, got %v", err)
		}
	})
}

func TestAppendRows_PersistsRowAndEntries(t *testing.T) {
	db := openTestDB(t)
	resetDB(t, db)
	seedCurrency(t, db, "NGN", 2)
	walletID := seedWallet(t, db)
	from := seedAccount(t, db, walletID, "NGN", "liability", accountOpts{balance: 1000, availableBalance: 1000})
	to := seedAccount(t, db, walletID, "NGN", "liability", accountOpts{})
	_, l0 := newLayer1(db)
	groupID := uuid.NewString()

	// Commit this one (don't rollback) so we can verify post-commit state.
	err := db.Transaction(func(tx *gorm.DB) error {
		_, lerr := l0.AppendTransactionRows(tx, core.AppendOpts{Rows: []core.RowInput{{
			GroupID:          groupID,
			Status:           core.StatusExecuted,
			Type:             core.TypeDebit,
			PrimaryAccountID: from,
			Currency:         "NGN",
			Entries: []core.EntryInput{
				{AccountID: from, Type: core.TypeDebit, Amount: 250},
				{AccountID: to, Type: core.TypeCredit, Amount: 250},
			},
		}}})
		if lerr != nil {
			return errors.New(lerr.Error())
		}
		return nil
	})
	if err != nil {
		t.Fatalf("commit append: %v", err)
	}

	// Balances updated correctly per §11: DEBIT EXECUTED-skip-lien primary = -1/-1,
	// secondary = +1/+1.
	fBal, fAvail := loadAccount(t, db, from)
	if fBal != 750 || fAvail != 750 {
		t.Fatalf("from balance: got (%d,%d), want (750,750)", fBal, fAvail)
	}
	tBal, tAvail := loadAccount(t, db, to)
	if tBal != 250 || tAvail != 250 {
		t.Fatalf("to balance: got (%d,%d), want (250,250)", tBal, tAvail)
	}

	// One row, two entries persisted.
	var rowCount int64
	if err := db.Table(`"WalletTransactions"`).Where(`group_id = ?`, groupID).Count(&rowCount).Error; err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if rowCount != 1 {
		t.Fatalf("expected 1 transaction row, got %d", rowCount)
	}
	var entryCount int64
	if err := db.Raw(
		`SELECT COUNT(*) FROM "WalletEntries" e JOIN "WalletTransactions" t ON e.transaction_id = t.id WHERE t.group_id = ?`,
		groupID,
	).Scan(&entryCount).Error; err != nil {
		t.Fatalf("count entries: %v", err)
	}
	if entryCount != 2 {
		t.Fatalf("expected 2 entries, got %d", entryCount)
	}
}
