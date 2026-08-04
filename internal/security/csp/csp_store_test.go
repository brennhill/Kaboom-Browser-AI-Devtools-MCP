// csp_store_test.go — Tests CSP observation accumulation and timestamps.
// Docs: docs/features/feature/security-hardening/index.md

package csp

import (
	"testing"
	"time"
)

func TestCSPOriginAccumulatorPersistsAfterBufferWrap(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()

	// Record an early origin
	gen.RecordOrigin("https://early-cdn.example.com", "script", "https://myapp.com/")
	gen.RecordOrigin("https://early-cdn.example.com", "script", "https://myapp.com/dashboard")
	gen.RecordOrigin("https://early-cdn.example.com", "script", "https://myapp.com/settings")

	// Simulate 2000 more requests from other origins (accumulator should keep all)
	for i := 0; i < 2000; i++ {
		gen.RecordOrigin("https://other.example.com", "connect", "https://myapp.com/page")
	}

	resp := gen.Generate(Params{Mode: "moderate"})

	// Early origin should still be present
	assertContains(t, resp.Directives["script-src"], "https://early-cdn.example.com")
}

func TestCSPObservationCountIncrements(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()

	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/")
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/")
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/")

	// Check internal state
	gen.mu.RLock()
	entry := gen.origins["https://cdn.example.com|script"]
	gen.mu.RUnlock()

	if entry == nil {
		t.Fatal("expected origin entry, got nil")
	}
	if entry.Count != 3 {
		t.Errorf("expected count=3, got %d", entry.Count)
	}
}

func TestCSPAccumulatorClearsOnReset(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()

	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/")

	gen.Reset()

	gen.mu.RLock()
	count := len(gen.origins)
	gen.mu.RUnlock()

	if count != 0 {
		t.Errorf("expected 0 origins after reset, got %d", count)
	}
}

// --- Confidence Scoring Tests ---

func TestCSPPagesVisitedCount(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()

	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/")
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/dashboard")
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/settings")
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/profile")
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/about")
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/contact")

	resp := gen.Generate(Params{Mode: "moderate"})

	if resp.Observations.PagesVisited != 6 {
		t.Errorf("expected pages_visited=6, got %d", resp.Observations.PagesVisited)
	}
}

func TestObservationsIncludeTotalResources(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()

	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/")
	gen.RecordOrigin("https://fonts.example.com", "font", "https://myapp.com/")
	gen.RecordOrigin("https://api.example.com", "connect", "https://myapp.com/")

	resp := gen.Generate(Params{Mode: "moderate"})

	if resp.Observations.TotalResources != 3 {
		t.Errorf("expected total_resources=3, got %d", resp.Observations.TotalResources)
	}
}

func TestObservationsUniqueOrigins(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()

	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/")
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/page2")
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/page3")
	gen.RecordOrigin("https://fonts.example.com", "font", "https://myapp.com/")
	gen.RecordOrigin("https://fonts.example.com", "font", "https://myapp.com/page2")
	gen.RecordOrigin("https://fonts.example.com", "font", "https://myapp.com/page3")

	resp := gen.Generate(Params{Mode: "moderate"})

	if resp.Observations.UniqueOrigins != 2 {
		t.Errorf("expected unique_origins=2, got %d", resp.Observations.UniqueOrigins)
	}
}

// --- Security Hardening Safety Tests ---

func TestCSPConcurrentAccess(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()

	done := make(chan bool, 10)

	// Write from multiple goroutines
	for i := 0; i < 5; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/page")
			}
			done <- true
		}(i)
	}

	// Read concurrently
	for i := 0; i < 5; i++ {
		go func() {
			gen.Generate(Params{Mode: "moderate"})
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

// --- Inline Scripts NOT Hashed Tests ---

func TestCSPFirstSeenTimestamp(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()

	before := time.Now()
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/")
	after := time.Now()

	gen.mu.RLock()
	entry := gen.origins["https://cdn.example.com|script"]
	gen.mu.RUnlock()

	if entry.FirstSeen.Before(before) || entry.FirstSeen.After(after) {
		t.Error("FirstSeen timestamp out of expected range")
	}
}

func TestCSPLastSeenUpdates(t *testing.T) {
	t.Parallel()
	gen := NewGenerator()
	current := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)
	gen.now = func() time.Time { return current }

	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/")
	current = current.Add(time.Millisecond)
	gen.RecordOrigin("https://cdn.example.com", "script", "https://myapp.com/dashboard")

	gen.mu.RLock()
	entry := gen.origins["https://cdn.example.com|script"]
	gen.mu.RUnlock()

	if !entry.LastSeen.After(entry.FirstSeen) {
		t.Error("expected LastSeen to be after FirstSeen")
	}
}

// --- Helper Functions ---

func assertContains(t *testing.T, slice []string, value string) {
	t.Helper()
	for _, s := range slice {
		if s == value {
			return
		}
	}
	t.Errorf("expected slice to contain %q, got %v", value, slice)
}

func assertNotContains(t *testing.T, slice []string, value string) {
	t.Helper()
	for _, s := range slice {
		if s == value {
			t.Errorf("expected slice to NOT contain %q", value)
			return
		}
	}
}

// --- RecordOriginFromBody Tests ---
