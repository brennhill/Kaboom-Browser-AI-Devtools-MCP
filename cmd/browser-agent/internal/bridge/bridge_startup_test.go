// bridge_startup_test.go — Unit tests for bridge_startup.go: leader election via the
// startup lock, stale-lock reclaim, and the pure/HTTP-probe status helpers.

package bridge

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	statecfg "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

func TestBridgeStartupLock_SingleLeaderElection(t *testing.T) {
	t.Setenv(statecfg.StateDirEnv, t.TempDir())
	port := 7890

	lockA, acquired, err := tryAcquireBridgeStartupLock(port)
	if err != nil {
		t.Fatalf("tryAcquireBridgeStartupLock() error = %v", err)
	}
	if !acquired || lockA == nil {
		t.Fatal("first lock acquisition should succeed")
	}

	lockB, acquired, err := tryAcquireBridgeStartupLock(port)
	if err != nil {
		t.Fatalf("second tryAcquireBridgeStartupLock() error = %v", err)
	}
	if acquired || lockB != nil {
		t.Fatal("second lock acquisition should not succeed while first leader holds lock")
	}

	lockA.release()

	lockC, acquired, err := tryAcquireBridgeStartupLock(port)
	if err != nil {
		t.Fatalf("third tryAcquireBridgeStartupLock() error = %v", err)
	}
	if !acquired || lockC == nil {
		t.Fatal("third lock acquisition should succeed after release")
	}
	lockC.release()
}

func TestClearStaleBridgeStartupLock_RemovesDeadOwner(t *testing.T) {
	t.Setenv(statecfg.StateDirEnv, t.TempDir())
	port := 7891
	path := writeBridgeStartupLockForTest(t, port, bridgeStartupLockRecord{
		PID:       -1,
		Port:      port,
		CreatedAt: time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano),
	})

	if removed := clearStaleBridgeStartupLock(port, daemonStartupLockStaleAfter); !removed {
		t.Fatal("clearStaleBridgeStartupLock() = false, want true for dead owner")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("lock file should be removed, stat err = %v", err)
	}
}

func TestClearStaleBridgeStartupLock_PreservesRecentLiveOwner(t *testing.T) {
	t.Setenv(statecfg.StateDirEnv, t.TempDir())
	port := 7892
	path := writeBridgeStartupLockForTest(t, port, bridgeStartupLockRecord{
		PID:       os.Getpid(),
		Port:      port,
		CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
	})

	if removed := clearStaleBridgeStartupLock(port, time.Minute); removed {
		t.Fatal("clearStaleBridgeStartupLock() = true, want false for recent live owner")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("lock file should remain, stat err = %v", err)
	}
}

func writeBridgeStartupLockForTest(t *testing.T, port int, record bridgeStartupLockRecord) string {
	t.Helper()
	path, err := bridgeStartupLockPath(port)
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
	if IsServerRunning(59991) {
		t.Fatal("IsServerRunning(59991) = true, want false (nothing listening)")
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
	if ok, _, _ := runningServerVersionCompatible(59992); ok {
		t.Fatal("expected not-compatible for a dead port")
	}
}

func TestRunningServerVersionCompatible_Non200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	if ok, _, _ := runningServerVersionCompatible(portOfURL(t, srv.URL)); ok {
		t.Fatal("expected not-compatible for non-200 /health")
	}
}

func TestRunningServerVersionCompatible_HealthServed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"kaboom-browser-devtools","version":"0.8.4"}`))
	}))
	defer srv.Close()
	// Exercises the read-body, health metadata, and service/version compatibility path.
	_, _, _ = runningServerVersionCompatible(portOfURL(t, srv.URL))
}
