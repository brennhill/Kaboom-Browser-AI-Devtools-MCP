// harness.go — Owns deterministic binary, process, state, and cleanup fixtures for browser-agent integration suites.

package integrationtest

import (
	"context"
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
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/lifecycle/instance/registry"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/lifecycle/procctl"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/lifecycle/testsupport/testowner"
	statecfg "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/platform/state/paths"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/terminal/adapters"
)

var binaryCache struct {
	once sync.Once
	path string
	err  error
}

func sourceDirectory() string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return ""
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}

func buildArgs(outputPath string, collectSubprocessCoverage bool) []string {
	if collectSubprocessCoverage {
		return []string{"build", "-cover", "-coverpkg=./...", "-o", outputPath, "."}
	}
	return []string{"build", "-cover", "-o", outputPath, "."}
}

func configuredBinary() string { return os.Getenv("KABOOM_INTEGRATION_BINARY") }

func usesCoverage(mode, coverageDirectory, subprocessCoverageDirectory string) bool {
	return mode != "" || coverageDirectory != "" || subprocessCoverageDirectory != ""
}

func instrumented() bool {
	return raceEnabled || os.Getenv("KABOOM_INTEGRATION_INSTRUMENTED") == "1" ||
		usesCoverage(testing.CoverMode(), os.Getenv("GOCOVERDIR"), os.Getenv("KABOOM_GO_COVERDIR"))
}

// Instrumented reports whether race or coverage instrumentation is active.
func Instrumented() bool { return instrumented() }

func startTimeout(isInstrumented bool) time.Duration {
	if isInstrumented {
		return 30 * time.Second
	}
	return 5 * time.Second
}

// StartTimeout is the maximum readiness poll budget for a spawned daemon.
func StartTimeout() time.Duration { return startTimeout(instrumented()) }

// ResponseTimeout expands an ordinary response budget under instrumentation.
func ResponseTimeout(ordinary time.Duration) time.Duration {
	if instrumented() {
		return 30 * time.Second
	}
	return ordinary
}

// ReservedPortBase and ReservedPortEnd bound the range integration daemons
// prefer.
//
// A daemon that outlives its test used to land on an OS-assigned port, so it
// could not be found or killed by number — the hunt for twelve stale daemons
// had to match on process name instead. A known band makes a leak diagnosable.
//
// The band deliberately starts above the user's daemon (7890), its terminal
// server (7891) and the CLI integration suite's pinned 7899.
const (
	ReservedPortBase = 7900
	ReservedPortEnd  = 7998
)

// portStride is how many consecutive ports one daemon occupies: its HTTP port
// and its terminal server at port+terminal.PortOffset.
const portStride = terminal.PortOffset + 1

// freePortInBand returns a stride-aligned port in [base, end] whose whole
// daemon footprint (HTTP port and terminal port) is bindable, or 0 if none is.
//
// Safety comes from binding, not from bookkeeping: a port is only returned
// after this process has successfully bound it and its terminal neighbour.
// Alignment matters because a daemon whose terminal port is busy reclaims it by
// killing the holder: unaligned picks let one package's daemon land on another
// package's terminal port and the two kill each other ("server did not start").
// The starting slot is derived from the pid so parallel test processes begin
// at different points and do not all contend for the bottom of the band.
func freePortInBand(base, end int) int {
	slots := (end - base + 1) / portStride
	if slots <= 0 {
		return 0
	}
	offset := os.Getpid() % slots
	for i := 0; i < slots; i++ {
		port := base + ((offset+i)%slots)*portStride
		if footprintFree(port) {
			return port
		}
	}
	return 0
}

// footprintFree reports whether every port a daemon on port would bind is free.
func footprintFree(port int) bool {
	for p := port; p < port+portStride; p++ {
		listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
		if err != nil {
			return false
		}
		if closeErr := listener.Close(); closeErr != nil {
			return false
		}
	}
	return true
}

// FreePort returns a currently unbound loopback TCP port, preferring the
// reserved band so a leaked daemon can be found by number.
func FreePort(t *testing.T) int {
	t.Helper()
	if port := freePortInBand(ReservedPortBase, ReservedPortEnd); port != 0 {
		return port
	}
	// Band exhausted — a heavily parallel run, or leftovers holding it. Fall
	// back to an ephemeral port rather than failing tests over port supply,
	// still requiring the terminal neighbour to be free.
	for attempt := 0; attempt < 32; attempt++ {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("find free port: %v", err)
		}
		port := listener.Addr().(*net.TCPAddr).Port
		if err := listener.Close(); err != nil {
			t.Fatalf("release free-port probe: %v", err)
		}
		if port+portStride <= 65536 && footprintFree(port) {
			return port
		}
	}
	t.Fatal("find free port: no ephemeral port with a free terminal neighbour after 32 attempts")
	return 0
}

// BuildBinary builds the browser-agent once per integration test process.
func BuildBinary(t *testing.T) string {
	t.Helper()
	if configured := configuredBinary(); configured != "" {
		return configured
	}
	binaryCache.once.Do(func() {
		directory, err := os.MkdirTemp("", "kaboom-integration-binary-*")
		if err != nil {
			binaryCache.err = err
			return
		}
		binaryCache.path = filepath.Join(directory, "kaboom-test-binary")
		command := exec.Command("go", buildArgs(binaryCache.path, os.Getenv("KABOOM_GO_COVERDIR") != "")...) // #nosec G204 -- fixed test build arguments
		command.Dir = sourceDirectory()
		if output, err := command.CombinedOutput(); err != nil {
			binaryCache.err = fmt.Errorf("build browser-agent: %w; output: %s", err, output)
		}
	})
	if binaryCache.err != nil {
		t.Fatalf("BuildBinary: %v", binaryCache.err)
	}
	return binaryCache.path
}

// isolatedRegistry returns the instance-registry directory a test daemon uses.
// An explicit KABOOM_REGISTRY_DIR (a test that shares one registry on purpose)
// wins; otherwise each daemon gets a private one under its state directory.
// Inheriting the real ~/.kaboom/registry put every integration daemon under the
// machine-wide admission cap alongside parallel packages and the developer's
// own daemon, and the cap evicted them mid-test.
func isolatedRegistry(t *testing.T, stateDirectory string) string {
	t.Helper()
	if explicit := strings.TrimSpace(os.Getenv(instancereg.DirEnv)); explicit != "" {
		return explicit
	}
	directory, err := os.MkdirTemp(stateDirectory, "registry-")
	if err != nil {
		t.Fatalf("create isolated instance registry: %v", err)
	}
	return directory
}

// StartServer returns a configured command and registers bounded daemon cleanup.
func StartServer(t *testing.T, binary string, args ...string) *exec.Cmd {
	t.Helper()
	stateDirectory := os.Getenv(statecfg.StateDirEnv)
	if stateDirectory == "" {
		var err error
		stateDirectory, err = os.MkdirTemp("", "kaboom-integration-state-*")
		if err != nil {
			t.Fatalf("create integration state directory: %v", err)
		}
		t.Cleanup(func() {
			if err := removeStateDirectory(stateDirectory); err != nil {
				t.Errorf("remove integration state directory: %v", err)
			}
		})
	}
	registry := isolatedRegistry(t, stateDirectory)
	port := parsePort(args)
	if port > 0 {
		t.Cleanup(func() { stopServer(t, binary, port, stateDirectory, registry) })
	}
	command := exec.Command(binary, args...) // #nosec G204 -- test-owned binary and arguments
	// The daemon exits on its own once this test process is gone. t.Cleanup
	// above covers the normal path, but it never runs when the test binary is
	// killed (go test timeout, CI cancellation, Ctrl-C), which is how strays
	// accumulate and hold ports for hours.
	command.Env = append(os.Environ(),
		statecfg.StateDirEnv+"="+stateDirectory,
		testowner.OwnerPIDEnv+"="+strconv.Itoa(os.Getpid()),
		instancereg.DirEnv+"="+registry,
	)
	if root := os.Getenv("KABOOM_GO_COVERDIR"); root != "" {
		coverageDirectory, err := os.MkdirTemp(root, "browser-")
		if err != nil {
			t.Fatalf("create subprocess coverage directory: %v", err)
		}
		command.Env = append(command.Env, "GOCOVERDIR="+coverageDirectory)
	}
	return command
}

func removeStateDirectory(directory string) error {
	deadline := time.Now().Add(2 * time.Second)
	var lastError error
	for time.Now().Before(deadline) {
		if lastError = os.RemoveAll(directory); lastError == nil {
			return nil
		}
		timer := time.NewTimer(25 * time.Millisecond)
		<-timer.C
	}
	return lastError
}

func parsePort(args []string) int {
	for index, argument := range args {
		value := ""
		if argument == "--port" && index+1 < len(args) {
			value = args[index+1]
		} else if strings.HasPrefix(argument, "--port=") {
			value = strings.TrimPrefix(argument, "--port=")
		}
		if value != "" {
			port, err := strconv.Atoi(value)
			if err == nil && port > 0 {
				return port
			}
			return 0
		}
	}
	return 0
}

func stopServer(t *testing.T, binary string, port int, stateDirectory, registry string) {
	t.Helper()
	// Record the owners before asking the daemon to stop: once it releases the
	// port it can still be flushing logs into the state directory, and
	// t.TempDir removal fails ("directory not empty") if it outlives cleanup.
	owners, err := procctl.FindProcessOnPort(port)
	if err != nil {
		t.Logf("test daemon owner lookup on port %d: %v", port, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binary, "--stop", "--port", strconv.Itoa(port)) // #nosec G204 -- test-owned binary
	command.Env = append(os.Environ(), statecfg.StateDirEnv+"="+stateDirectory, instancereg.DirEnv+"="+registry)
	command.Stdout, command.Stderr = io.Discard, io.Discard
	if err := command.Run(); err != nil && ctx.Err() == nil {
		t.Logf("graceful test daemon cleanup on port %d: %v", port, err)
	}
	if remaining, err := procctl.FindProcessOnPort(port); err != nil {
		t.Logf("test daemon owner lookup on port %d: %v", port, err)
	} else {
		owners = append(owners, remaining...)
	}
	for _, pid := range owners {
		if !procctl.IsProcessAlive(pid) {
			continue
		}
		if err := procctl.KillProcessByPID(pid); err != nil {
			t.Logf("force test daemon cleanup pid %d: %v", pid, err)
		}
	}
	waitForExit(t, owners, 3*time.Second)
	procctl.RemovePIDFile(port)
}

// waitForExit polls until every pid has exited or the deadline passes, so a
// daemon cannot keep writing into a test's state directory after cleanup.
func waitForExit(t *testing.T, pids []int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for _, pid := range pids {
		for procctl.IsProcessAlive(pid) {
			if time.Now().After(deadline) {
				t.Errorf("test daemon pid %d still alive %s after cleanup", pid, timeout)
				return
			}
			timer := time.NewTimer(20 * time.Millisecond)
			<-timer.C
		}
	}
}
