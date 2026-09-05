// instance_cap_test.go — The end-to-end proof of the machine-wide cap: real
// processes, real ports, real kernel locks. Everything else in this feature is a
// unit test of one decision; this is the test that would have failed before the
// feature existed.
// Docs: docs/core/reliability/zombie-prevention.md

package integrationtest_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/integrationtest"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/instancereg"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/procidentity"
	statecfg "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/testsync"
)

// launchDaemons starts count daemons AT THE SAME TIME, each with its own state
// directory but sharing one machine registry — the exact shape that used to
// produce `count` daemons and 2*count held ports.
func launchDaemons(t *testing.T, binary, registryDir string, count int) []*exec.Cmd {
	t.Helper()
	commands := make([]*exec.Cmd, 0, count)
	var launch sync.WaitGroup
	var mu sync.Mutex
	start := make(chan struct{})

	for index := 0; index < count; index++ {
		launch.Add(1)
		go func(index int) {
			defer launch.Done()
			stateDir := filepath.Join(t.TempDir(), "state-"+strconv.Itoa(index))
			port := integrationtest.FreePort(t)
			command := exec.Command(binary, // #nosec G204 -- test-owned binary
				"--daemon", "--port", strconv.Itoa(port), "--state-dir", stateDir)
			command.Env = append(os.Environ(),
				instancereg.DirEnv+"="+registryDir,
				statecfg.StateDirEnv+"="+stateDir,
			)
			<-start // release all launches together
			if err := command.Start(); err != nil {
				t.Errorf("start daemon %d: %v", index, err)
				return
			}
			mu.Lock()
			commands = append(commands, command)
			mu.Unlock()
		}(index)
	}
	close(start)
	launch.Wait()
	return commands
}

// The headline guarantee. Eight daemons launch simultaneously into one machine;
// exactly one may hold the singleton lock.
func TestOnlyOneProductionDaemonSurvivesAConcurrentLaunch(t *testing.T) {
	binary := integrationtest.BuildBinary(t)
	registryDir := t.TempDir()

	const launched = 8
	commands := launchDaemons(t, binary, registryDir, launched)
	t.Cleanup(func() {
		for _, command := range commands {
			if command.Process != nil {
				_ = command.Process.Kill()
				_, _ = command.Process.Wait()
			}
		}
	})

	// Let the losers defer and exit; the winner binds.
	var daemons []instancereg.Record
	testsync.Eventually(t, integrationtest.StartTimeout()+10*time.Second,
		"exactly one production daemon to hold the machine cap", func() bool {
			daemons = onlyDaemons(readRegistry(t, registryDir))
			if len(daemons) > 1 {
				t.Fatalf("%d production daemons registered at once; the machine cap is 1:\n%+v",
					len(daemons), daemons)
			}
			return len(daemons) == 1
		})
	if len(daemons) != 1 {
		t.Fatalf("after launching %d daemons, %d are registered; want exactly 1", launched, len(daemons))
	}
	// Guard against a vacuous pass: the survivor must actually be running and
	// holding the port it registered.
	survivor := daemons[0]
	if !processAlive(survivor.PID, survivor.Identity) {
		t.Fatalf("the surviving daemon (pid %d) is not running", survivor.PID)
	}
	if len(survivor.Ports) == 0 {
		t.Fatalf("the surviving daemon registered no ports: %+v", survivor)
	}
	t.Logf("survivor: pid=%d ports=%v version=%s state_dir=%s",
		survivor.PID, survivor.Ports, survivor.Version, survivor.StateDir)

	// Every loser must have exited CLEANLY (exit 0) rather than crashing or
	// lingering: a deferring daemon is a normal outcome, not a failure.
	exited := 0
	for _, command := range commands {
		if command.Process == nil {
			continue
		}
		done := make(chan error, 1)
		go func(c *exec.Cmd) { done <- c.Wait() }(command)
		select {
		case err := <-done:
			exited++
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() != 0 {
					t.Errorf("a deferring daemon exited %d, want 0", exitErr.ExitCode())
				}
			}
		case <-time.After(2 * time.Second):
			// This one is the survivor still serving.
		}
	}
	if exited < launched-1 {
		t.Errorf("%d of %d daemons exited; want at least %d to have deferred",
			exited, launched, launched-1)
	}
}

// A deferring daemon must not leave a registry entry behind, or the census would
// report daemons that are not running and the cap would exhaust itself.
func TestDeferringDaemonsLeaveNoRegistryEntries(t *testing.T) {
	binary := integrationtest.BuildBinary(t)
	registryDir := t.TempDir()

	commands := launchDaemons(t, binary, registryDir, 4)
	t.Cleanup(func() {
		for _, command := range commands {
			if command.Process != nil {
				_ = command.Process.Kill()
				_, _ = command.Process.Wait()
			}
		}
	})

	// Synchronise on the losers actually exiting rather than guessing how long that
	// takes. Their exit is the event that must leave the registry clean, so waiting for
	// it is both faster and the thing the test is actually about.
	awaitDeferrals(t, commands, len(commands)-1, integrationtest.StartTimeout()+10*time.Second)

	records := readRegistry(t, registryDir)
	for _, record := range onlyDaemons(records) {
		if !processAlive(record.PID, record.Identity) {
			t.Errorf("registry lists pid %d, which is not running", record.PID)
		}
	}
}

// awaitDeferrals blocks until at least `want` of the launched daemons have exited, or
// the timeout elapses. Returns how many exited, so a caller can assert on the count.
func awaitDeferrals(t *testing.T, commands []*exec.Cmd, want int, timeout time.Duration) int {
	t.Helper()
	exits := make(chan struct{}, len(commands))
	for _, command := range commands {
		if command.Process == nil {
			continue
		}
		go func(c *exec.Cmd) {
			_, _ = c.Process.Wait()
			exits <- struct{}{}
		}(command)
	}
	deadline := time.After(timeout)
	exited := 0
	for exited < want {
		select {
		case <-exits:
			exited++
		case <-deadline:
			return exited
		}
	}
	return exited
}

func readRegistry(t *testing.T, dir string) []instancereg.Record {
	t.Helper()
	t.Setenv(instancereg.DirEnv, dir)
	records, err := instancereg.List()
	if err != nil {
		t.Fatalf("read registry: %v", err)
	}
	return records
}

func onlyDaemons(records []instancereg.Record) []instancereg.Record {
	var daemons []instancereg.Record
	for _, record := range records {
		if record.Role == instancereg.RoleDaemon && !record.Parallel {
			daemons = append(daemons, record)
		}
	}
	return daemons
}

// processAlive is identity-checked, not pid-checked: a recycled pid must not make
// a dead daemon look alive, which is the defect procidentity exists to close.
func processAlive(pid int, recorded procidentity.Info) bool {
	return procidentity.IsAlive(pid, recorded)
}
