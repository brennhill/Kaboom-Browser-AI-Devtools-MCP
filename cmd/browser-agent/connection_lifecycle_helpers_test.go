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
			return
		}
		warmTestBinary(testBinaryPath)
	})
	if testBinaryErr != nil {
		t.Fatalf("buildTestBinary: %v", testBinaryErr)
	}
	return testBinaryPath
}

// warmTestBinary runs the freshly built binary once so no timed test pays the
// one-time cold-exec cost.
//
// Measured on this repo: the first exec of a just-built 16 MB coverage binary
// takes ~520ms (page-in from disk + macOS code-signature validation); every exec
// after is ~0ms. That 520ms landed inside whichever test spawned first, which is
// why TestFastStart_ClientCompatibilityMatrix/claude_code — the first subtest —
// was the one that blew its read timeout under full-suite load while subtests
// 2..4 passed. Paying it here means every measurement is steady state.
//
// Best-effort: a failure here is not a test failure. `--version` is a pure
// print-and-exit path, so it touches no ports, no state dir, and no daemon.
func warmTestBinary(binary string) {
	cmd := exec.Command(binary, "--version") // #nosec G204 -- test-only: binary is from buildTestBinary(t) // nosemgrep: go.lang.security.audit.dangerous-exec-command.dangerous-exec-command, go_subproc_rule-subproc -- test spawns own binary
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	_ = cmd.Run()
}

func getTestStateDir(t *testing.T) string {
	t.Helper()
	testStateOnce.Do(func() {
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
