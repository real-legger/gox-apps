package wallets

import (
	"github.com/thescaffold/gox-packages/libs/core/crud"
)

type WalletController struct {
	crud.CrudResource[Wallet, CreateWalletDto, UpdateWalletDto]

	entity *WalletEntity `inject:""`
}

func (c *WalletController) OnRegister() {
	c.Hydrate(c.entity, crud.Config[Wallet, CreateWalletDto, UpdateWalletDto]{
		Name:       "Wallet",
		Searchable: []string{"name", "id", "tenant_id"},
	})
}
