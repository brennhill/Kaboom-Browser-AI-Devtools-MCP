// stop_test.go — Tests daemon stop and cleanup ownership.

package procctl

import (
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestCleanupPIDFilesRemovesCanonicalPIDFile(t *testing.T) {
	t.Setenv("KABOOM_STATE_DIR", t.TempDir())
	ports := []int{7890, 7910, 17890}
	for _, port := range ports {
		path := PIDFilePath(port)
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("MkdirAll() error = %v", err)
		}
		if err := os.WriteFile(path, []byte("424242"), 0o600); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
	}

	CleanupPIDFiles()

	for _, port := range ports {
		if pid := ReadPIDFile(port); pid != 0 {
			t.Fatalf("ReadPIDFile(%d) = %d after cleanup, want 0", port, pid)
		}
	}
}

func TestStopUsesLocalShutdownEndpoint(t *testing.T) {
	t.Setenv("KABOOM_STATE_DIR", t.TempDir())
	var shutdownCalled atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/shutdown" {
			t.Errorf("request = %s %s, want POST /shutdown", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		shutdownCalled.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	_, portText, err := net.SplitHostPort(strings.TrimPrefix(server.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatal(err)
	}

	Stop(port, func(int) bool {
		t.Fatal("process lookup should not run after successful HTTP shutdown")
		return true
	})

	if !shutdownCalled.Load() {
		t.Fatal("shutdown endpoint was not called")
	}
	if _, err := os.Stat(PIDFilePath(port)); !os.IsNotExist(err) {
		t.Fatalf("PID file survived stop: %v", err)
	}
}

func TestStopUsesOwnedPIDFile(t *testing.T) {
	t.Setenv("KABOOM_STATE_DIR", t.TempDir())
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Skipf("sleep unavailable: %v", err)
	}
	waited := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(waited)
	}()
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		<-waited
	})

	const port = 17890
	path := PIDFilePath(port)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strconv.Itoa(cmd.Process.Pid)), 0o600); err != nil {
		t.Fatal(err)
	}

	Stop(port, func(int) bool {
		t.Fatal("fallback should not run when the PID file identifies a live process")
		return true
	})

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("PID file survived stop: %v", err)
	}
}

func TestStopHandlesAlreadyStoppedPort(t *testing.T) {
	t.Setenv("KABOOM_STATE_DIR", t.TempDir())
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}

	Stop(port, func(int) bool {
		t.Fatal("empty process lookup should not need a running-state probe")
		return false
	})
}

func TestForceCleanupWithNoMatchingProcesses(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture uses POSIX shell scripts")
	}
	bin := t.TempDir()
	for _, name := range []string{"lsof", "pkill"} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
	t.Setenv("KABOOM_STATE_DIR", t.TempDir())
	ForceCleanup()
	if err := ForceCleanupQuietly(); err != nil {
		t.Fatal(err)
	}
}

func TestKillWindowsKaboomProcessesParsesSuccesses(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture uses a POSIX shell script")
	}
	bin := t.TempDir()
	taskkill := filepath.Join(bin, "taskkill")
	if err := os.WriteFile(taskkill, []byte("#!/bin/sh\necho SUCCESS one\necho terminated two\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	if got := killWindowsKaboomProcesses(); got != 2 {
		t.Fatalf("killWindowsKaboomProcesses = %d", got)
	}
	if got := killWindowsKaboomProcessesQuietly(); got != 0 {
		t.Fatalf("quiet killed = %d", got)
	}
}

func TestTerminateProcessStopsChild(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture uses POSIX shell")
	}
	cmd := exec.Command("sh", "-c", "trap 'exit 0' TERM; while :; do sleep 1; done")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	killed, failed := terminateProcess(cmd.Process.Pid)
	if killed != 1 || failed != 0 {
		t.Fatalf("terminateProcess = %d, %d", killed, failed)
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("child survived termination")
	}
}
