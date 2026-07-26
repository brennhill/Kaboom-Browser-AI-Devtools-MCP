// Purpose: Tests for snapshot name validation, insertion-order removal, and end-to-end Compare output.
// Docs: docs/features/feature/request-session-correlation/index.md

// helpers_test.go — Tests for helpers.go and the Compare orchestration.
// Covers: validateName, removeFromOrder, Compare wiring through snapdiff.
package session

import (
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/performance"
)

// ============================================
// validateName
// ============================================

func TestValidateName_EmptyName(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	err := sm.validateName("")
	if err == nil {
		t.Fatal("Expected error for empty name")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("Error should mention empty: %v", err)
	}
}

func TestValidateName_ReservedName(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	err := sm.validateName("current")
	if err == nil {
		t.Fatal("Expected error for reserved name 'current'")
	}
	if !strings.Contains(err.Error(), "reserved") {
		t.Errorf("Error should mention reserved: %v", err)
	}
}

func TestValidateName_TooLong(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	longName := strings.Repeat("a", maxSnapshotNameLen+1)
	err := sm.validateName(longName)
	if err == nil {
		t.Fatal("Expected error for name exceeding max length")
	}
	if !strings.Contains(err.Error(), "50") {
		t.Errorf("Error should mention max length: %v", err)
	}
}

func TestValidateName_ExactMaxLength(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	exactName := strings.Repeat("x", maxSnapshotNameLen)
	err := sm.validateName(exactName)
	if err != nil {
		t.Errorf("Expected no error for name at exact max length, got: %v", err)
	}
}

func TestValidateName_ValidName(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	err := sm.validateName("my-snapshot-v2")
	if err != nil {
		t.Errorf("Expected no error for valid name, got: %v", err)
	}
}

// ============================================
// removeFromOrder (indirect test through Delete)
// ============================================

func TestRemoveFromOrder_FirstElement(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	sm.Capture("a", "")
	sm.Capture("b", "")
	sm.Capture("c", "")

	sm.Delete("a")
	list := sm.List()

	if len(list) != 2 {
		t.Fatalf("Expected 2, got %d", len(list))
	}
	if list[0].Name != "b" || list[1].Name != "c" {
		t.Errorf("Expected [b, c], got [%s, %s]", list[0].Name, list[1].Name)
	}
}

func TestRemoveFromOrder_LastElement(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	sm.Capture("a", "")
	sm.Capture("b", "")
	sm.Capture("c", "")

	sm.Delete("c")
	list := sm.List()

	if len(list) != 2 {
		t.Fatalf("Expected 2, got %d", len(list))
	}
	if list[0].Name != "a" || list[1].Name != "b" {
		t.Errorf("Expected [a, b], got [%s, %s]", list[0].Name, list[1].Name)
	}
}

// ============================================
// Integration: Performance with snapshot manager
// ============================================

func TestDiffPerformance_IntegrationWithCompare(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	mock.performance = &performance.Snapshot{
		Timing:  performance.Timing{Load: 500},
		Network: performance.NetworkSummary{RequestCount: 5, TransferSize: 25000},
	}
	sm.Capture("baseline", "")

	// Make it much worse
	mock.performance = &performance.Snapshot{
		Timing:  performance.Timing{Load: 5000},
		Network: performance.NetworkSummary{RequestCount: 50, TransferSize: 250000},
	}
	sm.Capture("degraded", "")

	diff, err := sm.Compare("baseline", "degraded")
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}

	if diff.Performance.LoadTime == nil {
		t.Fatal("Expected non-nil LoadTime")
	}
	if diff.Performance.LoadTime.Before != 500 {
		t.Errorf("Expected before=500, got %v", diff.Performance.LoadTime.Before)
	}
	if diff.Performance.LoadTime.After != 5000 {
		t.Errorf("Expected after=5000, got %v", diff.Performance.LoadTime.After)
	}
	if diff.Performance.LoadTime.Change != "+900%" {
		t.Errorf("Expected '+900%%', got %q", diff.Performance.LoadTime.Change)
	}
	if !diff.Performance.LoadTime.Regression {
		t.Error("Expected regression=true")
	}

	// All 3 metrics should have regressed (10x increase)
	if diff.Summary.PerformanceRegressions != 3 {
		t.Errorf("Expected 3 performance regressions, got %d", diff.Summary.PerformanceRegressions)
	}
}
