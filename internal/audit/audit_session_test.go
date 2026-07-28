// audit_session_test.go — Tests audit client sessions and correlation.
// Docs: docs/features/feature/enterprise-audit/index.md

package audit

import (
	"sync"
	"testing"
)

// ============================================
// Test: Client identification from initialize message
// ============================================

func TestAuditTrail_ClientIdentification(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    ClientIdentifier
		expected string
	}{
		{"claude-code", ClientIdentifier{Name: "claude-code", Version: "1.0.0"}, "claude-code"},
		{"Cursor uppercase", ClientIdentifier{Name: "Cursor", Version: "0.45"}, "cursor"},
		{"Windsurf uppercase", ClientIdentifier{Name: "Windsurf", Version: "2.0"}, "windsurf"},
		{"cline lowercase", ClientIdentifier{Name: "cline", Version: "3.1"}, "cline"},
		{"unknown client preserved", ClientIdentifier{Name: "my-custom-tool", Version: "0.1"}, "my-custom-tool"},
		{"empty name", ClientIdentifier{Name: "", Version: "1.0"}, "unknown"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			trail := NewAuditTrail(AuditConfig{MaxEntries: 100, Enabled: true})
			clientID := trail.IdentifyClient(tc.input)
			if clientID != tc.expected {
				t.Errorf("expected client ID %q, got %q", tc.expected, clientID)
			}
		})
	}
}

// ============================================
// Test: Session ID is unique per connection
// ============================================

func TestAuditTrail_SessionIDUnique(t *testing.T) {
	t.Parallel()
	trail := NewAuditTrail(AuditConfig{MaxEntries: 100, Enabled: true})

	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		sess := trail.CreateAuditSession(ClientIdentifier{Name: "claude-code", Version: "1.0"})
		if seen[sess.ID] {
			t.Fatalf("duplicate session ID: %s", sess.ID)
		}
		seen[sess.ID] = true
	}
}

// ============================================
// Test: Session ID format (hex-encoded, 32 chars)
// ============================================

func TestAuditTrail_SessionIDFormat(t *testing.T) {
	t.Parallel()
	trail := NewAuditTrail(AuditConfig{MaxEntries: 100, Enabled: true})
	sess := trail.CreateAuditSession(ClientIdentifier{Name: "cursor", Version: "1.0"})

	if len(sess.ID) != 32 {
		t.Errorf("expected session ID length 32, got %d: %q", len(sess.ID), sess.ID)
	}

	// Verify it's valid hex
	for _, c := range sess.ID {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			t.Errorf("session ID contains non-hex character: %c in %q", c, sess.ID)
			break
		}
	}
}

// ============================================
// Test: Session correlates entries
// ============================================

func TestAuditTrail_SessionCorrelation(t *testing.T) {
	t.Parallel()
	trail := NewAuditTrail(AuditConfig{MaxEntries: 1000, Enabled: true, RedactParams: false})

	sess := trail.CreateAuditSession(ClientIdentifier{Name: "claude-code", Version: "1.0"})

	trail.Record(AuditEntry{AuditSessionID: sess.ID, ClientID: "claude-code", ToolName: "observe", Success: true})
	trail.Record(AuditEntry{AuditSessionID: sess.ID, ClientID: "claude-code", ToolName: "analyze", Success: true})
	trail.Record(AuditEntry{AuditSessionID: "other-session", ClientID: "cursor", ToolName: "observe", Success: true})

	results := trail.Query(AuditFilter{AuditSessionID: sess.ID})
	if len(results) != 2 {
		t.Fatalf("expected 2 entries for session, got %d", len(results))
	}
	for _, r := range results {
		if r.AuditSessionID != sess.ID {
			t.Errorf("expected session ID %q, got %q", sess.ID, r.AuditSessionID)
		}
	}
}

// ============================================
// Test: Session tracks tool call count
// ============================================

func TestAuditTrail_SessionToolCallCount(t *testing.T) {
	t.Parallel()
	trail := NewAuditTrail(AuditConfig{MaxEntries: 1000, Enabled: true, RedactParams: false})

	sess := trail.CreateAuditSession(ClientIdentifier{Name: "claude-code", Version: "1.0"})

	trail.Record(AuditEntry{AuditSessionID: sess.ID, ClientID: "claude-code", ToolName: "observe", Success: true})
	trail.Record(AuditEntry{AuditSessionID: sess.ID, ClientID: "claude-code", ToolName: "analyze", Success: true})
	trail.Record(AuditEntry{AuditSessionID: sess.ID, ClientID: "claude-code", ToolName: "generate", Success: true})

	info := trail.GetAuditSession(sess.ID)
	if info == nil {
		t.Fatal("expected session to exist")
	}
	if info.ToolCalls != 3 {
		t.Errorf("expected 3 tool calls, got %d", info.ToolCalls)
	}
}

// ============================================
// Test: Session info stores client identity
// ============================================

func TestAuditTrail_SessionStoresClientIdentity(t *testing.T) {
	t.Parallel()
	trail := NewAuditTrail(AuditConfig{MaxEntries: 100, Enabled: true})

	sess := trail.CreateAuditSession(ClientIdentifier{Name: "Windsurf", Version: "2.5.0"})

	info := trail.GetAuditSession(sess.ID)
	if info == nil {
		t.Fatal("expected session to exist")
	}
	if info.ClientID != "windsurf" {
		t.Errorf("expected client_id 'windsurf', got %q", info.ClientID)
	}
	if info.StartedAt.IsZero() {
		t.Error("expected started_at to be set")
	}
}

// ============================================
// Test: Concurrent session creation is safe
// ============================================

func TestAuditTrail_ConcurrentSessionCreation(t *testing.T) {
	t.Parallel()
	trail := NewAuditTrail(AuditConfig{MaxEntries: 10000, Enabled: true})

	var wg sync.WaitGroup
	sessions := make([]string, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sess := trail.CreateAuditSession(ClientIdentifier{Name: "test", Version: "1.0"})
			sessions[idx] = sess.ID
		}(i)
	}

	wg.Wait()

	// All session IDs should be unique
	seen := make(map[string]bool)
	for _, id := range sessions {
		if id == "" {
			t.Error("got empty session ID")
			continue
		}
		if seen[id] {
			t.Errorf("duplicate session ID: %s", id)
		}
		seen[id] = true
	}
}
