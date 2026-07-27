// Purpose: Tests for capture HTTP debug logging with redaction.
// Docs: docs/features/feature/backend-log-streaming/index.md

// debug_logger_test.go — Tests for the DebugLogger sub-struct.
// Verifies circular buffer behavior, concurrent safety, and snapshot isolation.
package capture

import (
	"sync"
	"testing"
	"time"
)

func TestDebugLogger_LogPollingActivity(t *testing.T) {
	t.Parallel()
	dl := NewDebugLogger()

	entry := PollingLogEntry{
		Timestamp: time.Now(),
		Endpoint:  "sync",
		Method:    "POST",
	}
	dl.LogPollingActivity(entry)

	// Verify entry was stored
	logs := dl.GetPollingLog()
	found := false
	for _, e := range logs {
		if e.Endpoint == "sync" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("Expected polling log to contain the sync entry")
	}
}

func TestDebugLogger_LogHTTPDebugEntry(t *testing.T) {
	t.Parallel()
	dl := NewDebugLogger()

	entry := HTTPDebugEntry{
		Timestamp: time.Now(),
		Endpoint:  "/settings",
		Method:    "POST",
	}
	dl.LogHTTPDebugEntry(entry)

	logs := dl.GetHTTPDebugLog()
	found := false
	for _, e := range logs {
		if e.Endpoint == "/settings" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("Expected HTTP debug log to contain the /settings entry")
	}
}

func TestDebugLogger_CircularBufferWrapping(t *testing.T) {
	t.Parallel()
	dl := NewDebugLogger()

	// Write 60 entries (exceeds buffer size of 50)
	for i := 0; i < 60; i++ {
		dl.LogHTTPDebugEntry(HTTPDebugEntry{
			Endpoint:       "/test",
			ResponseStatus: i,
		})
	}

	logs := dl.GetHTTPDebugLog()
	if len(logs) != 50 {
		t.Fatalf("Expected 50 entries, got %d", len(logs))
	}

	// After 60 writes into 50 slots, entries 0-9 are overwritten by 50-59.
	// No entry with ResponseStatus in [0,9] should survive.
	for _, e := range logs {
		if e.ResponseStatus >= 0 && e.ResponseStatus < 10 {
			t.Fatalf("Found overwritten entry with status %d — circular buffer did not wrap correctly", e.ResponseStatus)
		}
	}
	// Verify new entries (50-59) exist
	hasNew := false
	for _, e := range logs {
		if e.ResponseStatus >= 50 {
			hasNew = true
			break
		}
	}
	if !hasNew {
		t.Fatal("Expected entries with ResponseStatus >= 50")
	}
}

func TestDebugLogger_GetHTTPDebugLogReturnsCopy(t *testing.T) {
	t.Parallel()
	dl := NewDebugLogger()

	dl.LogHTTPDebugEntry(HTTPDebugEntry{
		Endpoint: "/original",
	})

	logs1 := dl.GetHTTPDebugLog()

	// Mutate the returned slice
	logs1[0].Endpoint = "/mutated"

	// Get again — should be unaffected
	logs2 := dl.GetHTTPDebugLog()
	found := false
	for _, e := range logs2 {
		if e.Endpoint == "/original" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("GetHTTPDebugLog did not return an independent copy — mutation leaked back")
	}
}

func TestDebugLogger_ConcurrentWrites(t *testing.T) {
	t.Parallel()
	dl := NewDebugLogger()
	var wg sync.WaitGroup

	// 10 goroutines writing 100 entries each
	for g := 0; g < 10; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				dl.LogHTTPDebugEntry(HTTPDebugEntry{
					Endpoint:       "/concurrent",
					ResponseStatus: id*100 + i,
				})
				dl.LogPollingActivity(PollingLogEntry{
					Endpoint: "sync",
				})
			}
		}(g)
	}

	wg.Wait()

	// Should not panic, and log should have exactly 50 entries
	logs := dl.GetHTTPDebugLog()
	if len(logs) != 50 {
		t.Fatalf("Expected 50 entries after concurrent writes, got %d", len(logs))
	}
}

func TestDebugLogger_PollingLogCircularWrapping(t *testing.T) {
	t.Parallel()
	dl := NewDebugLogger()

	for i := 0; i < 55; i++ {
		dl.LogPollingActivity(PollingLogEntry{
			Endpoint:   "sync",
			QueryCount: i,
		})
	}

	logs := dl.GetPollingLog()
	if len(logs) != 50 {
		t.Fatalf("Expected 50 entries, got %d", len(logs))
	}

	// Entries 0-4 should be overwritten by 50-54
	for _, e := range logs {
		if e.QueryCount > 0 && e.QueryCount < 5 {
			t.Fatalf("Found overwritten entry with QueryCount %d", e.QueryCount)
		}
	}
}
