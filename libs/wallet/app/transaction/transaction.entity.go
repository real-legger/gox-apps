package transaction

import (
	"encoding/json"
	"time"

	"github.com/awesome-goose/goose/modules/sql"
)

// Transaction mirrors the WalletTransactions table. Append-only — there is
// no Create/Update/Delete CRUD; rows are produced by the core service.
type Transaction struct {
	Id               int64           `gorm:"primaryKey;column:id;autoIncrement"             json:"id"`
	GroupId          string          `gorm:"column:group_id;type:uuid;not null"             json:"group_id"`
	Status           string          `gorm:"column:status;type:varchar(16);not null"        json:"status"`
	PriorStatus      *string         `gorm:"column:prior_status;type:varchar(16)"           json:"prior_status,omitempty"`
	Type             string          `gorm:"column:type;type:varchar(8);not null"           json:"type"`
	PrimaryAccountId int64           `gorm:"column:primary_account_id;not null"             json:"primary_account_id"`
	Currency         string          `gorm:"column:currency;type:varchar(16);not null"      json:"currency"`
	Tags             json.RawMessage `gorm:"column:tags;type:jsonb"                         json:"tags,omitempty"`
	IdempotencyKey   *string         `gorm:"column:idempotency_key;type:uuid"               json:"idempotency_key,omitempty"`
	FxPairGroupId    *string         `gorm:"column:fx_pair_group_id;type:uuid"              json:"fx_pair_group_id,omitempty"`
	CreatedAt        *time.Time      `gorm:"column:created_at"                              json:"created_at,omitempty"`
}

func (Transaction) TableName() string { return "WalletTransactions" }

type TransactionEntity struct {
	*sql.Entity[Transaction] `inject:""`
}

func (e *TransactionEntity) OnRegister() {
	e.Hydrate(
		"WalletTransactions",
		[]string{"group_id", "status", "type", "currency"},
		nil,
		nil,
		nil,
		nil,
		nil,
		"created_at desc, id desc",
	)
}
