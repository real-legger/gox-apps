package currency

import "github.com/awesome-goose/goose/modules/sql"

// Currency mirrors the WalletCurrencies table. PK is `code` (not `id`).
type Currency struct {
	Code     string `gorm:"primaryKey;column:code;type:varchar(16);not null" json:"code"`
	Exponent int    `gorm:"column:exponent;not null"                          json:"exponent"`
	Name     string `gorm:"column:name;type:varchar(64);not null"             json:"name"`
}

func (Currency) TableName() string { return "WalletCurrencies" }

type CurrencyEntity struct {
	*sql.Entity[Currency] `inject:""`
}

func (e *CurrencyEntity) OnRegister() {
	e.Hydrate(
		"WalletCurrencies",
		[]string{"code", "name"},
		nil,
		nil,
		nil,
		nil,
		nil,
		"code asc",
	)
}
