// orphan_reaping_test.go — A test daemon must not outlive the process that started it.

package runtimeintegration

import (
	"os"
	"os/exec"
	"strconv"
	"testing"
	"time"

	testprocess "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/integrationtest"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/procctl"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/testowner"
)

// t.Cleanup stops daemons on the normal path, but it never runs when the test
// binary is killed outright — a `go test` timeout, a cancelled CI job, Ctrl-C.
// Twelve daemons were found alive twenty hours after such a run, each still
// holding a port and a state directory. The daemon therefore watches the
// process that owns it and exits on its own once that process is gone.
func TestTestDaemonExitsWhenItsOwnerIsKilled(t *testing.T) {
	if testing.Short() {
		t.Skip("skips orphan reaping integration in short mode")
	}
	binary := testprocess.BuildBinary(t)
	port := testprocess.FreePort(t)

	// A stand-in owner this test can kill without taking itself down.
	owner := exec.Command("/bin/sh", "-c", "sleep 120")
	if err := owner.Start(); err != nil {
		t.Fatalf("start stand-in owner: %v", err)
	}
	ownerPID := owner.Process.Pid
	defer func() {
		_ = owner.Process.Kill()
		_, _ = owner.Process.Wait()
	}()

	stateDirectory, err := os.MkdirTemp("", "kaboom-orphan-state-*")
	if err != nil {
		t.Fatalf("create state directory: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(stateDirectory) })

	daemon := exec.Command(binary, "--daemon", "--port", strconv.Itoa(port), "--state-dir", stateDirectory) // #nosec G204 -- test-owned binary
	daemon.Env = append(os.Environ(), testowner.OwnerPIDEnv+"="+strconv.Itoa(ownerPID))
	if err := daemon.Start(); err != nil {
		t.Fatalf("start daemon: %v", err)
	}
	daemonPID := daemon.Process.Pid
	t.Cleanup(func() {
		_ = procctl.KillProcessByPID(daemonPID)
		_, _ = daemon.Process.Wait()
	})

	// Ticker rather than Sleep: the repo bans wall-clock sleeps in tests because
	// they encode a guess about timing instead of waiting on the condition.
	startupPoll := time.NewTicker(50 * time.Millisecond)
	defer startupPoll.Stop()
	deadline := time.After(testprocess.StartTimeout())
	for !procctl.IsProcessAlive(daemonPID) {
		select {
		case <-deadline:
			t.Fatal("daemon did not come up before the startup timeout")
		case <-startupPoll.C:
		}
	}
	if !procctl.IsProcessAlive(daemonPID) {
		t.Fatal("daemon did not come up")
	}

	// Kill the owner the way an interrupted `go test` dies: no cleanup runs.
	if err := owner.Process.Kill(); err != nil {
		t.Fatalf("kill stand-in owner: %v", err)
	}
	_, _ = owner.Process.Wait()

	exited := make(chan struct{})
	go func() {
		_, _ = daemon.Process.Wait()
		close(exited)
	}()

	// Generous relative to the poll interval so this cannot flake on a slow box.
	select {
	case <-exited:
	case <-time.After(20 * testowner.DefaultInterval):
		t.Fatalf("daemon pid %d outlived its owner pid %d", daemonPID, ownerPID)
	}
}
