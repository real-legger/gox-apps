package migrations

import "github.com/awesome-goose/goose/modules/sql"

type CreateWalletConfigRules struct{ sql.BaseMigration }

func (m *CreateWalletConfigRules) Run(q *sql.Query) error {
	stmts := []string{
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
	}
	for _, s := range stmts {
		if _, err := q.Exec(s); err != nil {
			return err
		}
	}
	return nil
}
