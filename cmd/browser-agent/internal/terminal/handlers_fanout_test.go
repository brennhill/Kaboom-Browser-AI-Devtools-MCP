// handlers_fanout_test.go — a subscribe that fails because the fanout is FULL
// (32-subscriber cap) must NOT be reported to the browser as `exited`. Only a
// genuinely closed fanout (the shell already exited) is an `exited`. Treating a
// full-fanout refusal as `exited` makes the client set processExited=true and stop
// reconnecting on a perfectly healthy shell (finding A).

package terminal

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/pty"
)

func TestHandleTerminalWS_FanoutFullDoesNotReportExited(t *testing.T) {
	mgr := pty.NewManager()
	defer mgr.StopAll()

	res, err := mgr.Start(pty.StartConfig{ID: "full", Cmd: "/bin/sh", Args: []string{"-c", "exec cat"}, Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	sess, err := mgr.Get("full")
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	relays := NewMap()
	// Pre-create the relay and fill its fanout to the subscriber cap, so the
	// handler's own Subscribe returns ErrFanoutFull (a live-shell refusal), not
	// ErrFanoutClosed (a dead shell). cat with no input produces no output, so the
	// fillers are never dropped for backpressure.
	relay := relays.GetOrCreate("full", sess, "")
	for i := 0; ; i++ {
		if _, e := relay.Fanout().Subscribe(fmt.Sprintf("filler-%d", i)); e != nil {
			break // reached ErrFanoutFull — the fanout is now at its cap
		}
	}

	srv := newWSTestServer(t, mgr, relays)
	addr := strings.TrimPrefix(srv.URL, "http://")

	conn, rw := wsHandshake(t, addr, res.Token)
	defer func() { _ = conn.Close() }()

	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	sawExited := false
	sawClose := false
	for i := 0; i < 20 && !sawClose; i++ {
		_, op, payload, ferr := testWSReadFrame(rw)
		if ferr != nil {
			break
		}
		switch op {
		case 0x1:
			if strings.Contains(string(payload), "\"exited\"") {
				sawExited = true
			}
		case 0x8:
			sawClose = true
		}
	}
	if sawExited {
		t.Fatal("a full-fanout refusal (live shell) must NOT be reported as `exited` — the client would stop reconnecting on a healthy shell")
	}
	if !sawClose {
		t.Fatal("expected a plain close frame so the client's reconnect backoff retries")
	}
}
