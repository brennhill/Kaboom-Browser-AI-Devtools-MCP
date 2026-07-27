// connect_test.go — Tests for the post-install extension connect loop.
package nativeinstall

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// portFromURL extracts the numeric port from an httptest server URL.
func portFromURL(t *testing.T, raw string) int {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("url.Parse(%q): %v", raw, err)
	}
	p, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatalf("port from %q: %v", raw, err)
	}
	return p
}

func TestConnectPhase(t *testing.T) {
	cases := []struct {
		h    Health
		want string
	}{
		{Health{}, "daemon_unreachable"},
		{Health{Reachable: true}, "waiting_extension"},
		{Health{Reachable: true, ExtensionConnected: true}, "connected"},
	}
	for _, tc := range cases {
		if got := connectPhase(tc.h); got != tc.want {
			t.Fatalf("connectPhase(%+v) = %q, want %q", tc.h, got, tc.want)
		}
	}
}

// fakeClock advances virtual time on every inter-poll wait and hands back an
// already-ready channel, so the loop terminates without real timers.
func fakeClock() (func() time.Time, func(time.Duration) <-chan time.Time) {
	t := time.Unix(0, 0)
	now := func() time.Time { return t }
	after := func(d time.Duration) <-chan time.Time {
		t = t.Add(d)
		ch := make(chan time.Time, 1)
		ch <- t
		return ch
	}
	return now, after
}

func TestWaitForExtensionConnected_ConnectsAfterPolls(t *testing.T) {
	now, after := fakeClock()
	// unreachable → up-but-waiting → connected
	seq := []Health{
		{},
		{Reachable: true},
		{Reachable: true, ExtensionConnected: true},
	}
	i := 0
	var lines []string
	res := waitForExtensionConnected(context.Background(), 7890, 30*time.Second, 750*time.Millisecond, connectWaitDeps{
		fetch: func(context.Context, int) Health {
			h := seq[i]
			if i < len(seq)-1 {
				i++
			}
			return h
		},
		now:   now,
		after: after,
		sink:  func(s string) { lines = append(lines, s) },
	})
	if !res.connected {
		t.Fatalf("expected connected, got %+v", res)
	}
	// Only the two non-connected phases narrate, each once.
	if len(lines) != 2 {
		t.Fatalf("expected 2 progress lines, got %d: %v", len(lines), lines)
	}
	if !strings.Contains(lines[0], "server to come up") || !strings.Contains(lines[1], "load the extension") {
		t.Fatalf("unexpected progress lines: %v", lines)
	}
}

func TestWaitForExtensionConnected_Timeout(t *testing.T) {
	now, after := fakeClock()
	res := waitForExtensionConnected(context.Background(), 7890, 2*time.Second, 750*time.Millisecond, connectWaitDeps{
		fetch: func(context.Context, int) Health { return Health{Reachable: true} },
		now:   now,
		after: after,
	})
	if res.connected {
		t.Fatal("expected timeout, got connected")
	}
	if res.lastPhase != "waiting_extension" {
		t.Fatalf("lastPhase = %q, want waiting_extension", res.lastPhase)
	}
}

func TestWaitForExtensionConnected_TimeoutUnreachable(t *testing.T) {
	now, after := fakeClock()
	res := waitForExtensionConnected(context.Background(), 7890, 1500*time.Millisecond, 750*time.Millisecond, connectWaitDeps{
		fetch: func(context.Context, int) Health { return Health{} },
		now:   now,
		after: after,
	})
	if res.connected || res.lastPhase != "daemon_unreachable" {
		t.Fatalf("expected unreachable timeout, got %+v", res)
	}
}

func TestWaitForExtensionConnected_AbortSkips(t *testing.T) {
	now, after := fakeClock()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Ctrl-C before the first poll: must skip, not poll.
	polled := false
	res := waitForExtensionConnected(ctx, 7890, 30*time.Second, 750*time.Millisecond, connectWaitDeps{
		fetch: func(context.Context, int) Health { polled = true; return Health{} },
		now:   now,
		after: after,
	})
	if !res.aborted || res.connected {
		t.Fatalf("expected aborted, got %+v", res)
	}
	if polled {
		t.Fatal("must not poll once the context is already cancelled")
	}
}

func TestWaitForExtensionConnected_AbortDuringWaitReturnsPromptly(t *testing.T) {
	now, _ := fakeClock()
	ctx, cancel := context.WithCancel(context.Background())
	// A never-ready timer stands in for a poll interval that has not elapsed: if
	// the wait were not context-aware, the select would block forever. A prompt
	// Ctrl-C must instead return via ctx.Done() without waiting out the interval.
	neverAfter := func(time.Duration) <-chan time.Time { return make(chan time.Time) }
	polls := 0
	res := waitForExtensionConnected(ctx, 7890, 30*time.Second, 750*time.Millisecond, connectWaitDeps{
		fetch: func(context.Context, int) Health {
			polls++
			cancel() // Ctrl-C arrives mid-wait, right after the first poll.
			return Health{Reachable: true}
		},
		now:   now,
		after: neverAfter,
	})
	if !res.aborted || res.connected {
		t.Fatalf("expected a prompt aborted result, got %+v", res)
	}
	if res.lastPhase != "waiting_extension" {
		t.Fatalf("lastPhase = %q, want waiting_extension", res.lastPhase)
	}
	if polls != 1 {
		t.Fatalf("expected exactly one poll before the abort, got %d", polls)
	}
}

func TestConnectHintLine(t *testing.T) {
	waiting := connectHintLine("waiting_extension", 7890, "/x/ext")
	unreachable := connectHintLine("daemon_unreachable", 7890, "/x/ext")
	if waiting == unreachable {
		t.Fatal("hints for the two phases must differ")
	}
	if !strings.Contains(waiting, "/x/ext") {
		t.Fatalf("waiting hint must name the extension folder: %q", waiting)
	}
	if !strings.Contains(unreachable, "7890") {
		t.Fatalf("unreachable hint must name the port: %q", unreachable)
	}
}

func TestInstallWaitDisabled(t *testing.T) {
	cases := []struct {
		key  string
		val  string
		want bool
	}{
		{"", "", false},
		{"KABOOM_NO_WAIT", "1", true},
		{"KABOOM_INSTALL_NO_WAIT", "true", true},
		{"KABOOM_NO_WAIT", "0", false},
		{"KABOOM_NO_WAIT", "false", false},
	}
	for _, tc := range cases {
		t.Setenv("KABOOM_NO_WAIT", "")
		t.Setenv("KABOOM_INSTALL_NO_WAIT", "")
		if tc.key != "" {
			t.Setenv(tc.key, tc.val)
		}
		if got := installWaitDisabled(); got != tc.want {
			t.Fatalf("installWaitDisabled(%s=%q) = %v, want %v", tc.key, tc.val, got, tc.want)
		}
	}
}

func TestFetchInstallHealth_LiveServer(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":"0.8.6","capture":{"extension_connected":true}}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	h := FetchHealth(context.Background(), portFromURL(t, srv.URL), 2*time.Second)
	if !h.Reachable || !h.ExtensionConnected || h.Version != "0.8.6" {
		t.Fatalf("FetchHealth = %+v, want reachable+connected+v0.8.6", h)
	}
}

func TestFetchInstallHealth_UpButExtensionNotConnected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"version":"0.8.6","capture":{"extension_connected":false}}`))
	}))
	defer srv.Close()

	h := FetchHealth(context.Background(), portFromURL(t, srv.URL), 2*time.Second)
	if !h.Reachable || h.ExtensionConnected {
		t.Fatalf("FetchHealth = %+v, want reachable but not connected", h)
	}
}

func TestFetchInstallHealth_NonOKIsReachableOnly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	h := FetchHealth(context.Background(), portFromURL(t, srv.URL), 2*time.Second)
	if !h.Reachable || h.ExtensionConnected {
		t.Fatalf("FetchHealth(503) = %+v, want reachable only", h)
	}
}

func TestFetchInstallHealth_UnreachableIsZeroValue(t *testing.T) {
	// Start then immediately close to obtain a definitely-closed port.
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	port := portFromURL(t, srv.URL)
	srv.Close()

	h := FetchHealth(context.Background(), port, 500*time.Millisecond)
	if h.Reachable {
		t.Fatalf("FetchHealth(closed) = %+v, want unreachable", h)
	}
}

func TestIsTerminal_PipeIsNotATerminal(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	defer func() { _ = r.Close(); _ = w.Close() }()
	if isTerminal(r) {
		t.Fatal("a pipe must not be reported as a terminal")
	}
	if isTerminal(nil) {
		t.Fatal("nil must not be reported as a terminal")
	}
}
