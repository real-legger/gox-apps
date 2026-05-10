package app

import "github.com/thescaffold/gox-packages/libs/core/events"

const Name = "wallet"

var AllMigrations = Migrations

var DefaultPermissions = map[string][]string{
	"guest":         {"guest:read:apps:wallet:remote:*:self", "guest:read:apps:wallet:dynamic:*:self", "guest:create:apps:wallet:file:*:self", "guest:update:apps:wallet:file:*:self", "guest:read:apps:wallet:file:*:self"},
	"member":        {"member:create:apps:wallet:*:*:workspace", "member:update:apps:wallet:*:*:self", "member:read:apps:wallet:*:*:workspace", "member:delete:apps:wallet:*:*:self"},
	"admin":         {"admin:*:apps:wallet:*:*:workspace"},
	"global-member": {"global-member:*:apps:wallet:*:*:global"},
	"global-admin":  {"global-admin:*:apps:wallet:*:*:global"},
}

// Subscriptions — wallet has no event subscriptions.
var Subscriptions = map[string]events.EventHandler{}

// TODO: i18n — translation path: translations/en/ntx/apps/wallet.yaml
