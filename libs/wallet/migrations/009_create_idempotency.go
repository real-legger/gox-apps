package migrations

import "github.com/awesome-goose/goose/modules/sql"

type CreateWalletIdempotency struct{ sql.BaseMigration }

func (m *CreateWalletIdempotency) Run(q *sql.Query) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS "WalletIdempotency" (
			"key"          UUID         PRIMARY KEY,
			"wallet_id"    varchar(64)  NOT NULL REFERENCES "WalletWallets"("id"),
			"request_hash" bytea        NOT NULL,
			"response"     jsonb        NOT NULL,
			"expires_at"   timestamptz  NOT NULL,
			"created_at"   timestamptz  NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS "idx_wallet_idempotency_expires" ON "WalletIdempotency" ("expires_at")`,
	}
	for _, s := range stmts {
		if _, err := q.Exec(s); err != nil {
			return err
		}
	}
	return nil
}
