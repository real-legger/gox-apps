package currency

import (
	"github.com/awesome-goose/goose/modules/sql"
	"github.com/awesome-goose/goose/types"
)

type CurrencyModule struct{}

func (m *CurrencyModule) Imports() []types.Module {
	return []types.Module{
		sql.Child(&sql.Config{}),
		ROUTES,
	}
}

func (m *CurrencyModule) Exports() []any {
	return []any{&CurrencyService{}}
}

func (m *CurrencyModule) Declarations() []any {
	return []any{
		&CurrencyService{},
		&CurrencyEntity{},
		&CurrencyController{},
	}
}
