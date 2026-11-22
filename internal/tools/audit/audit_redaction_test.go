// audit_redaction_test.go — Tests audit parameter redaction and redaction events.
// Docs: docs/features/feature/enterprise-audit/index.md

package audit

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// ============================================
// Test: Parameter redaction - bearer tokens
// ============================================

func TestAuditTrail_RedactBearerToken(t *testing.T) {
	t.Parallel()
	trail := NewAuditTrail(AuditConfig{
		MaxEntries:   1000,
		Enabled:      true,
		RedactParams: true,
	})

	params := `{"authorization": "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.test"}`
	entry := AuditEntry{
		AuditSessionID: "s1",
		ClientID:       "claude-code",
		ToolName:       "observe",
		Parameters:     params,
		Success:        true,
	}

	trail.Record(entry)

	results := trail.Query(AuditFilter{})
	if len(results) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(results))
	}

	if strings.Contains(results[0].Parameters, "eyJhbGci") {
		t.Error("expected bearer token to be redacted")
	}
	if !strings.Contains(results[0].Parameters, "[REDACTED]") {
		t.Error("expected [REDACTED] placeholder in parameters")
	}
}

// ============================================
// Test: Parameter redaction - API keys
// ============================================

func TestAuditTrail_RedactAPIKey(t *testing.T) {
	t.Parallel()
	trail := NewAuditTrail(AuditConfig{
		MaxEntries:   1000,
		Enabled:      true,
		RedactParams: true,
	})

	params := `{"config": "api_key=sk-abc123xyz456 something"}`
	entry := AuditEntry{
		AuditSessionID: "s1",
		ClientID:       "cursor",
		ToolName:       "configure",
		Parameters:     params,
		Success:        true,
	}

	trail.Record(entry)

	results := trail.Query(AuditFilter{})
	if strings.Contains(results[0].Parameters, "sk-abc123xyz456") {
		t.Error("expected API key to be redacted")
	}
	if !strings.Contains(results[0].Parameters, "[REDACTED]") {
		t.Error("expected [REDACTED] placeholder in parameters")
	}
}

// ============================================
// Test: Parameter redaction - regular params preserved
// ============================================

func TestAuditTrail_RegularParamsPreserved(t *testing.T) {
	t.Parallel()
	trail := NewAuditTrail(AuditConfig{
		MaxEntries:   1000,
		Enabled:      true,
		RedactParams: true,
	})

	params := `{"mode": "console", "limit": 50, "selector": ".btn-primary"}`
	entry := AuditEntry{
		AuditSessionID: "s1",
		ClientID:       "claude-code",
		ToolName:       "observe",
		Parameters:     params,
		Success:        true,
	}

	trail.Record(entry)

	results := trail.Query(AuditFilter{})
	// Regular params should be preserved as-is
	if results[0].Parameters != params {
		t.Errorf("expected regular params to be preserved.\nGot:      %q\nExpected: %q", results[0].Parameters, params)
	}
}

// ============================================
// Test: Redaction events logged separately
// ============================================

func TestAuditTrail_RedactionEventLogging(t *testing.T) {
	t.Parallel()
	trail := NewAuditTrail(AuditConfig{
		MaxEntries:   1000,
		Enabled:      true,
		RedactParams: true,
	})

	event := RedactionEvent{
		Timestamp:      time.Now(),
		AuditSessionID: "sess-100",
		ToolName:       "get_network_bodies",
		FieldPath:      "entries[0].response.headers.authorization",
		PatternName:    "bearer_token",
	}

	trail.RecordRedaction(event)

	events := trail.QueryRedactions(AuditFilter{AuditSessionID: "sess-100"})
	if len(events) != 1 {
		t.Fatalf("expected 1 redaction event, got %d", len(events))
	}

	got := events[0]
	if got.ToolName != "get_network_bodies" {
		t.Errorf("expected tool_name 'get_network_bodies', got %q", got.ToolName)
	}
	if got.FieldPath != "entries[0].response.headers.authorization" {
		t.Errorf("expected field_path, got %q", got.FieldPath)
	}
	if got.PatternName != "bearer_token" {
		t.Errorf("expected pattern_name 'bearer_token', got %q", got.PatternName)
	}
}

// ============================================
// Test: Redaction events - no content stored
// ============================================

func TestAuditTrail_RedactionNoContent(t *testing.T) {
	t.Parallel()
	trail := NewAuditTrail(AuditConfig{MaxEntries: 1000, Enabled: true})

	event := RedactionEvent{
		Timestamp:      time.Now(),
		AuditSessionID: "sess-200",
		ToolName:       "observe",
		FieldPath:      "entries[5].message",
		PatternName:    "credit_card",
	}

	trail.RecordRedaction(event)

	// Serialize the event and verify no sensitive content fields
	events := trail.QueryRedactions(AuditFilter{})
	data, err := json.Marshal(events[0])
	if err != nil {
		t.Fatal(err)
	}

	// The JSON should only contain the known fields, nothing else
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	allowed := map[string]bool{
		"timestamp": true, "audit_session_id": true, "tool_name": true,
		"field_path": true, "pattern_name": true,
	}
	for key := range raw {
		if !allowed[key] {
			t.Errorf("unexpected field %q in redaction event (potential content leak)", key)
		}
	}
}

// ============================================
// Test: Redaction of JWT tokens in parameters
// ============================================

func TestAuditTrail_RedactJWT(t *testing.T) {
	t.Parallel()
	trail := NewAuditTrail(AuditConfig{
		MaxEntries:   1000,
		Enabled:      true,
		RedactParams: true,
	})

	jwt := "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U"
	params := `{"token": "` + jwt + `"}`
	trail.Record(AuditEntry{
		AuditSessionID: "s1",
		ToolName:       "observe",
		Parameters:     params,
		Success:        true,
	})

	results := trail.Query(AuditFilter{})
	if strings.Contains(results[0].Parameters, "eyJhbGci") {
		t.Error("expected JWT to be redacted from parameters")
	}
	if !strings.Contains(results[0].Parameters, "[REDACTED]") {
		t.Error("expected [REDACTED] placeholder")
	}
}

// ============================================
// Test: Redaction of GitHub tokens
// ============================================

func TestAuditTrail_RedactGitHubToken(t *testing.T) {
	t.Parallel()
	trail := NewAuditTrail(AuditConfig{
		MaxEntries:   1000,
		Enabled:      true,
		RedactParams: true,
	})

	params := `{"auth": "Bearer ghp_ABCDEFghijklMNOPQRSTuvwxyz0123456789"}`
	trail.Record(AuditEntry{
		AuditSessionID: "s1",
		ToolName:       "observe",
		Parameters:     params,
		Success:        true,
	})

	results := trail.Query(AuditFilter{})
	if strings.Contains(results[0].Parameters, "ghp_") {
		t.Error("expected GitHub token to be redacted")
	}
}

// ============================================
// Test: Redaction events bounded by max entries
// ============================================

func TestAuditTrail_RedactionEventsBounded(t *testing.T) {
	t.Parallel()
	trail := NewAuditTrail(AuditConfig{MaxEntries: 5, Enabled: true})

	for i := 0; i < 10; i++ {
		trail.RecordRedaction(RedactionEvent{
			Timestamp:      time.Now(),
			AuditSessionID: "s1",
			ToolName:       "observe",
			FieldPath:      "field",
			PatternName:    "bearer_token",
		})
	}

	events := trail.QueryRedactions(AuditFilter{Limit: 20})
	if len(events) > 5 {
		t.Errorf("expected at most 5 redaction events (bounded), got %d", len(events))
	}
}

// ============================================
// Test: Redaction of session cookies
// ============================================

func TestAuditTrail_RedactSessionCookie(t *testing.T) {
	t.Parallel()
	trail := NewAuditTrail(AuditConfig{
		MaxEntries:   1000,
		Enabled:      true,
		RedactParams: true,
	})

	params := `{"cookie": "session=ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij"}`
	trail.Record(AuditEntry{
		AuditSessionID: "s1",
		ToolName:       "observe",
		Parameters:     params,
		Success:        true,
	})

	results := trail.Query(AuditFilter{})
	if strings.Contains(results[0].Parameters, "ABCDEFGHIJKLMNOP") {
		t.Error("expected session cookie to be redacted")
	}
}
