package tagdef

import (
	"database/sql/driver"
	"errors"
	"strings"

	"github.com/awesome-goose/goose/modules/sql"
)

// StringArray maps Postgres text[] ↔ Go []string.
type StringArray []string

func (a StringArray) Value() (driver.Value, error) {
	if len(a) == 0 {
		return nil, nil
	}
	// Postgres array literal: {"a","b"}
	parts := make([]string, len(a))
	for i, s := range a {
		parts[i] = `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
	}
	return "{" + strings.Join(parts, ",") + "}", nil
}

func (a *StringArray) Scan(src any) error {
	if src == nil {
		*a = nil
		return nil
	}
	var s string
	switch v := src.(type) {
	case string:
		s = v
	case []byte:
		s = string(v)
	default:
		return errors.New("StringArray: unsupported scan type")
	}
	s = strings.TrimPrefix(strings.TrimSuffix(s, "}"), "{")
	if s == "" {
		*a = nil
		return nil
	}
	out := []string{}
	cur := []byte{}
	inQuote := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"' && !inQuote:
			inQuote = true
		case c == '"' && inQuote:
			inQuote = false
		case c == ',' && !inQuote:
			out = append(out, string(cur))
			cur = cur[:0]
		default:
			cur = append(cur, c)
		}
	}
	if len(cur) > 0 {
		out = append(out, string(cur))
	}
	*a = out
	return nil
}

// TagDefinition mirrors WalletTagDefinitions. PK is `key`.
type TagDefinition struct {
	Key           string      `gorm:"primaryKey;column:key;type:varchar(64);not null" json:"key"`
	Description   *string     `gorm:"column:description;type:text"                    json:"description,omitempty"`
	AppliesTo     string      `gorm:"column:applies_to;type:varchar(16);not null"     json:"applies_to"`
	AllowedValues StringArray `gorm:"column:allowed_values;type:text[]"               json:"allowed_values,omitempty"`
}

func (TagDefinition) TableName() string { return "WalletTagDefinitions" }

type TagDefEntity struct {
	*sql.Entity[TagDefinition] `inject:""`
}

func (e *TagDefEntity) OnRegister() {
	e.Hydrate(
		"WalletTagDefinitions",
		[]string{"key", "applies_to"},
		nil,
		nil,
		nil,
		nil,
		nil,
		"key asc",
	)
}
