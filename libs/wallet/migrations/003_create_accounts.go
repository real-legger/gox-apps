package migrations

import "github.com/awesome-goose/goose/modules/sql"

type CreateWalletAccounts struct{ sql.BaseMigration }

func (m *CreateWalletAccounts) Run(q *sql.Query) error {
	stmts := []string{
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
	}
	for _, s := range stmts {
		if _, err := q.Exec(s); err != nil {
			return err
		}
	}
	return nil
}
