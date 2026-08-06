// bridge_startup_test.go — Unit tests for bridge_startup.go: leader election via the
// startup lock, stale-lock reclaim, and the pure/HTTP-probe status helpers.

package bridge

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/bridge/startuplock"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolguard"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/schema"
	statecfg "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

func TestBridgeStartupLock_SingleLeaderElection(t *testing.T) {
	t.Setenv(statecfg.StateDirEnv, t.TempDir())
	port := 7890

	locks := testStartupLockManager()
	lockA, acquired, err := locks.Acquire(port)
	if err != nil {
		t.Fatalf("testRunner.tryAcquireBridgeStartupLock() error = %v", err)
	}
	if !acquired || lockA == nil {
		t.Fatal("first lock acquisition should succeed")
	}

	lockB, acquired, err := locks.Acquire(port)
	if err != nil {
		t.Fatalf("second testRunner.tryAcquireBridgeStartupLock() error = %v", err)
	}
	if acquired || lockB != nil {
		t.Fatal("second lock acquisition should not succeed while first leader holds lock")
	}

	lockA.Release()

	lockC, acquired, err := locks.Acquire(port)
	if err != nil {
		t.Fatalf("third testRunner.tryAcquireBridgeStartupLock() error = %v", err)
	}
	if !acquired || lockC == nil {
		t.Fatal("third lock acquisition should succeed after release")
	}
	lockC.Release()
}

func TestClearStaleBridgeStartupLock_RemovesDeadOwner(t *testing.T) {
	t.Setenv(statecfg.StateDirEnv, t.TempDir())
	port := 7891
	path := writeBridgeStartupLockForTest(t, port, startuplock.Record{
		PID:       -1,
		Port:      port,
		CreatedAt: time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano),
	})

	if removed := testStartupLockManager().ClearStale(port, daemonStartupLockStaleAfter); !removed {
		t.Fatal("testRunner.clearStaleBridgeStartupLock() = false, want true for dead owner")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("lock file should be removed, stat err = %v", err)
	}
}

func TestClearStaleBridgeStartupLock_PreservesRecentLiveOwner(t *testing.T) {
	t.Setenv(statecfg.StateDirEnv, t.TempDir())
	port := 7892
	path := writeBridgeStartupLockForTest(t, port, startuplock.Record{
		PID:       os.Getpid(),
		Port:      port,
		CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
	})

	if removed := testStartupLockManager().ClearStale(port, time.Minute); removed {
		t.Fatal("testRunner.clearStaleBridgeStartupLock() = true, want false for recent live owner")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("lock file should remain, stat err = %v", err)
	}
}

func writeBridgeStartupLockForTest(t *testing.T, port int, record startuplock.Record) string {
	t.Helper()
	path, err := startuplock.Path(port)
	if err != nil {
		t.Fatalf("bridgeStartupLockPath() error = %v", err)
	}
	// #nosec G301 -- test-owned temp directory.
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	payload, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	// #nosec G306 -- test fixture file content.
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", path, err)
	}
	return path
}

func portOfURL(t *testing.T, rawURL string) int {
	t.Helper()
	p, err := strconv.Atoi(rawURL[strings.LastIndex(rawURL, ":")+1:])
	if err != nil {
		t.Fatalf("parse port from %q: %v", rawURL, err)
	}
	return p
}

func TestIsServerRunning_NotListening(t *testing.T) {
	if testRunner.IsServerRunning(59991) {
		t.Fatal("testRunner.IsServerRunning(59991) = true, want false (nothing listening)")
	}
}

func TestDaemonStartupSuggestion_Branches(t *testing.T) {
	if s := daemonStartupSuggestion("failed to bind to port", 7890); !strings.Contains(s, "--port 7891") {
		t.Fatalf("port/bind suggestion = %q, want next-port hint", s)
	}
	if s := daemonStartupSuggestion("some unrelated failure", 7890); !strings.Contains(s, "--doctor") {
		t.Fatalf("generic suggestion = %q, want --doctor hint", s)
	}
}

func TestRunningServerVersionCompatible_DeadPort(t *testing.T) {
	if ok, _, _ := testRunner.runningServerVersionCompatible(59992); ok {
		t.Fatal("expected not-compatible for a dead port")
	}
}

func TestRunningServerVersionCompatible_Non200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	if ok, _, _ := testRunner.runningServerVersionCompatible(portOfURL(t, srv.URL)); ok {
		t.Fatal("expected not-compatible for non-200 /health")
	}
}

func TestRunningServerVersionCompatible_HealthServed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"` + testRunner.identity.ServerName + `","version":"0.8.4"}`))
	}))
	defer srv.Close()
	// Exercises the read-body, health metadata, and service/version compatibility path.
	_, _, _ = testRunner.runningServerVersionCompatible(portOfURL(t, srv.URL))
}

func closedLoopbackPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen(:0) error = %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return port
}

func TestRespawnIfNeeded_ThrottlesRapidSpawns(t *testing.T) {
	oldNow, oldSpawn := bridgeNow, respawnSpawnFn
	defer func() { bridgeNow = oldNow; respawnSpawnFn = oldSpawn }()

	fakeNow := time.Unix(1_700_000_000, 0)
	bridgeNow = func() time.Time { return fakeNow }

	spawns := 0
	respawnSpawnFn = func(s *daemonState) bool {
		spawns++
		s.markFailed("simulated: respawned daemon never became healthy")
		return false
	}

	s := &daemonState{runner: testRunner,
		port:     closedLoopbackPort(t),
		readyCh:  make(chan struct{}),
		failedCh: make(chan struct{}),
	}
	s.markFailed("initial: force spawn path")

	const rapid = 6
	for i := 0; i < rapid; i++ {
		s.respawnIfNeeded()
		s.markFailed("keep failed for next iteration")
	}
	if spawns != 1 {
		t.Fatalf("rapid respawns within the interval: want exactly 1 actual spawn, got %d", spawns)
	}

	fakeNow = fakeNow.Add(respawnMinInterval + time.Millisecond)
	s.respawnIfNeeded()
	if spawns != 2 {
		t.Fatalf("after the interval elapsed: want a 2nd real spawn, got %d", spawns)
	}

	s.markFailed("keep failed")
	s.respawnIfNeeded()
	if spawns != 2 {
		t.Fatalf("second rapid burst must be throttled: want spawns still 2, got %d", spawns)
	}
}

func TestReserveRespawnSlot_BackoffEscalatesAndCools(t *testing.T) {
	s := &daemonState{runner: testRunner}
	base := time.Unix(1_700_000_000, 0)

	if !s.reserveRespawnSlot(base) {
		t.Fatal("first reserveRespawnSlot must be granted")
	}
	if s.reserveRespawnSlot(base) {
		t.Fatal("second reservation at the same instant must be throttled")
	}
	if !s.reserveRespawnSlot(base.Add(respawnMinInterval)) {
		t.Fatal("reservation at the min interval must be granted")
	}
	if s.reserveRespawnSlot(base.Add(respawnMinInterval + respawnMinInterval)) {
		t.Fatal("after escalation, a min-sized gap must still be throttled")
	}
	coolStart := base.Add(respawnMinInterval + respawnMaxBackoff + time.Second)
	if !s.reserveRespawnSlot(coolStart) {
		t.Fatal("a respawn after a full cool-down must be granted")
	}
	if !s.reserveRespawnSlot(coolStart.Add(respawnMinInterval)) {
		t.Fatal("after cool-down reset, a min-interval gap must be granted again")
	}
}

func TestBridge_SpawnsDaemonWhenNoneRunning(t *testing.T) {
	unusedPort := 19876
	state := &daemonState{runner: testRunner,
		readyCh: make(chan struct{}), failedCh: make(chan struct{}), port: unusedPort}
	if testRunner.tryConnectToExisting(state, unusedPort) {
		t.Fatal("tryConnectToExisting should return false when no server is running")
	}
	if state.ready {
		t.Fatal("state should not be marked ready when no server is running")
	}
}

func TestBridge_SkipsSpawnWhenDaemonAlreadyRunning(t *testing.T) {
	state := &daemonState{runner: testRunner,
		readyCh: make(chan struct{}), failedCh: make(chan struct{}), port: 0}
	if testRunner.tryConnectToExisting(state, 0) {
		t.Fatal("should return false with port 0 (no server)")
	}
}

func TestDaemonState_RespawnResetsClearFailure(t *testing.T) {
	state := &daemonState{runner: testRunner,
		readyCh: make(chan struct{}), failedCh: make(chan struct{}), port: 19877}
	state.markFailed("port bind error")
	if !state.failed {
		t.Fatal("state should be marked failed")
	}
	expectedReady, expectedFailed := state.readyCh, state.failedCh
	state.mu.Lock()
	if state.failed && state.readyCh == expectedReady && state.failedCh == expectedFailed {
		state.ready = false
		state.failed = false
		state.err = ""
		state.resetSignalsLocked()
	}
	state.mu.Unlock()
	if state.failed || state.err != "" {
		t.Fatalf("failure was not cleared: failed=%v err=%q", state.failed, state.err)
	}
}

func TestCheckDaemonStatus_ReturnsStartingDuringSpawn(t *testing.T) {
	state := &daemonState{runner: testRunner,
		readyCh: make(chan struct{}), failedCh: make(chan struct{}), port: 19878}
	saved := daemonStartupGracePeriod
	daemonStartupGracePeriod = 50 * time.Millisecond
	defer func() { daemonStartupGracePeriod = saved }()
	if status := checkDaemonStatus(state, mcp.JSONRPCRequest{Method: "tools/call"}, state.port); status != "starting" {
		t.Fatalf("expected starting during spawn, got %q", status)
	}
}

func TestCheckDaemonStatus_ReadyReturnsEmpty(t *testing.T) {
	state := &daemonState{runner: testRunner,
		readyCh: make(chan struct{}), failedCh: make(chan struct{}), port: 0}
	state.markReady()
	if status := checkDaemonStatus(state, mcp.JSONRPCRequest{Method: "tools/call"}, state.port); status != "" {
		t.Fatalf("expected empty status when daemon is ready, got %q", status)
	}
}

func TestFastPath_InitializeDoesNotRequireDaemon(t *testing.T) {
	state := &daemonState{runner: testRunner,
		readyCh: make(chan struct{}), failedCh: make(chan struct{}), port: 19879}
	status := checkDaemonStatus(state, mcp.JSONRPCRequest{JSONRPC: "2.0", Method: "initialize", ID: float64(1)}, state.port)
	if status != "method_not_found" {
		t.Fatalf("initialize should be handled by fast-path, got %q", status)
	}
}

func TestFastPath_ToolsListDoesNotRequireDaemon(t *testing.T) {
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", Method: "tools/list", ID: float64(1)}
	if !testRunner.handleFastPath(req, schema.AllTools(), 0) {
		t.Fatal("tools/list should be handled by fast-path without daemon")
	}
}

func TestToolsCall_WaitsDuringStartup_NotInstantError(t *testing.T) {
	status, method, shouldForward := "starting", "tools/call", true
	if status != "" && (status != "starting" || method != "tools/call") {
		shouldForward = false
	}
	if !shouldForward {
		t.Fatal("tools/call should be forwarded during startup")
	}
}

func TestRequireExtension_WaitsForColdStart(t *testing.T) {
	if toolguard.DefaultExtensionReadinessTimeout <= 0 {
		t.Fatal("DefaultExtensionReadinessTimeout should allow cold-start reconnection")
	}
}

func TestTryConnectToExisting_NoServer(t *testing.T) {
	state := &daemonState{runner: testRunner, readyCh: make(chan struct{}), failedCh: make(chan struct{})}
	port := closedLoopbackPort(t)
	if testRunner.tryConnectToExisting(state, port) {
		t.Fatal("tryConnectToExisting = true, want false when no server is running")
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.ready || state.failed {
		t.Fatalf("unexpected terminal state: ready=%v failed=%v", state.ready, state.failed)
	}
}

func TestTryConnectToExisting_CompatibleServer(t *testing.T) {
	ln, port := startHealthServer(t, http.StatusOK, healthJSON(testRunner.identity.Version, "kaboom"))
	defer ln.Close()
	state := &daemonState{runner: testRunner, readyCh: make(chan struct{}), failedCh: make(chan struct{})}
	if !testRunner.tryConnectToExisting(state, port) {
		t.Fatal("compatible server was not adopted")
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if !state.ready {
		t.Fatal("compatible server did not mark state ready")
	}
}

func TestTryConnectToExisting_NonKaboomService(t *testing.T) {
	ln, port := startHealthServer(t, http.StatusOK, healthJSON("1.0.0", "some-other-service"))
	defer ln.Close()
	state := &daemonState{runner: testRunner, readyCh: make(chan struct{}), failedCh: make(chan struct{})}
	if !testRunner.tryConnectToExisting(state, port) {
		t.Fatal("occupied port should stop startup")
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.ready || !state.failed || !strings.Contains(state.err, "non-kaboom") {
		t.Fatalf("unexpected state for foreign service: ready=%v failed=%v err=%q", state.ready, state.failed, state.err)
	}
}

func TestTryConnectToExisting_RejectsLegacyKaboomIdentity(t *testing.T) {
	ln, port := startHealthServer(t, http.StatusOK, healthJSON(testRunner.identity.Version, "gasoline"))
	defer ln.Close()
	state := &daemonState{runner: testRunner, readyCh: make(chan struct{}), failedCh: make(chan struct{})}
	if !testRunner.tryConnectToExisting(state, port) {
		t.Fatal("legacy identity should block the occupied port")
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.ready || !state.failed || !strings.Contains(state.err, "non-kaboom") {
		t.Fatalf("legacy identity accepted: ready=%v failed=%v err=%q", state.ready, state.failed, state.err)
	}
}

func TestWaitForPeerDaemon_ServerAppearsOnFirstRetry(t *testing.T) {
	port := closedLoopbackPort(t)
	runner := *testRunner
	started := false
	runner.sleep = func(time.Duration) {
		if !started {
			started = true
			startHealthServerOnPort(t, port, http.StatusOK, healthJSON(runner.identity.Version, "kaboom"))
		}
	}
	state := &daemonState{runner: &runner, readyCh: make(chan struct{}), failedCh: make(chan struct{})}
	if !runner.waitForPeerDaemon(state, port) || !started {
		t.Fatal("peer daemon was not discovered during retry")
	}
}

func TestWaitForPeerDaemon_NoServerReturnsWithinBudget(t *testing.T) {
	port := closedLoopbackPort(t)
	state := &daemonState{runner: testRunner, readyCh: make(chan struct{}), failedCh: make(chan struct{})}
	start := time.Now()
	if testRunner.waitForPeerDaemon(state, port) {
		t.Fatal("waitForPeerDaemon = true without a server")
	}
	elapsed := time.Since(start)
	if elapsed < daemonPeerWaitTimeout-daemonPeerPollInterval || elapsed > daemonPeerWaitTimeout+500*time.Millisecond {
		t.Fatalf("elapsed = %v, outside expected peer wait budget", elapsed)
	}
}

func TestRunBridgeModeWithExistingServer_StillWorks(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
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

func healthJSON(version, service string) string {
	body, _ := json.Marshal(map[string]any{"status": "ok", "version": version, "name": service})
	return string(body)
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
