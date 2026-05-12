package migrations

import "github.com/awesome-goose/goose/modules/sql"

type CreateWalletCurrencies struct{ sql.BaseMigration }

func (m *CreateWalletCurrencies) Run(q *sql.Query) error {
	_, err := q.Exec(`
		CREATE TABLE IF NOT EXISTS "WalletCurrencies" (
			"code"     varchar(16) PRIMARY KEY,
			"exponent" int         NOT NULL CHECK ("exponent" >= 0),
			"name"     varchar(64) NOT NULL
		)
	`)
	return err
}
