package tests

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/real-legger/gox-apps/libs/wallet/pkg/core"
)

// unmarshalLien decodes a Layer1 JSON response into a LienResult; failure is fatal.
func unmarshalLien(t *testing.T, raw json.RawMessage) core.LienResult {
	t.Helper()
	var r core.LienResult
	if err := json.Unmarshal(raw, &r); err != nil {
		t.Fatalf("decode LienResult: %v\nraw=%s", err, string(raw))
	}
	return r
}

func unmarshalConvert(t *testing.T, raw json.RawMessage) core.ConvertResult {
	t.Helper()
	var r core.ConvertResult
	if err := json.Unmarshal(raw, &r); err != nil {
		t.Fatalf("decode ConvertResult: %v\nraw=%s", err, string(raw))
	}
	return r
}

// ── Lien ─────────────────────────────────────────────────────────────────────

func TestLien_InvalidAmount(t *testing.T) {
	db := openTestDB(t)
	resetDB(t, db)
	seedCurrency(t, db, "NGN", 2)
	walletID := seedWallet(t, db)
	from := seedAccount(t, db, walletID, "NGN", "liability", accountOpts{balance: 1000, availableBalance: 1000})
	to := seedAccount(t, db, walletID, "NGN", "liability", accountOpts{})
	l1, _ := newLayer1(db)

	for _, amount := range []int64{0, -1} {
		_, err := l1.Lien(core.LienOpts{
			WalletID: walletID, FromAccountID: from, ToAccountID: to,
			Amount: amount, Currency: "NGN",
		})
		if err == nil || err.Code != core.ErrInvalidAmount {
			t.Fatalf("amount=%d: expected ErrInvalidAmount, got %v", amount, err)
		}
	}
}

func TestLien_SelfTransfer(t *testing.T) {
	db := openTestDB(t)
	resetDB(t, db)
	seedCurrency(t, db, "NGN", 2)
	walletID := seedWallet(t, db)
	acct := seedAccount(t, db, walletID, "NGN", "liability", accountOpts{balance: 1000, availableBalance: 1000})
	l1, _ := newLayer1(db)

	_, err := l1.Lien(core.LienOpts{
		WalletID: walletID, FromAccountID: acct, ToAccountID: acct,
		Amount: 100, Currency: "NGN",
	})
	if err == nil || err.Code != core.ErrInvalidRequest {
		t.Fatalf("expected ErrInvalidRequest for self-transfer, got %v", err)
	}
}

func TestLien_AccountNotInWallet(t *testing.T) {
	db := openTestDB(t)
	resetDB(t, db)
	seedCurrency(t, db, "NGN", 2)
	walletA := seedWallet(t, db)
	walletB := seedWallet(t, db)
	from := seedAccount(t, db, walletA, "NGN", "liability", accountOpts{balance: 1000, availableBalance: 1000})
	to := seedAccount(t, db, walletB, "NGN", "liability", accountOpts{})
	l1, _ := newLayer1(db)

	_, err := l1.Lien(core.LienOpts{
		WalletID: walletB, FromAccountID: from, ToAccountID: to, // from belongs to walletA
		Amount: 100, Currency: "NGN",
	})
	if err == nil || err.Code != core.ErrInvalidRequest {
		t.Fatalf("expected ErrInvalidRequest, got %v", err)
	}
}

func TestLien_CurrencyMismatch(t *testing.T) {
	db := openTestDB(t)
	resetDB(t, db)
	seedCurrency(t, db, "NGN", 2)
	seedCurrency(t, db, "USD", 2)
	walletID := seedWallet(t, db)
	from := seedAccount(t, db, walletID, "USD", "liability", accountOpts{balance: 1000, availableBalance: 1000})
	to := seedAccount(t, db, walletID, "USD", "liability", accountOpts{})
	l1, _ := newLayer1(db)

	_, err := l1.Lien(core.LienOpts{
		WalletID: walletID, FromAccountID: from, ToAccountID: to,
		Amount: 100, Currency: "NGN", // accounts are USD
	})
	if err == nil || err.Code != core.ErrCurrencyMismatch {
		t.Fatalf("expected ErrCurrencyMismatch, got %v", err)
	}
}

func TestLien_HappyAppliesFee(t *testing.T) {
	db := openTestDB(t)
	resetDB(t, db)
	seedCurrency(t, db, "NGN", 2)
	walletID := seedWallet(t, db)
	from := seedAccount(t, db, walletID, "NGN", "liability", accountOpts{balance: 1000, availableBalance: 1000})
	to := seedAccount(t, db, walletID, "NGN", "liability", accountOpts{})
	feeAcct := seedAccount(t, db, walletID, "NGN", "revenue", accountOpts{})
	// 10 unit flat fee on any tx with product=transfer.
	seedConfigRule(t, db, "accumulating", "fee",
		`{"product":"transfer"}`,
		`{"account_id":`+jsonInt(feeAcct)+`,"amount":10}`,
		10,
	)
	l1, _ := newLayer1(db)

	raw, err := l1.Lien(core.LienOpts{
		WalletID: walletID, FromAccountID: from, ToAccountID: to,
		Amount: 100, Currency: "NGN",
		Tags: core.Tags{"product": "transfer"},
	})
	if err != nil {
		t.Fatalf("Lien: %v", err)
	}
	res := unmarshalLien(t, raw)
	if res.Status != core.StatusLien {
		t.Fatalf("status: got %q, want LIEN", res.Status)
	}
	// Expect 3 entries: from debit 110, to credit 100, fee credit 10.
	if len(res.Entries) != 3 {
		t.Fatalf("entries: got %d, want 3", len(res.Entries))
	}
	// LIEN holds available_balance on the from account by total debit (100+10).
	fBal, fAvail := loadAccount(t, db, from)
	if fBal != 1000 || fAvail != 890 {
		t.Fatalf("from after lien: got (%d,%d), want (1000,890)", fBal, fAvail)
	}
}

// ── Execute ──────────────────────────────────────────────────────────────────

func TestExecute_NotInLien(t *testing.T) {
	db := openTestDB(t)
	resetDB(t, db)
	seedCurrency(t, db, "NGN", 2)
	walletID := seedWallet(t, db)
	from := seedAccount(t, db, walletID, "NGN", "liability", accountOpts{balance: 1000, availableBalance: 1000})
	to := seedAccount(t, db, walletID, "NGN", "liability", accountOpts{})
	l1, _ := newLayer1(db)

	// Skip-lien: write the group as EXECUTED directly.
	raw, err := l1.ExecuteDirect(core.LienOpts{
		WalletID: walletID, FromAccountID: from, ToAccountID: to,
		Amount: 100, Currency: "NGN",
	})
	if err != nil {
		t.Fatalf("ExecuteDirect: %v", err)
	}
	groupID := unmarshalLien(t, raw).GroupID

	// Execute (LIEN→EXECUTED) on already-executed group must fail.
	_, err = l1.Execute(core.ExecuteOpts{WalletID: walletID, GroupID: groupID})
	if err == nil || err.Code != core.ErrInvalidStatusTransition {
		t.Fatalf("expected ErrInvalidStatusTransition, got %v", err)
	}
}

func TestExecute_HappyFromLien(t *testing.T) {
	db := openTestDB(t)
	resetDB(t, db)
	seedCurrency(t, db, "NGN", 2)
	walletID := seedWallet(t, db)
	from := seedAccount(t, db, walletID, "NGN", "liability", accountOpts{balance: 1000, availableBalance: 1000})
	to := seedAccount(t, db, walletID, "NGN", "liability", accountOpts{})
	l1, _ := newLayer1(db)

	raw, err := l1.Lien(core.LienOpts{
		WalletID: walletID, FromAccountID: from, ToAccountID: to,
		Amount: 200, Currency: "NGN",
	})
	if err != nil {
		t.Fatalf("Lien: %v", err)
	}
	groupID := unmarshalLien(t, raw).GroupID

	if _, err = l1.Execute(core.ExecuteOpts{WalletID: walletID, GroupID: groupID}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	// After execute (skip-lien LIEN→EXECUTED): balance moves the rest of the way.
	fBal, fAvail := loadAccount(t, db, from)
	if fBal != 800 || fAvail != 800 {
		t.Fatalf("from after execute: got (%d,%d), want (800,800)", fBal, fAvail)
	}
	tBal, tAvail := loadAccount(t, db, to)
	if tBal != 200 || tAvail != 200 {
		t.Fatalf("to after execute: got (%d,%d), want (200,200)", tBal, tAvail)
	}
}

// ── Reverse ──────────────────────────────────────────────────────────────────

func TestReverse_AlreadyReversed(t *testing.T) {
	db := openTestDB(t)
	resetDB(t, db)
	seedCurrency(t, db, "NGN", 2)
	walletID := seedWallet(t, db)
	from := seedAccount(t, db, walletID, "NGN", "liability", accountOpts{balance: 1000, availableBalance: 1000})
	to := seedAccount(t, db, walletID, "NGN", "liability", accountOpts{})
	l1, _ := newLayer1(db)

	raw, err := l1.ExecuteDirect(core.LienOpts{
		WalletID: walletID, FromAccountID: from, ToAccountID: to,
		Amount: 50, Currency: "NGN",
	})
	if err != nil {
		t.Fatalf("seed exec: %v", err)
	}
	groupID := unmarshalLien(t, raw).GroupID

	if _, err = l1.Reverse(core.ReverseOpts{WalletID: walletID, GroupID: groupID}); err != nil {
		t.Fatalf("first reverse: %v", err)
	}
	_, err = l1.Reverse(core.ReverseOpts{WalletID: walletID, GroupID: groupID})
	if err == nil || err.Code != core.ErrGroupAlreadyReversed {
		t.Fatalf("expected ErrGroupAlreadyReversed, got %v", err)
	}
}

func TestReverse_FromLienRestoresAvailable(t *testing.T) {
	db := openTestDB(t)
	resetDB(t, db)
	seedCurrency(t, db, "NGN", 2)
	walletID := seedWallet(t, db)
	from := seedAccount(t, db, walletID, "NGN", "liability", accountOpts{balance: 1000, availableBalance: 1000})
	to := seedAccount(t, db, walletID, "NGN", "liability", accountOpts{})
	l1, _ := newLayer1(db)

	raw, err := l1.Lien(core.LienOpts{
		WalletID: walletID, FromAccountID: from, ToAccountID: to,
		Amount: 100, Currency: "NGN",
	})
	if err != nil {
		t.Fatalf("Lien: %v", err)
	}
	groupID := unmarshalLien(t, raw).GroupID

	if _, err = l1.Reverse(core.ReverseOpts{WalletID: walletID, GroupID: groupID}); err != nil {
		t.Fatalf("Reverse: %v", err)
	}
	// Reverse of LIEN: from's available restored to 1000; balance untouched throughout.
	fBal, fAvail := loadAccount(t, db, from)
	if fBal != 1000 || fAvail != 1000 {
		t.Fatalf("from after reverse-of-lien: got (%d,%d), want (1000,1000)", fBal, fAvail)
	}
	tBal, tAvail := loadAccount(t, db, to)
	if tBal != 0 || tAvail != 0 {
		t.Fatalf("to after reverse-of-lien: got (%d,%d), want (0,0)", tBal, tAvail)
	}
}

func TestReverse_FromExecutedRestoresBoth(t *testing.T) {
	db := openTestDB(t)
	resetDB(t, db)
	seedCurrency(t, db, "NGN", 2)
	walletID := seedWallet(t, db)
	from := seedAccount(t, db, walletID, "NGN", "liability", accountOpts{balance: 1000, availableBalance: 1000})
	to := seedAccount(t, db, walletID, "NGN", "liability", accountOpts{})
	l1, _ := newLayer1(db)

	raw, err := l1.ExecuteDirect(core.LienOpts{
		WalletID: walletID, FromAccountID: from, ToAccountID: to,
		Amount: 100, Currency: "NGN",
	})
	if err != nil {
		t.Fatalf("exec: %v", err)
	}
	groupID := unmarshalLien(t, raw).GroupID

	if _, err = l1.Reverse(core.ReverseOpts{WalletID: walletID, GroupID: groupID}); err != nil {
		t.Fatalf("Reverse: %v", err)
	}
	fBal, fAvail := loadAccount(t, db, from)
	if fBal != 1000 || fAvail != 1000 {
		t.Fatalf("from after reverse: got (%d,%d), want (1000,1000)", fBal, fAvail)
	}
	tBal, tAvail := loadAccount(t, db, to)
	if tBal != 0 || tAvail != 0 {
		t.Fatalf("to after reverse: got (%d,%d), want (0,0)", tBal, tAvail)
	}
}

// ── Convert ──────────────────────────────────────────────────────────────────

func TestConvert_SameCurrency(t *testing.T) {
	db := openTestDB(t)
	resetDB(t, db)
	seedCurrency(t, db, "NGN", 2)
	walletID := seedWallet(t, db)
	from := seedAccount(t, db, walletID, "NGN", "liability", accountOpts{balance: 1000, availableBalance: 1000})
	to := seedAccount(t, db, walletID, "NGN", "liability", accountOpts{})
	l1, _ := newLayer1(db)

	_, err := l1.Convert(core.ConvertOpts{
		WalletID: walletID, FromAccountID: from, ToAccountID: to, Amount: 100,
	})
	if err == nil || err.Code != core.ErrSameCurrencyConvert {
		t.Fatalf("expected ErrSameCurrencyConvert, got %v", err)
	}
}

func TestConvert_RateNotFound(t *testing.T) {
	db := openTestDB(t)
	resetDB(t, db)
	seedCurrency(t, db, "NGN", 2)
	seedCurrency(t, db, "USD", 2)
	walletID := seedWallet(t, db)
	from := seedAccount(t, db, walletID, "NGN", "liability", accountOpts{balance: 100000, availableBalance: 100000})
	to := seedAccount(t, db, walletID, "USD", "liability", accountOpts{})
	l1, _ := newLayer1(db)

	_, err := l1.Convert(core.ConvertOpts{
		WalletID: walletID, FromAccountID: from, ToAccountID: to, Amount: 100,
	})
	if err == nil || err.Code != core.ErrRateNotFound {
		t.Fatalf("expected ErrRateNotFound, got %v", err)
	}
}

func TestConvert_RateExpired(t *testing.T) {
	db := openTestDB(t)
	resetDB(t, db)
	seedCurrency(t, db, "NGN", 2)
	seedCurrency(t, db, "USD", 2)
	walletID := seedWallet(t, db)
	from := seedAccount(t, db, walletID, "NGN", "liability", accountOpts{balance: 100000, availableBalance: 100000})
	to := seedAccount(t, db, walletID, "USD", "liability", accountOpts{})

	pastFrom := time.Now().Add(-2 * time.Hour)
	pastTo := time.Now().Add(-1 * time.Hour)
	seedRate(t, db, "NGN", "USD", "0.00065", pastFrom, &pastTo)
	l1, _ := newLayer1(db)

	_, err := l1.Convert(core.ConvertOpts{
		WalletID: walletID, FromAccountID: from, ToAccountID: to, Amount: 100,
	})
	if err == nil || err.Code != core.ErrRateNotFound {
		t.Fatalf("expected ErrRateNotFound for expired rate, got %v", err)
	}
}

func TestConvert_TargetRoundsToZero(t *testing.T) {
	db := openTestDB(t)
	resetDB(t, db)
	seedCurrency(t, db, "NGN", 2)
	seedCurrency(t, db, "USD", 2)
	walletID := seedWallet(t, db)
	from := seedAccount(t, db, walletID, "NGN", "liability", accountOpts{balance: 100000, availableBalance: 100000})
	to := seedAccount(t, db, walletID, "USD", "liability", accountOpts{})
	// Tiny rate; floor(1 * 0.000000000001) = 0.
	seedRate(t, db, "NGN", "USD", "0.000000000001", time.Now().Add(-time.Hour), nil)
	l1, _ := newLayer1(db)

	_, err := l1.Convert(core.ConvertOpts{
		WalletID: walletID, FromAccountID: from, ToAccountID: to, Amount: 1,
	})
	if err == nil || err.Code != core.ErrInvalidAmount {
		t.Fatalf("expected ErrInvalidAmount for zero-rounded target, got %v", err)
	}
}

func TestConvert_Happy(t *testing.T) {
	db := openTestDB(t)
	resetDB(t, db)
	seedCurrency(t, db, "NGN", 2)
	seedCurrency(t, db, "USD", 2)
	walletID := seedWallet(t, db)
	from := seedAccount(t, db, walletID, "NGN", "liability", accountOpts{balance: 100000, availableBalance: 100000})
	to := seedAccount(t, db, walletID, "USD", "liability", accountOpts{})
	fxFrom := seedAccount(t, db, walletID, "NGN", "asset", accountOpts{})
	fxTo := seedAccount(t, db, walletID, "USD", "asset", accountOpts{balance: 10000, availableBalance: 10000})

	seedRate(t, db, "NGN", "USD", "0.001", time.Now().Add(-time.Hour), nil) // 1000 NGN → 1 USD
	seedConfigRule(t, db, "lookup", "fx_target",
		`{"from_currency":"NGN","to_currency":"USD"}`,
		`{"from_account_id":`+jsonInt(fxFrom)+`,"to_account_id":`+jsonInt(fxTo)+`}`,
		100,
	)
	l1, _ := newLayer1(db)

	raw, err := l1.Convert(core.ConvertOpts{
		WalletID: walletID, FromAccountID: from, ToAccountID: to, Amount: 5000,
	})
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	res := unmarshalConvert(t, raw)
	if res.SourceAmount != 5000 || res.TargetAmount != 5 {
		t.Fatalf("amounts: got (src=%d, tgt=%d), want (5000, 5)", res.SourceAmount, res.TargetAmount)
	}
	// User from-account: -5000 NGN. fxFrom: +5000 NGN. fxTo: -5 USD. User to: +5 USD.
	if b, _ := loadAccount(t, db, from); b != 95000 {
		t.Fatalf("from balance after convert: got %d, want 95000", b)
	}
	if b, _ := loadAccount(t, db, fxFrom); b != 5000 {
		t.Fatalf("fxFrom balance: got %d, want 5000", b)
	}
	if b, _ := loadAccount(t, db, fxTo); b != 9995 {
		t.Fatalf("fxTo balance: got %d, want 9995", b)
	}
	if b, _ := loadAccount(t, db, to); b != 5 {
		t.Fatalf("to balance: got %d, want 5", b)
	}
}

func TestConvert_FxIntermediateInsufficient(t *testing.T) {
	db := openTestDB(t)
	resetDB(t, db)
	seedCurrency(t, db, "NGN", 2)
	seedCurrency(t, db, "USD", 2)
	walletID := seedWallet(t, db)
	from := seedAccount(t, db, walletID, "NGN", "liability", accountOpts{balance: 100000, availableBalance: 100000})
	to := seedAccount(t, db, walletID, "USD", "liability", accountOpts{})
	fxFrom := seedAccount(t, db, walletID, "NGN", "asset", accountOpts{})
	// fxTo is PREPAID with zero balance; debiting it on the convert will overdraw.
	fxTo := seedAccount(t, db, walletID, "USD", "prepaid", accountOpts{})

	seedRate(t, db, "NGN", "USD", "0.001", time.Now().Add(-time.Hour), nil)
	seedConfigRule(t, db, "lookup", "fx_target",
		`{"from_currency":"NGN","to_currency":"USD"}`,
		`{"from_account_id":`+jsonInt(fxFrom)+`,"to_account_id":`+jsonInt(fxTo)+`}`,
		100,
	)
	l1, _ := newLayer1(db)

	_, err := l1.Convert(core.ConvertOpts{
		WalletID: walletID, FromAccountID: from, ToAccountID: to, Amount: 5000,
	})
	if err == nil || err.Code != core.ErrFxIntermediateInsufficient {
		t.Fatalf("expected ErrFxIntermediateInsufficient, got %v", err)
	}
}

// jsonInt formats an int64 as a JSON number — used to build inline JSON
// payloads for seedConfigRule without needing fmt in every test.
func jsonInt(n int64) string {
	b, _ := json.Marshal(n)
	return string(b)
}

// idemKey returns a UUID for use as an idempotency key (DB column type is UUID).
func idemKey() *string {
	s := uuid.NewString()
	return &s
}
