package app

import (
	"github.com/awesome-goose/goose/modules/sql"
	"github.com/awesome-goose/goose/types"
	"github.com/real-legger/gox-apps/libs/wallet/app/account"
	"github.com/real-legger/gox-apps/libs/wallet/app/currency"
	"github.com/real-legger/gox-apps/libs/wallet/app/entry"
	"github.com/real-legger/gox-apps/libs/wallet/app/rate"
	"github.com/real-legger/gox-apps/libs/wallet/app/rule"
	"github.com/real-legger/gox-apps/libs/wallet/app/tagdef"
	"github.com/real-legger/gox-apps/libs/wallet/app/transaction"
	"github.com/real-legger/gox-apps/libs/wallet/app/wallets"
	"github.com/real-legger/gox-apps/libs/wallet/migrations"
	"github.com/real-legger/gox-apps/libs/wallet/pkg/core"
	"github.com/thescaffold/gox-packages/libs/core/module"
)

// Migrations contains schema migrations run by this app, ordered by FK dependency.
var Migrations = []sql.Migration{
	&migrations.CreateWalletCurrencies{},
	&migrations.CreateWalletWallets{},
	&migrations.CreateWalletAccounts{},
	&migrations.CreateWalletTransactions{},
	&migrations.CreateWalletEntries{},
	&migrations.CreateWalletTagDefinitions{},
	&migrations.CreateWalletConfigRules{},
	&migrations.CreateWalletRates{},
	&migrations.CreateWalletIdempotency{},
	&migrations.CreateWalletCurrentStateView{},
}

// Seeders contains data seeders run after migrations.
var Seeders = []sql.Seeder{}

type AppModule struct{}

func (m *AppModule) Imports() []types.Module {
	return []types.Module{
		module.New(module.CoreConfig{}),
		sql.Child(&sql.Config{
			Migrations: Migrations,
			Seeders:    Seeders,
		}),
		// Ledger orchestration (lien/execute/reverse/convert) lives in its own module.
		&core.LedgerModule{},
		// Per-table CRUD modules.
		&currency.CurrencyModule{},
		&wallets.WalletsModule{},
		&account.AccountModule{},
		&transaction.TransactionModule{},
		&entry.EntryModule{},
		&tagdef.TagDefModule{},
		&rule.RuleModule{},
		&rate.RateModule{},
		ROUTES,
	}
}

func (m *AppModule) Exports() []any {
	return []any{&AppService{}}
}

func (m *AppModule) Declarations() []any {
	return []any{
		&AppService{},
		&AppController{},
	}
}
