package wallets

type WalletService struct {
	entity *WalletEntity `inject:""`
}

func (s *WalletService) Get(id string) (*Wallet, error) {
	return s.entity.First("id = ?", id)
}

func (s *WalletService) ListByTenant(tenantId string) ([]Wallet, error) {
	return s.entity.Some("tenant_id = ?", tenantId)
}
