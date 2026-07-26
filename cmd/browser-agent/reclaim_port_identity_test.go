// reclaim_port_identity_test.go -- reclaimPort must only ever kill OUR daemon.
//
// Regression: reclaimPort took raw PIDs from `lsof -tiTCP:<port> -sTCP:LISTEN` and
// SIGTERM/SIGKILLed every one of them, with a self-PID check as the ONLY guard.
// Nothing verified the process was a Kaboom daemon, so any unrelated program the
// user happened to be running on 7890/7891 — a dev server, a database, another
// tool — was killed on daemon startup. terminal_supervisor calls this before every
// restart attempt (up to 8), so a foreign listener could be killed repeatedly.

package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// swapProcessCommand installs a fake process-command lookup and returns a restore func.
func swapProcessCommand(f func(int) string) func() {
	old := daemonProcessCommand
	daemonProcessCommand = f
	return func() { daemonProcessCommand = old }
}

func TestReclaimPort_NeverKillsAForeignProcess(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "foreign.log")
	server, err := NewServer(logFile, 200)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	const port = 7891
	var killed []int
	restore := swapReclaimDeps(
		func(int) ([]int, error) { return []int{4242}, nil },
		func(pid int, _ bool) { killed = append(killed, pid) },
		func(int, time.Duration) bool { return true },
		func(int) bool { return true }, // still running: we did not free it
	)
	defer restore()
	restoreCmd := swapProcessCommand(func(int) string {
		return "/opt/homebrew/opt/postgresql@16/bin/postgres -D /opt/homebrew/var/postgresql@16"
	})
	defer restoreCmd()

	if reclaimPort(server, port, "terminal") {
		t.Fatal("a port held by a foreign process must not be reported as freed")
	}
	if len(killed) != 0 {
		t.Fatalf("a foreign process must NEVER be killed, got kills for %v", killed)
	}

	server.logs.Shutdown(2 * time.Second)
	// Declining to reclaim is a real, diagnosable outcome — it must not be silent
	// (rule 25), or "the terminal port is busy" becomes unexplainable.
	events := readLifecycleEventsFromLogFile(t, logFile)
	found := false
	for _, evt := range events {
		if name, _ := evt["event"].(string); name == "port_reclaim_skipped_foreign" {
			found = true
			if pid, _ := evt["owner_pid"].(float64); int(pid) != 4242 {
				t.Errorf("skipped event owner_pid = %v, want 4242", evt["owner_pid"])
			}
		}
	}
	if !found {
		t.Error("declining to kill a foreign process must be logged (port_reclaim_skipped_foreign)")
	}
}

func TestReclaimPort_StillKillsOurOwnDaemon(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "ours.log")
	server, err := NewServer(logFile, 200)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	self, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error = %v", err)
	}

	const port = 7890
	var killed []int
	restore := swapReclaimDeps(
		func(int) ([]int, error) { return []int{4242}, nil },
		func(pid int, _ bool) { killed = append(killed, pid) },
		func(int, time.Duration) bool { return true },
		func(int) bool { return false },
	)
	defer restore()
	// A leftover daemon reports our own executable path — the whole point of the
	// feature. The identity check must not disable the reclaim it exists for.
	restoreCmd := swapProcessCommand(func(int) string { return self + " --daemon --port 7890" })
	defer restoreCmd()

	if !reclaimPort(server, port, "main") {
		t.Fatal("reclaimPort should free a port held by our own leftover daemon")
	}
	if len(killed) != 1 || killed[0] != 4242 {
		t.Fatalf("our own leftover daemon must still be reclaimed, got kills %v", killed)
	}
	server.logs.Shutdown(2 * time.Second)
}

func TestReclaimPort_UnknownCommandIsNotKilled(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "unknown.log")
	server, err := NewServer(logFile, 200)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	var killed []int
	restore := swapReclaimDeps(
		func(int) ([]int, error) { return []int{4242}, nil },
		func(pid int, _ bool) { killed = append(killed, pid) },
		func(int, time.Duration) bool { return true },
		func(int) bool { return true },
	)
	defer restore()
	// `ps` failed / the process vanished. We cannot prove it is ours, so we must
	// not kill it: an unidentifiable process is treated as foreign, never as ours.
	restoreCmd := swapProcessCommand(func(int) string { return "" })
	defer restoreCmd()

	reclaimPort(server, 7891, "terminal")
	if len(killed) != 0 {
		t.Fatalf("an unidentifiable process must not be killed, got kills %v", killed)
	}
	server.logs.Shutdown(2 * time.Second)
}

func TestProcessLooksLikeOurDaemon(t *testing.T) {
	t.Parallel()

	ours := []string{
		"/usr/local/bin/kaboom-agentic-browser --daemon",
		"/Users/x/.kaboom/bin/kaboom-agentic-browser",
		"/tmp/go-build123/b001/browser-agent.test -test.run=X",
		"/opt/homebrew/bin/browser-agent --daemon --port 7890",
	}
	for _, cmdline := range ours {
		if !processLooksLikeOurDaemon(cmdline, "/usr/local/bin/kaboom-agentic-browser") {
			t.Errorf("must recognize our own daemon: %q", cmdline)
		}
	}

	foreign := []string{
		"",
		"/opt/homebrew/opt/postgresql@16/bin/postgres -D /var/pg",
		"node /Users/x/project/server.js",
		"python3 -m http.server 7890",
		"/Applications/Docker.app/Contents/MacOS/com.docker.backend",
		// Merely mentioning the port must not qualify anything.
		"nc -l 7890",
	}
	for _, cmdline := range foreign {
		if processLooksLikeOurDaemon(cmdline, "/usr/local/bin/kaboom-agentic-browser") {
			t.Errorf("must NOT claim a foreign process: %q", cmdline)
		}
	}
}
