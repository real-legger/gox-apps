package app

import (
	"github.com/awesome-goose/goose/modules/sql"
	"github.com/real-legger/gox-apps/libs/wallet/pkg/core"
)

// AppService is the wallet app's top-level service. It re-exports the
// core service for convenience.
type AppService struct {
	db     *sql.Db      `inject:""`
	layer0 *core.Layer0 `inject:""`
}

func (s *AppService) GetHello() string { return "wallet" }
