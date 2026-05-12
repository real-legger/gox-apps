package migrations

import "github.com/awesome-goose/goose/modules/sql"

type CreateWalletEntries struct{ sql.BaseMigration }

func (m *CreateWalletEntries) Run(q *sql.Query) error {
	stmts := []string{
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
	}
	for _, s := range stmts {
		if _, err := q.Exec(s); err != nil {
			return err
		}
	}
	return nil
}
