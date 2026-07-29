// bridge_detach_stdio_test.go -- Regression test for the bridge->daemon SIGPIPE crash-loop.
// A bridge-spawned daemon must be genuinely persistent: its standard streams must be
// detached from the bridge, not routed through bridge-owned OS pipes.
package bridge

import (
	"io"
	"os"
	"testing"
)

// TestBuildDaemonCmdDetachesStdio guards against the crash-loop where a
// bridge-spawned daemon died ~4s after the bridge's stdin_eof.
//
// Mechanism it prevents: os/exec routes Stdout/Stderr through an OS pipe whenever
// the assigned writer is NOT an *os.File (io.Discard is such a writer). The pipe's
// read-end lives in the BRIDGE process. Setsid detaches the daemon's session/process
// group but NOT the inherited pipe fds. When the bridge exits on stdin_eof, the
// pipe read-ends close; the daemon's next stderr write hits a broken pipe on fd 2,
// and Go's default disposition terminates the process with SIGPIPE.
//
// A genuinely detached daemon must have nil (=> os/exec connects fd to /dev/null,
// which never breaks) or an *os.File for both streams.
func TestBuildDaemonCmdDetachesStdio(t *testing.T) {
	// NB: do NOT call initTestDeps here — it replaces the shared test runner
	// installed by TestMain, which would clobber its transport hooks for
	// every test that runs after this one (the FastPath tests break with the stub's
	// 2024-11-05 protocol / broken Content-Length writer). TestMain already provides
	// DaemonProcessArgv0, which is all buildDaemonCmd needs.
	s := &daemonState{runner: testRunner, port: 7890}
	cmd, err := s.buildDaemonCmd()
	if err != nil {
		t.Fatalf("buildDaemonCmd: %v", err)
	}

	assertDetachedStream(t, "stdout", cmd.Stdout)
	assertDetachedStream(t, "stderr", cmd.Stderr)

	// stdin must likewise never be tied to the bridge.
	if cmd.Stdin != nil {
		if _, ok := cmd.Stdin.(*os.File); !ok {
			t.Errorf("stdin = %T: must be nil (=> /dev/null) or *os.File so the daemon is detached", cmd.Stdin)
		}
	}
}

// assertDetachedStream fails if w would make os/exec create a bridge-owned pipe.
func assertDetachedStream(t *testing.T, name string, w io.Writer) {
	t.Helper()
	if w == nil {
		return // nil => os/exec connects the fd to /dev/null: fully detached.
	}
	if w == io.Discard {
		t.Fatalf("%s = io.Discard: os/exec routes it through a bridge-owned pipe, so the "+
			"spawned daemon dies with SIGPIPE on fd 2 when the bridge exits. Use nil (=> /dev/null) or an *os.File.", name)
	}
	if _, ok := w.(*os.File); !ok {
		t.Fatalf("%s = %T: any non-*os.File writer makes os/exec create a bridge-owned pipe; "+
			"use nil (=> /dev/null) or an *os.File so the daemon survives the bridge's exit.", name, w)
	}
}
