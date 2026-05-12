package wallets

import (
	"encoding/json"
	"time"

	"github.com/awesome-goose/goose/modules/sql"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Wallet mirrors the WalletWallets table. PK is `id` (varchar 64).
type Wallet struct {
	Id        string          `gorm:"primaryKey;column:id;type:varchar(64);not null"          json:"id"`
	TenantId  string          `gorm:"column:tenant_id;type:varchar(64);not null;index"        json:"tenant_id"`
	Name      string          `gorm:"column:name;type:varchar(255);not null"                  json:"name"`
	Metadata  json.RawMessage `gorm:"column:metadata;type:jsonb;default:'{}'::jsonb"          json:"metadata,omitempty"`
	CreatedAt *time.Time      `gorm:"column:created_at;not null;default:now()"                json:"created_at,omitempty"`
}

func (Wallet) TableName() string { return "WalletWallets" }

// BeforeCreate mints a `wlt_<uuid>` id when none was supplied. Mirrors the
// goose BaseEntity convention but with a wallet-specific prefix.
func (w *Wallet) BeforeCreate(tx *gorm.DB) error {
	if w.Id == "" {
		w.Id = "wlt_" + uuid.New().String()
	}
	if w.CreatedAt == nil {
		now := time.Now().UTC()
		w.CreatedAt = &now
	}
	return nil
}

type WalletEntity struct {
	*sql.Entity[Wallet] `inject:""`
}

func (e *WalletEntity) OnRegister() {
	e.Hydrate(
		"WalletWallets",
		[]string{"id", "name", "tenant_id"},
		nil,
		nil,
		nil,
		nil,
		nil,
		"created_at desc",
	)
}
