package core

import (
	ntxctx "github.com/thescaffold/gox-packages/libs/core/context"
)

// CreateAccountInput is consumed by Service.CreateAccounts. The HTTP-facing
// CRUD surface for accounts lives in app/account; this DTO is the wire shape
// shared across both.
type CreateAccountInput struct {
	WalletID string `json:"wallet_id" binding:"required"`
	Currency string `json:"currency"  binding:"required"`
	Class    string `json:"class"     binding:"required"`
	Tags     Tags   `json:"tags,omitempty"`
	Metadata any    `json:"metadata,omitempty"`
}

// AppendRowsDto is the body for Layer 0 batch append.
type AppendRowsDto struct {
	Rows []RowInput        `json:"rows" binding:"required"`
	Ctx  ntxctx.NTXContext `context:"ntx"`
}

// Layer 1 request DTOs (each merges JSON body via `json:",merge"`).
type LienDto struct {
	Body LienOpts          `json:",merge"`
	Ctx  ntxctx.NTXContext `context:"ntx"`
}

type ExecuteDto struct {
	Body ExecuteOpts       `json:",merge"`
	Ctx  ntxctx.NTXContext `context:"ntx"`
}

type ReverseDto struct {
	Body ReverseOpts       `json:",merge"`
	Ctx  ntxctx.NTXContext `context:"ntx"`
}

type ExecuteDirectDto struct {
	Body ExecuteDirectOpts `json:",merge"`
	Ctx  ntxctx.NTXContext `context:"ntx"`
}

type ConvertDto struct {
	Body ConvertOpts       `json:",merge"`
	Ctx  ntxctx.NTXContext `context:"ntx"`
}
