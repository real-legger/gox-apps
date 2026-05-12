package account

import (
	ntxctx "github.com/thescaffold/gox-packages/libs/core/context"
)

// CreateAccountInput maps 1:1 to core.CreateAccountInput. Defined here to
// avoid the wallet/account → wallet/core import (the controller forwards
// to core.Service which performs vocabulary validation).
type CreateAccountInput struct {
	WalletId string            `json:"wallet_id" binding:"required"`
	Currency string            `json:"currency"  binding:"required"`
	Class    string            `json:"class"     binding:"required"`
	Tags     map[string]string `json:"tags,omitempty"`
	Metadata any               `json:"metadata,omitempty"`
}

type CreateAccountsDto struct {
	Accounts []CreateAccountInput `json:"accounts" binding:"required"`
	Ctx      ntxctx.NTXContext    `context:"ntx"`
}

type GetAccountReq struct {
	Id  string            `param:"id"`
	Ctx ntxctx.NTXContext `context:"ntx"`
}

type ListAccountsReq struct {
	Queries map[string]string `context:"queries"`
	Ctx     ntxctx.NTXContext `context:"ntx"`
}

// UpdateStatusDto changes only the lifecycle status (active/frozen/closed).
type UpdateStatusDto struct {
	Status string `json:"status" binding:"required"`
}

type UpdateStatusReq struct {
	Id   string            `param:"id"`
	Body UpdateStatusDto   `json:",merge"`
	Ctx  ntxctx.NTXContext `context:"ntx"`
}
