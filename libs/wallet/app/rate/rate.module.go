package rate

import (
	"github.com/awesome-goose/goose/modules/sql"
	"github.com/awesome-goose/goose/types"
)

type RateModule struct{}

func (m *RateModule) Imports() []types.Module {
	return []types.Module{
		sql.Child(&sql.Config{}),
		ROUTES,
	}
}

func (m *RateModule) Exports() []any {
	return []any{&RateService{}}
}

func (m *RateModule) Declarations() []any {
	return []any{
		&RateService{},
		&RateEntity{},
		&RateController{},
	}
}
