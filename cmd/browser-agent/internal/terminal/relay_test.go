// relay_test.go -- Tests for the per-session relay map, relay accessors,
// subscriber-ID generation, and prompt-wait injection.

package terminal

import (
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/pty"
)

// startCatRelay starts a deterministic `cat` PTY session and returns its relay.
// `cat` echoes stdin verbatim, so relay I/O is fully predictable.
func startCatRelay(t *testing.T, mgr *pty.Manager, id string) (*Map, *Relay) {
	t.Helper()
	res, err := mgr.Start(pty.StartConfig{ID: id, Cmd: "/bin/sh", Args: []string{"-c", "exec cat"}})
	if err != nil {
		t.Fatalf("start %s: %v", id, err)
	}
	sess, err := mgr.Get(res.SessionID)
	if err != nil {
		t.Fatalf("get %s: %v", id, err)
	}
	relays := NewMap()
	relay := relays.GetOrCreate(res.SessionID, sess, "/work/dir")
	return relays, relay
}

func TestNextWSSubID_UniqueAndPrefixed(t *testing.T) {
	t.Parallel()
	seen := make(map[string]bool, 128)
	for i := 0; i < 128; i++ {
		id := NextWSSubID()
		if !strings.HasPrefix(id, "ws-") {
			t.Fatalf("id %q missing ws- prefix", id)
		}
		if seen[id] {
			t.Fatalf("duplicate sub id %q", id)
		}
		seen[id] = true
	}
}

func TestMap_EmptyOperations(t *testing.T) {
	t.Parallel()
	m := NewMap()
	if m.Get("missing") != nil {
		t.Fatal("Get on empty map should return nil")
	}
	if m.WriteToFirst([]byte("data")) {
		t.Fatal("WriteToFirst on empty map should return false")
	}
	m.CloseAll()        // must not panic
	m.Remove("missing") // must not panic
}

func TestMap_GetOrCreateIsIdempotent(t *testing.T) {
	t.Parallel()
	mgr := pty.NewManager()
	defer mgr.StopAll()

	relays, relay := startCatRelay(t, mgr, "idem")
	// Second call returns the same relay instance (no duplicate reader loop).
	again := relays.GetOrCreate("idem", relay.sess, "/other")
	if again != relay {
		t.Fatal("GetOrCreate should return the existing relay")
	}
	if got := relays.Get("idem"); got != relay {
		t.Fatal("Get should return the created relay")
	}
}

func TestRelay_Accessors(t *testing.T) {
	t.Parallel()
	mgr := pty.NewManager()
	defer mgr.StopAll()

	_, relay := startCatRelay(t, mgr, "accessors")
	if relay.Fanout() == nil {
		t.Fatal("Fanout() returned nil")
	}
	if relay.WriteBuf() == nil {
		t.Fatal("WriteBuf() returned nil")
	}
	if relay.WorkspaceDir() != "/work/dir" {
		t.Fatalf("WorkspaceDir() = %q, want /work/dir", relay.WorkspaceDir())
	}
	// ExitCode is the zero value until the session exits; just confirm it reads.
	_ = relay.ExitCode()
}

func TestMap_WriteToFirstAndCloseAll(t *testing.T) {
	t.Parallel()
	mgr := pty.NewManager()
	defer mgr.StopAll()

	relays, _ := startCatRelay(t, mgr, "writer")
	if !relays.WriteToFirst([]byte("hello\n")) {
		t.Fatal("WriteToFirst should succeed with an active relay")
	}
	relays.CloseAll()
	if relays.Get("writer") != nil {
		t.Fatal("CloseAll should remove all relays")
	}
}

func TestMap_RemoveClosesRelay(t *testing.T) {
	t.Parallel()
	mgr := pty.NewManager()
	defer mgr.StopAll()

	relays, _ := startCatRelay(t, mgr, "removeme")
	relays.Remove("removeme")
	if relays.Get("removeme") != nil {
		t.Fatal("Remove should delete the relay")
	}
}

// TestWaitForPromptViaRelay_DetectsPrompt drives a live `cat` relay, feeding
// prompt characters until the helper injects its init command and returns.
func TestWaitForPromptViaRelay_DetectsPrompt(t *testing.T) {
	t.Parallel()
	mgr := pty.NewManager()
	defer mgr.StopAll()

	_, relay := startCatRelay(t, mgr, "prompt")

	done := make(chan struct{})
	go func() {
		WaitForPromptViaRelay(relay, "INIT_MARKER")
		close(done)
	}()

	stop := make(chan struct{})
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
				// cat echoes "$", which PromptChars recognizes as a prompt.
				_, _ = relay.WriteBuf().Write([]byte("$"))
				runtime.Gosched()
			}
		}
	}()

	select {
	case <-done:
		close(stop)
	case <-time.After(5 * time.Second):
		close(stop)
		t.Fatal("WaitForPromptViaRelay did not return after prompt detection")
	}
}

// TestWaitForPromptViaRelay_ClosedSession covers the path where the session has
// already ended, so the fanout subscription fails or closes and the helper
// returns promptly without waiting for the full InitTimeout.
func TestWaitForPromptViaRelay_ClosedSession(t *testing.T) {
	t.Parallel()
	mgr := pty.NewManager()
	defer mgr.StopAll()

	relays, relay := startCatRelay(t, mgr, "closed")
	// End the session: readLoop exits and closes the fanout.
	if err := mgr.Stop("closed"); err != nil {
		t.Fatalf("stop: %v", err)
	}
	relays.Remove("closed")

	done := make(chan struct{})
	go func() {
		WaitForPromptViaRelay(relay, "INIT_MARKER")
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("WaitForPromptViaRelay did not return for a closed session")
	}
}
