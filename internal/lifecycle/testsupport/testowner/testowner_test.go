// testowner_test.go — Contracts for the test-daemon orphan watchdog.

package testowner

import (
	"os"
	"sync/atomic"
	"testing"
	"time"
)

func TestWatchIsInertWithoutAnOwner(t *testing.T) {
	t.Setenv(OwnerPIDEnv, "")

	var exited atomic.Bool
	// Tripwire rather than a sleep. Watch returns before starting a goroutine
	// when there is no owner, so exit is structurally unreachable and waiting
	//20ms proved nothing. Failing the moment the liveness probe is called at all
	// catches a future regression that does start one, whenever it fires.
	stop, watching := Watch(
		func(int) bool { t.Error("an unowned daemon must never be polled for liveness"); return false },
		func() { exited.Store(true) },
		time.Millisecond,
	)
	defer stop()

	if watching {
		t.Fatal("Watch must not supervise a daemon that was not started by a test")
	}
	if exited.Load() {
		t.Fatal("Watch terminated a production daemon that has no test owner")
	}
}

func TestWatchIgnoresAnUnparseableOwner(t *testing.T) {
	t.Setenv(OwnerPIDEnv, "not-a-pid")

	var exited atomic.Bool
	stop, watching := Watch(
		func(int) bool { t.Error("an unparseable owner must never be polled for liveness"); return false },
		func() { exited.Store(true) },
		time.Millisecond,
	)
	defer stop()

	if watching {
		t.Fatal("an unparseable owner pid must not start the watchdog")
	}
	if exited.Load() {
		t.Fatal("an unparseable owner pid must not terminate the daemon")
	}
}

// The whole point: a `go test` killed with SIGKILL never runs t.Cleanup, so the
// daemon has to notice on its own that nobody owns it any more.
func TestWatchTerminatesWhenTheOwnerDisappears(t *testing.T) {
	t.Setenv(OwnerPIDEnv, "4242")

	alive := atomic.Bool{}
	alive.Store(true)
	exited := make(chan struct{})

	stop, watching := Watch(
		func(pid int) bool {
			if pid != 4242 {
				t.Errorf("watchdog polled pid %d, want 4242", pid)
			}
			return alive.Load()
		},
		func() { close(exited) },
		time.Millisecond,
	)
	defer stop()

	if !watching {
		t.Fatal("a daemon started by a test must be supervised")
	}
	select {
	case <-exited:
		t.Fatal("watchdog terminated the daemon while its owner was still alive")
	case <-time.After(20 * time.Millisecond):
	}

	alive.Store(false)
	select {
	case <-exited:
	case <-time.After(2 * time.Second):
		t.Fatal("watchdog did not terminate the daemon after its owner disappeared")
	}
}

func TestWatchStopsCleanly(t *testing.T) {
	t.Setenv(OwnerPIDEnv, "4242")

	var exited atomic.Bool
	stop, watching := Watch(func(int) bool { return false }, func() { exited.Store(true) }, time.Hour)
	if !watching {
		t.Fatal("expected the watchdog to start")
	}
	stop()
	stop() // idempotent: a second stop must not panic
}

func TestOwnerPIDEnvIsNotSetInThisProcess(t *testing.T) {
	// Guards against the harness leaking the variable into unrelated runs,
	// which would make every daemon supervise the wrong process.
	if _, ok := os.LookupEnv(OwnerPIDEnv); ok {
		t.Fatalf("%s must only be set on daemons the harness starts", OwnerPIDEnv)
	}
}
