package tests

import (
	"testing"
	"time"

	"github.com/real-legger/gox-apps/libs/wallet/pkg/core"
)

func TestEntryTypeFlip(t *testing.T) {
	if got := core.TypeDebit.Flip(); got != core.TypeCredit {
		t.Fatalf("DEBIT.Flip() = %q, want CREDIT", got)
	}
	if got := core.TypeCredit.Flip(); got != core.TypeDebit {
		t.Fatalf("CREDIT.Flip() = %q, want DEBIT", got)
	}
}

func TestTagsMergeNilSafe(t *testing.T) {
	var nilTags core.Tags
	b := core.Tags{"k": "v"}
	out := nilTags.Merge(b)
	if out["k"] != "v" || len(out) != 1 {
		t.Fatalf("nil.Merge(b) = %v, want {k:v}", out)
	}

	a := core.Tags{"k": "v"}
	out = a.Merge(nil)
	if out["k"] != "v" || len(out) != 1 {
		t.Fatalf("a.Merge(nil) = %v, want {k:v}", out)
	}

	// b wins on conflict per the Tags.Merge comment.
	a = core.Tags{"k": "a-val", "only_a": "1"}
	b = core.Tags{"k": "b-val", "only_b": "2"}
	out = a.Merge(b)
	if out["k"] != "b-val" {
		t.Fatalf("Merge collision: got k=%q, want b-val", out["k"])
	}
	if out["only_a"] != "1" || out["only_b"] != "2" {
		t.Fatalf("Merge dropped unique keys: %v", out)
	}

	// Merge must not mutate either input.
	if a["k"] != "a-val" {
		t.Fatalf("Merge mutated receiver a")
	}
	if b["k"] != "b-val" {
		t.Fatalf("Merge mutated arg b")
	}
}

func TestValidateAccountClass(t *testing.T) {
	for _, valid := range []string{"prepaid", "liability", "asset", "revenue", "expense"} {
		if err := core.ValidateAccountClass(valid); err != nil {
			t.Fatalf("class %q should be valid, got %v", valid, err)
		}
	}
	for _, invalid := range []string{"", "Prepaid", "PREPAID", "checking", "savings", " asset"} {
		err := core.ValidateAccountClass(invalid)
		if err == nil {
			t.Fatalf("class %q should be invalid", invalid)
		}
		if err.Code != core.ErrInvalidRequest {
			t.Fatalf("class %q: got code %q, want %q", invalid, err.Code, core.ErrInvalidRequest)
		}
	}
}

func TestParseTagsEmptyAndMalformed(t *testing.T) {
	if got := core.ParseTags(nil); len(got) != 0 {
		t.Fatalf("nil → %v, want empty", got)
	}
	if got := core.ParseTags([]byte{}); len(got) != 0 {
		t.Fatalf("empty bytes → %v, want empty", got)
	}
	if got := core.ParseTags([]byte("not json")); len(got) != 0 {
		t.Fatalf("malformed → %v, want empty (no panic)", got)
	}
	// Valid JSONB blob with mixed types — only string values survive.
	got := core.ParseTags([]byte(`{"product":"transfer","amount":1000,"flag":true,"sub":{"k":"v"}}`))
	if got["product"] != "transfer" {
		t.Fatalf("string tag dropped: %v", got)
	}
	if _, ok := got["amount"]; ok {
		t.Fatalf("non-string tag should be skipped: %v", got)
	}
	if _, ok := got["flag"]; ok {
		t.Fatalf("bool tag should be skipped: %v", got)
	}
	if _, ok := got["sub"]; ok {
		t.Fatalf("nested object tag should be skipped: %v", got)
	}
}

func TestJsonOrEmpty(t *testing.T) {
	if got := core.JsonOrEmpty(nil); got != "{}" {
		t.Fatalf("nil → %q, want {}", got)
	}
	if got := core.JsonOrEmpty(core.Tags{}); got != "{}" {
		t.Fatalf("empty → %q, want {}", got)
	}
	// Single key → exact JSON.
	if got := core.JsonOrEmpty(core.Tags{"k": "v"}); got != `{"k":"v"}` {
		t.Fatalf("single-key → %q, want {\"k\":\"v\"}", got)
	}
	// Multiple keys: parse-and-compare via ParseTags round-trip (Go map iteration is non-deterministic).
	round := core.ParseTags([]byte(core.JsonOrEmpty(core.Tags{"a": "1", "b": "2"})))
	if round["a"] != "1" || round["b"] != "2" || len(round) != 2 {
		t.Fatalf("JsonOrEmpty round-trip lost data: %v", round)
	}
}

func TestToTime(t *testing.T) {
	ref := time.Date(2026, 5, 12, 13, 30, 0, 0, time.UTC)

	if got, ok := core.ToTime(ref); !ok || !got.Equal(ref) {
		t.Fatalf("time.Time direct: got=%v ok=%v", got, ok)
	}
	if got, ok := core.ToTime(&ref); !ok || !got.Equal(ref) {
		t.Fatalf("*time.Time non-nil: got=%v ok=%v", got, ok)
	}
	var nilPtr *time.Time
	if _, ok := core.ToTime(nilPtr); ok {
		t.Fatalf("nil *time.Time should yield ok=false")
	}

	// String layouts that ToTime advertises.
	for _, s := range []string{
		"2026-05-12T13:30:00Z",
		"2026-05-12T13:30:00.000Z",
		"2026-05-12 13:30:00",
	} {
		if _, ok := core.ToTime(s); !ok {
			t.Fatalf("layout %q should parse", s)
		}
	}

	// Unparseable string and unrelated type.
	if _, ok := core.ToTime("not a date"); ok {
		t.Fatalf("garbage string should yield ok=false")
	}
	if _, ok := core.ToTime(12345); ok {
		t.Fatalf("int should yield ok=false")
	}
}
