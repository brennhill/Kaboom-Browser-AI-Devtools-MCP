// bridge_spawn_race_test.go — Tests for tryConnectToExisting and waitForPeerDaemon helpers.

package bridge

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

// --- tryConnectToExisting tests ---

func TestTryConnectToExisting_NoServer(t *testing.T) {
	state := &daemonState{runner: testRunner,
		readyCh:  make(chan struct{}),
		failedCh: make(chan struct{}),
	}
	// Use an ephemeral port that nothing is listening on.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	got := testRunner.tryConnectToExisting(state, port)
	if got {
		t.Fatal("testRunner.tryConnectToExisting() = true, want false when no server running")
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.ready {
		t.Fatal("state.ready should be false")
	}
	if state.failed {
		t.Fatal("state.failed should be false")
	}
}

func TestTryConnectToExisting_CompatibleServer(t *testing.T) {
	ln, port := startHealthServer(t, http.StatusOK, healthJSON(testRunner.identity.Version, "kaboom"))
	defer ln.Close()

	state := &daemonState{runner: testRunner,
		readyCh:  make(chan struct{}),
		failedCh: make(chan struct{}),
	}
	got := testRunner.tryConnectToExisting(state, port)
	if !got {
		t.Fatal("testRunner.tryConnectToExisting() = false, want true for compatible server")
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if !state.ready {
		t.Fatal("state.ready should be true")
	}
}

func TestTryConnectToExisting_NonKaboomService(t *testing.T) {
	ln, port := startHealthServer(t, http.StatusOK, healthJSON("1.0.0", "some-other-service"))
	defer ln.Close()

	state := &daemonState{runner: testRunner,
		readyCh:  make(chan struct{}),
		failedCh: make(chan struct{}),
	}
	got := testRunner.tryConnectToExisting(state, port)
	if !got {
		t.Fatal("testRunner.tryConnectToExisting() = false, want true (fatally blocked)")
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.ready {
		t.Fatal("state.ready should be false when non-kaboom service occupies port")
	}
	if !state.failed {
		t.Fatal("state.failed should be true when non-kaboom service occupies port")
	}
	if !strings.Contains(state.err, "non-kaboom") {
		t.Fatalf("state.err = %q, want mention of non-kaboom", state.err)
	}
}

func TestTryConnectToExisting_RejectsLegacyKaboomIdentity(t *testing.T) {
	ln, port := startHealthServer(t, http.StatusOK, healthJSON(testRunner.identity.Version, "gasoline"))
	defer ln.Close()

	state := &daemonState{runner: testRunner,
		readyCh:  make(chan struct{}),
		failedCh: make(chan struct{}),
	}
	if got := testRunner.tryConnectToExisting(state, port); !got {
		t.Fatal("testRunner.tryConnectToExisting() = false, want legacy identity to block the occupied port")
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.ready {
		t.Fatal("legacy identity must not be treated as the canonical daemon")
	}
	if !state.failed || !strings.Contains(state.err, "non-kaboom") {
		t.Fatalf("legacy identity should be reported as non-kaboom, got failed=%v err=%q", state.failed, state.err)
	}
}

// --- waitForPeerDaemon tests ---

func TestWaitForPeerDaemon_ServerAppearsOnFirstRetry(t *testing.T) {
	// Start server after a short delay so peer polling can discover it.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	runner := *testRunner
	started := false
	runner.sleep = func(time.Duration) {
		if !started {
			started = true
			startHealthServerOnPort(t, port, http.StatusOK, healthJSON(runner.identity.Version, "kaboom"))
		}
	}

	state := &daemonState{runner: &runner,
		readyCh:  make(chan struct{}),
		failedCh: make(chan struct{}),
	}
	got := runner.waitForPeerDaemon(state, port)

	if !got {
		t.Fatal("testRunner.waitForPeerDaemon() = false, want true when server appears during retry")
	}
	if !started {
		t.Fatal("peer retry delay was not exercised")
	}
}

func TestWaitForPeerDaemon_NoServerReturnsQuickly(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	state := &daemonState{runner: testRunner,
		readyCh:  make(chan struct{}),
		failedCh: make(chan struct{}),
	}
	start := time.Now()
	got := testRunner.waitForPeerDaemon(state, port)
	elapsed := time.Since(start)

	if got {
		t.Fatal("testRunner.waitForPeerDaemon() = true, want false when no server ever appears")
	}
	if elapsed < daemonPeerWaitTimeout-daemonPeerPollInterval {
		t.Fatalf("elapsed = %v, want >= %s", elapsed, daemonPeerWaitTimeout-daemonPeerPollInterval)
	}
	if elapsed > daemonPeerWaitTimeout+500*time.Millisecond {
		t.Fatalf("elapsed = %v, want <= %s", elapsed, daemonPeerWaitTimeout+500*time.Millisecond)
	}
}

// --- RunMode integration: existing test still passes (no regression) ---

func TestRunBridgeModeWithExistingServer_StillWorks(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen(:0) error = %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"status":"ok"}`)
	})
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":{}}`+"\n")
	})
	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	defer func() { _ = srv.Close() }()

	output := captureBridgeIO(t, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`+"\n", func() {
		testRunner.RunMode(port, "", 0)
	})
	if !strings.Contains(output, `"protocolVersion"`) {
		t.Fatalf("RunMode output missing initialize response: %q", output)
	}
}

// --- helpers ---

func healthJSON(ver, service string) string {
	m := map[string]any{
		"status":  "ok",
		"version": ver,
		"name":    service,
	}
	b, _ := json.Marshal(m)
	return string(b)
}

func startHealthServer(t *testing.T, status int, body string) (net.Listener, int) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	})
	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	return ln, port
}

func startHealthServerOnPort(t *testing.T, port, status int, body string) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		// Port may already be in use; return nil.
		return nil
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	})
	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	return ln
}
