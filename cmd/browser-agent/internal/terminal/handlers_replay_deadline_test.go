// handlers_replay_deadline_test.go — the scrollback-replay writes that run BEFORE
// wsLoop (replay chunks + replay_end + the subscribe-failure frames) must be bound
// by a write deadline, like every wsLoop frame. Terminal server has WriteTimeout:0,
// so a client that stalls its reader mid-replay (256KB scrollback > socket buffer)
// would otherwise block conn.Write forever — one leaked goroutine + fd per connect,
// uncapped, until fd exhaustion kills the daemon (finding B).

package terminal

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/pty"
)

type fakeConnAddr struct{}

func (fakeConnAddr) Network() string { return "test" }
func (fakeConnAddr) String() string  { return "test" }

// recordConn is a net.Conn that counts SetWriteDeadline calls (to prove replay
// writes are deadline-bound), succeeds all writes, and blocks Read until Close.
type recordConn struct {
	mu        sync.Mutex
	deadlines int
	closed    chan struct{}
	closeOnce sync.Once
}

func newRecordConn() *recordConn { return &recordConn{closed: make(chan struct{})} }

func (c *recordConn) Read(p []byte) (int, error) {
	<-c.closed
	return 0, io.EOF
}
func (c *recordConn) Write(p []byte) (int, error) { return len(p), nil }
func (c *recordConn) Close() error {
	c.closeOnce.Do(func() { close(c.closed) })
	return nil
}
func (c *recordConn) LocalAddr() net.Addr             { return fakeConnAddr{} }
func (c *recordConn) RemoteAddr() net.Addr            { return fakeConnAddr{} }
func (c *recordConn) SetDeadline(t time.Time) error   { return nil }
func (c *recordConn) SetReadDeadline(time.Time) error { return nil }
func (c *recordConn) SetWriteDeadline(time.Time) error {
	c.mu.Lock()
	c.deadlines++
	c.mu.Unlock()
	return nil
}
func (c *recordConn) writeDeadlineCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.deadlines
}

// hijackRecorder is a ResponseWriter+Hijacker that hands HandleTerminalWS the
// recordConn so we can observe the replay path's write deadlines.
type hijackRecorder struct {
	header http.Header
	conn   *recordConn
	rw     *bufio.ReadWriter
}

func (h *hijackRecorder) Header() http.Header         { return h.header }
func (h *hijackRecorder) Write(p []byte) (int, error) { return len(p), nil }
func (h *hijackRecorder) WriteHeader(int)             {}
func (h *hijackRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return h.conn, h.rw, nil
}

func TestHandleTerminalWS_ReplayWritesAreDeadlineBound(t *testing.T) {
	mgr := pty.NewManager()
	defer mgr.StopAll()

	// Preload scrollback so the connect path has history to replay: the shell
	// prints a marker, then execs cat (which stays alive and idle).
	res, err := mgr.Start(pty.StartConfig{ID: "b", Cmd: "/bin/sh", Args: []string{"-c", "printf PRELOADXYZ; exec cat"}, Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	sess, err := mgr.Get("b")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	relays := NewMap()
	relays.GetOrCreate("b", sess, "") // start the relay readLoop so scrollback fills
	waitForTrue(t, func() bool { return strings.Contains(string(sess.Scrollback()), "PRELOAD") }, 3*time.Second)

	conn := newRecordConn()
	rw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
	hrec := &hijackRecorder{header: make(http.Header), conn: conn, rw: rw}
	req := httptest.NewRequest("GET", "/terminal/ws?token="+res.Token, nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")

	done := make(chan struct{})
	go func() {
		defer close(done)
		HandleTerminalWS(hrec, req, testDeps(), mgr, relays)
	}()

	// The replay (scrollback chunk + replay_end) runs before wsLoop. On the fixed
	// code each replay frame goes through the deadline-aware writer, so the conn
	// records SetWriteDeadline. On the buggy code the replay used the raw codec with
	// no deadline (and the idle wsLoop — cat produces nothing, ping is 30s away —
	// sets none), so this count stays 0 and the poll times out.
	waitForTrue(t, func() bool { return conn.writeDeadlineCount() > 0 }, 3*time.Second)

	_ = conn.Close() // unblock the upstream reader so the handler returns
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("handler did not return after conn close")
	}
}
