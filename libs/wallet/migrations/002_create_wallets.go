package migrations

import "github.com/awesome-goose/goose/modules/sql"

type CreateWalletWallets struct{ sql.BaseMigration }

func (m *CreateWalletWallets) Run(q *sql.Query) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS "WalletWallets" (
			"id"         varchar(64)  PRIMARY KEY,
			"tenant_id"  varchar(64)  NOT NULL,
			"name"       varchar(255) NOT NULL,
			"metadata"   jsonb        NOT NULL DEFAULT '{}'::jsonb,
			"created_at" timestamptz  NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS "idx_wallet_wallets_tenant" ON "WalletWallets" ("tenant_id")`,
	}
	for _, s := range stmts {
		if _, err := q.Exec(s); err != nil {
			return err
		}
	}
	return nil
}
