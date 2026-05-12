package entry

import ntxctx "github.com/thescaffold/gox-packages/libs/core/context"

type ListEntriesReq struct {
	Queries map[string]string `context:"queries"`
	Ctx     ntxctx.NTXContext `context:"ntx"`
}

type GetEntryReq struct {
	Id  string            `param:"id"`
	Ctx ntxctx.NTXContext `context:"ntx"`
}
