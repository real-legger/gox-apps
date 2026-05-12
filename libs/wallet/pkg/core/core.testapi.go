package core

import (
	"github.com/awesome-goose/goose/modules/sql"
	"gorm.io/gorm"
)

// NewLayer1ForTest constructs a Layer1 wired to the given gorm.DB and Layer0,
// bypassing the DI container. Production callers obtain Layer1 via the goose
// module (see core.module.go); this constructor exists only for integration
// tests that need a Layer1 against a test database.
func NewLayer1ForTest(db *gorm.DB, l0 *Layer0) *Layer1 {
	return &Layer1{
		db:     &sql.Db{DB: db},
		layer0: l0,
	}
}
