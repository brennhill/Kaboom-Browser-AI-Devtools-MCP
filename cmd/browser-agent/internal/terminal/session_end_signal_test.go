// session_end_signal_test.go — the downstream pump must tell a genuine session
// end (fanout closed by readLoop) apart from a slow-subscriber drop (fanout
// backpressure closing one channel while the shell runs). Before the fix both
// closed the subscriber channel and were reported to the browser as `exited`,
// so a merely-slow terminal (big build, backgrounded tab) was declared dead.

package terminal

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/pty"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/testsync"
)

func waitForTrue(t *testing.T, cond func() bool, within time.Duration) {
	t.Helper()
	testsync.Eventually(t, within, "terminal condition", cond)
}

// A slow-subscriber drop must NOT mark the relay ended; a real session end must.
func TestRelay_EndedDistinguishesSessionEndFromDrop(t *testing.T) {
	sess, err := pty.Spawn(pty.SpawnConfig{Cmd: "/bin/sh", Args: []string{"-c", "exec cat"}, Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	relay := NewRelay(sess, "")

	if relay.Ended() {
		t.Fatal("Ended() must be false while the session is alive")
	}

	// Force a slow-subscriber drop: fill its buffer without reading so Fanout
	// closes just this channel (cat with no input produces no output, so the
	// relay's own readLoop never broadcasts — the drop is deterministic).
	slow, err := relay.Fanout().Subscribe("slow")
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	for i := 0; i < 200; i++ {
		relay.Fanout().Broadcast([]byte("x"))
	}
	for range slow { //nolint:revive // drain until the dropped channel is closed
	}
	if relay.Ended() {
		t.Fatal("a slow-subscriber drop must NOT mark the relay ended (session still alive)")
	}

	// A genuine session end must mark it ended.
	_ = sess.Close()
	<-relay.done
	if !relay.Ended() {
		t.Fatal("relay completion must retain the genuine session-end state")
	}
}

// End-to-end: stopping a connected session sends the browser an `exited` frame
// (the downstream `!ok`+Ended branch), so the terminal correctly reports death.
func TestHandleTerminalWS_SessionEndSendsExited(t *testing.T) {
	mgr := pty.NewManager()
	defer mgr.StopAll()

	res, err := mgr.Start(pty.StartConfig{ID: "end", Cmd: "/bin/sh", Args: []string{"-c", "exec cat"}, Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	deps := testDeps()
	relays := NewMap()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		HandleTerminalWS(w, r, deps, mgr, relays)
	}))
	t.Cleanup(srv.Close)
	addr := strings.TrimPrefix(srv.URL, "http://")

	conn, rw := wsHandshake(t, addr, res.Token)
	defer func() { _ = conn.Close() }()

	// Drain replay_end.
	if op, payload := readFrame(t, conn, rw); op != 0x1 || !strings.Contains(string(payload), "replay_end") {
		t.Fatalf("expected replay_end, got op=%#x payload=%q", op, payload)
	}

	// replay_end proves the downstream subscriber is installed, so stopping the
	// session now deterministically closes that subscription.
	_ = mgr.Stop("end")

	// Expect an "exited" text frame within a few seconds.
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	sawExited := false
	for i := 0; i < 100; i++ {
		_, op, payload, ferr := testWSReadFrame(rw)
		if ferr != nil {
			break
		}
		if op == 0x1 && strings.Contains(string(payload), "\"exited\"") {
			sawExited = true
			break
		}
	}
	if !sawExited {
		t.Fatal("expected an `exited` frame after the session ended while connected")
	}
}
