// Purpose: Tests for daemon lifecycle policy and shutdown, exercised through the
// REAL main-side wiring (Reclaimer.LifecycleDeps) so the seams this package hands to
// daemonlife — pid files, port probes, the Server logger — stay connected.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/procctl"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/daemonlife"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

// daemonLockForTest mirrors the on-disk daemon lock record. This test writes and
// reads the file directly rather than reaching into daemonlife's unexported
// helpers, so it verifies the persisted contract instead of package internals.
type daemonLockForTest struct {
	PID          int    `json:"pid"`
	Port         int    `json:"port"`
	StateDir     string `json:"state_dir"`
	Version      string `json:"version,omitempty"`
	UpdatedAt    string `json:"updated_at"`
	InstallEpoch int64  `json:"install_epoch,omitempty"`
}

func daemonLockPathForTest(t *testing.T) string {
	t.Helper()
	path, err := state.InRoot("run", "daemon.lock.json")
	if err != nil {
		t.Fatalf("state.InRoot() error = %v", err)
	}
	return path
}

func writeDaemonLockForTest(t *testing.T, rec daemonLockForTest) {
	t.Helper()
	if rec.UpdatedAt == "" {
		rec.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	path := daemonLockPathForTest(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	data, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("json.Marshal(lock) error = %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

// readDaemonLockForTest returns nil when no lock file exists.
func readDaemonLockForTest(t *testing.T) *daemonLockForTest {
	t.Helper()
	data, err := os.ReadFile(daemonLockPathForTest(t))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatalf("ReadFile(daemon lock) error = %v", err)
	}
	var rec daemonLockForTest
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatalf("json.Unmarshal(daemon lock) error = %v", err)
	}
	return &rec
}

func writeDaemonPIDFileForTest(t *testing.T, port int, pid int) {
	t.Helper()
	path := procctl.PIDFilePath(port)
	if path == "" {
		t.Fatal("procctl.PIDFilePath returned empty path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(strconv.Itoa(pid)), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func readLifecycleEventsFromLogFile(t *testing.T, logFile string) []map[string]any {
	t.Helper()
	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", logFile, err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	events := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("json.Unmarshal(log line) error = %v, line=%q", err, line)
		}
		if event, _ := entry["event"].(string); event != "" {
			events = append(events, entry)
		}
	}
	return events
}

func TestEnforceDaemonStartupPolicy_DefaultTakeover(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(state.StateDirEnv, stateRoot)

	const existingPID = 42424
	const existingPort = 7890
	const requestedPort = 7891

	writeDaemonLockForTest(t, daemonLockForTest{
		PID:      existingPID,
		Port:     existingPort,
		StateDir: stateRoot,
		Version:  "0.7.7",
	})
	writeDaemonPIDFileForTest(t, existingPort, existingPID)

	logFile := filepath.Join(t.TempDir(), "daemon-policy.log")
	server, err := NewServer(logFile, 200)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	deps := server.daemonRecovery.LifecycleDeps()
	deps.IsProcessAlive = func(pid int) bool { return pid == existingPID }
	deps.IsServerRunning = func(port int) bool { return port == existingPort }
	deps.TryShutdown = func(port int) bool { return port == existingPort }
	waitCalls := 0
	deps.WaitForPortRelease = func(port int, _ time.Duration) bool {
		if port != existingPort {
			return false
		}
		waitCalls++
		return waitCalls >= 2
	}
	terminatedPIDs := make([]int, 0, 2)
	deps.TerminatePID = func(pid int, _ bool) {
		terminatedPIDs = append(terminatedPIDs, pid)
	}

	if err := daemonlife.EnforceStartupPolicy(deps, requestedPort, daemonlife.LaunchOptions{}); err != nil {
		t.Fatalf("daemonlife.EnforceStartupPolicy() error = %v", err)
	}

	if len(terminatedPIDs) != 1 || terminatedPIDs[0] != existingPID {
		t.Fatalf("terminate calls = %v, want [%d]", terminatedPIDs, existingPID)
	}

	lockAfter := readDaemonLockForTest(t)
	if lockAfter != nil {
		t.Fatalf("daemon lock should be removed after takeover, got %+v", *lockAfter)
	}

	if _, err := os.Stat(procctl.PIDFilePath(existingPort)); !os.IsNotExist(err) {
		t.Fatalf("pid file for existing port should be removed, stat err = %v", err)
	}

	server.logs.Shutdown(2 * time.Second)
	events := readLifecycleEventsFromLogFile(t, logFile)
	var takeover map[string]any
	for _, evt := range events {
		if evtName, _ := evt["event"].(string); evtName == "daemon_takeover" {
			takeover = evt
			break
		}
	}
	if takeover == nil {
		t.Fatal("expected daemon_takeover lifecycle event")
	}
	if got, _ := takeover["existing_pid"].(float64); int(got) != existingPID {
		t.Fatalf("daemon_takeover existing_pid = %v, want %d", takeover["existing_pid"], existingPID)
	}
	if got, _ := takeover["existing_port"].(float64); int(got) != existingPort {
		t.Fatalf("daemon_takeover existing_port = %v, want %d", takeover["existing_port"], existingPort)
	}
	if got, _ := takeover["new_pid"].(float64); int(got) != os.Getpid() {
		t.Fatalf("daemon_takeover new_pid = %v, want %d", takeover["new_pid"], os.Getpid())
	}
	if takeoverFlag, _ := takeover["takeover"].(bool); !takeoverFlag {
		t.Fatalf("daemon_takeover takeover = %v, want true", takeover["takeover"])
	}
	if stateDir, _ := takeover["state_dir"].(string); stateDir != stateRoot {
		t.Fatalf("daemon_takeover state_dir = %q, want %q", stateDir, stateRoot)
	}
}

func TestEnforceDaemonStartupPolicy_SafetyGuardRejectsPIDMismatch(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(state.StateDirEnv, stateRoot)

	const existingPID = 51515
	const existingPort = 7900

	writeDaemonLockForTest(t, daemonLockForTest{
		PID:      existingPID,
		Port:     existingPort,
		StateDir: stateRoot,
		Version:  "0.7.7",
	})

	writeDaemonPIDFileForTest(t, existingPort, existingPID+1)

	logFile := filepath.Join(t.TempDir(), "daemon-policy-mismatch.log")
	server, err := NewServer(logFile, 200)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	defer server.logs.Shutdown(2 * time.Second)

	deps := server.daemonRecovery.LifecycleDeps()
	deps.IsProcessAlive = func(pid int) bool { return pid == existingPID }
	deps.IsServerRunning = func(port int) bool { return port == existingPort }
	terminated := false
	deps.TerminatePID = func(_ int, _ bool) { terminated = true }

	err = daemonlife.EnforceStartupPolicy(deps, 7901, daemonlife.LaunchOptions{})
	if err == nil {
		t.Fatal("daemonlife.EnforceStartupPolicy() error = nil, want ownership mismatch error")
	}
	if !strings.Contains(err.Error(), "ownership mismatch") {
		t.Fatalf("error = %q, want ownership mismatch guidance", err.Error())
	}
	if terminated {
		t.Fatal("safety guard should not terminate process on PID mismatch")
	}
}

func TestEnforceDaemonStartupPolicy_ParallelRequiresIsolatedStateDir(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(state.StateDirEnv, stateRoot)

	writeDaemonLockForTest(t, daemonLockForTest{
		PID:      30303,
		Port:     7920,
		StateDir: stateRoot,
		Version:  "0.7.7",
	})

	logFile := filepath.Join(t.TempDir(), "daemon-policy-parallel.log")
	server, err := NewServer(logFile, 200)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	defer server.logs.Shutdown(2 * time.Second)

	deps := server.daemonRecovery.LifecycleDeps()
	deps.IsProcessAlive = func(pid int) bool { return pid == 30303 }
	terminated := false
	shutdownCalled := false
	deps.TerminatePID = func(_ int, _ bool) { terminated = true }
	deps.TryShutdown = func(_ int) bool {
		shutdownCalled = true
		return false
	}

	err = daemonlife.EnforceStartupPolicy(deps, 7921, daemonlife.LaunchOptions{Parallel: true})
	if err == nil {
		t.Fatal("daemonlife.EnforceStartupPolicy() error = nil, want isolated state-dir error")
	}
	if !strings.Contains(err.Error(), "isolated --state-dir") {
		t.Fatalf("error = %q, want isolated state-dir guidance", err.Error())
	}
	if terminated || shutdownCalled {
		t.Fatalf("parallel mode should not takeover/kill existing daemon; terminated=%v shutdownCalled=%v", terminated, shutdownCalled)
	}
}

func TestEnforceDaemonStartupPolicy_ReclaimsStaleLockOnPIDMismatchWhenPortIdle(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(state.StateDirEnv, stateRoot)

	const existingPID = 61616
	const existingPort = 7930

	writeDaemonLockForTest(t, daemonLockForTest{
		PID:      existingPID,
		Port:     existingPort,
		StateDir: stateRoot,
		Version:  "0.7.7",
	})

	// PID mismatch: port pid file does not match lock owner.
	writeDaemonPIDFileForTest(t, existingPort, existingPID+1)

	logFile := filepath.Join(t.TempDir(), "daemon-policy-reclaim.log")
	server, err := NewServer(logFile, 200)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	deps := server.daemonRecovery.LifecycleDeps()
	deps.IsProcessAlive = func(pid int) bool { return pid == existingPID }
	deps.IsServerRunning = func(port int) bool { return false }
	terminated := false
	deps.TerminatePID = func(_ int, _ bool) { terminated = true }

	if err := daemonlife.EnforceStartupPolicy(deps, 7931, daemonlife.LaunchOptions{}); err != nil {
		t.Fatalf("daemonlife.EnforceStartupPolicy() error = %v, want stale lock reclaimed", err)
	}
	if terminated {
		t.Fatal("stale lock reclaim should not terminate any process")
	}
	lockAfter := readDaemonLockForTest(t)
	if lockAfter != nil {
		t.Fatalf("daemon lock should be removed after stale reclaim, got %+v", *lockAfter)
	}
	if _, err := os.Stat(procctl.PIDFilePath(existingPort)); !os.IsNotExist(err) {
		t.Fatalf("pid file for stale lock port should be removed, stat err = %v", err)
	}

	server.logs.Shutdown(2 * time.Second)
	events := readLifecycleEventsFromLogFile(t, logFile)
	found := false
	for _, evt := range events {
		if evtName, _ := evt["event"].(string); evtName == "daemon_lock_reclaimed_stale_mismatch" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected daemon_lock_reclaimed_stale_mismatch lifecycle event")
	}
}
