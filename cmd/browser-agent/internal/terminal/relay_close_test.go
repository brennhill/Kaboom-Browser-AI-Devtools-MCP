// relay_close_test.go — Relay.Close must actually tear the relay down (finding
// S5). It used to close only the write buffer: readLoop kept blocking on
// sess.Read, the PTY session stayed alive, the fanout stayed open, and a relay
// whose session ended on its own was never removed from the Map — so every
// stopped/dead terminal leaked a goroutine, a PTY fd and a map entry.

package terminal

import (
	"errors"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/pty"
)

// spawnCat starts a `cat` PTY session: it echoes stdin and never exits on its
// own, so the relay's readLoop stays blocked in sess.Read until something closes
// the session — exactly the state Close has to break out of.
func spawnCat(t *testing.T) *pty.Session {
	t.Helper()
	sess, err := pty.Spawn(pty.SpawnConfig{Cmd: "/bin/sh", Args: []string{"-c", "exec cat"}, Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	return sess
}

func TestRelay_CloseTearsDownSessionFanoutAndReadLoop(t *testing.T) {
	sess := spawnCat(t)
	relay := NewRelay(sess, "")

	relay.Close()

	if sess.IsAlive() {
		t.Fatal("Relay.Close must close the session — otherwise readLoop stays blocked in sess.Read forever")
	}
	if _, err := relay.Fanout().Subscribe("late"); !errors.Is(err, pty.ErrFanoutClosed) {
		t.Fatalf("fanout should be closed after Relay.Close, Subscribe gave %v", err)
	}
	if _, err := relay.WriteBuf().Write([]byte("x")); !errors.Is(err, pty.ErrWriteBufferClosed) {
		t.Fatalf("write buffer should be closed after Relay.Close, Write gave %v", err)
	}

	// Close is called from Map.Remove, Map.ReplaceRelay and Map.CloseAll, and the
	// readLoop's own teardown runs concurrently — it must be idempotent.
	relay.Close()
}

// A relay whose session ends on its own (the user types `exit`) must not linger in
// the Map. Nothing removed it: Remove is only called by /terminal/stop, so an
// ordinary shell exit leaked the entry — and the next reconnect found a relay
// bound to a dead session with a closed fanout.
func TestMap_DropsRelayWhenTheSessionEndsOnItsOwn(t *testing.T) {
	m := NewMap()
	sess, err := pty.Spawn(pty.SpawnConfig{Cmd: "/bin/sh", Args: []string{"-c", "exit 0"}, Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })

	relay := m.GetOrCreate("gone", sess, "")
	<-relay.done
	if m.Get("gone") != nil {
		t.Fatal("a relay whose session exited was never removed from the Map — the entry, its goroutine and its fd leak")
	}
}

// Self-removal must not evict a relay that has already been replaced: the old
// relay's readLoop exits AFTER ReplaceRelay installed the fresh one, and it must
// leave the newcomer alone.
func TestMap_DeadRelayDoesNotEvictItsReplacement(t *testing.T) {
	m := NewMap()
	dead := spawnCat(t)
	fresh := spawnCat(t)

	m.GetOrCreate("id", dead, "")
	replacement := m.ReplaceRelay("id", fresh, "dir") // closes the old relay

	// ReplaceRelay closes the old relay and its completion barrier includes the
	// self-removal callback, so the postcondition is immediately authoritative.
	if m.Get("id") != replacement {
		t.Fatal("the dead relay's self-removal evicted its replacement")
	}
}
