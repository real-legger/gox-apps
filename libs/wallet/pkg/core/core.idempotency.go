package core

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"sort"
	"time"
)

// CanonicalHash returns SHA-256 of canonical-JSON form of v as defined in PLAN.md §13:
// keys sorted lexically, no whitespace, integers (not strings) for numbers, ISO-8601 timestamps.
//
// We achieve this by JSON-encoding into a generic value, then re-encoding with sorted keys.
func CanonicalHash(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := writeCanonical(&buf, generic); err != nil {
		return nil, err
	}
	sum := sha256.Sum256(buf.Bytes())
	return sum[:], nil
}

func writeCanonical(buf *bytes.Buffer, v any) error {
	switch x := v.(type) {
	case nil:
		buf.WriteString("null")
	case bool:
		if x {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
	case string:
		s, err := json.Marshal(x)
		if err != nil {
			return err
		}
		buf.Write(s)
	case float64:
		// Preserve integer formatting where possible.
		s, err := json.Marshal(x)
		if err != nil {
			return err
		}
		buf.Write(s)
	case []any:
		buf.WriteByte('[')
		for i, item := range x {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := writeCanonical(buf, item); err != nil {
				return err
			}
		}
		buf.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		buf.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			ks, err := json.Marshal(k)
			if err != nil {
				return err
			}
			buf.Write(ks)
			buf.WriteByte(':')
			if err := writeCanonical(buf, x[k]); err != nil {
				return err
			}
		}
		buf.WriteByte('}')
	default:
		s, err := json.Marshal(x)
		if err != nil {
			return err
		}
		buf.Write(s)
	}
	return nil
}

// IdempotencyOutcome reports the result of an idempotency check.
type IdempotencyOutcome struct {
	// Found = true and Conflict = false → cached response replay.
	Found bool
	// Conflict = true → same key, different request hash or wallet.
	Conflict bool
	// Cached holds the prior response if Found and not Conflict.
	Cached     json.RawMessage
	OriginalAt *time.Time
}
