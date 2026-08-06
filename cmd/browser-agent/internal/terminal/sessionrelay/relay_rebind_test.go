// relay_rebind_test.go -- GetOrCreate must never hand back a relay bound to a DEAD
// session when the caller supplies a live one.
//
// Regression (the permanent fake "exited"): nothing removes a relay when its shell
// dies, so the Map keeps an entry whose fanout is closed. Manager.Start evicts a
// dead session BEFORE it spawns, and discards `replaced` if the spawn then fails —
// so the next successful Start reports Replaced=false, the handler takes the
// GetOrCreate path, and GetOrCreate returned the stale dead-bound relay because it
// ignored its `sess` argument entirely. Every WS connect then hit ErrFanoutClosed
// and shipped {"type":"exited"}, which sets processExited=true in the browser and
// disables reconnect permanently. The terminal showed "[Process exited]" over a
// perfectly healthy shell until the daemon was restarted.
//
// The invariant belongs in the primitive, not in each caller (CLAUDE.md rule 19):
// the handler must not have to compute `Replaced` correctly for the relay map to
// stay consistent with the session manager.

package sessionrelay

import (
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/pty"
)

func TestMap_GetOrCreateRebindsWhenSessionDiffers(t *testing.T) {
	mgr := pty.NewManager()
	defer mgr.StopAll()

	relays, first := startCatRelay(t, mgr, "rebind")

	// Kill the shell: this is the state that produces the bug — a relay left in the
	// map bound to a session whose process is gone and whose fanout is closed.
	if err := first.sess.Close(); err != nil {
		t.Fatalf("close first session: %v", err)
	}
	_ = first.sess.Wait(time.Second)
	if first.sess.IsAlive() {
		t.Fatal("first session should be dead before the rebind")
	}

	// A fresh session under the SAME id — exactly what Manager.Start produces after
	// it self-heals a dead session (or after an evict-then-failed-spawn, where the
	// caller is told Replaced=false).
	res2, err := mgr.Start(pty.StartConfig{ID: "rebind-2", Cmd: "/bin/sh", Args: []string{"-c", "exec cat"}})
	if err != nil {
		t.Fatalf("start second session: %v", err)
	}
	second, err := mgr.Get(res2.SessionID)
	if err != nil {
		t.Fatalf("get second session: %v", err)
	}

	got := relays.GetOrCreate("rebind", second, "/work/dir")
	if got == first {
		t.Fatal("GetOrCreate returned the STALE relay for a different session — this is the permanent fake-exited bug")
	}
	if got.sess != second {
		t.Fatalf("the returned relay must be bound to the supplied session")
	}
	if cur := relays.Get("rebind"); cur != got {
		t.Fatal("the map must now hold the rebound relay, not the stale one")
	}
}

func TestMap_GetOrCreateStillIdempotentForSameSession(t *testing.T) {
	mgr := pty.NewManager()
	defer mgr.StopAll()

	relays, relay := startCatRelay(t, mgr, "same")

	// The rebind guard must not cause churn on the normal path: the same session
	// must keep returning the same relay, or every WS reconnect would spawn a new
	// reader loop and drop scrollback.
	again := relays.GetOrCreate("same", relay.sess, "/other")
	if again != relay {
		t.Fatal("GetOrCreate must stay idempotent for the same session")
	}
}
