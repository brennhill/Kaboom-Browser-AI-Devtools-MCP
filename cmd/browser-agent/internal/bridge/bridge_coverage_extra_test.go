// bridge_coverage_extra_test.go -- Additional unit tests raising coverage of telemetry,
// push relay, startup-lock, fingerprint, and daemon status/respawn helpers.
// These tests are deterministic: no sleeps on the critical path and no real daemon spawns.

package bridge

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	internbridge "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/bridge"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/push"
	statecfg "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

func TestForceKillOnPortHandlesLookupFailuresAndSkipsSelf(t *testing.T) {
	runner := NewRunner(Identity{}, Transport{}, Protocol{}, Lifecycle{
		FindProcessOnPort: func(int) ([]int, error) { return nil, errors.New("lookup unavailable") },
	})
	runner.forceKillOnPort(7890)

	runner.lifecycle.FindProcessOnPort = func(int) ([]int, error) {
		return []int{os.Getpid(), 99999999}, nil
	}
	runner.forceKillOnPort(7890)
}

func TestNormalizeMCPPayloadRejectsProtocolNoise(t *testing.T) {
	valid := []byte("  {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{}}  ")
	if got := normalizeMCPPayload(valid); string(got) != strings.TrimSpace(string(valid)) {
		t.Fatalf("normalized valid payload = %q", got)
	}
	var response mcp.JSONRPCResponse
	if err := json.Unmarshal(normalizeMCPPayload([]byte("diagnostic noise")), &response); err != nil {
		t.Fatalf("invalid payload replacement is not JSON: %v", err)
	}
	if response.Error == nil || response.Error.Code != -32603 {
		t.Fatalf("invalid payload replacement = %+v", response)
	}
}

// --- Fast-path event telemetry ---

func TestRecordFastPathEvent_WritesTelemetryLog(t *testing.T) {
	t.Setenv(statecfg.StateDirEnv, t.TempDir())
	resetFastPathCounters()

	testRunner.RecordFastPathEvent("tools/call", true, 0)
	testRunner.RecordFastPathEvent("tools/call", false, -32000)
	FlushFastPathTelemetry()

	path, err := FastPathTelemetryLogPath()
	if err != nil {
		t.Fatalf("FastPathTelemetryLogPath() error = %v", err)
	}
	summary := summarizeFastPathTelemetryLog(path, 100)
	if summary.total != 2 {
		t.Fatalf("summary.total = %d, want 2", summary.total)
	}
	if summary.success != 1 || summary.failure != 1 {
		t.Fatalf("summary success/failure = %d/%d, want 1/1", summary.success, summary.failure)
	}
	if summary.errorCodes[-32000] != 1 {
		t.Fatalf("errorCodes[-32000] = %d, want 1", summary.errorCodes[-32000])
	}
}

func TestResetFastPathCounters_ResetsSuccessCount(t *testing.T) {
	t.Setenv(statecfg.StateDirEnv, t.TempDir())
	resetFastPathCounters()

	testRunner.RecordFastPathEvent("tools/call", true, 0)
	testRunner.RecordFastPathEvent("tools/call", true, 0)
	ResetFastPathCounters()
	testRunner.RecordFastPathEvent("tools/call", true, 0)
	FlushFastPathTelemetry()

	path, err := FastPathTelemetryLogPath()
	if err != nil {
		t.Fatalf("FastPathTelemetryLogPath() error = %v", err)
	}
	data, err := os.ReadFile(path) //nolint:gosec // test-owned temp path
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 {
		t.Fatalf("log lines = %d, want 3", len(lines))
	}
	var last map[string]any
	if err := json.Unmarshal([]byte(lines[2]), &last); err != nil {
		t.Fatalf("unmarshal last line: %v", err)
	}
	if got, _ := last["success_count"].(float64); got != 1 {
		t.Fatalf("success_count after reset = %v, want 1", last["success_count"])
	}
}

// --- Fast-path resource-read telemetry ---

func TestRecordFastPathResourceRead_CountersAndLog(t *testing.T) {
	t.Setenv(statecfg.StateDirEnv, t.TempDir())
	resetFastPathResourceReadCounters()

	testRunner.RecordFastPathResourceRead("kaboom://capabilities", true, 0)
	testRunner.RecordFastPathResourceRead("kaboom://capabilities", false, 404)
	FlushFastPathTelemetry()

	success, failure := SnapshotFastPathResourceReadCounters()
	if success != 1 || failure != 1 {
		t.Fatalf("snapshot success/failure = %d/%d, want 1/1", success, failure)
	}

	path, err := FastPathResourceReadLogPath()
	if err != nil {
		t.Fatalf("FastPathResourceReadLogPath() error = %v", err)
	}
	data, err := os.ReadFile(path) //nolint:gosec // test-owned temp path
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("resource-read log lines = %d, want 2", len(lines))
	}

	ResetFastPathResourceReadCounters()
	success, failure = SnapshotFastPathResourceReadCounters()
	if success != 0 || failure != 0 {
		t.Fatalf("snapshot after reset = %d/%d, want 0/0", success, failure)
	}
}

// --- Push relay ---

func TestBuildPushNotification(t *testing.T) {
	t.Parallel()

	ev := push.PushEvent{Type: "screenshot", PageURL: "https://example.com/page"}
	payload := BuildPushNotification(ev)
	if payload == nil {
		t.Fatal("BuildPushNotification returned nil")
	}
	var notif map[string]any
	if err := json.Unmarshal(payload, &notif); err != nil {
		t.Fatalf("unmarshal notification: %v", err)
	}
	if notif["method"] != "notifications/message" {
		t.Fatalf("method = %v, want notifications/message", notif["method"])
	}
	params, _ := notif["params"].(map[string]any)
	data, _ := params["data"].(map[string]any)
	if data["type"] != "screenshot" {
		t.Fatalf("data.type = %v, want screenshot", data["type"])
	}
	if data["page_url"] != "https://example.com/page" {
		t.Fatalf("data.page_url = %v, want https://example.com/page", data["page_url"])
	}
}

func TestRelayPendingPushEvents_RelaysEvents(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/push/drain" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"events": []push.PushEvent{{ID: "1", Type: "chat", PageURL: "https://x.test", Message: "hi"}},
			"count":  1,
		})
	}))
	defer srv.Close()

	client := &http.Client{}
	output := captureBridgeIO(t, "", func() {
		testRunner.relayPendingPushEvents(client, srv.URL)
	})
	if !strings.Contains(output, "sampling/createMessage") {
		t.Fatalf("expected sampling/createMessage in relayed output, got: %q", output)
	}
}

func TestRelayPendingPushEvents_NoOutputCases(t *testing.T) {
	cases := []struct {
		name    string
		handler http.HandlerFunc
	}{
		{
			name: "zero count",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				_ = json.NewEncoder(w).Encode(map[string]any{"events": []push.PushEvent{}, "count": 0})
			},
		},
		{
			name: "non-200 status",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
		},
		{
			name: "invalid json",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte("{not-json"))
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(tc.handler)
			defer srv.Close()

			client := &http.Client{}
			output := captureBridgeIO(t, "", func() {
				testRunner.relayPendingPushEvents(client, srv.URL)
			})
			if strings.TrimSpace(output) != "" {
				t.Fatalf("expected no relayed output, got: %q", output)
			}
		})
	}
}

func TestRelayPendingPushEvents_UnreachableEndpoint(t *testing.T) {
	// Nothing listening on this port: client.Do returns an error, no output.
	port := freeLocalPort(t)
	endpoint := "http://127.0.0.1:" + strconv.Itoa(port)

	client := &http.Client{Timeout: time.Second}
	output := captureBridgeIO(t, "", func() {
		testRunner.relayPendingPushEvents(client, endpoint)
	})
	if strings.TrimSpace(output) != "" {
		t.Fatalf("expected no output for unreachable endpoint, got: %q", output)
	}
}

// --- bridgeRequestIDString additional cases ---

type coverageStringer struct{}

func (coverageStringer) String() string { return "stringer-val" }

func TestBridgeRequestIDString_AdditionalCases(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		id   any
		want string
	}{
		{name: "empty raw", id: json.RawMessage(``), want: ""},
		{name: "non-parseable raw", id: json.RawMessage(`{bad`), want: "{bad"},
		{name: "stringer", id: coverageStringer{}, want: "stringer-val"},
		{name: "int", id: 7, want: "7"},
		{name: "int64", id: int64(9), want: "9"},
		{name: "default bool", id: true, want: "true"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := bridgeRequestIDString(tc.id); got != tc.want {
				t.Fatalf("bridgeRequestIDString(%v) = %q, want %q", tc.id, got, tc.want)
			}
		})
	}
}

// --- Startup lock record parsing ---

func TestReadBridgeStartupLockRecord_NonExistent(t *testing.T) {
	t.Setenv(statecfg.StateDirEnv, t.TempDir())
	path, err := bridgeStartupLockPath(7901)
	if err != nil {
		t.Fatalf("bridgeStartupLockPath() error = %v", err)
	}
	rec, err := readBridgeStartupLockRecord(path)
	if err != nil {
		t.Fatalf("readBridgeStartupLockRecord() error = %v, want nil", err)
	}
	if rec != nil {
		t.Fatalf("record = %+v, want nil for non-existent file", rec)
	}
}

func TestReadBridgeStartupLockRecord_Malformed(t *testing.T) {
	t.Setenv(statecfg.StateDirEnv, t.TempDir())
	path, err := bridgeStartupLockPath(7902)
	if err != nil {
		t.Fatalf("bridgeStartupLockPath() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	if err := os.WriteFile(path, []byte("{not valid json"), 0o600); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	if _, err := readBridgeStartupLockRecord(path); err == nil {
		t.Fatal("expected error for malformed lock record")
	}
}

func TestParseBridgeStartupLockTime(t *testing.T) {
	t.Parallel()

	if _, err := parseBridgeStartupLockTime(time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatalf("RFC3339Nano parse error = %v", err)
	}
	if _, err := parseBridgeStartupLockTime(time.Now().UTC().Format(time.RFC3339)); err != nil {
		t.Fatalf("RFC3339 parse error = %v", err)
	}
	if _, err := parseBridgeStartupLockTime("not-a-timestamp"); err == nil {
		t.Fatal("expected error for invalid timestamp")
	}
}

// --- clearStaleBridgeStartupLock edge cases ---

func TestClearStaleBridgeStartupLock_NoRecord(t *testing.T) {
	t.Setenv(statecfg.StateDirEnv, t.TempDir())
	if removed := testRunner.clearStaleBridgeStartupLock(7903, time.Minute); removed {
		t.Fatal("testRunner.clearStaleBridgeStartupLock() = true, want false when no lock exists")
	}
}

func TestClearStaleBridgeStartupLock_RemovesExpiredLiveOwner(t *testing.T) {
	t.Setenv(statecfg.StateDirEnv, t.TempDir())
	port := 7904
	writeBridgeStartupLockForTest(t, port, bridgeStartupLockRecord{
		PID:       os.Getpid(), // alive
		Port:      port,
		CreatedAt: time.Now().Add(-time.Hour).UTC().Format(time.RFC3339Nano),
	})
	if removed := testRunner.clearStaleBridgeStartupLock(port, time.Second); !removed {
		t.Fatal("testRunner.clearStaleBridgeStartupLock() = false, want true for expired lock")
	}
}

func TestClearStaleBridgeStartupLock_RemovesMalformedRecord(t *testing.T) {
	t.Setenv(statecfg.StateDirEnv, t.TempDir())
	port := 7905
	path, err := bridgeStartupLockPath(port)
	if err != nil {
		t.Fatalf("bridgeStartupLockPath() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	if err := os.WriteFile(path, []byte("{corrupt"), 0o600); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}
	if removed := testRunner.clearStaleBridgeStartupLock(port, time.Minute); !removed {
		t.Fatal("testRunner.clearStaleBridgeStartupLock() = false, want true for malformed record")
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("malformed lock should be removed, stat err = %v", statErr)
	}
}

func TestClearStaleBridgeStartupLock_RemovesInvalidCreatedAt(t *testing.T) {
	t.Setenv(statecfg.StateDirEnv, t.TempDir())
	port := 7906
	writeBridgeStartupLockForTest(t, port, bridgeStartupLockRecord{
		PID:       os.Getpid(), // alive
		Port:      port,
		CreatedAt: "not-a-real-time",
	})
	if removed := testRunner.clearStaleBridgeStartupLock(port, time.Minute); !removed {
		t.Fatal("testRunner.clearStaleBridgeStartupLock() = false, want true for invalid created_at")
	}
}

// --- Lock release ownership ---

func TestBridgeStartupLockRelease_NonOwnerKeepsFile(t *testing.T) {
	t.Setenv(statecfg.StateDirEnv, t.TempDir())
	port := 7907
	path := writeBridgeStartupLockForTest(t, port, bridgeStartupLockRecord{
		PID:       999999, // different owner
		Port:      port,
		CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
	})

	lock := &bridgeStartupLock{path: path, pid: 12345}
	lock.release()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("lock file should remain when releaser is not owner, stat err = %v", err)
	}

	// nil and empty-path releases must be no-ops (no panic).
	var nilLock *bridgeStartupLock
	nilLock.release()
	(&bridgeStartupLock{}).release()
}

// --- daemonStartupSuggestion ---

func TestDaemonStartupSuggestion(t *testing.T) {
	t.Parallel()

	portSuggestion := daemonStartupSuggestion("bind: address already in use", 7890)
	if !strings.Contains(portSuggestion, "--port 7891") {
		t.Fatalf("port suggestion = %q, want it to suggest --port 7891", portSuggestion)
	}

	genericSuggestion := daemonStartupSuggestion("some unrelated failure", 7890)
	if !strings.Contains(genericSuggestion, "--doctor") {
		t.Fatalf("generic suggestion = %q, want it to suggest --doctor", genericSuggestion)
	}
}

// --- runningServerVersionCompatible error/edge paths ---

func TestRunningServerVersionCompatible_ErrorPaths(t *testing.T) {
	oldVersion := testRunner.identity.Version
	testRunner.identity.Version = "9.9.9"
	t.Cleanup(func() { testRunner.identity.Version = oldVersion })

	// Connection error: nothing is listening.
	unusedPort := freeLocalPort(t)
	if ok, v, s := testRunner.runningServerVersionCompatible(unusedPort); ok || v != "" || s != "" {
		t.Fatalf("connection error case = (%v, %q, %q), want (false, \"\", \"\")", ok, v, s)
	}

	var mode string
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		switch mode {
		case "status500":
			w.WriteHeader(http.StatusInternalServerError)
		case "nonkaboom":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"name":"some-other-service","version":"1.2.3"}`))
		case "missingversion":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"name":"kaboom","version":""}`))
		default:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"name":"kaboom","version":"9.9.9"}`))
		}
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen error = %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	srv := &http.Server{Handler: mux} //nolint:gosec // test server, no timeouts needed
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })

	mode = "status500"
	if ok, v, s := testRunner.runningServerVersionCompatible(port); ok || v != "" || s != "" {
		t.Fatalf("status500 case = (%v, %q, %q), want (false, \"\", \"\")", ok, v, s)
	}

	mode = "nonkaboom"
	if ok, v, s := testRunner.runningServerVersionCompatible(port); ok || v != "1.2.3" || s != "some-other-service" {
		t.Fatalf("nonkaboom case = (%v, %q, %q), want (false, \"1.2.3\", \"some-other-service\")", ok, v, s)
	}

	mode = "missingversion"
	if ok, v, s := testRunner.runningServerVersionCompatible(port); ok || v != "<missing>" || s != "kaboom" {
		t.Fatalf("missingversion case = (%v, %q, %q), want (false, \"<missing>\", \"kaboom\")", ok, v, s)
	}

	mode = "match"
	if ok, v, s := testRunner.runningServerVersionCompatible(port); !ok || v != "9.9.9" || s != "kaboom" {
		t.Fatalf("match case = (%v, %q, %q), want (true, \"9.9.9\", \"kaboom\")", ok, v, s)
	}
}

// --- checkDaemonStatus failed branch + respawn without a valid port ---

func TestCheckDaemonStatus_FailedStateReturnsSuggestion(t *testing.T) {
	state := &daemonState{runner: testRunner, readyCh: make(chan struct{}), failedCh: make(chan struct{})}
	state.markFailed("bind: address already in use")

	status := checkDaemonStatus(state, mcp.JSONRPCRequest{Method: "tools/call"}, 7890)
	if !strings.Contains(status, "Server failed to start") {
		t.Fatalf("status = %q, want daemon startup suggestion", status)
	}
	if !strings.Contains(status, "--port 7891") {
		t.Fatalf("status = %q, want it to suggest an alternate port", status)
	}
}

func TestCheckDaemonStatus_NonDaemonMethodReturnsMethodNotFound(t *testing.T) {
	state := &daemonState{runner: testRunner, readyCh: make(chan struct{}), failedCh: make(chan struct{})}
	if status := checkDaemonStatus(state, mcp.JSONRPCRequest{Method: "some/other"}, 7890); status != "method_not_found" {
		t.Fatalf("status = %q, want method_not_found", status)
	}
}

// --- respawnIfNeeded peer-wait paths (deterministic via pre-closed channels) ---

func TestRespawnIfNeeded_WaitForPeerReadySignal(t *testing.T) {
	state := &daemonState{runner: testRunner, readyCh: make(chan struct{}), failedCh: make(chan struct{})}
	close(state.readyCh) // simulate a concurrent respawn leader signaling ready
	if !state.respawnIfNeeded() {
		t.Fatal("respawnIfNeeded() = false, want true when peer readyCh is closed")
	}
}

func TestRespawnIfNeeded_WaitForPeerFailedSignal(t *testing.T) {
	state := &daemonState{runner: testRunner, readyCh: make(chan struct{}), failedCh: make(chan struct{})}
	close(state.failedCh) // simulate a concurrent respawn leader signaling failure
	if state.respawnIfNeeded() {
		t.Fatal("respawnIfNeeded() = true, want false when peer failedCh is closed")
	}
}

func TestRespawnIfNeeded_InvalidPortMarksFailed(t *testing.T) {
	state := &daemonState{runner: testRunner, readyCh: make(chan struct{}), failedCh: make(chan struct{})}
	state.markFailed("prior failure") // failed state so planRespawnAttempt does not wait for a peer
	if state.respawnIfNeeded() {
		t.Fatal("respawnIfNeeded() = true, want false for invalid (zero) port")
	}
	if got := DaemonFailureErr(state); !strings.Contains(got, "valid daemon port") {
		t.Fatalf("failure err = %q, want it to mention a valid daemon port", got)
	}
}

// --- waitForRespawnPeerSignals ---

func TestWaitForRespawnPeerSignals(t *testing.T) {
	t.Parallel()

	readyCh := make(chan struct{})
	close(readyCh)
	if res := waitForRespawnPeerSignals(respawnPlan{readyCh: readyCh, failedCh: make(chan struct{})}, time.Second); !res.ready {
		t.Fatalf("ready result = %+v, want ready", res)
	}

	failedCh := make(chan struct{})
	close(failedCh)
	if res := waitForRespawnPeerSignals(respawnPlan{readyCh: make(chan struct{}), failedCh: failedCh}, time.Second); !res.failed {
		t.Fatalf("failed result = %+v, want failed", res)
	}

	if res := waitForRespawnPeerSignals(respawnPlan{readyCh: make(chan struct{}), failedCh: make(chan struct{})}, 10*time.Millisecond); !res.timedOut {
		t.Fatalf("timeout result = %+v, want timedOut", res)
	}

	// timeout <= 0 falls back to the default; a pre-closed ready channel returns immediately.
	readyCh2 := make(chan struct{})
	close(readyCh2)
	if res := waitForRespawnPeerSignals(respawnPlan{readyCh: readyCh2, failedCh: make(chan struct{})}, 0); !res.ready {
		t.Fatalf("default-timeout result = %+v, want ready", res)
	}
}

// --- handleBridgeRestart non-restart fast-path rejection ---

func TestHandleBridgeRestart_NonRestartReturnsFalse(t *testing.T) {
	req := mcp.JSONRPCRequest{
		Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"errors"}}`),
	}
	state := &daemonState{runner: testRunner, readyCh: make(chan struct{}), failedCh: make(chan struct{})}
	if testRunner.handleBridgeRestart(req, state, 7890, internbridge.StdioFramingLine) {
		t.Fatal("testRunner.handleBridgeRestart() = true, want false for non-restart request")
	}
}

// --- helpers ---

func freeLocalPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen error = %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return port
}
