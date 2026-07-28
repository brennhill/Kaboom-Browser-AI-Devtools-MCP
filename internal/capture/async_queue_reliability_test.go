// Purpose: Tests for capture synchronization protocol and data delivery.
// Docs: docs/features/feature/backend-log-streaming/index.md

// async_queue_reliability_test.go — Test async queue under various timing conditions
// Ensures commands don't expire before extension can poll them.
package capture

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

// TestAsyncQueueReliability tests that commands survive timing jitter
func TestAsyncQueueReliability(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		commandCount   int
		pollInterval   time.Duration
		pollJitter     time.Duration
		expectedSucces float64 // 0.0 to 1.0
	}{
		{
			name:           "Normal polling (100ms interval, no jitter)",
			commandCount:   4,
			pollInterval:   40 * time.Millisecond,
			pollJitter:     0,
			expectedSucces: 1.0,
		},
		{
			name:           "Polling with jitter (100ms ± 50ms)",
			commandCount:   4,
			pollInterval:   40 * time.Millisecond,
			pollJitter:     15 * time.Millisecond,
			expectedSucces: 1.0,
		},
		{
			name:           "Slow polling (300ms interval)",
			commandCount:   4,
			pollInterval:   80 * time.Millisecond,
			pollJitter:     0,
			expectedSucces: 1.0,
		},
		{
			name:           "Rapid commands (within queue limit)",
			commandCount:   8,
			pollInterval:   20 * time.Millisecond,
			pollJitter:     0,
			expectedSucces: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			capture := NewCapture()
			defer capture.Close()

			// Background cleanup runs automatically in NewCapture()

			var mu sync.Mutex
			commandsSent := 0
			commandsReceived := 0
			commandsExpired := 0

			// Simulate extension polling
			stopPolling := make(chan bool)
			pollingDone := make(chan bool)

			go func() {
				ticker := time.NewTicker(tt.pollInterval)
				defer ticker.Stop()
				defer close(pollingDone)

				for {
					select {
					case <-stopPolling:
						return
					case <-ticker.C:
						// Add jitter if specified
						if tt.pollJitter > 0 {
							jitter := time.Duration(float64(tt.pollJitter) * (2.0*float64(time.Now().UnixNano()%1000)/1000.0 - 1.0))
							time.Sleep(jitter)
						}

						// Poll for queries
						pendingQueries := capture.Queries().GetPendingQueries()

						mu.Lock()
						commandsReceived += len(pendingQueries)
						mu.Unlock()

						// Simulate extension processing and posting results
						for _, query := range pendingQueries {
							result := json.RawMessage(`{"success": true}`)
							capture.Queries().SetQueryResult(query.ID, result)
						}
					}
				}
			}()

			// Send commands at poll interval to match extension's pickup rate
			// This prevents queue overflow (max size = 5)
			commandInterval := tt.pollInterval
			for i := 0; i < tt.commandCount; i++ {
				query := queries.PendingQuery{
					Type:          "execute",
					Params:        json.RawMessage(fmt.Sprintf(`{"script":"console.log(%d)"}`, i)),
					CorrelationID: fmt.Sprintf("test_%d", i),
				}

				capture.Queries().CreatePendingQueryWithTimeout(query, 5*time.Second, "")

				mu.Lock()
				commandsSent++
				mu.Unlock()

				time.Sleep(commandInterval)
			}

			// Wait for polling to catch up with a bounded deadline.
			deadline := time.Now().Add(tt.pollInterval*time.Duration(tt.commandCount+4) + 300*time.Millisecond)
			for time.Now().Before(deadline) {
				if len(capture.Queries().GetPendingQueries()) == 0 {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}

			// Stop polling
			close(stopPolling)
			<-pollingDone

			// Check for expired commands
			commandsExpired = capture.Queries().QueueDepth()

			mu.Lock()
			sent := commandsSent
			received := commandsReceived
			mu.Unlock()

			successRate := float64(received) / float64(sent)

			t.Logf("Commands sent: %d, received: %d, expired: %d, success rate: %.2f%%",
				sent, received, commandsExpired, successRate*100)

			if successRate < tt.expectedSucces {
				t.Errorf("Success rate %.2f%% is below expected %.2f%%",
					successRate*100, tt.expectedSucces*100)
			}

			if commandsExpired > 0 {
				t.Errorf("Found %d expired commands (should be 0 with 5s timeout)", commandsExpired)
			}
		})
	}
}

// TestAsyncQueueTimeout verifies that commands expire after their timeout.
// Uses a short timeout (3s) to test the mechanism without slowing the suite.
func TestAsyncQueueTimeout(t *testing.T) {
	t.Parallel()
	capture := NewCapture()
	defer capture.Close()

	query := queries.PendingQuery{
		Type:          "execute",
		Params:        json.RawMessage(`{"script":"test"}`),
		CorrelationID: "timeout_test",
	}

	id, _ := capture.Queries().CreatePendingQueryWithTimeout(query, 250*time.Millisecond, "")

	// Wait slightly less than timeout
	time.Sleep(120 * time.Millisecond)

	// Should still be in queue
	pendingQueries := capture.Queries().GetPendingQueries()
	if len(pendingQueries) != 1 {
		t.Errorf("Expected 1 pending query before timeout, got %d", len(pendingQueries))
	}

	// Wait for expiration with polling (avoid long fixed sleeps).
	expired := false
	deadline := time.Now().Add(700 * time.Millisecond)
	for time.Now().Before(deadline) {
		pendingQueries = capture.Queries().GetPendingQueries()
		if len(pendingQueries) == 0 {
			expired = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !expired {
		t.Errorf("Expected pending query to expire within deadline, still have %d pending", len(pendingQueries))
	}

	// Result should not exist
	_, found := capture.Queries().TakeQueryResult(id)
	if found {
		t.Error("Result should not exist for expired query")
	}
}

// TestAsyncQueueConcurrentAccess tests thread safety under concurrent load
func TestAsyncQueueConcurrentAccess(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	// Background cleanup runs automatically in NewCapture()

	const numGoroutines = 10
	const commandsPerGoroutine = 20

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*commandsPerGoroutine)

	// Spawn multiple goroutines creating commands
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < commandsPerGoroutine; j++ {
				query := queries.PendingQuery{
					Type:          "execute",
					Params:        json.RawMessage(fmt.Sprintf(`{"id":%d}`, j)),
					CorrelationID: fmt.Sprintf("g%d_cmd%d", goroutineID, j),
				}
				capture.Queries().CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, "")
			}
		}(i)
	}

	// Spawn multiple goroutines polling
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < commandsPerGoroutine; j++ {
				pendingQueries := capture.Queries().GetPendingQueries()
				for _, query := range pendingQueries {
					result := json.RawMessage(`{"ok":true}`)
					capture.Queries().SetQueryResult(query.ID, result)
				}
				time.Sleep(10 * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Error(err)
	}

	// All commands should have been processed or expired
	pendingQueries := capture.Queries().GetPendingQueries()
	if len(pendingQueries) > 5 {
		t.Errorf("Too many pending queries after concurrent test: %d (max 5 due to queue limit)", len(pendingQueries))
	}
}

// BenchmarkAsyncQueue measures queue throughput
func BenchmarkAsyncQueue(b *testing.B) {
	capture := NewCapture()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		query := queries.PendingQuery{
			Type:          "execute",
			Params:        json.RawMessage(`{"script":"test"}`),
			CorrelationID: fmt.Sprintf("bench_%d", i),
		}
		capture.Queries().CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, "")

		// Simulate extension picking it up
		pendingQueries := capture.Queries().GetPendingQueries()
		if len(pendingQueries) > 0 {
			result := json.RawMessage(`{"ok":true}`)
			capture.Queries().SetQueryResult(pendingQueries[0].ID, result)
		}
	}
}
