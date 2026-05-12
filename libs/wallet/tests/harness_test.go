package tests

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/real-legger/gox-apps/libs/wallet/pkg/core"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Integration tests in this package require WALLET_TEST_DB_URL to point at a
// postgres database the tests are free to mutate (typically a dedicated db
// like "wallet_test"). When the env var is unset, integration tests call
// t.Skip and pure-function tests still run.

const dsnEnvVar = "WALLET_TEST_DB_URL"

var (
	dbOnce   sync.Once
	sharedDB *gorm.DB
	dbErr    error
)

// openTestDB returns a process-singleton *gorm.DB. On first call it dials the
// DSN, runs the schema, and stashes the result. Subsequent calls return the
// same handle. Tests must call resetDB() to clear data between cases.
func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv(dsnEnvVar)
	if dsn == "" {
		t.Skipf("%s not set; skipping integration test", dsnEnvVar)
	}
	dbOnce.Do(func() {
		db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
			Logger:                 logger.Default.LogMode(logger.Silent),
			SkipDefaultTransaction: true,
		})
		if err != nil {
			dbErr = fmt.Errorf("open: %w", err)
			return
		}
		if err := applySchema(db); err != nil {
			dbErr = fmt.Errorf("schema: %w", err)
			return
		}
		sharedDB = db
	})
	if dbErr != nil {
		t.Fatalf("test db setup: %v", dbErr)
	}
	return sharedDB
}

// resetDB truncates every wallet table so the next test sees a clean slate.
// CASCADE handles FKs; RESTART IDENTITY resets sequences so test assertions
// on returned account IDs stay predictable.
func resetDB(t *testing.T, db *gorm.DB) {
	t.Helper()
	stmt := `TRUNCATE TABLE
		"WalletEntries",
		"WalletTransactions",
		"WalletIdempotency",
		"WalletAccounts",
		"WalletConfigRules",
		"WalletRates",
		"WalletTagDefinitions",
		"WalletWallets",
		"WalletCurrencies"
		RESTART IDENTITY CASCADE`
	if err := db.Exec(stmt).Error; err != nil {
		t.Fatalf("truncate: %v", err)
	}
}

// applySchema runs all migrations in migrations/. The SQL is inlined here
// because *sql.Query has unexported fields and cannot be constructed outside
// the goose package; the migrations are CREATE ... IF NOT EXISTS so re-running
// is a no-op.
func applySchema(db *gorm.DB) error {
	stmts := []string{
		// 001 currencies
		`CREATE TABLE IF NOT EXISTS "WalletCurrencies" (
			"code"     varchar(16) PRIMARY KEY,
			"exponent" int         NOT NULL CHECK ("exponent" >= 0),
			"name"     varchar(64) NOT NULL
		)`,
		// 002 wallets
		`CREATE TABLE IF NOT EXISTS "WalletWallets" (
			"id"         varchar(64)  PRIMARY KEY,
			"tenant_id"  varchar(64)  NOT NULL,
			"name"       varchar(255) NOT NULL,
			"metadata"   jsonb        NOT NULL DEFAULT '{}'::jsonb,
			"created_at" timestamptz  NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS "idx_wallet_wallets_tenant" ON "WalletWallets" ("tenant_id")`,
		// 003 accounts
		`CREATE TABLE IF NOT EXISTS "WalletAccounts" (
			"id"                BIGINT      PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
			"wallet_id"         varchar(64) NOT NULL REFERENCES "WalletWallets"("id"),
			"currency"          varchar(16) NOT NULL REFERENCES "WalletCurrencies"("code"),
			"class"             varchar(32) NOT NULL CHECK ("class" IN ('prepaid','liability','asset','revenue','expense')),
			"tags"              jsonb       NOT NULL DEFAULT '{}'::jsonb,
			"status"            varchar(32) NOT NULL DEFAULT 'active' CHECK ("status" IN ('active','frozen','closed')),
			"balance"           BIGINT      NOT NULL DEFAULT 0,
			"available_balance" BIGINT      NOT NULL DEFAULT 0,
			"metadata"          jsonb       NOT NULL DEFAULT '{}'::jsonb,
			"created_at"        timestamptz NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS "idx_wallet_accounts_wallet"   ON "WalletAccounts" ("wallet_id")`,
		`CREATE INDEX IF NOT EXISTS "idx_wallet_accounts_currency" ON "WalletAccounts" ("currency")`,
		`CREATE INDEX IF NOT EXISTS "idx_wallet_accounts_tags"     ON "WalletAccounts" USING GIN ("tags")`,
		// 004 transactions
		`CREATE TABLE IF NOT EXISTS "WalletTransactions" (
			"id"                 BIGINT      PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
			"group_id"           UUID        NOT NULL,
			"status"             varchar(16) NOT NULL CHECK ("status" IN ('LIEN','EXECUTED','REVERSED')),
			"prior_status"       varchar(16) CHECK ("prior_status" IN ('LIEN','EXECUTED')),
			"type"               varchar(8)  NOT NULL CHECK ("type" IN ('DEBIT','CREDIT')),
			"primary_account_id" BIGINT      NOT NULL REFERENCES "WalletAccounts"("id"),
			"currency"           varchar(16) NOT NULL REFERENCES "WalletCurrencies"("code"),
			"tags"               jsonb       NOT NULL DEFAULT '{}'::jsonb,
			"idempotency_key"    UUID,
			"fx_pair_group_id"   UUID,
			"created_at"         timestamptz NOT NULL DEFAULT NOW(),
			CONSTRAINT "wallet_tx_valid_transition" CHECK (
				("status" = 'LIEN'     AND "prior_status" IS NULL) OR
				("status" = 'EXECUTED' AND ("prior_status" IS NULL OR "prior_status" = 'LIEN')) OR
				("status" = 'REVERSED' AND "prior_status" IN ('LIEN','EXECUTED'))
			)
		)`,
		`CREATE INDEX IF NOT EXISTS "idx_wallet_tx_group_created"   ON "WalletTransactions" ("group_id","created_at" DESC,"id" DESC)`,
		`CREATE INDEX IF NOT EXISTS "idx_wallet_tx_status"          ON "WalletTransactions" ("status")`,
		`CREATE INDEX IF NOT EXISTS "idx_wallet_tx_idempotency"     ON "WalletTransactions" ("idempotency_key") WHERE "idempotency_key" IS NOT NULL`,
		`CREATE INDEX IF NOT EXISTS "idx_wallet_tx_tags"            ON "WalletTransactions" USING GIN ("tags")`,
		`CREATE INDEX IF NOT EXISTS "idx_wallet_tx_primary_account" ON "WalletTransactions" ("primary_account_id")`,
		`CREATE INDEX IF NOT EXISTS "idx_wallet_tx_fx_pair"         ON "WalletTransactions" ("fx_pair_group_id") WHERE "fx_pair_group_id" IS NOT NULL`,
		`CREATE OR REPLACE RULE "wallet_tx_no_update" AS ON UPDATE TO "WalletTransactions" DO INSTEAD NOTHING`,
		`CREATE OR REPLACE RULE "wallet_tx_no_delete" AS ON DELETE TO "WalletTransactions" DO INSTEAD NOTHING`,
		// 005 entries
		`CREATE TABLE IF NOT EXISTS "WalletEntries" (
			"id"             BIGINT      PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
			"transaction_id" BIGINT      NOT NULL REFERENCES "WalletTransactions"("id"),
			"account_id"     BIGINT      NOT NULL REFERENCES "WalletAccounts"("id"),
			"type"           varchar(8)  NOT NULL CHECK ("type" IN ('DEBIT','CREDIT')),
			"amount"         BIGINT      NOT NULL CHECK ("amount" > 0),
			"created_at"     timestamptz NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS "idx_wallet_entries_tx"      ON "WalletEntries" ("transaction_id")`,
		`CREATE INDEX IF NOT EXISTS "idx_wallet_entries_account" ON "WalletEntries" ("account_id")`,
		`CREATE OR REPLACE RULE "wallet_entries_no_update" AS ON UPDATE TO "WalletEntries" DO INSTEAD NOTHING`,
		`CREATE OR REPLACE RULE "wallet_entries_no_delete" AS ON DELETE TO "WalletEntries" DO INSTEAD NOTHING`,
		// 006 tag definitions
		`CREATE TABLE IF NOT EXISTS "WalletTagDefinitions" (
			"key"            varchar(64) PRIMARY KEY,
			"description"    text,
			"applies_to"     varchar(16) NOT NULL CHECK ("applies_to" IN ('account','transaction','both')),
			"allowed_values" text[]
		)`,
		// 007 config rules
		`CREATE TABLE IF NOT EXISTS "WalletConfigRules" (
			"id"         BIGINT      PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
			"kind"       varchar(16) NOT NULL CHECK ("kind" IN ('lookup','accumulating')),
			"category"   varchar(64) NOT NULL,
			"match"      jsonb       NOT NULL,
			"priority"   int         NOT NULL DEFAULT 0,
			"payload"    jsonb       NOT NULL,
			"active"     boolean     NOT NULL DEFAULT TRUE,
			"created_at" timestamptz NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS "idx_wallet_cfg_kind_category" ON "WalletConfigRules" ("kind","category") WHERE "active"`,
		`CREATE INDEX IF NOT EXISTS "idx_wallet_cfg_match"         ON "WalletConfigRules" USING GIN ("match")`,
		// 008 rates
		`CREATE TABLE IF NOT EXISTS "WalletRates" (
			"id"            BIGINT          PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
			"from_currency" varchar(16)     NOT NULL REFERENCES "WalletCurrencies"("code"),
			"to_currency"   varchar(16)     NOT NULL REFERENCES "WalletCurrencies"("code"),
			"rate"          NUMERIC(30, 12) NOT NULL CHECK ("rate" > 0),
			"valid_from"    timestamptz     NOT NULL,
			"valid_to"      timestamptz,
			"source"        varchar(64),
			"created_at"    timestamptz     NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS "idx_wallet_rates_lookup" ON "WalletRates" ("from_currency","to_currency","valid_from" DESC)`,
		// 009 idempotency
		`CREATE TABLE IF NOT EXISTS "WalletIdempotency" (
			"key"          UUID         PRIMARY KEY,
			"wallet_id"    varchar(64)  NOT NULL REFERENCES "WalletWallets"("id"),
			"request_hash" bytea        NOT NULL,
			"response"     jsonb        NOT NULL,
			"expires_at"   timestamptz  NOT NULL,
			"created_at"   timestamptz  NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS "idx_wallet_idempotency_expires" ON "WalletIdempotency" ("expires_at")`,
		// 010 view
		`CREATE OR REPLACE VIEW "WalletCurrentTransactionState" AS
			SELECT DISTINCT ON ("group_id") *
			FROM "WalletTransactions"
			ORDER BY "group_id", "created_at" DESC, "id" DESC`,
	}
	for _, s := range stmts {
		if err := db.Exec(s).Error; err != nil {
			return fmt.Errorf("apply: %s\n%w", s, err)
		}
	}
	return nil
}

// ── fixtures ─────────────────────────────────────────────────────────────────

// seedCurrency inserts a currency, idempotently.
func seedCurrency(t *testing.T, db *gorm.DB, code string, exponent int) {
	t.Helper()
	err := db.Exec(
		`INSERT INTO "WalletCurrencies" (code, exponent, name) VALUES (?, ?, ?)
		 ON CONFLICT (code) DO NOTHING`,
		code, exponent, code,
	).Error
	if err != nil {
		t.Fatalf("seedCurrency %s: %v", code, err)
	}
}

// seedWallet inserts a wallet row; returns the wallet_id.
func seedWallet(t *testing.T, db *gorm.DB) string {
	t.Helper()
	id := "w_" + uuid.NewString()
	err := db.Exec(
		`INSERT INTO "WalletWallets" (id, tenant_id, name) VALUES (?, ?, ?)`,
		id, "test-tenant", "test-wallet",
	).Error
	if err != nil {
		t.Fatalf("seedWallet: %v", err)
	}
	return id
}

// accountOpts overrides defaults on seeded accounts.
type accountOpts struct {
	balance          int64
	availableBalance int64
	status           string // "active" | "frozen" | "closed"
	tags             string // JSON
}

func seedAccount(t *testing.T, db *gorm.DB, walletID, currency, class string, opts accountOpts) int64 {
	t.Helper()
	if opts.status == "" {
		opts.status = "active"
	}
	if opts.tags == "" {
		opts.tags = "{}"
	}
	var id int64
	err := db.Raw(
		`INSERT INTO "WalletAccounts"
			(wallet_id, currency, class, tags, status, balance, available_balance)
		 VALUES (?, ?, ?, ?::jsonb, ?, ?, ?)
		 RETURNING id`,
		walletID, currency, class, opts.tags, opts.status, opts.balance, opts.availableBalance,
	).Scan(&id).Error
	if err != nil || id == 0 {
		t.Fatalf("seedAccount: %v (id=%d)", err, id)
	}
	return id
}

// seedRate inserts an FX rate. validTo=nil means open-ended.
func seedRate(t *testing.T, db *gorm.DB, from, to, rate string, validFrom time.Time, validTo *time.Time) {
	t.Helper()
	err := db.Exec(
		`INSERT INTO "WalletRates" (from_currency, to_currency, rate, valid_from, valid_to)
		 VALUES (?, ?, ?::numeric, ?, ?)`,
		from, to, rate, validFrom, validTo,
	).Error
	if err != nil {
		t.Fatalf("seedRate: %v", err)
	}
}

// seedConfigRule inserts a config rule. match and payload are JSON strings.
func seedConfigRule(t *testing.T, db *gorm.DB, kind, category, match, payload string, priority int) {
	t.Helper()
	err := db.Exec(
		`INSERT INTO "WalletConfigRules" (kind, category, match, priority, payload, active)
		 VALUES (?, ?, ?::jsonb, ?, ?::jsonb, TRUE)`,
		kind, category, match, priority, payload,
	).Error
	if err != nil {
		t.Fatalf("seedConfigRule: %v", err)
	}
}

// loadAccount fetches the current row state for assertions.
func loadAccount(t *testing.T, db *gorm.DB, id int64) (balance, available int64) {
	t.Helper()
	type row struct {
		Balance          int64 `gorm:"column:balance"`
		AvailableBalance int64 `gorm:"column:available_balance"`
	}
	var r row
	err := db.Table(`"WalletAccounts"`).Select(`balance, available_balance`).Where(`id = ?`, id).Scan(&r).Error
	if err != nil {
		t.Fatalf("loadAccount %d: %v", id, err)
	}
	return r.Balance, r.AvailableBalance
}

// newLayer1 constructs Layer1 + Layer0 wired to the test db.
func newLayer1(db *gorm.DB) (*core.Layer1, *core.Layer0) {
	l0 := &core.Layer0{}
	l1 := core.NewLayer1ForTest(db, l0)
	return l1, l0
}
