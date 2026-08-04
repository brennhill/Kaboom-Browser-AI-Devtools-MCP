// Purpose: Contract tests for connection PID tracking.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package main

import (
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/procctl"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

func freePortForTest(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen(:0) error = %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return port
}

func TestCleanupStalePIDFile_AliveUnrelatedProcessDoesNotBlock(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses unix sleep process for deterministic live PID")
	}

	stateRoot := t.TempDir()
	t.Setenv(state.StateDirEnv, stateRoot)

	port := freePortForTest(t)
	if bridgeRuntime().IsServerRunning(port) {
		t.Fatalf("test precondition failed: port %d unexpectedly in use", port)
	}

	sleepCmd := exec.Command("sh", "-c", "sleep 30")
	if err := sleepCmd.Start(); err != nil {
		t.Fatalf("sleep process start error = %v", err)
	}
	t.Cleanup(func() {
		if sleepCmd.Process != nil {
			_ = sleepCmd.Process.Kill()
		}
	})

	pidPath := procctl.PIDFilePath(port)
	if pidPath == "" {
		t.Fatal("procctl.PIDFilePath returned empty path")
	}
	if err := os.MkdirAll(filepath.Dir(pidPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(pidPath), err)
	}
	if err := os.WriteFile(pidPath, []byte("999999"), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", pidPath, err)
	}
	// Replace with a real live PID that does NOT own the port.
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(sleepCmd.Process.Pid)), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", pidPath, err)
	}

	logFile := filepath.Join(t.TempDir(), "pid-reuse-contract.jsonl")
	server, err := NewServer(logFile, 100)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	defer server.logs.Shutdown(2 * time.Second)

	err = cleanupStalePIDFile(server, port)
	if err != nil {
		t.Fatalf("cleanupStalePIDFile() error = %v, want nil for unrelated live PID", err)
	}
	if _, statErr := os.Stat(pidPath); !os.IsNotExist(statErr) {
		t.Fatalf("expected stale pid file %q removed, stat err = %v", pidPath, statErr)
	}
}

func TestCleanupPIDFiles_CoversCrossWrapperKnownPorts(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(state.StateDirEnv, stateRoot)

	ports := []int{7890, 7910, 17890}
	for _, port := range ports {
		pidPath := procctl.PIDFilePath(port)
		if pidPath == "" {
			t.Fatalf("procctl.PIDFilePath(%d) returned empty path", port)
		}
		if err := os.MkdirAll(filepath.Dir(pidPath), 0o755); err != nil {
			t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(pidPath), err)
		}
		if err := os.WriteFile(pidPath, []byte("12345"), 0o600); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", pidPath, err)
		}
	}

	procctl.CleanupPIDFiles()

	for _, port := range ports {
		pidPath := procctl.PIDFilePath(port)
		if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
			t.Fatalf("expected procctl.CleanupPIDFiles to remove %q, stat err = %v", pidPath, err)
		}
	}
}
