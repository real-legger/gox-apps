package currency

import ntxctx "github.com/thescaffold/gox-packages/libs/core/context"

type CreateCurrencyDto struct {
	Code     string `json:"code"     binding:"required"`
	Exponent int    `json:"exponent"`
	Name     string `json:"name"     binding:"required"`
}

type UpdateCurrencyDto struct {
	Exponent *int    `json:"exponent,omitempty"`
	Name     *string `json:"name,omitempty"`
}

type CreateCurrencyReq struct {
	Body CreateCurrencyDto `json:",merge"`
	Ctx  ntxctx.NTXContext `context:"ntx"`
}
type UpdateCurrencyReq struct {
	Code string            `param:"code"`
	Body UpdateCurrencyDto `json:",merge"`
	Ctx  ntxctx.NTXContext `context:"ntx"`
}
type GetCurrencyReq struct {
	Code string            `param:"code"`
	Ctx  ntxctx.NTXContext `context:"ntx"`
}
type DeleteCurrencyReq = GetCurrencyReq
type ListCurrencyReq struct {
	Queries map[string]string `context:"queries"`
	Ctx     ntxctx.NTXContext `context:"ntx"`
}
