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
