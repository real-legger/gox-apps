package account

import (
	"github.com/awesome-goose/goose/modules/sql"
	"github.com/awesome-goose/goose/types"
)

type AccountModule struct{}

func (m *AccountModule) Imports() []types.Module {
	return []types.Module{
		sql.Child(&sql.Config{}),
		ROUTES,
	}
}

func (m *AccountModule) Exports() []any {
	return []any{&AccountService{}}
}

func (m *AccountModule) Declarations() []any {
	return []any{
		&AccountService{},
		&AccountEntity{},
		&AccountController{},
	}
}
