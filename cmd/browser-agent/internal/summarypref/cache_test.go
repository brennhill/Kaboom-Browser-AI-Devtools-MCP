// cache_test.go — Verifies session summary preference caching and argument injection.

package summarypref

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statediag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statefault"
)

func TestCacheLoadsOnceUntilInvalidated(t *testing.T) {
	loads := 0
	cache := New(func() ([]byte, error) {
		loads++
		return []byte(`{"summary":true}`), nil
	}, nil)

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
		{name: "missing state", load: func() ([]byte, error) { return nil, statediag.ErrAbsent }},
		{name: "load failure", load: func() ([]byte, error) { return nil, errors.New("unavailable") }},
		{name: "empty value", load: func() ([]byte, error) { return nil, nil }},
		{name: "invalid JSON", load: func() ([]byte, error) { return []byte(`{`), nil }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if New(tt.load, nil).Enabled() {
				t.Fatal("expected summary preference to be disabled")
			}
		})
	}
}

func TestNilCacheIsDisabledAndPreservesArguments(t *testing.T) {
	var cache *Cache
	input := json.RawMessage(`{"what":"errors"}`)

	if cache.Enabled() {
		t.Fatal("nil cache must be disabled")
	}
	cache.Invalidate()
	if got := cache.Inject(input); string(got) != string(input) {
		t.Fatalf("nil cache changed arguments: got %q, want %q", got, input)
	}
}

func TestCacheInjectPreservesExplicitResponseMode(t *testing.T) {
	cache := New(func() ([]byte, error) { return []byte(`{"summary":true}`), nil }, nil)

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
	cache := New(func() ([]byte, error) { return []byte(`{"summary":true}`), nil }, nil)

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

func TestCacheCanonicalFaultFallbackAndRecoveryDoesNotLeakState(t *testing.T) {
	t.Parallel()
	const private = "private-response-preference"
	valid := []byte(`{"summary":true,"private":"private-response-preference"}`)

	for _, tt := range []struct {
		kind statefault.Kind
	}{
		{kind: statefault.Read},
		{kind: statefault.Cancellation},
		{kind: statefault.Corruption},
		{kind: statefault.PartialWrite},
	} {
		t.Run(string(tt.kind), func(t *testing.T) {
			scenario := statefault.New(tt.kind, private)
			faulted := true
			load := func() ([]byte, error) {
				if !faulted {
					return valid, nil
				}
				if tt.kind == statefault.Corruption || tt.kind == statefault.PartialWrite {
					return scenario.Payload(valid), nil
				}
				return nil, scenario.Error()
			}
			collector := statediag.NewCollector()
			cache := New(load, collector)
			if cache.Enabled() {
				t.Fatal("recovered preference must use disabled fallback")
			}
			got := collector.Snapshot()
			if len(got) != 1 || got[0].Name != "response_mode_state" || got[0].Fix == "" {
				t.Fatalf("diagnostics = %#v, want actionable response-mode warning", got)
			}
			if strings.Contains(got[0].Detail, private) {
				t.Fatalf("diagnostic leaked persisted state: %#v", got[0])
			}

			faulted = false
			cache.Invalidate()
			if !cache.Enabled() {
				t.Fatal("valid preference did not recover after canonical fault cleared")
			}
			recovered := collector.Snapshot()
			if len(recovered) != 1 || recovered[0].Lifecycle != statediag.LifecycleRecovered {
				t.Fatalf("recovered diagnostics = %#v", recovered)
			}
		})
	}
}

func TestCacheReloadsPersistedPreferenceAfterRestartGeneration(t *testing.T) {
	loads := 0
	load := func() ([]byte, error) {
		loads++
		return []byte(`{"summary":true}`), nil
	}
	if !New(load, nil).Enabled() {
		t.Fatal("initial generation did not load preference")
	}
	if statefault.New(statefault.Restart, "private").NextGeneration(1) != 2 {
		t.Fatal("restart fixture did not advance generation")
	}
	if !New(load, nil).Enabled() || loads != 2 {
		t.Fatalf("restart did not reload persisted preference; loads=%d", loads)
	}
}

func TestCacheDoesNotReportNormallyAbsentState(t *testing.T) {
	t.Parallel()

	collector := statediag.NewCollector()
	if New(func() ([]byte, error) { return nil, statediag.ErrAbsent }, collector).Enabled() {
		t.Fatal("absent preference must use disabled fallback")
	}
	if got := collector.Snapshot(); len(got) != 0 {
		t.Fatalf("absent state diagnostics = %#v, want none", got)
	}
}
