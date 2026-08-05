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
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/procctl"

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
	testStateMu    sync.Mutex
	testStateDirs  = make(map[string]string)
	// testCoverDir is set from GOCOVERDIR env var; when non-empty, instrumented
	// binaries spawned via startServerCmd write coverage data to this directory.
	testCoverDir string
)

func init() {
	if dir := os.Getenv("KABOOM_GO_COVERDIR"); dir != "" {
		testCoverDir = dir
	}
}

func buildTestBinary(t *testing.T) string {
	t.Helper()
	testBinaryOnce.Do(func() {
		testBinaryPath = filepath.Join(os.TempDir(), "kaboom-test-binary")
		args := testBinaryBuildArgs(testBinaryPath, testCoverDir != "")
		cmd := exec.Command("go", args...) // #nosec G204,G202 -- fixed test build arguments
		if output, err := cmd.CombinedOutput(); err != nil {
			testBinaryErr = fmt.Errorf("failed to build kaboom: %v\nOutput: %s", err, output)
		}
	})
	if testBinaryErr != nil {
		t.Fatalf("buildTestBinary: %v", testBinaryErr)
	}
	return testBinaryPath
}

func testBinaryBuildArgs(outputPath string, collectSubprocessCoverage bool) []string {
	args := []string{"build", "-cover", "-o", outputPath, "."}
	if collectSubprocessCoverage {
		// Black-box tests exercise cross-package production paths that package tests
		// cannot observe. Instrument the full module once in the shared test binary;
		// isolated per-test state keeps those daemon launches deterministic.
		args = []string{"build", "-cover", "-coverpkg=./...", "-o", outputPath, "."}
	}
	return args
}

func TestTestBinaryBuildArgsCoversBlackBoxDependencies(t *testing.T) {
	ordinary := strings.Join(testBinaryBuildArgs("/tmp/kaboom", false), " ")
	if ordinary != "build -cover -o /tmp/kaboom ." {
		t.Fatalf("ordinary build args = %q", ordinary)
	}
	covered := strings.Join(testBinaryBuildArgs("/tmp/kaboom", true), " ")
	if covered != "build -cover -coverpkg=./... -o /tmp/kaboom ." {
		t.Fatalf("covered build args = %q", covered)
	}
	if !strings.Contains(covered, "./...") {
		t.Fatalf("black-box instrumentation must include production dependencies: %q", covered)
	}
}

func getTestStateDir(t *testing.T) string {
	t.Helper()
	// Honor an externally provided state dir. The reliability soak gate sets
	// KABOOM_STATE_DIR so the server spawned by startServerCmd writes its fast-path
	// telemetry to the same root that `--doctor` later inspects.
	if ext := os.Getenv(statecfg.StateDirEnv); ext != "" {
		return ext
	}

	testStateMu.Lock()
	defer testStateMu.Unlock()
	if stateDir, ok := testStateDirs[t.Name()]; ok {
		return stateDir
	}
	stateDir, err := os.MkdirTemp("", "kaboom-test-state-*")
	if err != nil {
		t.Fatalf("create isolated test state directory: %v", err)
	}
	testStateDirs[t.Name()] = stateDir
	t.Cleanup(func() {
		testStateMu.Lock()
		delete(testStateDirs, t.Name())
		testStateMu.Unlock()
		if err := removeTestStateDir(stateDir); err != nil {
			t.Errorf("remove isolated test state directory: %v", err)
		}
	})
	return stateDir
}

func removeTestStateDir(stateDir string) error {
	var err error
	for attempt := 0; attempt < 6; attempt++ {
		if err = os.RemoveAll(stateDir); err == nil {
			return nil
		}
		runtime.Gosched()
	}
	return err
}

func TestGetTestStateDirIsolatesUnrelatedTests(t *testing.T) {
	if os.Getenv(statecfg.StateDirEnv) != "" {
		t.Skip("external state directory intentionally overrides test isolation")
	}
	paths := make(chan string, 2)
	for _, name := range []string{"first", "second"} {
		name := name
		t.Run(name, func(t *testing.T) {
			first := getTestStateDir(t)
			if second := getTestStateDir(t); second != first {
				t.Fatalf("same test received different state directories: %q != %q", first, second)
			}
			paths <- first
		})
	}
	close(paths)
	first := <-paths
	second := <-paths
	if first == second {
		t.Fatalf("unrelated tests shared state directory %q", first)
	}
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
		coverDir, err := os.MkdirTemp(testCoverDir, "browser-")
		if err != nil {
			t.Fatalf("create subprocess coverage directory: %v", err)
		}
		cmd.Env = append(cmd.Env, "GOCOVERDIR="+coverDir)
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
	pids, err := procctl.FindProcessOnPort(port)
	if err == nil {
		for _, pid := range pids {
			_ = procctl.KillProcessByPID(pid)
		}
	}
	procctl.RemovePIDFile(port)
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
