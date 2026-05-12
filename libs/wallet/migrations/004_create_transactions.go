package migrations

import "github.com/awesome-goose/goose/modules/sql"

type CreateWalletTransactions struct{ sql.BaseMigration }

func (m *CreateWalletTransactions) Run(q *sql.Query) error {
	stmts := []string{
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
	}
	for _, s := range stmts {
		if _, err := q.Exec(s); err != nil {
			return err
		}
	}
	return nil
}
