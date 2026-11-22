// audit_trail_test.go — Tests audit recording, retention, concurrency, and reset.
// Docs: docs/features/feature/enterprise-audit/index.md

package audit

import (
	"os"
	"sync"
	"testing"
)

func TestAuditPackageRespectsTenFileBoundary(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	files := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			files++
		}
	}
	if files > 10 {
		t.Fatalf("audit package has %d files; want at most 10 change-coupled owners", files)
	}
}

func TestAuditPackageHasNoTypeAliasFacade(t *testing.T) {
	if _, err := os.Stat("type_aliases.go"); !os.IsNotExist(err) {
		t.Fatalf("type_aliases.go compatibility facade must not exist (stat error: %v)", err)
	}
}

// ============================================
// Test: Recording a tool call creates an entry
// ============================================

func TestAuditTrail_RecordEntry(t *testing.T) {
	t.Parallel()
	trail := NewAuditTrail(AuditConfig{
		MaxEntries:   1000,
		Enabled:      true,
		RedactParams: false,
	})

	entry := AuditEntry{
		AuditSessionID: "sess-001",
		ClientID:       "claude-code",
		ToolName:       "observe",
		Parameters:     `{"mode":"console"}`,
		ResponseSize:   2048,
		Duration:       15,
		Success:        true,
	}

	trail.Record(entry)

	results := trail.Query(AuditFilter{})
	if len(results) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(results))
	}

	got := results[0]
	if got.ID == "" {
		t.Error("expected entry to have an ID assigned")
	}
	if got.Timestamp.IsZero() {
		t.Error("expected entry to have a timestamp assigned")
	}
	if got.AuditSessionID != "sess-001" {
		t.Errorf("expected session_id 'sess-001', got %q", got.AuditSessionID)
	}
	if got.ClientID != "claude-code" {
		t.Errorf("expected client_id 'claude-code', got %q", got.ClientID)
	}
	if got.ToolName != "observe" {
		t.Errorf("expected tool_name 'observe', got %q", got.ToolName)
	}
	if got.Parameters != `{"mode":"console"}` {
		t.Errorf("expected parameters preserved, got %q", got.Parameters)
	}
	if got.ResponseSize != 2048 {
		t.Errorf("expected response_size 2048, got %d", got.ResponseSize)
	}
	if got.Duration != 15 {
		t.Errorf("expected duration 15, got %d", got.Duration)
	}
	if !got.Success {
		t.Error("expected success to be true")
	}
	if got.ErrorMessage != "" {
		t.Errorf("expected empty error_message, got %q", got.ErrorMessage)
	}
}

// ============================================
// Test: Entry with error message
// ============================================

func TestAuditTrail_RecordErrorEntry(t *testing.T) {
	t.Parallel()
	trail := NewAuditTrail(AuditConfig{
		MaxEntries:   1000,
		Enabled:      true,
		RedactParams: false,
	})

	entry := AuditEntry{
		AuditSessionID: "sess-002",
		ClientID:       "cursor",
		ToolName:       "query_dom",
		Parameters:     `{"selector":".btn"}`,
		ResponseSize:   0,
		Duration:       5,
		Success:        false,
		ErrorMessage:   "no active tab",
	}

	trail.Record(entry)

	results := trail.Query(AuditFilter{})
	if len(results) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(results))
	}
	if results[0].Success {
		t.Error("expected success to be false")
	}
	if results[0].ErrorMessage != "no active tab" {
		t.Errorf("expected error_message 'no active tab', got %q", results[0].ErrorMessage)
	}
}

// ============================================
// Test: Max entries enforced (FIFO eviction)
// ============================================

func TestAuditTrail_FIFOEviction(t *testing.T) {
	t.Parallel()
	trail := NewAuditTrail(AuditConfig{
		MaxEntries:   5,
		Enabled:      true,
		RedactParams: false,
	})

	// Record 8 entries; only last 5 should remain
	for i := 0; i < 8; i++ {
		trail.Record(AuditEntry{
			AuditSessionID: "s1",
			ToolName:       "tool-" + string(rune('A'+i)),
			Success:        true,
		})
	}

	results := trail.Query(AuditFilter{Limit: 10})
	if len(results) != 5 {
		t.Fatalf("expected 5 entries (max), got %d", len(results))
	}

	// The oldest entries (tool-A, tool-B, tool-C) should be evicted
	// The remaining entries should be tool-D through tool-H
	for _, r := range results {
		if r.ToolName == "tool-A" || r.ToolName == "tool-B" || r.ToolName == "tool-C" {
			t.Errorf("expected old entry %q to be evicted", r.ToolName)
		}
	}
}

// ============================================
// Test: Concurrent recording is safe
// ============================================

func TestAuditTrail_ConcurrentSafety(t *testing.T) {
	t.Parallel()
	trail := NewAuditTrail(AuditConfig{
		MaxEntries:   10000,
		Enabled:      true,
		RedactParams: false,
	})

	var wg sync.WaitGroup
	numGoroutines := 50
	entriesPerGoroutine := 100

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < entriesPerGoroutine; i++ {
				trail.Record(AuditEntry{
					AuditSessionID: "concurrent-sess",
					ClientID:       "test-client",
					ToolName:       "observe",
					Success:        true,
				})
			}
		}(g)
	}

	wg.Wait()

	results := trail.Query(AuditFilter{Limit: 10000})
	expected := numGoroutines * entriesPerGoroutine
	if len(results) != expected {
		t.Errorf("expected %d entries after concurrent writes, got %d", expected, len(results))
	}
}

// ============================================
// Test: Disabled audit trail silently drops entries
// ============================================

func TestAuditTrail_DisabledDropsEntries(t *testing.T) {
	t.Parallel()
	trail := NewAuditTrail(AuditConfig{
		MaxEntries:   1000,
		Enabled:      false,
		RedactParams: false,
	})

	trail.Record(AuditEntry{
		AuditSessionID: "s1",
		ClientID:       "claude-code",
		ToolName:       "observe",
		Success:        true,
	})

	results := trail.Query(AuditFilter{})
	if len(results) != 0 {
		t.Errorf("expected 0 entries when disabled, got %d", len(results))
	}
}

// ============================================
// Test: Duration correctly captured
// ============================================

func TestAuditTrail_DurationCaptured(t *testing.T) {
	t.Parallel()
	trail := NewAuditTrail(AuditConfig{
		MaxEntries:   1000,
		Enabled:      true,
		RedactParams: false,
	})

	trail.Record(AuditEntry{
		AuditSessionID: "s1",
		ToolName:       "observe",
		Duration:       42,
		Success:        true,
	})

	results := trail.Query(AuditFilter{})
	if results[0].Duration != 42 {
		t.Errorf("expected duration 42, got %d", results[0].Duration)
	}
}

// ============================================
// Test: Response size correctly captured
// ============================================

func TestAuditTrail_ResponseSizeCaptured(t *testing.T) {
	t.Parallel()
	trail := NewAuditTrail(AuditConfig{
		MaxEntries:   1000,
		Enabled:      true,
		RedactParams: false,
	})

	trail.Record(AuditEntry{
		AuditSessionID: "s1",
		ToolName:       "get_network_bodies",
		ResponseSize:   15360,
		Success:        true,
	})

	results := trail.Query(AuditFilter{})
	if results[0].ResponseSize != 15360 {
		t.Errorf("expected response_size 15360, got %d", results[0].ResponseSize)
	}
}

// ============================================
// Test: Default config values
// ============================================

func TestAuditTrail_DefaultConfig(t *testing.T) {
	t.Parallel()
	trail := NewAuditTrail(AuditConfig{})

	// Default should be: MaxEntries=10000, Enabled=true, RedactParams=true
	// Record something to verify it works with defaults
	trail.Record(AuditEntry{AuditSessionID: "s1", ToolName: "observe", Success: true})

	results := trail.Query(AuditFilter{})
	if len(results) != 1 {
		t.Errorf("expected default config to allow recording, got %d entries", len(results))
	}
}

// ============================================
// Test: Clear() resets all state including sessions
// ============================================

func TestAuditTrail_Clear_ResetsEntriesAndSessions(t *testing.T) {
	t.Parallel()
	trail := NewAuditTrail(AuditConfig{
		MaxEntries:   1000,
		Enabled:      true,
		RedactParams: false,
	})

	// Create a session and record entries under it
	sess := trail.CreateAuditSession(ClientIdentifier{Name: "claude-code", Version: "1.0"})
	trail.Record(AuditEntry{AuditSessionID: sess.ID, ToolName: "observe", Success: true})
	trail.Record(AuditEntry{AuditSessionID: sess.ID, ToolName: "analyze", Success: true})
	trail.RecordRedaction(RedactionEvent{AuditSessionID: sess.ID, ToolName: "observe", PatternName: "bearer_token"})

	// Verify pre-state
	if len(trail.Query(AuditFilter{})) != 2 {
		t.Fatal("expected 2 entries before clear")
	}
	if trail.GetAuditSession(sess.ID) == nil {
		t.Fatal("session should exist before clear")
	}

	// Clear
	cleared := trail.Clear()
	if cleared != 2 {
		t.Fatalf("Clear() should return 2, got %d", cleared)
	}

	// Entries must be empty
	if entries := trail.Query(AuditFilter{}); len(entries) != 0 {
		t.Fatalf("expected 0 entries after clear, got %d", len(entries))
	}

	// Redaction events must be empty
	if redactions := trail.QueryRedactions(AuditFilter{}); len(redactions) != 0 {
		t.Fatalf("expected 0 redactions after clear, got %d", len(redactions))
	}

	// Sessions must be reset — stale ToolCalls counters must not persist (B3)
	if trail.GetAuditSession(sess.ID) != nil {
		t.Fatal("sessions should be cleared — stale session with old ToolCalls counter persists (B3 bug)")
	}
}

func TestAuditTrail_Clear_EmptyTrail(t *testing.T) {
	t.Parallel()
	trail := NewAuditTrail(AuditConfig{
		MaxEntries:   100,
		Enabled:      true,
		RedactParams: false,
	})

	// Clear on empty trail should return 0, not panic
	cleared := trail.Clear()
	if cleared != 0 {
		t.Fatalf("Clear() on empty trail should return 0, got %d", cleared)
	}
}
