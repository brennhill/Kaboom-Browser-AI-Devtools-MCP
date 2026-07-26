// Purpose: Tests for checkpoint resolution tracking and lifecycle.

// ai_checkpoint_resolution_test.go — Tests for checkpoint resolution, namespacing, and multi-client isolation.
package checkpoint

import (
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/server"
	gasTypes "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// ============================================
// resolveCheckpoint: auto mode (empty name)
// ============================================

func TestResolveCheckpoint_AutoMode(t *testing.T) {
	t.Parallel()

	fake := &fakeLogReader{}
	cm := NewCheckpointManager(fake, capture.NewCapture())
	now := time.Now()

	cp, isNamed := cm.resolveCheckpoint("", "", now)

	if isNamed {
		t.Error("auto mode should return isNamed=false")
	}
	if cp == nil {
		t.Fatal("auto mode should return non-nil checkpoint")
	}
}

// ============================================
// resolveCheckpoint: auto mode with existing auto checkpoint
// ============================================

func TestResolveCheckpoint_AutoModeWithExisting(t *testing.T) {
	t.Parallel()

	fake := &fakeLogReader{}
	cm := NewCheckpointManager(fake, capture.NewCapture())
	now := time.Now()

	// Set auto checkpoint
	cm.autoCheckpoint = &Checkpoint{
		Name:           "auto",
		CreatedAt:      now.Add(-5 * time.Second),
		LogTotal:       10,
		KnownEndpoints: make(map[string]endpointState),
	}

	cp, isNamed := cm.resolveCheckpoint("", "", now)

	if isNamed {
		t.Error("auto mode should return isNamed=false")
	}
	if cp.LogTotal != 10 {
		t.Errorf("expected auto checkpoint LogTotal=10, got %d", cp.LogTotal)
	}
}

// ============================================
// resolveCheckpoint: named checkpoint with clientID
// ============================================

func TestResolveCheckpoint_NamedWithClientID(t *testing.T) {
	t.Parallel()

	fake := &fakeLogReader{}
	cm := NewCheckpointManager(fake, capture.NewCapture())
	now := time.Now()

	// Create a namespaced checkpoint
	cm.namedCheckpoints["client-a:test-cp"] = &Checkpoint{
		Name:           "test-cp",
		LogTotal:       42,
		CreatedAt:      now,
		KnownEndpoints: make(map[string]endpointState),
	}

	cp, isNamed := cm.resolveCheckpoint("test-cp", "client-a", now)

	if !isNamed {
		t.Error("named checkpoint should return isNamed=true")
	}
	if cp.LogTotal != 42 {
		t.Errorf("expected LogTotal=42, got %d", cp.LogTotal)
	}
}

// ============================================
// resolveCheckpoint: fallback to non-namespaced name
// ============================================

func TestResolveCheckpoint_FallbackToNonNamespaced(t *testing.T) {
	t.Parallel()

	fake := &fakeLogReader{}
	cm := NewCheckpointManager(fake, capture.NewCapture())
	now := time.Now()

	// Store without clientID prefix
	cm.namedCheckpoints["shared-cp"] = &Checkpoint{
		Name:           "shared-cp",
		LogTotal:       99,
		CreatedAt:      now,
		KnownEndpoints: make(map[string]endpointState),
	}

	// Look up with clientID (should fall back to non-namespaced)
	cp, isNamed := cm.resolveCheckpoint("shared-cp", "client-b", now)

	if !isNamed {
		t.Error("fallback named checkpoint should return isNamed=true")
	}
	if cp.LogTotal != 99 {
		t.Errorf("expected LogTotal=99, got %d", cp.LogTotal)
	}
}

// ============================================
// resolveCheckpoint: timestamp string
// ============================================

func TestResolveCheckpoint_Timestamp(t *testing.T) {
	t.Parallel()

	t0 := time.Now().Add(-20 * time.Second).UTC()
	t1 := t0.Add(10 * time.Second)

	fake := &fakeLogReader{
		snapshot: server.LogSnapshot{
			Entries:    []gasTypes.LogEntry{{"level": "info"}, {"level": "info"}},
			TotalAdded: 2,
		},
		timestamps: []time.Time{t0, t1},
	}
	cm := NewCheckpointManager(fake, capture.NewCapture())

	cp, isNamed := cm.resolveCheckpoint(t0.Format(time.RFC3339Nano), "", time.Now())

	if !isNamed {
		t.Error("timestamp checkpoint should return isNamed=true")
	}
	if cp == nil {
		t.Fatal("timestamp checkpoint should not be nil")
	}
}

// ============================================
// resolveCheckpoint: invalid name creates fallback checkpoint
// ============================================

func TestResolveCheckpoint_InvalidNameFallback(t *testing.T) {
	t.Parallel()

	fake := &fakeLogReader{}
	cm := NewCheckpointManager(fake, capture.NewCapture())
	now := time.Now()

	cp, isNamed := cm.resolveCheckpoint("nonexistent-checkpoint-name", "", now)

	if !isNamed {
		t.Error("fallback should return isNamed=true")
	}
	if cp.KnownEndpoints == nil {
		t.Error("fallback checkpoint should have initialized KnownEndpoints")
	}
}

// ============================================
// CreateCheckpoint: client ID namespacing
// ============================================

func TestCreateCheckpoint_ClientIDNamespacing(t *testing.T) {
	t.Parallel()

	fake := &fakeLogReader{}
	cm := NewCheckpointManager(fake, capture.NewCapture())

	// Create same name for different clients
	if err := cm.CreateCheckpoint("deploy", "client-a"); err != nil {
		t.Fatalf("CreateCheckpoint(client-a) error = %v", err)
	}
	if err := cm.CreateCheckpoint("deploy", "client-b"); err != nil {
		t.Fatalf("CreateCheckpoint(client-b) error = %v", err)
	}

	if cm.GetNamedCheckpointCount() != 2 {
		t.Errorf("expected 2 checkpoints (one per client), got %d", cm.GetNamedCheckpointCount())
	}

	// Both should exist with different keys
	if _, ok := cm.namedCheckpoints["client-a:deploy"]; !ok {
		t.Error("missing client-a:deploy checkpoint")
	}
	if _, ok := cm.namedCheckpoints["client-b:deploy"]; !ok {
		t.Error("missing client-b:deploy checkpoint")
	}
}

// ============================================
// CreateCheckpoint: no client ID
// ============================================

func TestCreateCheckpoint_NoClientID(t *testing.T) {
	t.Parallel()

	fake := &fakeLogReader{}
	cm := NewCheckpointManager(fake, capture.NewCapture())

	if err := cm.CreateCheckpoint("simple", ""); err != nil {
		t.Fatalf("CreateCheckpoint error = %v", err)
	}

	if _, ok := cm.namedCheckpoints["simple"]; !ok {
		t.Error("missing 'simple' checkpoint (no client prefix)")
	}
}

// ============================================
// CreateCheckpoint: update existing
// ============================================

func TestCreateCheckpoint_UpdateExisting(t *testing.T) {
	t.Parallel()

	fake := &fakeLogReader{
		snapshot: server.LogSnapshot{TotalAdded: 5},
	}
	cm := NewCheckpointManager(fake, capture.NewCapture())

	if err := cm.CreateCheckpoint("update-test", ""); err != nil {
		t.Fatalf("first CreateCheckpoint error = %v", err)
	}

	// Update buffer state
	fake.snapshot.TotalAdded = 10

	if err := cm.CreateCheckpoint("update-test", ""); err != nil {
		t.Fatalf("second CreateCheckpoint error = %v", err)
	}

	// Should still be 1 checkpoint, not 2
	if cm.GetNamedCheckpointCount() != 1 {
		t.Errorf("expected 1 checkpoint after update, got %d", cm.GetNamedCheckpointCount())
	}

	cp := cm.namedCheckpoints["update-test"]
	if cp.LogTotal != 10 {
		t.Errorf("updated checkpoint LogTotal = %d, want 10", cp.LogTotal)
	}
}

// ============================================
// CreateCheckpoint: max name length boundary
// ============================================

func TestCreateCheckpoint_NameLengthBoundary(t *testing.T) {
	t.Parallel()

	fake := &fakeLogReader{}
	cm := NewCheckpointManager(fake, capture.NewCapture())

	// Exactly at max length
	exactMax := strings.Repeat("a", maxCheckpointNameLen)
	if err := cm.CreateCheckpoint(exactMax, ""); err != nil {
		t.Errorf("name at exact max length should be accepted, got error: %v", err)
	}

	// One over max length
	overMax := strings.Repeat("b", maxCheckpointNameLen+1)
	if err := cm.CreateCheckpoint(overMax, ""); err == nil {
		t.Error("name over max length should be rejected")
	}
}

// ============================================
// CreateCheckpoint: stores original name, not namespaced
// ============================================

func TestCreateCheckpoint_StoresOriginalName(t *testing.T) {
	t.Parallel()

	fake := &fakeLogReader{}
	cm := NewCheckpointManager(fake, capture.NewCapture())

	if err := cm.CreateCheckpoint("my-cp", "client-x"); err != nil {
		t.Fatalf("CreateCheckpoint error = %v", err)
	}

	cp := cm.namedCheckpoints["client-x:my-cp"]
	if cp.Name != "my-cp" {
		t.Errorf("stored checkpoint Name = %q, want 'my-cp'", cp.Name)
	}
}

// ============================================
// GetChangesSince: named checkpoint doesn't advance auto
// ============================================

func TestGetChangesSince_NamedDoesNotAdvanceAuto(t *testing.T) {
	t.Parallel()

	fake := &fakeLogReader{
		snapshot: server.LogSnapshot{
			Entries:    []gasTypes.LogEntry{{"level": "info", "message": "test"}},
			TotalAdded: 1,
		},
		timestamps: []time.Time{time.Now()},
	}
	cm := NewCheckpointManager(fake, capture.NewCapture())

	if err := cm.CreateCheckpoint("named", ""); err != nil {
		t.Fatalf("CreateCheckpoint error = %v", err)
	}

	// Capture auto checkpoint before named query
	autoBefore := cm.autoCheckpoint

	cm.GetChangesSince(GetChangesSinceParams{
		Checkpoint: "named",
		Include:    []string{"console"},
	}, "")

	// Auto checkpoint should not have changed
	if cm.autoCheckpoint != autoBefore {
		t.Error("named query should not advance auto checkpoint")
	}
}

// ============================================
// GetChangesSince: auto mode advances auto checkpoint
// ============================================

func TestGetChangesSince_AutoAdvances(t *testing.T) {
	t.Parallel()

	fake := &fakeLogReader{
		snapshot: server.LogSnapshot{
			Entries:    []gasTypes.LogEntry{{"level": "error", "message": "err"}},
			TotalAdded: 1,
		},
		timestamps: []time.Time{time.Now()},
	}
	cm := NewCheckpointManager(fake, capture.NewCapture())

	if cm.autoCheckpoint != nil {
		t.Fatal("auto checkpoint should be nil initially")
	}

	cm.GetChangesSince(GetChangesSinceParams{Include: []string{"console"}}, "")

	if cm.autoCheckpoint == nil {
		t.Fatal("auto checkpoint should be set after first auto query")
	}
}

// ============================================
// GetChangesSince: token count calculation
// ============================================

func TestGetChangesSince_TokenCount(t *testing.T) {
	t.Parallel()

	fake := &fakeLogReader{
		snapshot: server.LogSnapshot{
			Entries:    []gasTypes.LogEntry{{"level": "error", "message": "test error for tokens"}},
			TotalAdded: 1,
		},
		timestamps: []time.Time{time.Now()},
	}
	cm := NewCheckpointManager(fake, capture.NewCapture())

	resp := cm.GetChangesSince(GetChangesSinceParams{Include: []string{"console"}}, "")

	if resp.TokenCount <= 0 {
		t.Errorf("TokenCount = %d, want > 0", resp.TokenCount)
	}
}

// ============================================
// findPositionAtTime: edge cases
// ============================================

func TestFindPositionAtTime_EdgeCases(t *testing.T) {
	t.Parallel()

	cm := &CheckpointManager{}
	t0 := time.Now().Add(-30 * time.Second)
	t1 := t0.Add(10 * time.Second)
	t2 := t1.Add(10 * time.Second)

	addedAt := []time.Time{t0, t1, t2}

	// Time before all entries
	pos := cm.findPositionAtTime(addedAt, 3, t0.Add(-10*time.Second))
	if pos != 0 {
		t.Errorf("before all entries: pos = %d, want 0", pos)
	}

	// Time after all entries
	pos = cm.findPositionAtTime(addedAt, 3, t2.Add(10*time.Second))
	if pos != 3 {
		t.Errorf("after all entries: pos = %d, want 3", pos)
	}

	// Time at first entry
	pos = cm.findPositionAtTime(addedAt, 3, t0)
	if pos != 1 {
		t.Errorf("at first entry: pos = %d, want 1", pos)
	}

	// Time between entries
	pos = cm.findPositionAtTime(addedAt, 3, t0.Add(5*time.Second))
	if pos != 1 {
		t.Errorf("between t0 and t1: pos = %d, want 1", pos)
	}
}

// ============================================
// Checkpoint eviction: order tracking
// ============================================

func TestCheckpointEviction_OrderTracking(t *testing.T) {
	t.Parallel()

	fake := &fakeLogReader{}
	cm := NewCheckpointManager(fake, capture.NewCapture())

	// Fill beyond max
	for i := 0; i < maxNamedCheckpoints+5; i++ {
		name := strings.Repeat("a", i%26+1)
		// Ensure unique names
		name = name + string(rune('0'+i%10))
		if err := cm.CreateCheckpoint(name, ""); err != nil {
			t.Fatalf("CreateCheckpoint(%d) error = %v", i, err)
		}
	}

	if cm.GetNamedCheckpointCount() != maxNamedCheckpoints {
		t.Errorf("count = %d, want %d after eviction", cm.GetNamedCheckpointCount(), maxNamedCheckpoints)
	}

	// namedOrder should match namedCheckpoints
	if len(cm.namedOrder) != len(cm.namedCheckpoints) {
		t.Errorf("namedOrder len=%d != namedCheckpoints len=%d", len(cm.namedOrder), len(cm.namedCheckpoints))
	}
}

// ============================================
// applySeverityFilter: errors_only strips warnings but keeps console errors
// ============================================

func TestApplySeverityFilter_ErrorsOnly(t *testing.T) {
	t.Parallel()

	cm := &CheckpointManager{}

	resp := &DiffResponse{
		Console: &ConsoleDiff{
			TotalNew: 3,
			Errors:   []ConsoleEntry{{Message: "err", Count: 1}},
			Warnings: []ConsoleEntry{{Message: "warn", Count: 2}},
		},
		WebSocket: &WebSocketDiff{
			TotalNew:       1,
			Disconnections: []WSDisconnection{{URL: "ws://a"}},
		},
	}

	cm.applySeverityFilter(resp, "errors_only")

	if len(resp.Console.Warnings) != 0 {
		t.Error("errors_only should strip console warnings")
	}
	if len(resp.Console.Errors) != 1 {
		t.Error("errors_only should keep console errors")
	}
	// WebSocket with only disconnections (no errors) should be nil
	if resp.WebSocket != nil {
		t.Error("errors_only should nil WebSocket with only disconnections")
	}
}

// ============================================
// applySeverityFilter: errors_only keeps WS errors
// ============================================

func TestApplySeverityFilter_ErrorsOnlyKeepsWSErrors(t *testing.T) {
	t.Parallel()

	cm := &CheckpointManager{}

	resp := &DiffResponse{
		WebSocket: &WebSocketDiff{
			TotalNew: 2,
			Errors:   []WSError{{URL: "ws://a", Message: "conn reset"}},
		},
	}

	cm.applySeverityFilter(resp, "errors_only")

	// WebSocket with errors should be kept
	if resp.WebSocket == nil {
		t.Error("errors_only should keep WebSocket with errors")
	}
}

// ============================================
// resolveTimestampCheckpoint: RFC3339 format
// ============================================

func TestResolveTimestampCheckpoint_RFC3339(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	fake := &fakeLogReader{
		snapshot:   server.LogSnapshot{TotalAdded: 5},
		timestamps: []time.Time{now.Add(-5 * time.Second), now.Add(-3 * time.Second), now.Add(-1 * time.Second)},
	}
	cm := NewCheckpointManager(fake, capture.NewCapture())

	// RFC3339 (without nanos)
	cp := cm.resolveTimestampCheckpoint(now.Format(time.RFC3339))
	if cp == nil {
		t.Fatal("should resolve RFC3339 timestamp")
	}

	// RFC3339Nano
	cp = cm.resolveTimestampCheckpoint(now.Format(time.RFC3339Nano))
	if cp == nil {
		t.Fatal("should resolve RFC3339Nano timestamp")
	}
}
