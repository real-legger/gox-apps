package tagdef

import (
	"github.com/awesome-goose/goose/modules/sql"
	"github.com/awesome-goose/goose/types"
)

type TagDefModule struct{}

func (m *TagDefModule) Imports() []types.Module {
	return []types.Module{
		sql.Child(&sql.Config{}),
		ROUTES,
	}
}

func (m *TagDefModule) Exports() []any {
	return []any{&TagDefService{}}
}

func (m *TagDefModule) Declarations() []any {
	return []any{
		&TagDefService{},
		&TagDefEntity{},
		&TagDefController{},
	}
}
