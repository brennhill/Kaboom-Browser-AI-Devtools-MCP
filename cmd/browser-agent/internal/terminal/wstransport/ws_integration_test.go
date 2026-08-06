// ws_integration_test.go -- End-to-end tests for the terminal WebSocket relay.
// Drives Handle + wsLoop over a real loopback connection against a
// deterministic `cat` PTY session, reusing the package's WS codec helpers.

package wstransport

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/terminal/sessionrelay"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/pty"
)

// wsHandshake dials addr, performs the WebSocket upgrade for token, and returns
// the connection plus a buffered reader/writer positioned at the first frame.
func wsHandshake(t *testing.T, addr, token string) (net.Conn, *bufio.ReadWriter) {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	rw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
	fmt.Fprintf(rw,
		"GET /terminal/ws?token=%s HTTP/1.1\r\nHost: test\r\nUpgrade: websocket\r\n"+
			"Connection: Upgrade\r\nSec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n"+
			"Sec-WebSocket-Version: 13\r\n\r\n", token)
	if err := rw.Flush(); err != nil {
		t.Fatalf("flush handshake: %v", err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	status, err := rw.ReadString('\n')
	if err != nil {
		t.Fatalf("read status line: %v", err)
	}
	if !strings.Contains(status, "101") {
		t.Fatalf("expected 101 handshake, got %q", strings.TrimSpace(status))
	}
	for { // consume headers through the blank line
		line, err := rw.ReadString('\n')
		if err != nil {
			t.Fatalf("read header: %v", err)
		}
		if line == "\r\n" || line == "\n" {
			break
		}
	}
	return conn, rw
}

// readFrame reads one WebSocket frame with a fresh deadline.
func readFrame(t *testing.T, conn net.Conn, rw *bufio.ReadWriter) (byte, []byte) {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, opcode, payload, err := testWSReadFrame(rw)
	if err != nil {
		t.Fatalf("read frame: %v", err)
	}
	return opcode, payload
}

func newWSTestServer(t *testing.T, mgr *pty.Manager, relays *sessionrelay.Map) *httptest.Server {
	t.Helper()
	deps := testDeps()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Handle(w, r, deps, mgr, relays)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestHandle_EchoControlAndClose(t *testing.T) {
	mgr := pty.NewManager()
	defer mgr.StopAll()

	res, err := mgr.Start(pty.StartConfig{ID: "ws-echo", Cmd: "/bin/sh", Args: []string{"-c", "exec cat"}, Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	relays := sessionrelay.NewMap()
	srv := newWSTestServer(t, mgr, relays)
	addr := strings.TrimPrefix(srv.URL, "http://")

	conn, rw := wsHandshake(t, addr, res.Token)
	defer func() { _ = conn.Close() }()

	// First frame is the replay_end marker (scrollback is empty for a fresh cat).
	op, payload := readFrame(t, conn, rw)
	if op != 0x1 || !strings.Contains(string(payload), "replay_end") {
		t.Fatalf("expected replay_end text frame, got op=%#x payload=%q", op, payload)
	}

	// Upstream binary keystrokes -> PTY -> echoed downstream.
	if err := testWSWriteFrame(rw, 0x2, []byte("kaboom\n")); err != nil {
		t.Fatalf("write keystrokes: %v", err)
	}
	found := false
	for i := 0; i < 10 && !found; i++ {
		op, payload = readFrame(t, conn, rw)
		if op == 0x2 && bytes.Contains(payload, []byte("kaboom")) {
			found = true
		}
	}
	if !found {
		t.Fatal("did not receive echoed keystrokes downstream")
	}

	// Upstream text control message (resize) is accepted without tearing down.
	if err := testWSWriteFrame(rw, 0x1, []byte(`{"type":"resize","cols":100,"rows":30}`)); err != nil {
		t.Fatalf("write control: %v", err)
	}

	// Upstream ping -> server replies pong.
	if err := testWSWriteFrame(rw, 0x9, []byte("pingpayload")); err != nil {
		t.Fatalf("write ping: %v", err)
	}
	// Skip any residual echoed data/text frames — PTY echo framing (e.g. a trailing
	// "kaboom\r\n" frame) varies by platform — until the pong control frame arrives.
	for i := 0; i < 10 && op != 0xA; i++ {
		op, payload = readFrame(t, conn, rw)
	}
	if op != 0xA || string(payload) != "pingpayload" {
		t.Fatalf("expected pong echo, got op=%#x payload=%q", op, payload)
	}

	// Client-initiated close -> server echoes a close frame and stops relaying.
	if err := testWSWriteFrame(rw, 0x8, nil); err != nil {
		t.Fatalf("write close: %v", err)
	}
	// Likewise skip any residual data frames before the close frame.
	for i := 0; i < 10 && op != 0x8; i++ {
		op, _ = readFrame(t, conn, rw)
	}
	if op != 0x8 {
		t.Fatalf("expected close frame from server, got op=%#x", op)
	}

	// Session must survive the WebSocket close (reconnect semantics).
	if sess, err := mgr.Get("ws-echo"); err != nil || !sess.IsAlive() {
		t.Fatal("PTY session should stay alive after WebSocket close")
	}
}

func TestHandle_SessionExitNotifies(t *testing.T) {
	mgr := pty.NewManager()
	defer mgr.StopAll()

	res, err := mgr.Start(pty.StartConfig{ID: "ws-exit", Cmd: "/bin/sh", Args: []string{"-c", "exec cat"}})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	relays := sessionrelay.NewMap()
	srv := newWSTestServer(t, mgr, relays)
	addr := strings.TrimPrefix(srv.URL, "http://")

	conn, rw := wsHandshake(t, addr, res.Token)
	defer func() { _ = conn.Close() }()

	// Drain the replay_end marker.
	if op, payload := readFrame(t, conn, rw); op != 0x1 || !strings.Contains(string(payload), "replay_end") {
		t.Fatalf("expected replay_end, got op=%#x payload=%q", op, payload)
	}

	// Kill the session: fanout closes, downstream must emit an "exited" notice.
	if err := mgr.Stop("ws-exit"); err != nil {
		t.Fatalf("stop: %v", err)
	}

	sawExit := false
	sawClose := false
	for i := 0; i < 10 && !sawClose; i++ {
		op, payload := readFrame(t, conn, rw)
		switch op {
		case 0x1:
			if strings.Contains(string(payload), "exited") {
				sawExit = true
			}
		case 0x8:
			sawClose = true
		case 0x2:
			// trailing PTY output before EOF — ignore
		}
	}
	if !sawExit {
		t.Fatal("expected an exited notification after session stop")
	}
	if !sawClose {
		t.Fatal("expected a close frame after session exit")
	}
}

func TestHandle_HijackUnsupported(t *testing.T) {
	mgr := pty.NewManager()
	defer mgr.StopAll()

	res, err := mgr.Start(pty.StartConfig{ID: "ws-hijack", Cmd: "/bin/sh", Args: []string{"-c", "exec cat"}})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	deps := testDeps()
	relays := sessionrelay.NewMap()

	// httptest.ResponseRecorder does not implement http.Hijacker.
	req := httptest.NewRequest("GET", "/terminal/ws?token="+res.Token, nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	rec := httptest.NewRecorder()
	Handle(rec, req, deps, mgr, relays)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 when hijacking unsupported, got %d", rec.Code)
	}
}
