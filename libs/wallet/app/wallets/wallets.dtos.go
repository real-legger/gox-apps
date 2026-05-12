package wallets

import "encoding/json"

type CreateWalletDto struct {
	Id       string          `json:"id,omitempty"`
	TenantId string          `json:"tenant_id" binding:"required"`
	Name     string          `json:"name"      binding:"required"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

type UpdateWalletDto struct {
	TenantId *string         `json:"tenant_id,omitempty"`
	Name     *string         `json:"name,omitempty"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
}
