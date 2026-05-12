package migrations

import "github.com/awesome-goose/goose/modules/sql"

type CreateWalletRates struct{ sql.BaseMigration }

func (m *CreateWalletRates) Run(q *sql.Query) error {
	stmts := []string{
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
	}
	for _, s := range stmts {
		if _, err := q.Exec(s); err != nil {
			return err
		}
	}
	return nil
}
