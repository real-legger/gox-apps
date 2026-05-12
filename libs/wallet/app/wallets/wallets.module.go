package wallets

import (
	"github.com/awesome-goose/goose/modules/sql"
	"github.com/awesome-goose/goose/types"
)

type WalletsModule struct{}

func (m *WalletsModule) Imports() []types.Module {
	return []types.Module{
		sql.Child(&sql.Config{}),
		ROUTES,
	}
}

func (m *WalletsModule) Exports() []any {
	return []any{&WalletService{}}
}

func (m *WalletsModule) Declarations() []any {
	return []any{
		&WalletService{},
		&WalletEntity{},
		&WalletController{},
	}
}
