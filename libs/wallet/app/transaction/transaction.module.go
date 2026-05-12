package transaction

import (
	"github.com/awesome-goose/goose/modules/sql"
	"github.com/awesome-goose/goose/types"
)

type TransactionModule struct{}

func (m *TransactionModule) Imports() []types.Module {
	return []types.Module{
		sql.Child(&sql.Config{}),
		ROUTES,
	}
}

func (m *TransactionModule) Exports() []any {
	return []any{&TransactionService{}}
}

func (m *TransactionModule) Declarations() []any {
	return []any{
		&TransactionService{},
		&TransactionEntity{},
		&TransactionController{},
	}
}
