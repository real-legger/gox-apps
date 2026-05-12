package core

import (
	"encoding/json"
	"time"
)

// ── Status & Type ─────────────────────────────────────────────────────────────

type TxStatus string

const (
	StatusLien     TxStatus = "LIEN"
	StatusExecuted TxStatus = "EXECUTED"
	StatusReversed TxStatus = "REVERSED"
)

type EntryType string

const (
	TypeDebit  EntryType = "DEBIT"
	TypeCredit EntryType = "CREDIT"
)

func (t EntryType) Flip() EntryType {
	if t == TypeDebit {
		return TypeCredit
	}
	return TypeDebit
}

// ── Account class & status ────────────────────────────────────────────────────

type AccountClass string

const (
	ClassPrepaid   AccountClass = "prepaid"
	ClassLiability AccountClass = "liability"
	ClassAsset     AccountClass = "asset"
	ClassRevenue   AccountClass = "revenue"
	ClassExpense   AccountClass = "expense"
)

type AccountStatus string

const (
	AccountActive AccountStatus = "active"
	AccountFrozen AccountStatus = "frozen"
	AccountClosed AccountStatus = "closed"
)

// ── Tags ──────────────────────────────────────────────────────────────────────

type Tags map[string]string

// Merge returns a + b with b winning on conflict.
func (a Tags) Merge(b Tags) Tags {
	out := make(Tags, len(a)+len(b))
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}

// ── Domain rows ───────────────────────────────────────────────────────────────

type Currency struct {
	Code     string `json:"code"`
	Exponent int    `json:"exponent"`
	Name     string `json:"name"`
}

type Wallet struct {
	ID        string          `json:"id"`
	TenantID  string          `json:"tenant_id"`
	Name      string          `json:"name"`
	Metadata  json.RawMessage `json:"metadata,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
}

type Account struct {
	ID               int64           `json:"id"`
	WalletID         string          `json:"wallet_id"`
	Currency         string          `json:"currency"`
	Class            AccountClass    `json:"class"`
	Tags             Tags            `json:"tags"`
	Status           AccountStatus   `json:"status"`
	Balance          int64           `json:"balance"`
	AvailableBalance int64           `json:"available_balance"`
	Metadata         json.RawMessage `json:"metadata,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
}

type TransactionRow struct {
	ID               int64     `json:"id"`
	GroupID          string    `json:"group_id"`
	Status           TxStatus  `json:"status"`
	PriorStatus      *TxStatus `json:"prior_status,omitempty"`
	Type             EntryType `json:"type"`
	PrimaryAccountID int64     `json:"primary_account_id"`
	Currency         string    `json:"currency"`
	Tags             Tags      `json:"tags"`
	IdempotencyKey   *string   `json:"idempotency_key,omitempty"`
	FxPairGroupID    *string   `json:"fx_pair_group_id,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	Entries          []Entry   `json:"entries,omitempty"`
}

type Entry struct {
	ID            int64     `json:"id"`
	TransactionID int64     `json:"transaction_id"`
	AccountID     int64     `json:"account_id"`
	Type          EntryType `json:"type"`
	Amount        int64     `json:"amount"`
	CreatedAt     time.Time `json:"created_at"`
}

type TagDefinition struct {
	Key           string   `json:"key"`
	Description   *string  `json:"description,omitempty"`
	AppliesTo     string   `json:"applies_to"` // 'account' | 'transaction' | 'both'
	AllowedValues []string `json:"allowed_values,omitempty"`
}

type ConfigRuleKind string

const (
	ConfigKindLookup       ConfigRuleKind = "lookup"
	ConfigKindAccumulating ConfigRuleKind = "accumulating"
)

type ConfigRule struct {
	ID        int64           `json:"id"`
	Kind      ConfigRuleKind  `json:"kind"`
	Category  string          `json:"category"`
	Match     Tags            `json:"match"`
	Priority  int             `json:"priority"`
	Payload   json.RawMessage `json:"payload"`
	Active    bool            `json:"active"`
	CreatedAt time.Time       `json:"created_at"`
}

type Rate struct {
	ID           int64      `json:"id"`
	FromCurrency string     `json:"from_currency"`
	ToCurrency   string     `json:"to_currency"`
	Rate         string     `json:"rate"` // NUMERIC stored as string for precision
	ValidFrom    time.Time  `json:"valid_from"`
	ValidTo      *time.Time `json:"valid_to,omitempty"`
	Source       *string    `json:"source,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

// EntryInput is used by callers to compose a transaction row.
type EntryInput struct {
	AccountID int64     `json:"account_id"`
	Type      EntryType `json:"type"`
	Amount    int64     `json:"amount"`
}

// RowInput is used by Layer 0 append_transaction_rows.
type RowInput struct {
	GroupID          string       `json:"group_id"`
	Status           TxStatus     `json:"status"`
	PriorStatus      *TxStatus    `json:"prior_status,omitempty"`
	Type             EntryType    `json:"type"`
	PrimaryAccountID int64        `json:"primary_account_id"`
	Currency         string       `json:"currency"`
	Tags             Tags         `json:"tags,omitempty"`
	IdempotencyKey   *string      `json:"idempotency_key,omitempty"`
	FxPairGroupID    *string      `json:"fx_pair_group_id,omitempty"`
	Entries          []EntryInput `json:"entries"`
}

// AppendResult is returned per appended row.
type AppendResult struct {
	Row     TransactionRow `json:"row"`
	Entries []Entry        `json:"entries"`
}

// GroupView returns full row history of a group with computed latest status.
type GroupView struct {
	GroupID      string           `json:"group_id"`
	LatestStatus TxStatus         `json:"latest_status"`
	Rows         []TransactionRow `json:"rows"`
}
