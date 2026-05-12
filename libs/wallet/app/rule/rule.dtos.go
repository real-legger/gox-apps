package rule

import "encoding/json"

type CreateRuleDto struct {
	Kind     string          `json:"kind"     binding:"required"`
	Category string          `json:"category" binding:"required"`
	Match    json.RawMessage `json:"match"    binding:"required"`
	Priority int             `json:"priority"`
	Payload  json.RawMessage `json:"payload"  binding:"required"`
	Active   *bool           `json:"active,omitempty"`
}

type UpdateRuleDto struct {
	Kind     *string         `json:"kind,omitempty"`
	Category *string         `json:"category,omitempty"`
	Match    json.RawMessage `json:"match,omitempty"`
	Priority *int            `json:"priority,omitempty"`
	Payload  json.RawMessage `json:"payload,omitempty"`
	Active   *bool           `json:"active,omitempty"`
}
