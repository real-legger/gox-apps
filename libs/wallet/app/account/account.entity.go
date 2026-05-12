package account

import (
	"encoding/json"
	"time"

	"github.com/awesome-goose/goose/modules/sql"
)

// Account mirrors the WalletAccounts table. Balances are managed by the
// core service; the CRUD endpoints expose only read + status changes.
type Account struct {
	Id               int64           `gorm:"primaryKey;column:id;autoIncrement"             json:"id"`
	WalletId         string          `gorm:"column:wallet_id;type:varchar(64);not null"     json:"wallet_id"`
	Currency         string          `gorm:"column:currency;type:varchar(16);not null"      json:"currency"`
	Class            string          `gorm:"column:class;type:varchar(32);not null"         json:"class"`
	Tags             json.RawMessage `gorm:"column:tags;type:jsonb"                         json:"tags,omitempty"`
	Status           string          `gorm:"column:status;type:varchar(32);not null"        json:"status"`
	Balance          int64           `gorm:"column:balance;not null"                        json:"balance"`
	AvailableBalance int64           `gorm:"column:available_balance;not null"              json:"available_balance"`
	Metadata         json.RawMessage `gorm:"column:metadata;type:jsonb"                     json:"metadata,omitempty"`
	CreatedAt        *time.Time      `gorm:"column:created_at"                              json:"created_at,omitempty"`
}

func (Account) TableName() string { return "WalletAccounts" }

type AccountEntity struct {
	*sql.Entity[Account] `inject:""`
}

func (e *AccountEntity) OnRegister() {
	e.Hydrate(
		"WalletAccounts",
		[]string{"wallet_id", "currency", "class"},
		nil,
		nil,
		nil,
		nil,
		nil,
		"id desc",
	)
}
