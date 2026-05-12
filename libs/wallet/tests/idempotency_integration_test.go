package tests

import (
	"bytes"
	"testing"

	"github.com/real-legger/gox-apps/libs/wallet/pkg/core"
)

// TestIdempotency_ReplayReturnsCached verifies that calling Lien twice with the
// same idempotency key + same payload returns the cached response byte-for-byte
// without writing a second row.
func TestIdempotency_ReplayReturnsCached(t *testing.T) {
	db := openTestDB(t)
	resetDB(t, db)
	seedCurrency(t, db, "NGN", 2)
	walletID := seedWallet(t, db)
	from := seedAccount(t, db, walletID, "NGN", "liability", accountOpts{balance: 1000, availableBalance: 1000})
	to := seedAccount(t, db, walletID, "NGN", "liability", accountOpts{})
	l1, _ := newLayer1(db)

	key := idemKey()
	opts := core.LienOpts{
		WalletID: walletID, FromAccountID: from, ToAccountID: to,
		Amount: 100, Currency: "NGN", IdempotencyKey: key,
	}
	first, err := l1.Lien(opts)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := l1.Lien(opts)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("replay returned different payload\nfirst=%s\nsecond=%s", string(first), string(second))
	}

	// Only one transaction row was actually written.
	var rowCount int64
	if err := db.Table(`"WalletTransactions"`).Where(`idempotency_key = ?`, *key).Count(&rowCount).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if rowCount != 1 {
		t.Fatalf("expected 1 transaction row, got %d", rowCount)
	}

	// Balances reflect a single lien (available -100), not double.
	_, fAvail := loadAccount(t, db, from)
	if fAvail != 900 {
		t.Fatalf("from available: got %d, want 900 (replay double-charged?)", fAvail)
	}
}

func TestIdempotency_ConflictOnDifferentPayload(t *testing.T) {
	db := openTestDB(t)
	resetDB(t, db)
	seedCurrency(t, db, "NGN", 2)
	walletID := seedWallet(t, db)
	from := seedAccount(t, db, walletID, "NGN", "liability", accountOpts{balance: 1000, availableBalance: 1000})
	to := seedAccount(t, db, walletID, "NGN", "liability", accountOpts{})
	l1, _ := newLayer1(db)

	key := idemKey()
	if _, err := l1.Lien(core.LienOpts{
		WalletID: walletID, FromAccountID: from, ToAccountID: to,
		Amount: 100, Currency: "NGN", IdempotencyKey: key,
	}); err != nil {
		t.Fatalf("first: %v", err)
	}
	// Same key, different amount.
	_, err := l1.Lien(core.LienOpts{
		WalletID: walletID, FromAccountID: from, ToAccountID: to,
		Amount: 200, Currency: "NGN", IdempotencyKey: key,
	})
	if err == nil || err.Code != core.ErrIdempotencyConflict {
		t.Fatalf("expected ErrIdempotencyConflict, got %v", err)
	}
}

func TestIdempotency_ConflictOnDifferentWallet(t *testing.T) {
	db := openTestDB(t)
	resetDB(t, db)
	seedCurrency(t, db, "NGN", 2)
	walletA := seedWallet(t, db)
	walletB := seedWallet(t, db)
	fromA := seedAccount(t, db, walletA, "NGN", "liability", accountOpts{balance: 1000, availableBalance: 1000})
	toA := seedAccount(t, db, walletA, "NGN", "liability", accountOpts{})
	fromB := seedAccount(t, db, walletB, "NGN", "liability", accountOpts{balance: 1000, availableBalance: 1000})
	toB := seedAccount(t, db, walletB, "NGN", "liability", accountOpts{})
	l1, _ := newLayer1(db)

	key := idemKey()
	if _, err := l1.Lien(core.LienOpts{
		WalletID: walletA, FromAccountID: fromA, ToAccountID: toA,
		Amount: 100, Currency: "NGN", IdempotencyKey: key,
	}); err != nil {
		t.Fatalf("first: %v", err)
	}
	// Same key but different wallet → conflict (even if payload is otherwise sensible).
	_, err := l1.Lien(core.LienOpts{
		WalletID: walletB, FromAccountID: fromB, ToAccountID: toB,
		Amount: 100, Currency: "NGN", IdempotencyKey: key,
	})
	if err == nil || err.Code != core.ErrIdempotencyConflict {
		t.Fatalf("expected ErrIdempotencyConflict, got %v", err)
	}
}

// TestIdempotency_NilKeyExecutesEveryCall verifies that omitting the
// idempotency key skips the cache lookup; each invocation creates a new row.
func TestIdempotency_NilKeyExecutesEveryCall(t *testing.T) {
	db := openTestDB(t)
	resetDB(t, db)
	seedCurrency(t, db, "NGN", 2)
	walletID := seedWallet(t, db)
	from := seedAccount(t, db, walletID, "NGN", "liability", accountOpts{balance: 1000, availableBalance: 1000})
	to := seedAccount(t, db, walletID, "NGN", "liability", accountOpts{})
	l1, _ := newLayer1(db)

	opts := core.LienOpts{
		WalletID: walletID, FromAccountID: from, ToAccountID: to,
		Amount: 100, Currency: "NGN", IdempotencyKey: nil,
	}
	if _, err := l1.Lien(opts); err != nil {
		t.Fatalf("first: %v", err)
	}
	if _, err := l1.Lien(opts); err != nil {
		t.Fatalf("second: %v", err)
	}

	// Two distinct transaction rows; available_balance debited twice.
	var rowCount int64
	if err := db.Table(`"WalletTransactions"`).Where(`primary_account_id = ?`, from).Count(&rowCount).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if rowCount != 2 {
		t.Fatalf("expected 2 rows without idempotency key, got %d", rowCount)
	}
	_, fAvail := loadAccount(t, db, from)
	if fAvail != 800 {
		t.Fatalf("from available: got %d, want 800 (two liens of 100)", fAvail)
	}
}
