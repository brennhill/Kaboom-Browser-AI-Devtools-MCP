// wire_enhanced_action_test.go — Round-trip marshal/unmarshal for WireEnhancedAction.
package types

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestTypesPackageRespectsTenFileBoundary(t *testing.T) {
	t.Parallel()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read types package: %v", err)
	}
	files := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			files++
		}
	}
	if files > 10 {
		t.Fatalf("internal/types contains %d files; maximum is 10", files)
	}
}

func TestWireEnhancedAction_MarshalRoundTrip(t *testing.T) {
	t.Parallel()

	// DurationMs: 3500 tests wire-level serialization fidelity;
	// in MVP, the extension always sends 0 (removal tracking not yet implemented).
	action := WireEnhancedAction{
		Type:           "transient",
		Timestamp:      1700000000000,
		URL:            "https://example.com",
		Value:          "Item saved successfully",
		Classification: "toast",
		DurationMs:     3500,
		Role:           "status",
	}

	data, err := json.Marshal(action)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded WireEnhancedAction
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.Classification != "toast" {
		t.Errorf("Classification = %q, want %q", decoded.Classification, "toast")
	}
	if decoded.DurationMs != 3500 {
		t.Errorf("DurationMs = %d, want 3500", decoded.DurationMs)
	}
	if decoded.Role != "status" {
		t.Errorf("Role = %q, want %q", decoded.Role, "status")
	}
	if decoded.Type != "transient" {
		t.Errorf("Type = %q, want %q", decoded.Type, "transient")
	}
	if decoded.Value != "Item saved successfully" {
		t.Errorf("Value = %q, want %q", decoded.Value, "Item saved successfully")
	}
}

func TestWireEnhancedAction_OmitemptyFields(t *testing.T) {
	t.Parallel()

	action := WireEnhancedAction{
		Type:      "click",
		Timestamp: 1700000000000,
	}

	data, err := json.Marshal(action)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	raw := string(data)
	for _, field := range []string{"classification", "duration_ms", "role"} {
		if strings.Contains(raw, field) {
			t.Errorf("JSON contains %q when empty, want omitted", field)
		}
	}
}
