package app

import (
	"github.com/real-legger/gox-apps/libs/wallet/pkg/core"
	"github.com/thescaffold/gox-packages/libs/core/events"
)

const Name = "wallet"

// Re-exports for host mono-repo consumers.
var AllMigrations = Migrations
var AllSeeders = Seeders

// DefaultPermissions mirrors the convention used by other gox-apps.
var DefaultPermissions = map[string][]string{
	"guest":         {"guest:read:apps:wallet:*:*:self"},
	"member":        {"member:create:apps:wallet:*:*:workspace", "member:update:apps:wallet:*:*:self", "member:read:apps:wallet:*:*:workspace", "member:delete:apps:wallet:*:*:self"},
	"admin":         {"admin:*:apps:wallet:*:*:workspace"},
	"global-member": {"global-member:*:apps:wallet:*:*:global"},
	"global-admin":  {"global-admin:*:apps:wallet:*:*:global"},
}

// Subscriptions — wallet has no event subscriptions yet.
var Subscriptions = map[string]events.EventHandler{}

// Domain type aliases for host consumption.
type (
	Account        = core.Account
	TransactionRow = core.TransactionRow
	Entry          = core.Entry
	GroupView      = core.GroupView
	Tags           = core.Tags
	TxStatus       = core.TxStatus
	EntryType      = core.EntryType
)

// Top-level exports mirroring the convention from gox-apps/libs/statics.
var (
	Entities        = []any{}
	Messages        = map[string]any{}
	UnsafeEventList = []string{}
	Paths           = []string{"translations/en/ntx/apps/wallet.yaml"}
	Jobs            = []any{}
	Crons           = []any{}
)
