package migrations

import "github.com/awesome-goose/goose/modules/sql"

type CreateWalletTagDefinitions struct{ sql.BaseMigration }

func (m *CreateWalletTagDefinitions) Run(q *sql.Query) error {
	_, err := q.Exec(`
		CREATE TABLE IF NOT EXISTS "WalletTagDefinitions" (
			"key"            varchar(64) PRIMARY KEY,
			"description"    text,
			"applies_to"     varchar(16) NOT NULL CHECK ("applies_to" IN ('account','transaction','both')),
			"allowed_values" text[]
		)
	`)
	return err
}
