// tools_configure_network_recording_test.go — Tests network recording state and filters.
// Docs: docs/features/feature/backend-log-streaming/index.md
package main

import (
	"sync"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolconfigure/netrecord"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// ============================================
// networkRecordingState.tryStart()
// ============================================

func TestNetworkRecordingState_TryStart_Success(t *testing.T) {
	t.Parallel()
	var s netrecord.NetworkRecordingState

	before := time.Now()
	startTime, ok := s.TryStart("example.com", "GET")
	after := time.Now()

	if !ok {
		t.Fatal("tryStart should return true on first call")
	}
	if startTime.IsZero() {
		t.Error("startTime should not be zero")
	}
	if startTime.Before(before) || startTime.After(after) {
		t.Errorf("startTime %v should be between %v and %v", startTime, before, after)
	}
}

func TestNetworkRecordingState_TryStart_AlreadyActive(t *testing.T) {
	t.Parallel()
	var s netrecord.NetworkRecordingState

	_, ok := s.TryStart("example.com", "GET")
	if !ok {
		t.Fatal("first tryStart should succeed")
	}

	startTime, ok := s.TryStart("other.com", "POST")
	if ok {
		t.Fatal("second tryStart should return false when already active")
	}
	if !startTime.IsZero() {
		t.Error("startTime should be zero on failure")
	}
}

func TestNetworkRecordingState_TryStart_EmptyFilters(t *testing.T) {
	t.Parallel()
	var s netrecord.NetworkRecordingState

	_, ok := s.TryStart("", "")
	if !ok {
		t.Fatal("tryStart with empty filters should succeed")
	}

	info := s.Info()
	if info.Domain != "" {
		t.Errorf("Domain should be empty, got %q", info.Domain)
	}
	if info.Method != "" {
		t.Errorf("Method should be empty, got %q", info.Method)
	}
}

// ============================================
// networkRecordingState.stop()
// ============================================

func TestNetworkRecordingState_Stop_Success(t *testing.T) {
	t.Parallel()
	var s netrecord.NetworkRecordingState

	origStart, _ := s.TryStart("example.com", "POST")

	snap, ok := s.Stop()
	if !ok {
		t.Fatal("stop should return true when active")
	}
	if !snap.Active {
		t.Error("snapshot Active should be true")
	}
	if snap.Domain != "example.com" {
		t.Errorf("snapshot Domain = %q, want %q", snap.Domain, "example.com")
	}
	if snap.Method != "POST" {
		t.Errorf("snapshot Method = %q, want %q", snap.Method, "POST")
	}
	if !snap.StartTime.Equal(origStart) {
		t.Errorf("snapshot StartTime = %v, want %v", snap.StartTime, origStart)
	}

	// After stop, all state fields should be reset
	info := s.Info()
	if info.Active {
		t.Error("state should be inactive after stop")
	}
	if !info.StartTime.IsZero() {
		t.Errorf("StartTime should be zero after stop, got %v", info.StartTime)
	}
	if info.Domain != "" {
		t.Errorf("Domain should be empty after stop, got %q", info.Domain)
	}
	if info.Method != "" {
		t.Errorf("Method should be empty after stop, got %q", info.Method)
	}
}

func TestNetworkRecordingState_Stop_NotActive(t *testing.T) {
	t.Parallel()
	var s netrecord.NetworkRecordingState

	snap, ok := s.Stop()
	if ok {
		t.Fatal("stop should return false when not active")
	}
	if snap.Active {
		t.Error("snapshot Active should be false when not active")
	}
	if !snap.StartTime.IsZero() {
		t.Errorf("snapshot StartTime should be zero, got %v", snap.StartTime)
	}
	if snap.Domain != "" {
		t.Errorf("snapshot Domain should be empty, got %q", snap.Domain)
	}
	if snap.Method != "" {
		t.Errorf("snapshot Method should be empty, got %q", snap.Method)
	}
}

func TestNetworkRecordingState_Stop_DoubleStop(t *testing.T) {
	t.Parallel()
	var s netrecord.NetworkRecordingState

	s.TryStart("example.com", "GET")

	_, ok := s.Stop()
	if !ok {
		t.Fatal("first stop should succeed")
	}

	_, ok = s.Stop()
	if ok {
		t.Fatal("second stop should fail")
	}
}

// ============================================
// networkRecordingState.info()
// ============================================

func TestNetworkRecordingState_Info_Active(t *testing.T) {
	t.Parallel()
	var s netrecord.NetworkRecordingState

	startTime, _ := s.TryStart("api.example.com", "PUT")

	info := s.Info()
	if !info.Active {
		t.Error("info Active should be true when recording")
	}
	if !info.StartTime.Equal(startTime) {
		t.Errorf("info StartTime = %v, want %v", info.StartTime, startTime)
	}
	if info.Domain != "api.example.com" {
		t.Errorf("info Domain = %q, want %q", info.Domain, "api.example.com")
	}
	if info.Method != "PUT" {
		t.Errorf("info Method = %q, want %q", info.Method, "PUT")
	}
}

func TestNetworkRecordingState_Info_Inactive(t *testing.T) {
	t.Parallel()
	var s netrecord.NetworkRecordingState

	info := s.Info()
	if info.Active {
		t.Error("info Active should be false when not recording")
	}
	if !info.StartTime.IsZero() {
		t.Error("info StartTime should be zero when not recording")
	}
	if info.Domain != "" {
		t.Errorf("info Domain should be empty, got %q", info.Domain)
	}
	if info.Method != "" {
		t.Errorf("info Method should be empty, got %q", info.Method)
	}
}

// ============================================
// Start → Stop → Start lifecycle
// ============================================

func TestNetworkRecordingState_StartStopRestart(t *testing.T) {
	t.Parallel()
	var s netrecord.NetworkRecordingState

	// First cycle
	_, ok := s.TryStart("first.com", "GET")
	if !ok {
		t.Fatal("first start should succeed")
	}

	snap, ok := s.Stop()
	if !ok {
		t.Fatal("first stop should succeed")
	}
	if snap.Domain != "first.com" {
		t.Errorf("first stop snapshot Domain = %q, want %q", snap.Domain, "first.com")
	}

	// Second cycle with different filters
	_, ok = s.TryStart("second.com", "POST")
	if !ok {
		t.Fatal("second start should succeed after stop")
	}

	info := s.Info()
	if info.Domain != "second.com" {
		t.Errorf("info Domain = %q, want %q", info.Domain, "second.com")
	}
	if info.Method != "POST" {
		t.Errorf("info Method = %q, want %q", info.Method, "POST")
	}
}

// ============================================
// Concurrent start/stop race test
// ============================================

func TestNetworkRecordingState_ConcurrentStartStop(t *testing.T) {
	t.Parallel()
	var s netrecord.NetworkRecordingState

	const goroutines = 100
	var wg sync.WaitGroup
	startSuccesses := make(chan bool, goroutines)
	stopSuccesses := make(chan bool, goroutines)

	// Half start, half stop concurrently
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			if n%2 == 0 {
				_, ok := s.TryStart("example.com", "GET")
				startSuccesses <- ok
			} else {
				_, ok := s.Stop()
				stopSuccesses <- ok
			}
		}(i)
	}

	wg.Wait()
	close(startSuccesses)
	close(stopSuccesses)

	// Count successes — the state machine must remain consistent.
	startCount := 0
	for ok := range startSuccesses {
		if ok {
			startCount++
		}
	}
	stopCount := 0
	for ok := range stopSuccesses {
		if ok {
			stopCount++
		}
	}

	// At least one start must succeed
	if startCount == 0 {
		t.Error("expected at least one successful start")
	}

	// Stops cannot exceed starts
	if stopCount > startCount {
		t.Errorf("stops (%d) should not exceed starts (%d)", stopCount, startCount)
	}

	// Difference should be 0 or 1 (either fully stopped or one active remains)
	diff := startCount - stopCount
	if diff < 0 || diff > 1 {
		t.Errorf("start/stop imbalance: starts=%d, stops=%d (diff should be 0 or 1)", startCount, stopCount)
	}

	// Final state must match the arithmetic
	info := s.Info()
	if diff == 0 && info.Active {
		t.Error("state should be inactive when all starts were stopped")
	}
	if diff == 1 && !info.Active {
		t.Error("state should be active when one start remains unstopped")
	}
}

func TestNetworkRecordingState_ConcurrentInfo(t *testing.T) {
	t.Parallel()
	var s netrecord.NetworkRecordingState
	s.TryStart("example.com", "GET")

	const goroutines = 50
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			snap := s.Info()
			// Must not panic and must return consistent snapshot
			if snap.Active && snap.Domain == "" {
				t.Error("active snapshot should have domain set")
			}
		}()
	}

	wg.Wait()
}

// ============================================
// netrecord.MatchesRecordingFilter() — table-driven
// ============================================

func TestMatchesRecordingFilter(t *testing.T) {
	t.Parallel()

	startTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		body      types.NetworkBody
		startTime time.Time
		domain    string
		method    string
		want      bool
	}{
		// --- No filters ---
		{
			name: "no filters — everything passes",
			body: types.NetworkBody{
				Timestamp: "2025-01-15T10:05:00Z",
				Method:    "GET",
				URL:       "https://example.com/api/data",
			},
			startTime: startTime,
			want:      true,
		},

		// --- Timestamp: RFC3339 ---
		{
			name: "RFC3339 timestamp after start — included",
			body: types.NetworkBody{
				Timestamp: "2025-01-15T10:05:00Z",
				Method:    "GET",
				URL:       "https://example.com/api",
			},
			startTime: startTime,
			want:      true,
		},
		{
			name: "RFC3339 timestamp before start — excluded",
			body: types.NetworkBody{
				Timestamp: "2025-01-15T09:55:00Z",
				Method:    "GET",
				URL:       "https://example.com/api",
			},
			startTime: startTime,
			want:      false,
		},
		{
			name: "RFC3339 timestamp equal to start — included (Before returns false for equal)",
			body: types.NetworkBody{
				Timestamp: "2025-01-15T10:00:00Z",
				Method:    "GET",
				URL:       "https://example.com/api",
			},
			startTime: startTime,
			want:      true,
		},

		// --- Timestamp: millisecond epoch ---
		{
			name: "ms-epoch timestamp after start — included",
			body: types.NetworkBody{
				Timestamp: "1736935500000", // 2025-01-15T10:05:00Z
				Method:    "GET",
				URL:       "https://example.com/api",
			},
			startTime: startTime,
			want:      true,
		},
		{
			name: "ms-epoch timestamp before start — excluded",
			body: types.NetworkBody{
				Timestamp: "1736934900000", // 2025-01-15T09:55:00Z
				Method:    "GET",
				URL:       "https://example.com/api",
			},
			startTime: startTime,
			want:      false,
		},

		// --- Timestamp: unparseable / empty ---
		{
			name: "unparseable timestamp — included (best-effort)",
			body: types.NetworkBody{
				Timestamp: "not-a-timestamp",
				Method:    "GET",
				URL:       "https://example.com/api",
			},
			startTime: startTime,
			want:      true,
		},
		{
			name: "empty timestamp — included",
			body: types.NetworkBody{
				Timestamp: "",
				Method:    "GET",
				URL:       "https://example.com/api",
			},
			startTime: startTime,
			want:      true,
		},
		{
			name: "zero startTime — timestamp not filtered",
			body: types.NetworkBody{
				Timestamp: "2020-01-01T00:00:00Z",
				Method:    "GET",
				URL:       "https://example.com/api",
			},
			startTime: time.Time{},
			want:      true,
		},

		// --- Domain filter: case-insensitive ---
		{
			name: "domain filter — matches case-insensitive (URL uppercase)",
			body: types.NetworkBody{
				Timestamp: "2025-01-15T10:05:00Z",
				Method:    "GET",
				URL:       "https://API.EXAMPLE.COM/data",
			},
			startTime: startTime,
			domain:    "api.example.com",
			want:      true,
		},
		{
			name: "domain filter — matches case-insensitive (filter uppercase)",
			body: types.NetworkBody{
				Timestamp: "2025-01-15T10:05:00Z",
				Method:    "GET",
				URL:       "https://api.example.com/data",
			},
			startTime: startTime,
			domain:    "API.EXAMPLE.COM",
			want:      true,
		},
		{
			name: "domain filter — partial match",
			body: types.NetworkBody{
				Timestamp: "2025-01-15T10:05:00Z",
				Method:    "GET",
				URL:       "https://api.example.com/data",
			},
			startTime: startTime,
			domain:    "example",
			want:      true,
		},
		{
			name: "domain filter — no match",
			body: types.NetworkBody{
				Timestamp: "2025-01-15T10:05:00Z",
				Method:    "GET",
				URL:       "https://other.com/api",
			},
			startTime: startTime,
			domain:    "example.com",
			want:      false,
		},
		{
			name: "domain filter — empty passes all",
			body: types.NetworkBody{
				Timestamp: "2025-01-15T10:05:00Z",
				Method:    "GET",
				URL:       "https://anything.com/path",
			},
			startTime: startTime,
			domain:    "",
			want:      true,
		},

		// --- Method filter: case-insensitive ---
		{
			name: "method filter — matches case-insensitive (body lowercase)",
			body: types.NetworkBody{
				Timestamp: "2025-01-15T10:05:00Z",
				Method:    "post",
				URL:       "https://example.com/api",
			},
			startTime: startTime,
			method:    "POST",
			want:      true,
		},
		{
			name: "method filter — matches case-insensitive (filter lowercase)",
			body: types.NetworkBody{
				Timestamp: "2025-01-15T10:05:00Z",
				Method:    "DELETE",
				URL:       "https://example.com/api",
			},
			startTime: startTime,
			method:    "delete",
			want:      true,
		},
		{
			name: "method filter — no match",
			body: types.NetworkBody{
				Timestamp: "2025-01-15T10:05:00Z",
				Method:    "GET",
				URL:       "https://example.com/api",
			},
			startTime: startTime,
			method:    "POST",
			want:      false,
		},
		{
			name: "method filter — empty passes all",
			body: types.NetworkBody{
				Timestamp: "2025-01-15T10:05:00Z",
				Method:    "PATCH",
				URL:       "https://example.com/api",
			},
			startTime: startTime,
			method:    "",
			want:      true,
		},

		// --- Combined filters ---
		{
			name: "all filters combined — all match",
			body: types.NetworkBody{
				Timestamp: "2025-01-15T10:05:00Z",
				Method:    "POST",
				URL:       "https://api.example.com/data",
			},
			startTime: startTime,
			domain:    "example.com",
			method:    "POST",
			want:      true,
		},
		{
			name: "all filters combined — domain mismatch",
			body: types.NetworkBody{
				Timestamp: "2025-01-15T10:05:00Z",
				Method:    "POST",
				URL:       "https://other.com/data",
			},
			startTime: startTime,
			domain:    "example.com",
			method:    "POST",
			want:      false,
		},
		{
			name: "all filters combined — method mismatch",
			body: types.NetworkBody{
				Timestamp: "2025-01-15T10:05:00Z",
				Method:    "GET",
				URL:       "https://api.example.com/data",
			},
			startTime: startTime,
			domain:    "example.com",
			method:    "POST",
			want:      false,
		},
		{
			name: "all filters combined — timestamp mismatch",
			body: types.NetworkBody{
				Timestamp: "2025-01-15T09:55:00Z",
				Method:    "POST",
				URL:       "https://api.example.com/data",
			},
			startTime: startTime,
			domain:    "example.com",
			method:    "POST",
			want:      false,
		},
		{
			name: "no filters at all — everything passes",
			body: types.NetworkBody{
				Method: "PATCH",
				URL:    "https://anything.com/any",
			},
			startTime: time.Time{},
			domain:    "",
			method:    "",
			want:      true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := netrecord.MatchesRecordingFilter(tc.body, tc.startTime, tc.domain, tc.method)
			if got != tc.want {
				t.Errorf("netrecord.MatchesRecordingFilter() = %v, want %v", got, tc.want)
			}
		})
	}
}
