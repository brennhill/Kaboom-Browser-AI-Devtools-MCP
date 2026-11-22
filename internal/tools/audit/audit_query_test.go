// audit_query_test.go — Tests audit queries, filters, ordering, and MCP responses.
// Docs: docs/features/feature/enterprise-audit/index.md

package audit

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

type auditTestClock struct{ now time.Time }

func (c *auditTestClock) Now() time.Time              { return c.now }
func (c *auditTestClock) Advance(delta time.Duration) { c.now = c.now.Add(delta) }

func useAuditTestClock(trail *AuditTrail) *auditTestClock {
	clock := &auditTestClock{now: time.Unix(100, 0)}
	trail.now = clock.Now
	return clock
}

// ============================================
// Test: Query with no filter returns latest (default limit 100)
// ============================================

func TestAuditTrail_QueryDefaultLimit(t *testing.T) {
	t.Parallel()
	trail := NewAuditTrail(AuditConfig{
		MaxEntries:   10000,
		Enabled:      true,
		RedactParams: false,
	})

	// Record 150 entries
	for i := 0; i < 150; i++ {
		trail.Record(AuditEntry{
			AuditSessionID: "sess-default",
			ClientID:       "claude-code",
			ToolName:       "observe",
			Success:        true,
		})
	}

	results := trail.Query(AuditFilter{})
	if len(results) != 100 {
		t.Fatalf("expected default limit of 100, got %d", len(results))
	}
}

// ============================================
// Test: Query by session_id
// ============================================

func TestAuditTrail_QueryBySessionID(t *testing.T) {
	t.Parallel()
	trail := NewAuditTrail(AuditConfig{
		MaxEntries:   1000,
		Enabled:      true,
		RedactParams: false,
	})

	trail.Record(AuditEntry{AuditSessionID: "sess-A", ClientID: "claude-code", ToolName: "observe", Success: true})
	trail.Record(AuditEntry{AuditSessionID: "sess-B", ClientID: "cursor", ToolName: "analyze", Success: true})
	trail.Record(AuditEntry{AuditSessionID: "sess-A", ClientID: "claude-code", ToolName: "generate", Success: true})
	trail.Record(AuditEntry{AuditSessionID: "sess-B", ClientID: "cursor", ToolName: "observe", Success: true})

	results := trail.Query(AuditFilter{AuditSessionID: "sess-A"})
	if len(results) != 2 {
		t.Fatalf("expected 2 entries for sess-A, got %d", len(results))
	}
	for _, r := range results {
		if r.AuditSessionID != "sess-A" {
			t.Errorf("expected session_id 'sess-A', got %q", r.AuditSessionID)
		}
	}
}

// ============================================
// Test: Query by tool_name
// ============================================

func TestAuditTrail_QueryByToolName(t *testing.T) {
	t.Parallel()
	trail := NewAuditTrail(AuditConfig{
		MaxEntries:   1000,
		Enabled:      true,
		RedactParams: false,
	})

	trail.Record(AuditEntry{AuditSessionID: "s1", ToolName: "observe", Success: true})
	trail.Record(AuditEntry{AuditSessionID: "s1", ToolName: "analyze", Success: true})
	trail.Record(AuditEntry{AuditSessionID: "s1", ToolName: "observe", Success: true})
	trail.Record(AuditEntry{AuditSessionID: "s1", ToolName: "generate", Success: true})

	results := trail.Query(AuditFilter{ToolName: "observe"})
	if len(results) != 2 {
		t.Fatalf("expected 2 entries for tool 'observe', got %d", len(results))
	}
	for _, r := range results {
		if r.ToolName != "observe" {
			t.Errorf("expected tool_name 'observe', got %q", r.ToolName)
		}
	}
}

// ============================================
// Test: Query with 'since' timestamp filter
// ============================================

func TestAuditTrail_QuerySince(t *testing.T) {
	t.Parallel()
	trail := NewAuditTrail(AuditConfig{
		MaxEntries:   1000,
		Enabled:      true,
		RedactParams: false,
	})
	clock := useAuditTestClock(trail)

	// Record entries with slight delays to ensure distinct timestamps
	trail.Record(AuditEntry{AuditSessionID: "s1", ToolName: "observe", Success: true})
	clock.Advance(time.Second)

	cutoff := clock.Now()
	clock.Advance(time.Second)

	trail.Record(AuditEntry{AuditSessionID: "s1", ToolName: "analyze", Success: true})
	trail.Record(AuditEntry{AuditSessionID: "s1", ToolName: "generate", Success: true})

	results := trail.Query(AuditFilter{Since: &cutoff})
	if len(results) != 2 {
		t.Fatalf("expected 2 entries after cutoff, got %d", len(results))
	}
}

// ============================================
// Test: Query with custom limit
// ============================================

func TestAuditTrail_QueryWithLimit(t *testing.T) {
	t.Parallel()
	trail := NewAuditTrail(AuditConfig{
		MaxEntries:   1000,
		Enabled:      true,
		RedactParams: false,
	})

	for i := 0; i < 50; i++ {
		trail.Record(AuditEntry{AuditSessionID: "s1", ToolName: "observe", Success: true})
	}

	results := trail.Query(AuditFilter{Limit: 10})
	if len(results) != 10 {
		t.Fatalf("expected 10 entries with limit=10, got %d", len(results))
	}
}

// ============================================
// Test: Empty filter returns all entries up to limit
// ============================================

func TestAuditTrail_EmptyFilterReturnsAll(t *testing.T) {
	t.Parallel()
	trail := NewAuditTrail(AuditConfig{
		MaxEntries:   1000,
		Enabled:      true,
		RedactParams: false,
	})

	trail.Record(AuditEntry{AuditSessionID: "s1", ToolName: "observe", Success: true})
	trail.Record(AuditEntry{AuditSessionID: "s2", ToolName: "analyze", Success: true})
	trail.Record(AuditEntry{AuditSessionID: "s3", ToolName: "generate", Success: true})

	results := trail.Query(AuditFilter{})
	if len(results) != 3 {
		t.Errorf("expected 3 entries with empty filter, got %d", len(results))
	}
}

// ============================================
// Test: Query returns reverse chronological order
// ============================================

func TestAuditTrail_ReverseChronologicalOrder(t *testing.T) {
	t.Parallel()
	trail := NewAuditTrail(AuditConfig{
		MaxEntries:   1000,
		Enabled:      true,
		RedactParams: false,
	})
	clock := useAuditTestClock(trail)

	trail.Record(AuditEntry{AuditSessionID: "s1", ToolName: "first", Success: true})
	clock.Advance(time.Second)
	trail.Record(AuditEntry{AuditSessionID: "s1", ToolName: "second", Success: true})
	clock.Advance(time.Second)
	trail.Record(AuditEntry{AuditSessionID: "s1", ToolName: "third", Success: true})

	results := trail.Query(AuditFilter{})
	if len(results) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(results))
	}

	// Reverse chronological: newest first
	if results[0].ToolName != "third" {
		t.Errorf("expected first result to be 'third', got %q", results[0].ToolName)
	}
	if results[2].ToolName != "first" {
		t.Errorf("expected last result to be 'first', got %q", results[2].ToolName)
	}
}

// ============================================
// Test: handleGetAuditLog MCP tool handler
// ============================================

func TestAuditTrail_HandleGetAuditLog(t *testing.T) {
	t.Parallel()
	trail := NewAuditTrail(AuditConfig{
		MaxEntries:   1000,
		Enabled:      true,
		RedactParams: false,
	})

	trail.Record(AuditEntry{AuditSessionID: "s1", ClientID: "claude-code", ToolName: "observe", Success: true})
	trail.Record(AuditEntry{AuditSessionID: "s1", ClientID: "claude-code", ToolName: "analyze", Success: true})

	// Call the MCP handler with empty params
	params := json.RawMessage(`{}`)
	result, err := trail.HandleGetAuditLog(params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Result should be serializable
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal result: %v", err)
	}

	if !strings.Contains(string(data), "observe") {
		t.Error("expected result to contain 'observe' tool entry")
	}
	if !strings.Contains(string(data), "analyze") {
		t.Error("expected result to contain 'analyze' tool entry")
	}
}

// ============================================
// Test: handleGetAuditLog with filter params
// ============================================

func TestAuditTrail_HandleGetAuditLogFiltered(t *testing.T) {
	t.Parallel()
	trail := NewAuditTrail(AuditConfig{
		MaxEntries:   1000,
		Enabled:      true,
		RedactParams: false,
	})

	trail.Record(AuditEntry{AuditSessionID: "s1", ToolName: "observe", Success: true})
	trail.Record(AuditEntry{AuditSessionID: "s2", ToolName: "analyze", Success: true})
	trail.Record(AuditEntry{AuditSessionID: "s1", ToolName: "generate", Success: true})

	params := json.RawMessage(`{"audit_session_id": "s1"}`)
	result, err := trail.HandleGetAuditLog(params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal result: %v", err)
	}

	if strings.Contains(string(data), "analyze") {
		t.Error("expected result to NOT contain session s2 entry 'analyze'")
	}
}

// ============================================
// Test: Multiple combined filters (session + tool)
// ============================================

func TestAuditTrail_CombinedFilters(t *testing.T) {
	t.Parallel()
	trail := NewAuditTrail(AuditConfig{
		MaxEntries:   1000,
		Enabled:      true,
		RedactParams: false,
	})

	trail.Record(AuditEntry{AuditSessionID: "s1", ToolName: "observe", Success: true})
	trail.Record(AuditEntry{AuditSessionID: "s1", ToolName: "analyze", Success: true})
	trail.Record(AuditEntry{AuditSessionID: "s2", ToolName: "observe", Success: true})
	trail.Record(AuditEntry{AuditSessionID: "s2", ToolName: "analyze", Success: true})

	results := trail.Query(AuditFilter{AuditSessionID: "s1", ToolName: "observe"})
	if len(results) != 1 {
		t.Fatalf("expected 1 entry with combined filter, got %d", len(results))
	}
	if results[0].AuditSessionID != "s1" || results[0].ToolName != "observe" {
		t.Errorf("unexpected entry: session=%q tool=%q", results[0].AuditSessionID, results[0].ToolName)
	}
}
