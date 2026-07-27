package main

import (
	"encoding/json"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/summarypref"
)

func handlerWithSummaryPreference(enabled bool) *ToolHandler {
	value := []byte(`{"summary":false}`)
	if enabled {
		value = []byte(`{"summary":true}`)
	}
	return &ToolHandler{summaryPrefs: summarypref.New(func() ([]byte, error) {
		return value, nil
	})}
}

func TestMaybeInjectSummary_NoPreference(t *testing.T) {
	t.Parallel()

	h := handlerWithSummaryPreference(false)
	args := json.RawMessage(`{"what":"errors","limit":10}`)
	result := h.maybeInjectSummary(args)

	// Should return args unchanged
	var parsed map[string]any
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := parsed["summary"]; ok {
		t.Error("expected no summary key when preference not set")
	}
}

func TestMaybeInjectSummary_PreferenceSet(t *testing.T) {
	t.Parallel()

	h := handlerWithSummaryPreference(true)

	args := json.RawMessage(`{"what":"errors","limit":10}`)
	result := h.maybeInjectSummary(args)

	var parsed map[string]any
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	summary, ok := parsed["summary"]
	if !ok {
		t.Fatal("expected summary key to be injected")
	}
	if summary != true {
		t.Errorf("expected summary=true, got %v", summary)
	}
}

func TestMaybeInjectSummary_ExplicitSummaryFalse(t *testing.T) {
	t.Parallel()

	h := handlerWithSummaryPreference(true)

	args := json.RawMessage(`{"what":"errors","summary":false}`)
	result := h.maybeInjectSummary(args)

	var parsed map[string]any
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// summary key was already present, so it should NOT be overridden
	summary, ok := parsed["summary"]
	if !ok {
		t.Fatal("expected summary key to still be present")
	}
	if summary != false {
		t.Errorf("expected summary=false (explicit override), got %v", summary)
	}
}

func TestMaybeInjectSummary_ExplicitFullTrue(t *testing.T) {
	t.Parallel()

	h := handlerWithSummaryPreference(true)

	args := json.RawMessage(`{"what":"errors","full":true}`)
	result := h.maybeInjectSummary(args)

	var parsed map[string]any
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// "full" is present, so summary should NOT be injected
	if _, ok := parsed["summary"]; ok {
		t.Error("expected no summary key when full=true is present")
	}
}

func TestInvalidateSummaryPref(t *testing.T) {
	t.Parallel()

	loads := 0
	h := &ToolHandler{summaryPrefs: summarypref.New(func() ([]byte, error) {
		loads++
		return []byte(`{"summary":true}`), nil
	})}

	if !h.loadSummaryPref() {
		t.Fatal("expected initial preference load")
	}
	h.invalidateSummaryPref()
	if !h.loadSummaryPref() {
		t.Fatal("expected preference reload after invalidation")
	}
	if loads != 2 {
		t.Fatalf("expected two loads after invalidation, got %d", loads)
	}
}

func TestMaybeInjectSummary_NilArgs(t *testing.T) {
	t.Parallel()

	h := handlerWithSummaryPreference(true)

	// nil args should get summary injected
	result := h.maybeInjectSummary(nil)

	var parsed map[string]any
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := parsed["summary"]; !ok {
		t.Error("expected summary key to be injected into nil args")
	}
}

func TestMaybeInjectSummary_EmptyArgs(t *testing.T) {
	t.Parallel()

	h := handlerWithSummaryPreference(true)

	result := h.maybeInjectSummary(json.RawMessage(`{}`))

	var parsed map[string]any
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	summary, ok := parsed["summary"]
	if !ok {
		t.Fatal("expected summary key to be injected into empty args")
	}
	if summary != true {
		t.Errorf("expected summary=true, got %v", summary)
	}
}
