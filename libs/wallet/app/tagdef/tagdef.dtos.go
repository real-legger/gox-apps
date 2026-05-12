package tagdef

import ntxctx "github.com/thescaffold/gox-packages/libs/core/context"

type CreateTagDefinitionDto struct {
	Key           string   `json:"key"           binding:"required"`
	Description   *string  `json:"description,omitempty"`
	AppliesTo     string   `json:"applies_to"    binding:"required"`
	AllowedValues []string `json:"allowed_values,omitempty"`
}

type UpdateTagDefinitionDto struct {
	Description   *string  `json:"description,omitempty"`
	AppliesTo     *string  `json:"applies_to,omitempty"`
	AllowedValues []string `json:"allowed_values,omitempty"`
}

type CreateTagDefReq struct {
	Body CreateTagDefinitionDto `json:",merge"`
	Ctx  ntxctx.NTXContext      `context:"ntx"`
}
type UpdateTagDefReq struct {
	Key  string                 `param:"key"`
	Body UpdateTagDefinitionDto `json:",merge"`
	Ctx  ntxctx.NTXContext      `context:"ntx"`
}
type GetTagDefReq struct {
	Key string            `param:"key"`
	Ctx ntxctx.NTXContext `context:"ntx"`
}
type DeleteTagDefReq = GetTagDefReq
type ListTagDefReq struct {
	Queries map[string]string `context:"queries"`
	Ctx     ntxctx.NTXContext `context:"ntx"`
}
