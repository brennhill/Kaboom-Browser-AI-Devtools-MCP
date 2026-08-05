// intent_store_test.go -- Tests for intent store: TTL, capacity, consume, dedup.
// Docs: docs/features/feature/auto-fix/index.md

package intent

import (
	"strings"
	"testing"
	"time"
)

func TestStore_AddAndPending(t *testing.T) {
	t.Parallel()
	s := NewStore()

	id := s.Add("http://localhost:3000", "qa_scan")
	if !strings.HasPrefix(id, "intent_") {
		t.Errorf("correlation ID should start with intent_, got %q", id)
	}

	pending := s.Pending()
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending, got %d", len(pending))
	}
	if pending[0].CorrelationID != id {
		t.Errorf("got %q, want %q", pending[0].CorrelationID, id)
	}
	if pending[0].PageURL != "http://localhost:3000" {
		t.Errorf("page_url = %q, want http://localhost:3000", pending[0].PageURL)
	}
}

func TestStore_ConsumeRemoves(t *testing.T) {
	t.Parallel()
	s := NewStore()

	id := s.Add("http://localhost:3000", "qa_scan")
	it := s.Consume(id)
	if it == nil {
		t.Fatal("expected intent, got nil")
	}
	if it.CorrelationID != id {
		t.Errorf("got %q, want %q", it.CorrelationID, id)
	}

	// Second consume returns nil
	if s.Consume(id) != nil {
		t.Error("expected nil on second consume")
	}

	// Pending should be empty
	if len(s.Pending()) != 0 {
		t.Errorf("expected 0 pending after consume, got %d", len(s.Pending()))
	}
}

func TestStore_ConsumeAll(t *testing.T) {
	t.Parallel()
	s := NewStore()

	s.Add("http://localhost:3000/a", "qa_scan")
	s.Add("http://localhost:3000/b", "qa_scan")

	all := s.ConsumeAll()
	if len(all) != 2 {
		t.Fatalf("expected 2, got %d", len(all))
	}
	if len(s.Pending()) != 0 {
		t.Error("expected 0 pending after consume all")
	}
}

func TestStore_MaxCapacity(t *testing.T) {
	t.Parallel()
	s := NewStore()

	s.Add("http://a", "qa_scan")
	s.Add("http://b", "qa_scan")
	s.Add("http://c", "qa_scan")
	id4 := s.Add("http://d", "qa_scan")

	pending := s.Pending()
	if len(pending) != MaxCount {
		t.Fatalf("expected %d, got %d", MaxCount, len(pending))
	}

	// Oldest (http://a) should have been evicted; newest (http://d) should be present
	for _, p := range pending {
		if p.PageURL == "http://a" {
			t.Error("oldest intent should have been evicted")
		}
	}
	found := false
	for _, p := range pending {
		if p.CorrelationID == id4 {
			found = true
		}
	}
	if !found {
		t.Error("newest intent should be present")
	}
}

func TestStore_TTLExpiry(t *testing.T) {
	t.Parallel()
	s := NewStore()

	// Manually insert an expired intent
	s.mu.Lock()
	s.items = append(s.items, Intent{
		CorrelationID: "intent_expired",
		PageURL:       "http://old",
		Action:        "qa_scan",
		CreatedAt:     time.Now().Add(-10 * time.Minute).Unix(),
	})
	s.mu.Unlock()

	// Expired intent should not appear
	pending := s.Pending()
	if len(pending) != 0 {
		t.Errorf("expected 0 pending (expired), got %d", len(pending))
	}

	// Consume should return nil for expired
	if s.Consume("intent_expired") != nil {
		t.Error("should not consume expired intent")
	}
}

func TestStore_ConsumeNonExistent(t *testing.T) {
	t.Parallel()
	s := NewStore()

	if s.Consume("nonexistent") != nil {
		t.Error("should return nil for nonexistent ID")
	}
}

func TestStore_EmptyPending(t *testing.T) {
	t.Parallel()
	s := NewStore()

	pending := s.Pending()
	if pending == nil {
		t.Error("pending should return empty slice, not nil")
	}
	if len(pending) != 0 {
		t.Errorf("expected 0, got %d", len(pending))
	}
}

func TestStore_NudgeAndClean(t *testing.T) {
	t.Parallel()
	s := NewStore()
	s.Add("http://localhost:3000", ActionQAScan)

	// First nudge — should return true (intent still active)
	for i := 0; i < MaxNudges; i++ {
		if !s.NudgeAndClean() {
			t.Fatalf("nudge %d should return true", i+1)
		}
	}

	// After max nudges, intent should be removed
	if s.NudgeAndClean() {
		t.Error("should return false after max nudges exhausted")
	}
	if len(s.Pending()) != 0 {
		t.Error("expected 0 pending after max nudges")
	}
}

func TestStore_NudgeAndClean_EmptyFastPath(t *testing.T) {
	t.Parallel()
	s := NewStore()

	// Should return false immediately without acquiring lock
	if s.NudgeAndClean() {
		t.Error("should return false when empty")
	}
}

func TestGenerateCorrelationID_Unique(t *testing.T) {
	t.Parallel()
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := GenerateCorrelationID()
		if seen[id] {
			t.Fatalf("duplicate correlation ID: %s", id)
		}
		seen[id] = true
	}
}
