package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// swapReclaimDeps installs fakes for reclaimPort's injectable dependencies and
// returns a restore func.
//
// It also stubs daemonProcessCommand so every owning PID reports OUR daemon.
// reclaimPort now refuses to kill a process it cannot identify as ours, so without
// this the fake PIDs below (which do not exist) would be treated as foreign and
// skipped — these tests are about the kill/force-kill/self-skip mechanics, not
// about identity, which reclaim_port_identity_test.go covers directly.
func setReclaimDeps(server *Server, find func(int) ([]int, error), term func(int, bool), wait func(int, time.Duration) bool, running func(int) bool) {
	server.daemonHost.findProcessOnPort = find
	server.daemonHost.terminatePID = term
	server.daemonHost.waitForPortRelease = wait
	server.daemonHost.isServerRunning = running
	server.daemonHost.processCommand = func(int) string { return "/usr/local/bin/kaboom-agentic-browser --daemon" }
}

func TestReclaimPort_KillsOwnersSkipsSelfAndReportsFreed(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "reclaim.log")
	server, err := NewServer(logFile, 200)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	const port = 7891
	self := os.Getpid()

	var graceful, forced []int
	setReclaimDeps(server,
		func(p int) ([]int, error) {
			if p != port {
				return nil, nil
			}
			return []int{self, 4242}, nil // self must be skipped; 4242 killed
		},
		func(pid int, force bool) {
			if force {
				forced = append(forced, pid)
			} else {
				graceful = append(graceful, pid)
			}
		},
		func(int, time.Duration) bool { return true }, // frees gracefully
		func(int) bool { return false },               // -> not running -> freed
	)

	if !reclaimPort(server, port, "terminal") {
		t.Fatal("reclaimPort should report the port freed")
	}
	if len(graceful) != 1 || graceful[0] != 4242 {
		t.Fatalf("graceful terminate = %v, want [4242] (self %d must be skipped)", graceful, self)
	}
	if len(forced) != 0 {
		t.Fatalf("no force-kill expected when the port frees gracefully, got %v", forced)
	}

	server.logs.Shutdown(2 * time.Second)
	events := readLifecycleEventsFromLogFile(t, logFile)
	var reclaimed map[string]any
	for _, evt := range events {
		if name, _ := evt["event"].(string); name == "port_reclaimed" {
			reclaimed = evt
		}
	}
	if reclaimed == nil {
		t.Fatal("expected a port_reclaimed lifecycle event")
	}
	if purpose, _ := reclaimed["purpose"].(string); purpose != "terminal" {
		t.Fatalf("port_reclaimed purpose = %q, want terminal", purpose)
	}
	if freed, _ := reclaimed["freed"].(bool); !freed {
		t.Fatalf("port_reclaimed freed = %v, want true", reclaimed["freed"])
	}
}

func TestReclaimPort_ForceKillsWhenGracefulFails(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "reclaim.log")
	server, err := NewServer(logFile, 200)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	const port = 7891
	var graceful, forced []int
	setReclaimDeps(server,
		func(int) ([]int, error) { return []int{4242}, nil },
		func(pid int, force bool) {
			if force {
				forced = append(forced, pid)
			} else {
				graceful = append(graceful, pid)
			}
		},
		func(int, time.Duration) bool { return false }, // never frees
		func(int) bool { return true },                 // still running -> not freed
	)

	if reclaimPort(server, port, "main") {
		t.Fatal("reclaimPort should report NOT freed when the port stays stuck")
	}
	if len(graceful) != 1 || graceful[0] != 4242 {
		t.Fatalf("graceful terminate = %v, want [4242]", graceful)
	}
	if len(forced) != 1 || forced[0] != 4242 {
		t.Fatalf("force kill = %v, want [4242] after the graceful wait fails", forced)
	}
	server.logs.Shutdown(2 * time.Second)
}

func TestReclaimPort_NoOwnersIsNoOp(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "reclaim.log")
	server, err := NewServer(logFile, 200)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	var terminated int
	setReclaimDeps(server,
		func(int) ([]int, error) { return nil, nil }, // nothing on the port
		func(int, bool) { terminated++ },
		func(int, time.Duration) bool { return true },
		func(int) bool { return false }, // port already free
	)

	if !reclaimPort(server, 7890, "main") {
		t.Fatal("an empty port should report freed")
	}
	if terminated != 0 {
		t.Fatalf("nothing should be killed when no process owns the port, got %d kills", terminated)
	}
	server.logs.Shutdown(2 * time.Second)
}
