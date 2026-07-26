// Purpose: Tests for session client registry tracking.
// Docs: docs/features/feature/pagination/index.md

package clientreg

import (
	"sync"
	"testing"
	"time"
)

// ============================================
// DeriveClientID
// ============================================

func TestDeriveClientID_Deterministic(t *testing.T) {
	t.Parallel()
	id1 := DeriveClientID("/Users/alice/project")
	id2 := DeriveClientID("/Users/alice/project")
	if id1 != id2 {
		t.Errorf("Same CWD should produce same ID: %s != %s", id1, id2)
	}
}

func TestDeriveClientID_UniquePerCWD(t *testing.T) {
	t.Parallel()
	id1 := DeriveClientID("/Users/alice/project-a")
	id2 := DeriveClientID("/Users/alice/project-b")
	if id1 == id2 {
		t.Errorf("Different CWDs should produce different IDs: both = %s", id1)
	}
}

func TestDeriveClientID_Length(t *testing.T) {
	t.Parallel()
	id := DeriveClientID("/some/path")
	if len(id) != clientIDLength {
		t.Errorf("Expected ID length %d, got %d (%s)", clientIDLength, len(id), id)
	}
}

func TestDeriveClientID_EmptyInput(t *testing.T) {
	t.Parallel()
	id := DeriveClientID("")
	if id != "" {
		t.Errorf("Empty CWD should produce empty ID, got %s", id)
	}
}

// ============================================
// ClientState
// ============================================

func TestNewClientState(t *testing.T) {
	t.Parallel()
	cs := NewClientState("/Users/alice/project")

	if cs.ID == "" {
		t.Error("Client ID should not be empty")
	}
	if cs.CWD != "/Users/alice/project" {
		t.Errorf("CWD mismatch: %s", cs.CWD)
	}
	if cs.CheckpointPrefix != cs.ID+":" {
		t.Errorf("Checkpoint prefix mismatch: %s", cs.CheckpointPrefix)
	}
	if cs.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
	if cs.LastSeenAt.IsZero() {
		t.Error("LastSeenAt should be set")
	}
}

func TestClientState_Touch(t *testing.T) {
	t.Parallel()
	cs := NewClientState("/test")
	before := cs.GetLastSeen()

	time.Sleep(1 * time.Millisecond)
	cs.Touch()

	after := cs.GetLastSeen()
	if !after.After(before) {
		t.Error("Touch should update LastSeenAt")
	}
}

func TestClientState_CursorUpdates(t *testing.T) {
	t.Parallel()
	cs := NewClientState("/test")

	// WS cursor
	cursor := BufferCursor{Position: 42, Timestamp: time.Now()}
	cs.UpdateWSCursor(cursor)
	got := cs.GetWSCursor()
	if got.Position != 42 {
		t.Errorf("WS cursor position: expected 42, got %d", got.Position)
	}

	// Network cursor
	cursor2 := BufferCursor{Position: 100, Timestamp: time.Now()}
	cs.UpdateNetworkCursor(cursor2)
	got2 := cs.GetNetworkCursor()
	if got2.Position != 100 {
		t.Errorf("Network cursor position: expected 100, got %d", got2.Position)
	}

	// Action cursor
	cursor3 := BufferCursor{Position: 7, Timestamp: time.Now()}
	cs.UpdateActionCursor(cursor3)
	got3 := cs.GetActionCursor()
	if got3.Position != 7 {
		t.Errorf("Action cursor position: expected 7, got %d", got3.Position)
	}
}

// ============================================
// ClientRegistry: Basic Operations
// ============================================

func TestClientRegistry_RegisterAndGet(t *testing.T) {
	t.Parallel()
	r := NewClientRegistry()

	cs := r.Register("/Users/alice/project")
	if cs == nil {
		t.Fatal("Register returned nil")
	}

	got := r.Get(cs.ID)
	if got == nil {
		t.Fatal("Get returned nil for registered client")
	}
	if got.CWD != "/Users/alice/project" {
		t.Errorf("CWD mismatch: %s", got.CWD)
	}
}

func TestClientRegistry_GetNonexistent(t *testing.T) {
	t.Parallel()
	r := NewClientRegistry()
	got := r.Get("nonexistent-id")
	if got != nil {
		t.Error("Get should return nil for nonexistent client")
	}
}

func TestClientRegistry_RegisterIdempotent(t *testing.T) {
	t.Parallel()
	r := NewClientRegistry()

	cs1 := r.Register("/Users/alice/project")
	cs2 := r.Register("/Users/alice/project")

	if cs1.ID != cs2.ID {
		t.Error("Re-registering same CWD should return same client")
	}
	if r.Count() != 1 {
		t.Errorf("Expected 1 client, got %d", r.Count())
	}
}

func TestClientRegistry_Unregister(t *testing.T) {
	t.Parallel()
	r := NewClientRegistry()

	cs := r.Register("/Users/alice/project")
	r.Unregister(cs.ID)

	if r.Count() != 0 {
		t.Errorf("Expected 0 clients after unregister, got %d", r.Count())
	}
	if r.Get(cs.ID) != nil {
		t.Error("Get should return nil after unregister")
	}
}

func TestClientRegistry_UnregisterNonexistent(t *testing.T) {
	t.Parallel()
	r := NewClientRegistry()
	// Should not panic
	r.Unregister("nonexistent-id")
}

func TestClientRegistry_List(t *testing.T) {
	t.Parallel()
	r := NewClientRegistry()
	r.Register("/project-a")
	r.Register("/project-b")
	r.Register("/project-c")

	list := r.List()
	if len(list) != 3 {
		t.Fatalf("Expected 3 clients, got %d", len(list))
	}

	// Check that all have non-empty fields
	for _, info := range list {
		if info.ID == "" {
			t.Error("Client ID should not be empty in list")
		}
		if info.CreatedAt == "" {
			t.Error("CreatedAt should not be empty in list")
		}
		if info.LastSeenAt == "" {
			t.Error("LastSeenAt should not be empty in list")
		}
	}
}

func TestClientRegistry_Count(t *testing.T) {
	t.Parallel()
	r := NewClientRegistry()
	if r.Count() != 0 {
		t.Errorf("Expected 0, got %d", r.Count())
	}

	r.Register("/a")
	r.Register("/b")
	if r.Count() != 2 {
		t.Errorf("Expected 2, got %d", r.Count())
	}

	cs := r.Register("/a") // re-register
	r.Unregister(cs.ID)
	if r.Count() != 1 {
		t.Errorf("Expected 1, got %d", r.Count())
	}
}

// ============================================
// ClientRegistry: LRU Eviction
// ============================================

func TestClientRegistry_LRUEviction(t *testing.T) {
	t.Parallel()
	r := NewClientRegistry()

	// Register maxClients clients
	ids := make([]string, 0, maxClients+1)
	for i := 0; i < maxClients; i++ {
		cs := r.Register(string(rune('a' + i)))
		ids = append(ids, cs.ID)
	}

	if r.Count() != maxClients {
		t.Fatalf("Expected %d clients, got %d", maxClients, r.Count())
	}

	// Register one more - should evict the oldest (first registered)
	cs := r.Register("/overflow")
	ids = append(ids, cs.ID)

	if r.Count() != maxClients {
		t.Errorf("Expected %d clients after eviction, got %d", maxClients, r.Count())
	}

	// First registered client should be evicted
	if r.Get(ids[0]) != nil {
		t.Error("Oldest client should have been evicted")
	}

	// Most recent should still be there
	if r.Get(cs.ID) == nil {
		t.Error("Newest client should still be present")
	}
}

func TestClientRegistry_LRUEvictionRespectsAccess(t *testing.T) {
	t.Parallel()
	r := NewClientRegistry()

	// Register maxClients clients
	ids := make([]string, 0, maxClients)
	for i := 0; i < maxClients; i++ {
		cs := r.Register(string(rune('a' + i)))
		ids = append(ids, cs.ID)
	}

	// Access the first client to move it to end of LRU
	r.Get(ids[0])

	// Register one more - should evict the second (now oldest in access order)
	r.Register("/overflow")

	// First client was accessed so should survive
	if r.Get(ids[0]) == nil {
		t.Error("Recently accessed client should not be evicted")
	}

	// Second client should be evicted
	if r.Get(ids[1]) != nil {
		t.Error("Least recently used client should be evicted")
	}
}

// ============================================
// ClientRegistry: GetOrDefault
// ============================================

func TestClientRegistry_GetOrDefault_EmptyID(t *testing.T) {
	t.Parallel()
	r := NewClientRegistry()
	cs := r.GetOrDefault("")

	if cs == nil {
		t.Fatal("GetOrDefault should return non-nil for empty ID")
	}
	if cs.CheckpointPrefix != "" {
		t.Error("Default client should have empty checkpoint prefix")
	}
}

func TestClientRegistry_GetOrDefault_ExistingClient(t *testing.T) {
	t.Parallel()
	r := NewClientRegistry()
	registered := r.Register("/test")

	got := r.GetOrDefault(registered.ID)
	if got.CWD != "/test" {
		t.Errorf("Should return existing client, got CWD=%s", got.CWD)
	}
}

func TestClientRegistry_GetOrDefault_UnknownID(t *testing.T) {
	t.Parallel()
	r := NewClientRegistry()
	got := r.GetOrDefault("unknown-id")

	if got == nil {
		t.Fatal("GetOrDefault should return non-nil")
	}
	if got.ID != "unknown-id" {
		t.Errorf("Should create client with provided ID, got %s", got.ID)
	}
	if got.CheckpointPrefix != "unknown-id:" {
		t.Errorf("Should set checkpoint prefix, got %s", got.CheckpointPrefix)
	}
}

// ============================================
// ClientRegistry: Idle Reaping
// ============================================

func TestClientRegistry_ReapIdle_RemovesStaleClients(t *testing.T) {
	t.Parallel()
	r := NewClientRegistry()

	// Register two clients.
	cs1 := r.Register("/stale")
	cs2 := r.Register("/fresh")

	// Backdate cs1's LastSeenAt to exceed the idle timeout.
	cs1.mu.Lock()
	cs1.LastSeenAt = time.Now().Add(-clientIdleTimeout - time.Minute)
	cs1.mu.Unlock()

	// cs2 stays fresh (just registered).
	reaped := r.ReapIdle()
	if reaped != 1 {
		t.Errorf("expected 1 reaped, got %d", reaped)
	}
	if r.Count() != 1 {
		t.Errorf("expected 1 client remaining, got %d", r.Count())
	}
	if r.Get(cs1.ID) != nil {
		t.Error("stale client should have been reaped")
	}
	if r.Get(cs2.ID) == nil {
		t.Error("fresh client should still be present")
	}
}

func TestClientRegistry_ReapIdle_NothingToReap(t *testing.T) {
	t.Parallel()
	r := NewClientRegistry()
	r.Register("/a")
	r.Register("/b")

	reaped := r.ReapIdle()
	if reaped != 0 {
		t.Errorf("expected 0 reaped, got %d", reaped)
	}
	if r.Count() != 2 {
		t.Errorf("expected 2 clients, got %d", r.Count())
	}
}

// ============================================
// ClientRegistry: Concurrency
// ============================================

func TestClientRegistry_ConcurrentAccess(t *testing.T) {
	t.Parallel()
	r := NewClientRegistry()
	var wg sync.WaitGroup

	// Concurrent registrations
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			cwd := string(rune('a' + (i % maxClients)))
			cs := r.Register(cwd)
			if cs == nil {
				t.Errorf("Register returned nil for goroutine %d", i)
				return
			}
			r.Get(cs.ID)
		}(i)
	}

	// Concurrent list/count
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.List()
			r.Count()
		}()
	}

	wg.Wait()

	// Should not panic or deadlock; verify count is sane
	if r.Count() > maxClients {
		t.Errorf("Client count %d exceeds max %d", r.Count(), maxClients)
	}
}

func TestClientRegistry_ConcurrentRegisterUnregister(t *testing.T) {
	t.Parallel()
	r := NewClientRegistry()
	var wg sync.WaitGroup

	// Rapid register/unregister cycles
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			cwd := string(rune('a' + (i % 5)))
			cs := r.Register(cwd)
			time.Sleep(time.Millisecond)
			r.Unregister(cs.ID)
		}(i)
	}

	wg.Wait()
	// If we get here without panic/deadlock, test passes
}
