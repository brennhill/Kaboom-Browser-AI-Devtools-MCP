// logger_test.go -- Tests for the Logger sub-struct.
// Verifies circular buffer behavior, concurrent safety, and snapshot isolation.
package debuglog

import (
	"sync"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/platform/types"
)

func TestLogger_LogPollingActivity(t *testing.T) {
	t.Parallel()
	dl := NewLogger()

	entry := types.PollingLogEntry{
		Timestamp: time.Now(),
		Endpoint:  "sync",
		Method:    "POST",
	}
	dl.LogPollingActivity(entry)

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

func TestLogger_LogHTTPDebugEntry(t *testing.T) {
	t.Parallel()
	dl := NewLogger()

	entry := types.HTTPDebugEntry{
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

func TestLogger_CircularBufferWrapping(t *testing.T) {
	t.Parallel()
	dl := NewLogger()

	for i := 0; i < 60; i++ {
		dl.LogHTTPDebugEntry(types.HTTPDebugEntry{
			Endpoint:       "/test",
			ResponseStatus: i,
		})
	}

	logs := dl.GetHTTPDebugLog()
	if len(logs) != 50 {
		t.Fatalf("Expected 50 entries, got %d", len(logs))
	}

	for _, e := range logs {
		if e.ResponseStatus >= 0 && e.ResponseStatus < 10 {
			t.Fatalf("Found overwritten entry with status %d", e.ResponseStatus)
		}
	}
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

func TestLogger_GetHTTPDebugLogReturnsCopy(t *testing.T) {
	t.Parallel()
	dl := NewLogger()

	dl.LogHTTPDebugEntry(types.HTTPDebugEntry{
		Endpoint: "/original",
	})

	logs1 := dl.GetHTTPDebugLog()
	logs1[0].Endpoint = "/mutated"

	logs2 := dl.GetHTTPDebugLog()
	found := false
	for _, e := range logs2 {
		if e.Endpoint == "/original" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("GetHTTPDebugLog did not return an independent copy")
	}
}

func TestLogger_ConcurrentWrites(t *testing.T) {
	t.Parallel()
	dl := NewLogger()
	var wg sync.WaitGroup

	for g := 0; g < 10; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				dl.LogHTTPDebugEntry(types.HTTPDebugEntry{
					Endpoint:       "/concurrent",
					ResponseStatus: id*100 + i,
				})
				dl.LogPollingActivity(types.PollingLogEntry{
					Endpoint: "sync",
				})
			}
		}(g)
	}

	wg.Wait()

	logs := dl.GetHTTPDebugLog()
	if len(logs) != 50 {
		t.Fatalf("Expected 50 entries after concurrent writes, got %d", len(logs))
	}
}

func TestLogger_PollingLogCircularWrapping(t *testing.T) {
	t.Parallel()
	dl := NewLogger()

	for i := 0; i < 55; i++ {
		dl.LogPollingActivity(types.PollingLogEntry{
			Endpoint:   "sync",
			QueryCount: i,
		})
	}

	logs := dl.GetPollingLog()
	if len(logs) != 50 {
		t.Fatalf("Expected 50 entries, got %d", len(logs))
	}

	for _, e := range logs {
		if e.QueryCount > 0 && e.QueryCount < 5 {
			t.Fatalf("Found overwritten entry with QueryCount %d", e.QueryCount)
		}
	}
}
