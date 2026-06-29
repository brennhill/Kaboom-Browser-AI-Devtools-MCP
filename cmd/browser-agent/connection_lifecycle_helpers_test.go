// Purpose: Tests for connection lifecycle helper utilities.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// connection_lifecycle_helpers_test.go — Shared helper functions for connection lifecycle tests.
// Contains: findFreePort, buildTestBinary, startServerCmd, stopTestServer, port utilities.
package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	statecfg "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

// Helper functions

func findFreePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to find free port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	return port
}

var (
	testBinaryOnce sync.Once
	testBinaryPath string
	testBinaryErr  error
	testStateOnce  sync.Once
	testStateDir   string
	testStateErr   error
	// testCoverDir is set from GOCOVERDIR env var; when non-empty, instrumented
	// binaries spawned via startServerCmd write coverage data to this directory.
	testCoverDir string
)

func init() {
	if dir := os.Getenv("GOCOVERDIR"); dir != "" {
		testCoverDir = dir
	}
}

func buildTestBinary(t *testing.T) string {
	t.Helper()
	testBinaryOnce.Do(func() {
		testBinaryPath = filepath.Join(os.TempDir(), "kaboom-test-binary")
		cmd := exec.Command("go", "build", "-cover", "-o", testBinaryPath, ".") // #nosec G204,G202 -- test binary from buildTestBinary(t)
		if output, err := cmd.CombinedOutput(); err != nil {
			testBinaryErr = fmt.Errorf("failed to build kaboom: %v\nOutput: %s", err, output)
		}
	})
	if testBinaryErr != nil {
		t.Fatalf("buildTestBinary: %v", testBinaryErr)
	}
	return testBinaryPath
}

func getTestStateDir(t *testing.T) string {
	t.Helper()
	testStateOnce.Do(func() {
		// Honor an externally provided state dir. The reliability soak gate sets
		// KABOOM_STATE_DIR so the server spawned by startServerCmd writes its fast-path
		// telemetry to the same root that `--check` later inspects; otherwise every
		// spawned server gets its own isolated temp dir.
		if ext := os.Getenv(statecfg.StateDirEnv); ext != "" {
			testStateDir = ext
			return
		}
		testStateDir, testStateErr = os.MkdirTemp("", "kaboom-test-state-*")
		if testStateErr != nil {
			testStateErr = fmt.Errorf("failed to create isolated test state dir: %w", testStateErr)
		}
	})
	if testStateErr != nil {
		t.Fatalf("getTestStateDir: %v", testStateErr)
	}
	return testStateDir
}

// startServerCmd creates an exec.Cmd for the test binary with GOCOVERDIR
// set in the environment when coverage collection is active.
//
// IMPORTANT: client-mode invocations can spawn a detached daemon process
// (`--daemon`) on the target port. Register per-test cleanup that always
// runs `--stop --port` to prevent daemon accumulation between test runs.
func startServerCmd(t *testing.T, binary string, args ...string) *exec.Cmd {
	t.Helper()
	stateDir := getTestStateDir(t)

	if port := parsePortArg(args); port > 0 {
		t.Cleanup(func() {
			stopTestServer(binary, port, stateDir)
		})
	}

	cmd := exec.Command(binary, args...) // #nosec G204 -- test-only: binary is from buildTestBinary(t) // nosemgrep: go.lang.security.audit.dangerous-exec-command.dangerous-exec-command, go_subproc_rule-subproc -- test spawns own binary
	cmd.Env = append(os.Environ(), statecfg.StateDirEnv+"="+stateDir)
	if testCoverDir != "" {
		cmd.Env = append(cmd.Env, "GOCOVERDIR="+testCoverDir)
	}
	return cmd
}

func parsePortArg(args []string) int {
	for i := 0; i < len(args); i++ {
		if args[i] == "--port" && i+1 < len(args) {
			if port, err := strconv.Atoi(args[i+1]); err == nil && port > 0 {
				return port
			}
			return 0
		}
		if strings.HasPrefix(args[i], "--port=") {
			raw := strings.TrimPrefix(args[i], "--port=")
			if port, err := strconv.Atoi(raw); err == nil && port > 0 {
				return port
			}
			return 0
		}
	}
	return 0
}

func stopTestServer(binary string, port int, stateDir string) {
	stopCmd := exec.Command(binary, "--stop", "--port", strconv.Itoa(port))
	stopCmd.Env = append(os.Environ(), statecfg.StateDirEnv+"="+stateDir)
	stopCmd.Stdout = io.Discard
	stopCmd.Stderr = io.Discard
	_ = stopCmd.Run()

	// Best-effort fallback if stop mode could not terminate all listeners.
	pids, err := findProcessOnPort(port)
	if err == nil {
		for _, pid := range pids {
			_ = killProcessByPID(pid)
		}
	}
	removePIDFile(port)
}

func checkSingleServerProcess(t *testing.T, port int) {
	t.Helper()
	cmd := exec.Command("lsof", "-ti", fmt.Sprintf(":%d", port))
	output, err := cmd.Output()
	if err != nil {
		// lsof returns exit status 1 when no process found, which is valid
		t.Logf("No process found on port %d (lsof returned error: %v)", port, err)
		return
	}

	pids := strings.Fields(string(output))
	if len(pids) != 1 {
		// Just log this as info - in concurrent tests there might be temporary extra processes
		t.Logf("Note: Found %d server processes on port %d (PIDs: %v) - expected 1", len(pids), port, pids)
	}
}
