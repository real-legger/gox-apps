package transaction

import ntxctx "github.com/thescaffold/gox-packages/libs/core/context"

type ListTransactionsReq struct {
	Queries map[string]string `context:"queries"`
	Ctx     ntxctx.NTXContext `context:"ntx"`
}

type GetTransactionReq struct {
	Id  string            `param:"id"`
	Ctx ntxctx.NTXContext `context:"ntx"`
}

type GetGroupReq struct {
	GroupId string            `param:"group_id"`
	Ctx     ntxctx.NTXContext `context:"ntx"`
}
