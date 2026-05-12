package rate

import (
	"time"

	"github.com/awesome-goose/goose/modules/sql"
)

// Rate mirrors WalletRates. The numeric `rate` is stored as a precise string
// to preserve the NUMERIC(30,12) precision through Go's JSON encoding.
type Rate struct {
	Id           int64      `gorm:"primaryKey;column:id;autoIncrement"            json:"id"`
	FromCurrency string     `gorm:"column:from_currency;type:varchar(16);not null" json:"from_currency"`
	ToCurrency   string     `gorm:"column:to_currency;type:varchar(16);not null"   json:"to_currency"`
	Rate         string     `gorm:"column:rate;type:numeric(30,12);not null"       json:"rate"`
	ValidFrom    time.Time  `gorm:"column:valid_from;not null"                     json:"valid_from"`
	ValidTo      *time.Time `gorm:"column:valid_to"                                json:"valid_to,omitempty"`
	Source       *string    `gorm:"column:source;type:varchar(64)"                 json:"source,omitempty"`
	CreatedAt    *time.Time `gorm:"column:created_at"                              json:"created_at,omitempty"`
}

func (Rate) TableName() string { return "WalletRates" }

type RateEntity struct {
	*sql.Entity[Rate] `inject:""`
}

func (e *RateEntity) OnRegister() {
	e.Hydrate(
		"WalletRates",
		[]string{"from_currency", "to_currency", "source"},
		nil,
		nil,
		nil,
		nil,
		nil,
		"valid_from desc",
	)
}
