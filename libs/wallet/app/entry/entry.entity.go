package entry

import (
	"time"

	"github.com/awesome-goose/goose/modules/sql"
)

// Entry mirrors the WalletEntries table. Append-only; no Update/Delete.
type Entry struct {
	Id            int64      `gorm:"primaryKey;column:id;autoIncrement"      json:"id"`
	TransactionId int64      `gorm:"column:transaction_id;not null"          json:"transaction_id"`
	AccountId     int64      `gorm:"column:account_id;not null"              json:"account_id"`
	Type          string     `gorm:"column:type;type:varchar(8);not null"    json:"type"`
	Amount        int64      `gorm:"column:amount;not null"                  json:"amount"`
	CreatedAt     *time.Time `gorm:"column:created_at"                       json:"created_at,omitempty"`
}

func (Entry) TableName() string { return "WalletEntries" }

type EntryEntity struct {
	*sql.Entity[Entry] `inject:""`
}

func (e *EntryEntity) OnRegister() {
	e.Hydrate(
		"WalletEntries",
		[]string{"transaction_id", "account_id", "type"},
		nil,
		nil,
		nil,
		nil,
		nil,
		"id desc",
	)
}
