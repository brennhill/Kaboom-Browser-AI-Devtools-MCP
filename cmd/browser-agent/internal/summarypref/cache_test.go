// cache_test.go — Verifies session summary preference caching and argument injection.

package summarypref

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestCacheLoadsOnceUntilInvalidated(t *testing.T) {
	loads := 0
	cache := New(func() ([]byte, error) {
		loads++
		return []byte(`{"summary":true}`), nil
	})

	if !cache.Enabled() || !cache.Enabled() {
		t.Fatal("expected the loaded summary preference to remain enabled")
	}
	if loads != 1 {
		t.Fatalf("expected one load before invalidation, got %d", loads)
	}

	cache.Invalidate()
	if !cache.Enabled() {
		t.Fatal("expected preference to reload after invalidation")
	}
	if loads != 2 {
		t.Fatalf("expected two loads after invalidation, got %d", loads)
	}
}

func TestCacheTreatsMissingInvalidAndFailedValuesAsDisabled(t *testing.T) {
	tests := []struct {
		name string
		load func() ([]byte, error)
	}{
		{name: "missing loader"},
		{name: "load failure", load: func() ([]byte, error) { return nil, errors.New("unavailable") }},
		{name: "empty value", load: func() ([]byte, error) { return nil, nil }},
		{name: "invalid JSON", load: func() ([]byte, error) { return []byte(`{`), nil }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if New(tt.load).Enabled() {
				t.Fatal("expected summary preference to be disabled")
			}
		})
	}
}

func TestCacheInjectPreservesExplicitResponseMode(t *testing.T) {
	cache := New(func() ([]byte, error) { return []byte(`{"summary":true}`), nil })

	for _, input := range []json.RawMessage{
		json.RawMessage(`{"summary":false}`),
		json.RawMessage(`{"full":true}`),
		json.RawMessage(`not-json`),
	} {
		if got := cache.Inject(input); string(got) != string(input) {
			t.Fatalf("expected explicit or invalid input %q to remain unchanged, got %q", input, got)
		}
	}
}

func TestCacheInjectAddsSummaryToUnsetArguments(t *testing.T) {
	cache := New(func() ([]byte, error) { return []byte(`{"summary":true}`), nil })

	for _, input := range []json.RawMessage{nil, json.RawMessage(`null`), json.RawMessage(`{}`)} {
		var got map[string]json.RawMessage
		if err := json.Unmarshal(cache.Inject(input), &got); err != nil {
			t.Fatalf("unmarshal injected arguments: %v", err)
		}
		if string(got["summary"]) != "true" {
			t.Fatalf("expected summary=true for %q, got %s", input, got["summary"])
		}
	}
}
