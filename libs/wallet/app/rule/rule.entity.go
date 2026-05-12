package rule

import (
	"encoding/json"
	"time"

	"github.com/awesome-goose/goose/modules/sql"
)

// Rule mirrors WalletConfigRules.
type Rule struct {
	Id        int64           `gorm:"primaryKey;column:id;autoIncrement"          json:"id"`
	Kind      string          `gorm:"column:kind;type:varchar(16);not null"       json:"kind"`
	Category  string          `gorm:"column:category;type:varchar(64);not null"   json:"category"`
	Match     json.RawMessage `gorm:"column:match;type:jsonb;not null"            json:"match"`
	Priority  int             `gorm:"column:priority;not null;default:0"          json:"priority"`
	Payload   json.RawMessage `gorm:"column:payload;type:jsonb;not null"          json:"payload"`
	Active    bool            `gorm:"column:active;not null;default:true"         json:"active"`
	CreatedAt *time.Time      `gorm:"column:created_at"                           json:"created_at,omitempty"`
}

func (Rule) TableName() string { return "WalletConfigRules" }

type RuleEntity struct {
	*sql.Entity[Rule] `inject:""`
}

func (e *RuleEntity) OnRegister() {
	e.Hydrate(
		"WalletConfigRules",
		[]string{"category", "kind"},
		nil,
		nil,
		nil,
		nil,
		nil,
		"priority desc, created_at asc",
	)
}
