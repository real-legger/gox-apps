package core

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// ValidateAccountClass enforces the enum from the DDL.
func ValidateAccountClass(class string) *LedgerError {
	switch AccountClass(class) {
	case ClassPrepaid, ClassLiability, ClassAsset, ClassRevenue, ClassExpense:
		return nil
	default:
		return ErrBadRequest(ErrInvalidRequest, "invalid account class", map[string]any{"class": class})
	}
}

// ValidateTagsAgainstVocabulary checks each tag against tag_definitions:
// the key must exist, applies_to must include the given context, and if
// allowed_values is set the value must be one of them.
func ValidateTagsAgainstVocabulary(db *gorm.DB, tags Tags, appliesTo string) *LedgerError {
	if len(tags) == 0 {
		return nil
	}
	keys := make([]string, 0, len(tags))
	for k := range tags {
		keys = append(keys, k)
	}
	type defRow struct {
		Key           string `gorm:"column:key"`
		AppliesTo     string `gorm:"column:applies_to"`
		AllowedValues string `gorm:"column:allowed_values"` // text[] serialized
	}
	var rows []defRow
	if err := db.Table(`"WalletTagDefinitions"`).
		Select(`"key", "applies_to", array_to_string("allowed_values", ',') AS "allowed_values"`).
		Where(`"key" IN ?`, keys).
		Scan(&rows).Error; err != nil {
		return ErrInternalf("tag definitions lookup failed: " + err.Error())
	}
	defs := make(map[string]defRow, len(rows))
	for _, r := range rows {
		defs[r.Key] = r
	}
	for k, v := range tags {
		def, ok := defs[k]
		if !ok {
			return ErrBadRequest(ErrInvalidTag, "tag key not in vocabulary", map[string]any{"key": k})
		}
		if def.AppliesTo != "both" && def.AppliesTo != appliesTo {
			return ErrBadRequest(ErrInvalidTag, "tag does not apply to this context", map[string]any{
				"key": k, "applies_to": def.AppliesTo, "context": appliesTo,
			})
		}
		if def.AllowedValues != "" {
			allowed := SplitAllowed(def.AllowedValues)
			if !Contains(allowed, v) {
				return ErrBadRequest(ErrInvalidTag, "tag value not in allowed_values", map[string]any{
					"key": k, "value": v,
				})
			}
		}
	}
	return nil
}

func SplitAllowed(s string) []string {
	out := []string{}
	cur := ""
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			out = append(out, cur)
			cur = ""
			continue
		}
		cur += string(s[i])
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

func Contains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}

// ParseTags decodes a JSONB blob (raw bytes) into a Tags map. Empty / nil → empty map.
func ParseTags(b []byte) Tags {
	if len(b) == 0 {
		return Tags{}
	}
	out := Tags{}
	// JSONB may arrive as map[string]any (mixed types). Decode loosely.
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		return Tags{}
	}
	for k, v := range raw {
		if s, ok := v.(string); ok {
			out[k] = s
		}
	}
	return out
}

// JsonOrEmpty marshals a Tags map (or nil) to a JSON string suitable for JSONB
// insertion. Returns "{}" when the input is nil/empty.
func JsonOrEmpty(t Tags) string {
	if len(t) == 0 {
		return "{}"
	}
	b, err := json.Marshal(t)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// ToTime extracts a time.Time from gorm's untyped any column. Postgres timestamptz
// is typically returned as time.Time; sqlite/strings can also appear.
func ToTime(v any) (time.Time, bool) {
	switch t := v.(type) {
	case time.Time:
		return t, true
	case *time.Time:
		if t == nil {
			return time.Time{}, false
		}
		return *t, true
	case string:
		// Try common layouts.
		layouts := []string{
			time.RFC3339Nano,
			time.RFC3339,
			"2006-01-02 15:04:05.999999-07",
			"2006-01-02 15:04:05.999999",
			"2006-01-02 15:04:05",
		}
		for _, layout := range layouts {
			if parsed, err := time.Parse(layout, t); err == nil {
				return parsed, true
			}
		}
	}
	return time.Time{}, false
}
