package migrations

import "github.com/awesome-goose/goose/modules/sql"

type CreateWalletCurrentStateView struct{ sql.BaseMigration }

func (m *CreateWalletCurrentStateView) Run(q *sql.Query) error {
	_, err := q.Exec(`
		CREATE OR REPLACE VIEW "WalletCurrentTransactionState" AS
			SELECT DISTINCT ON ("group_id") *
			FROM "WalletTransactions"
			ORDER BY "group_id", "created_at" DESC, "id" DESC
	`)
	return err
}
