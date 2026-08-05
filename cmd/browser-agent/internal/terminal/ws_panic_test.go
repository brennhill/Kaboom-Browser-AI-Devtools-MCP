// ws_panic_test.go -- Regression tests for the daemon "never crash" invariant.
// A panic in any per-connection wsLoop goroutine (downstream pump, ping
// keepalive, upstream reader) must tear down only that connection, log a
// structured event, and keep the PTY session + process alive.

package terminal

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/pty"
)

// TestGoConnWorker_RecoversPanicAndTearsDown verifies the wrapper itself:
// a panic inside fn is recovered (does not crash the process), closeConn is
// invoked exactly once for teardown, and a structured "terminal_ws_panic"
// event is emitted with session correlation.
func TestGoConnWorker_RecoversPanicAndTearsDown(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var events []map[string]any
	deps := Deps{
		Stderrf: func(string, ...any) {},
		LogEvent: func(event string, fields map[string]any) {
			if event != "terminal_ws_panic" {
				return
			}
			mu.Lock()
			events = append(events, fields)
			mu.Unlock()
		},
	}

	closed := make(chan struct{})
	var once sync.Once
	closeConn := func() { once.Do(func() { close(closed) }) }

	goConnWorker(deps, "sess-xyz", "downstream", closeConn, func() {
		panic("boom")
	})

	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("closeConn was not called after panic")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(events) != 1 {
		t.Fatalf("expected exactly one panic event, got %d", len(events))
	}
	if events[0]["session_id"] != "sess-xyz" {
		t.Fatalf("expected session correlation, got %v", events[0]["session_id"])
	}
	if events[0]["role"] != "downstream" {
		t.Fatalf("expected role=downstream, got %v", events[0]["role"])
	}
	if s, _ := events[0]["panic"].(string); !strings.Contains(s, "boom") {
		t.Fatalf("expected panic detail, got %v", events[0]["panic"])
	}
}

// panicOnBinaryWriteFrame wraps the real codec but panics on the first binary
// (0x2) downstream frame — simulating a write-on-closed-pipe / hostile-output
// fault in the downstream pump goroutine.
func panicOnBinaryWriteFrame(w *bufio.ReadWriter, opcode byte, payload []byte) error {
	if opcode == 0x2 {
		panic("simulated downstream write fault")
	}
	return testWSWriteFrame(w, opcode, payload)
}

// TestHandleTerminalWS_DownstreamPanicDoesNotCrashDaemon drives a real WS
// connection whose downstream writes panic once PTY output arrives. Before the
// fix this panicked in an unrecovered goroutine and crashed the whole test
// binary (the daemon). After the fix: the connection tears down, the PTY
// session survives for reconnect, and a structured panic event is logged.
func TestHandleTerminalWS_DownstreamPanicDoesNotCrashDaemon(t *testing.T) {
	mgr := pty.NewManager()
	defer mgr.StopAll()

	res, err := mgr.Start(pty.StartConfig{ID: "ws-panic", Cmd: "/bin/sh", Args: []string{"-c", "exec cat"}, Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	panicLogged := make(chan struct{})
	var panicLoggedOnce sync.Once
	deps := testDeps()
	deps.WSWriteFrame = panicOnBinaryWriteFrame
	deps.LogEvent = func(event string, fields map[string]any) {
		if event == "terminal_ws_panic" {
			panicLoggedOnce.Do(func() { close(panicLogged) })
		}
	}

	relays := NewMap()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		HandleTerminalWS(w, r, deps, mgr, relays)
	}))
	t.Cleanup(srv.Close)
	addr := strings.TrimPrefix(srv.URL, "http://")

	conn, rw := wsHandshake(t, addr, res.Token)
	defer func() { _ = conn.Close() }()

	// Drain the replay_end marker (text frame — not panicking).
	if op, payload := readFrame(t, conn, rw); op != 0x1 || !strings.Contains(string(payload), "replay_end") {
		t.Fatalf("expected replay_end, got op=%#x payload=%q", op, payload)
	}

	// Send keystrokes: cat echoes them, the downstream pump attempts a 0x2
	// write, which panics. The recover must fire without crashing the process.
	if err := testWSWriteFrame(rw, 0x2, []byte("kaboom\n")); err != nil {
		t.Fatalf("write keystrokes: %v", err)
	}

	// The connection must close (server tore it down). Reading eventually
	// returns an error once conn is closed by closeConn.
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	sawClosure := false
	for i := 0; i < 50; i++ {
		if _, _, _, ferr := testWSReadFrame(rw); ferr != nil {
			sawClosure = true
			break
		}
	}
	if !sawClosure {
		t.Fatal("expected the WS connection to be torn down after downstream panic")
	}

	select {
	case <-panicLogged:
	case <-time.After(2 * time.Second):
		t.Fatal("expected a structured terminal_ws_panic event to be logged")
	}

	// The PTY session must survive so the browser can reconnect.
	if sess, gerr := mgr.Get("ws-panic"); gerr != nil || !sess.IsAlive() {
		t.Fatal("PTY session should stay alive after a downstream panic")
	}
}
