// store_test.go — Defines the canonical capture log-store ownership contract.
package logstore

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

func TestExtensionStoreOwnsRedactionRetentionAndDetachedReads(t *testing.T) {
	store := NewExtension(func(value string) string {
		if value == "secret" {
			return "[REDACTED]"
		}
		return value
	})
	now := time.Unix(100, 0)
	store.AddAt([]types.ExtensionLog{{Message: "secret", Data: json.RawMessage(`{"token":"secret"}`)}}, now)

	entries := store.Entries()
	if len(entries) != 1 || entries[0].Message != "[REDACTED]" || entries[0].Timestamp != now {
		t.Fatalf("stored entry = %#v", entries)
	}
	entries[0].Message = "mutated"
	if store.Entries()[0].Message != "[REDACTED]" {
		t.Fatal("Entries returned an alias of store state")
	}
}

func TestExtensionStoreAppliesSinglePassEviction(t *testing.T) {
	t.Parallel()
	store := NewExtension(nil)
	total := ExtensionCapacity + ExtensionCapacity/2 + 1
	logs := make([]types.ExtensionLog, total)
	for index := range logs {
		logs[index] = types.ExtensionLog{Message: fmt.Sprintf("log-%d", index), Timestamp: time.Unix(int64(index), 0)}
	}
	store.Add(logs)

	entries := store.Entries()
	if len(entries) != ExtensionCapacity || store.Pressure().Dropped != int64(total-ExtensionCapacity) {
		t.Fatalf("extension retention = %d/%#v", len(entries), store.Pressure())
	}
	wantFirst := fmt.Sprintf("log-%d", total-ExtensionCapacity)
	if entries[0].Message != wantFirst || entries[len(entries)-1].Message != fmt.Sprintf("log-%d", total-1) {
		t.Fatalf("retained range = %q..%q, want %q..log-%d", entries[0].Message, entries[len(entries)-1].Message, wantFirst, total-1)
	}
}

func TestExtensionStorePressureRecoversAfterClear(t *testing.T) {
	store := NewExtension(nil)
	store.AddAt(make([]types.ExtensionLog, ExtensionCapacity+2), time.Unix(100, 0))
	if removed := store.Clear(); removed != ExtensionCapacity {
		t.Fatalf("Clear removed %d entries, want %d", removed, ExtensionCapacity)
	}
	stats := store.Pressure()
	if stats.Size != 0 || stats.Dropped != 2 || stats.OldestAge != 0 {
		t.Fatalf("pressure after clear = %#v", stats)
	}
}

func TestExtensionStoreInvalidJSONAndNilRedactorAreLossless(t *testing.T) {
	store := NewExtension(nil)
	store.Add([]types.ExtensionLog{{Message: "unchanged", Data: json.RawMessage(`not valid json`)}})
	entry := store.Entries()[0]
	if entry.Message != "unchanged" || string(entry.Data) != "not valid json" {
		t.Fatalf("nil-redactor entry = %#v", entry)
	}

	redacted := redactData(json.RawMessage(`not valid json`), func(value string) string { return "redacted:" + value })
	if string(redacted) != "redacted:not valid json" {
		t.Fatalf("invalid JSON fallback = %q", redacted)
	}
}

func TestRedactJSONValueTraversesNestedContainers(t *testing.T) {
	input := map[string]any{
		"key":     "secret_value",
		"nested":  map[string]any{"inner": "another_secret"},
		"array":   []any{"item1", "item2"},
		"number":  float64(42),
		"nil_val": nil,
	}
	result := redactJSONValue(input, strings.ToUpper).(map[string]any)
	if result["key"] != "SECRET_VALUE" || result["nested"].(map[string]any)["inner"] != "ANOTHER_SECRET" {
		t.Fatalf("nested redaction = %#v", result)
	}
	array := result["array"].([]any)
	if array[0] != "ITEM1" || array[1] != "ITEM2" || result["number"] != float64(42) || result["nil_val"] != nil {
		t.Fatalf("container redaction = %#v", result)
	}
}

func TestDiagnosticStoreRedactsHTTPPayloads(t *testing.T) {
	store := NewDiagnostic(func(string) string { return "[REDACTED]" })
	store.AddHTTP(types.HTTPDebugEntry{Endpoint: "/test", RequestBody: "secret"})
	entries := store.HTTPEntries()
	for _, entry := range entries {
		if entry.Endpoint == "/test" && entry.RequestBody == "[REDACTED]" {
			return
		}
	}
	t.Fatalf("redacted diagnostic entry not found: %#v", entries)
}
