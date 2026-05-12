package core

import (
	"github.com/awesome-goose/goose/modules/sql"
	"github.com/awesome-goose/goose/types"
)

// LedgerModule wires the Layer 0 / Layer 1 core services and HTTP routes.
type LedgerModule struct{}

func (m *LedgerModule) Imports() []types.Module {
	return []types.Module{
		sql.Child(&sql.Config{}),
	}
}

func (m *LedgerModule) Exports() []any {
	return []any{
		&Layer0{},
		&Layer1{},
	}
}

func (m *LedgerModule) Declarations() []any {
	return []any{
		&Layer0{},
		&Layer1{},
	}
}
