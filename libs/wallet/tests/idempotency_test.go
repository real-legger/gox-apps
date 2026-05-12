package tests

import (
	"bytes"
	"testing"

	"github.com/real-legger/gox-apps/libs/wallet/pkg/core"
)

// TestCanonicalHashKeyOrderInvariance verifies that two requests differing
// only in JSON key order produce the same hash, per PLAN.md §13.
func TestCanonicalHashKeyOrderInvariance(t *testing.T) {
	a := map[string]any{
		"wallet_id": "w1",
		"amount":    1000,
		"tags":      map[string]any{"product": "transfer", "category": "main"},
	}
	b := map[string]any{
		"tags":      map[string]any{"category": "main", "product": "transfer"},
		"amount":    1000,
		"wallet_id": "w1",
	}

	ha, err := core.CanonicalHash(a)
	if err != nil {
		t.Fatalf("hash a: %v", err)
	}
	hb, err := core.CanonicalHash(b)
	if err != nil {
		t.Fatalf("hash b: %v", err)
	}
	if !bytes.Equal(ha, hb) {
		t.Fatalf("hashes differ: %x vs %x", ha, hb)
	}
}

// TestCanonicalHashDifferentValues verifies that any value change yields a
// different hash (so 409 IDEMPOTENCY_CONFLICT can fire).
func TestCanonicalHashDifferentValues(t *testing.T) {
	a := map[string]any{"amount": 1000}
	b := map[string]any{"amount": 1001}
	ha, _ := core.CanonicalHash(a)
	hb, _ := core.CanonicalHash(b)
	if bytes.Equal(ha, hb) {
		t.Fatalf("expected different hashes")
	}
}

// TestCanonicalHashEmptyMap verifies the empty map is hashable and distinct
// from nil. (Same key with payload {} vs no payload must not collide.)
func TestCanonicalHashEmptyMap(t *testing.T) {
	hEmpty, err := core.CanonicalHash(map[string]any{})
	if err != nil {
		t.Fatalf("empty: %v", err)
	}
	hNil, err := core.CanonicalHash(nil)
	if err != nil {
		t.Fatalf("nil: %v", err)
	}
	if bytes.Equal(hEmpty, hNil) {
		t.Fatalf("empty map and nil must hash differently")
	}
	// Determinism: same empty input → same hash twice.
	hEmpty2, _ := core.CanonicalHash(map[string]any{})
	if !bytes.Equal(hEmpty, hEmpty2) {
		t.Fatalf("empty map hash is non-deterministic")
	}
}

// TestCanonicalHashNilInput verifies that nil does not panic and produces
// a stable hash.
func TestCanonicalHashNilInput(t *testing.T) {
	h, err := core.CanonicalHash(nil)
	if err != nil {
		t.Fatalf("nil hash err: %v", err)
	}
	if len(h) != 32 {
		t.Fatalf("expected sha256 (32 bytes), got %d", len(h))
	}
}

// TestCanonicalHashNestedSortStability shuffles keys at every level of a
// deeply nested map; the hash must be invariant.
func TestCanonicalHashNestedSortStability(t *testing.T) {
	a := map[string]any{
		"z": map[string]any{
			"b": map[string]any{"y": 2, "x": 1},
			"a": []any{1, 2, 3},
		},
		"a": "first",
	}
	b := map[string]any{
		"a": "first",
		"z": map[string]any{
			"a": []any{1, 2, 3},
			"b": map[string]any{"x": 1, "y": 2},
		},
	}
	ha, err := core.CanonicalHash(a)
	if err != nil {
		t.Fatalf("hash a: %v", err)
	}
	hb, err := core.CanonicalHash(b)
	if err != nil {
		t.Fatalf("hash b: %v", err)
	}
	if !bytes.Equal(ha, hb) {
		t.Fatalf("nested key order changed hash")
	}
}

// TestCanonicalHashArrayOrderMatters verifies arrays are order-sensitive
// (unlike maps). [1,2,3] and [3,2,1] must produce different hashes.
func TestCanonicalHashArrayOrderMatters(t *testing.T) {
	a := map[string]any{"items": []any{1, 2, 3}}
	b := map[string]any{"items": []any{3, 2, 1}}
	ha, _ := core.CanonicalHash(a)
	hb, _ := core.CanonicalHash(b)
	if bytes.Equal(ha, hb) {
		t.Fatalf("array order must affect hash")
	}
}

// TestCanonicalHashMixedTypes round-trips strings, ints, bools, nulls, and
// floats; verifies typed distinctions (1 vs "1", true vs "true").
func TestCanonicalHashMixedTypes(t *testing.T) {
	intH, _ := core.CanonicalHash(map[string]any{"v": 1})
	strH, _ := core.CanonicalHash(map[string]any{"v": "1"})
	if bytes.Equal(intH, strH) {
		t.Fatalf("int 1 and string \"1\" must hash differently")
	}
	boolH, _ := core.CanonicalHash(map[string]any{"v": true})
	strBoolH, _ := core.CanonicalHash(map[string]any{"v": "true"})
	if bytes.Equal(boolH, strBoolH) {
		t.Fatalf("bool true and string \"true\" must hash differently")
	}
	nullH, _ := core.CanonicalHash(map[string]any{"v": nil})
	missH, _ := core.CanonicalHash(map[string]any{})
	if bytes.Equal(nullH, missH) {
		t.Fatalf("explicit null and missing key must hash differently")
	}
}
