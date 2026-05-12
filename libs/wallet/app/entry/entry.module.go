package entry

import (
	"github.com/awesome-goose/goose/modules/sql"
	"github.com/awesome-goose/goose/types"
)

type EntryModule struct{}

func (m *EntryModule) Imports() []types.Module {
	return []types.Module{
		sql.Child(&sql.Config{}),
		ROUTES,
	}
}

func (m *EntryModule) Exports() []any {
	return []any{&EntryService{}}
}

func (m *EntryModule) Declarations() []any {
	return []any{
		&EntryService{},
		&EntryEntity{},
		&EntryController{},
	}
}
