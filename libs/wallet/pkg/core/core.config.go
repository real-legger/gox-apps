package core

import (
	"encoding/json"
	"strconv"

	"gorm.io/gorm"
)

// FeePayload describes an accumulating fee or tax outcome.
//
//	{ "account_id": 7, "amount": 10 }
//	or
//	{ "account_id": 7, "percent": "0.01" }   ← floor(percent * baseAmount)
type FeePayload struct {
	AccountID int64  `json:"account_id"`
	Amount    *int64 `json:"amount,omitempty"`
	Percent   string `json:"percent,omitempty"`
}

// LedgerPostingPayload describes a single core posting (lookup outcome).
//
//	{ "account_id": 99, "type": "DEBIT" }
type LedgerPostingPayload struct {
	AccountID int64     `json:"account_id"`
	Type      EntryType `json:"type"`
}

// FxTargetPayload identifies the FX intermediate accounts.
//
//	{ "from_account_id": 10, "to_account_id": 11 }
type FxTargetPayload struct {
	FromAccountID int64 `json:"from_account_id"`
	ToAccountID   int64 `json:"to_account_id"`
}

// ResolvedFee is the materialized outcome of a fee or tax rule against a
// specific base amount.
type ResolvedFee struct {
	Category  string // "fee" | "tax"
	AccountID int64
	Amount    int64
}

// ResolveAccumulatingFees walks all active accumulating rules in `category`
// whose match predicate is satisfied by `tags`, ordered by priority desc.
// It returns the materialized {account, amount} list. Compounding (i.e.
// later fees are applied on top of base + prior fees) is supported via the
// `compound` flag.
func ResolveAccumulatingFees(db *gorm.DB, category string, tags Tags, baseAmount int64, compound bool) ([]ResolvedFee, *LedgerError) {
	rules, err := loadActiveRules(db, ConfigKindAccumulating, category, tags)
	if err != nil {
		return nil, err
	}
	out := make([]ResolvedFee, 0, len(rules))
	running := baseAmount
	for _, r := range rules {
		var p FeePayload
		if jerr := json.Unmarshal(r.Payload, &p); jerr != nil {
			return nil, ErrInternalf("invalid fee payload on rule " + strconv.FormatInt(r.ID, 10))
		}
		amount := int64(0)
		switch {
		case p.Amount != nil:
			amount = *p.Amount
		case p.Percent != "":
			amount = applyPercent(running, p.Percent)
		default:
			return nil, ErrInternalf("fee rule has neither amount nor percent")
		}
		if amount == 0 {
			continue
		}
		out = append(out, ResolvedFee{Category: category, AccountID: p.AccountID, Amount: amount})
		if compound {
			running += amount
		}
	}
	return out, nil
}

// ResolveLookup returns the highest-priority matching `lookup` rule's payload,
// or a CONFIG_RULE_MISSING error if none match.
func ResolveLookup(db *gorm.DB, category string, tags Tags) (json.RawMessage, *LedgerError) {
	rules, err := loadActiveRules(db, ConfigKindLookup, category, tags)
	if err != nil {
		return nil, err
	}
	if len(rules) == 0 {
		return nil, ErrNotFoundf(ErrConfigRuleMissing, "no lookup rule matched", map[string]any{
			"category": category,
		})
	}
	return rules[0].Payload, nil
}

// ResolveLedgerPostings returns all `lookup`-kind core postings whose match
// is satisfied (multiple postings may apply). Empty list when none match.
func ResolveLedgerPostings(db *gorm.DB, tags Tags) ([]LedgerPostingPayload, *LedgerError) {
	rules, err := loadAllMatching(db, ConfigKindLookup, "core_posting", tags)
	if err != nil {
		return nil, err
	}
	out := make([]LedgerPostingPayload, 0, len(rules))
	for _, r := range rules {
		var p LedgerPostingPayload
		if jerr := json.Unmarshal(r.Payload, &p); jerr != nil {
			return nil, ErrInternalf("invalid core_posting payload on rule " + strconv.FormatInt(r.ID, 10))
		}
		out = append(out, p)
	}
	return out, nil
}

// loadActiveRules pulls candidate rules from DB and filters in-app to only
// those whose `match` predicate is fully satisfied by `tags`.
// Returned slice is sorted by priority desc, created_at asc.
func loadActiveRules(db *gorm.DB, kind ConfigRuleKind, category string, tags Tags) ([]ConfigRule, *LedgerError) {
	return loadAllMatching(db, kind, category, tags)
}

func loadAllMatching(db *gorm.DB, kind ConfigRuleKind, category string, tags Tags) ([]ConfigRule, *LedgerError) {
	var rows []struct {
		ID        int64  `gorm:"column:id"`
		Kind      string `gorm:"column:kind"`
		Category  string `gorm:"column:category"`
		Match     []byte `gorm:"column:match"`
		Priority  int    `gorm:"column:priority"`
		Payload   []byte `gorm:"column:payload"`
		Active    bool   `gorm:"column:active"`
		CreatedAt any    `gorm:"column:created_at"`
	}
	err := db.Table(`"WalletConfigRules"`).
		Where(`"kind" = ? AND "category" = ? AND "active" = TRUE`, string(kind), category).
		Order(`"priority" DESC, "created_at" ASC`).
		Scan(&rows).Error
	if err != nil {
		return nil, ErrInternalf("config rule lookup failed: " + err.Error())
	}

	out := make([]ConfigRule, 0, len(rows))
	for _, r := range rows {
		match := parseMatch(r.Match)
		if !matches(match, tags) {
			continue
		}
		c := ConfigRule{
			ID:       r.ID,
			Kind:     ConfigRuleKind(r.Kind),
			Category: r.Category,
			Match:    match,
			Priority: r.Priority,
			Payload:  r.Payload,
			Active:   r.Active,
		}
		if t, ok := ToTime(r.CreatedAt); ok {
			c.CreatedAt = t
		}
		out = append(out, c)
	}
	return out, nil
}

// parseMatch decodes the JSONB `match` predicate into Tags.
func parseMatch(b []byte) Tags {
	if len(b) == 0 {
		return Tags{}
	}
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		return Tags{}
	}
	out := Tags{}
	for k, v := range raw {
		if s, ok := v.(string); ok {
			out[k] = s
		}
	}
	return out
}

// matches returns true iff every (k, v) in `predicate` is present in `tags`.
func matches(predicate, tags Tags) bool {
	for k, v := range predicate {
		actual, ok := tags[k]
		if !ok || actual != v {
			return false
		}
	}
	return true
}

// applyPercent computes floor(base * percent) where percent is a decimal
// string like "0.01" (1%). Implementation uses naive parsing; sufficient
// for the expected fee precision of 4 decimals or fewer.
func applyPercent(base int64, percent string) int64 {
	// Find decimal point; treat as fixed-point with up to 8 decimals.
	const scale = 100_000_000 // 1e8
	whole, frac := splitDot(percent)
	w, _ := parseInt64(whole)
	f, fracLen := parseInt64(frac)
	for fracLen < 8 {
		f *= 10
		fracLen++
	}
	for fracLen > 8 {
		f /= 10
		fracLen--
	}
	micro := w*scale + f
	return base * micro / scale
}

func splitDot(s string) (string, string) {
	for i, c := range s {
		if c == '.' {
			return s[:i], s[i+1:]
		}
	}
	return s, ""
}

func parseInt64(s string) (int64, int) {
	var n int64
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return n, i
		}
		n = n*10 + int64(s[i]-'0')
	}
	return n, len(s)
}

