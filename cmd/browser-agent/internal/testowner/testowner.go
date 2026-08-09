// testowner.go — Terminates a test-started daemon when its owning test exits.
//
// The integration harness registers t.Cleanup to stop each daemon it starts,
// but t.Cleanup never runs when the test process itself is killed — a `go test`
// timeout, a CI cancellation, or Ctrl-C. The daemon is then orphaned and keeps
// its port and state directory indefinitely; twelve such strays were found
// alive after twenty hours.
//
// Cleanup that depends on the supervisor surviving cannot fix that, so the
// daemon supervises its owner instead: when the owning pid disappears, it exits.
package testowner

import (
	"os"
	"strconv"
	"sync"
	"time"
)

// OwnerPIDEnv names the process whose death should terminate this daemon.
// Only the test harness sets it; production daemons run unsupervised.
const OwnerPIDEnv = "KABOOM_TEST_OWNER_PID"

// DefaultInterval bounds how long an orphan can outlive its owner.
const DefaultInterval = 2 * time.Second

// Watch supervises the owning process named by OwnerPIDEnv and calls exit once
// it disappears. It reports whether supervision started: an unset or
// unparseable owner leaves the daemon untouched, so this is inert in
// production. The returned stop function is idempotent.
func Watch(alive func(int) bool, exit func(), interval time.Duration) (func(), bool) {
	noop := func() {}

	raw, present := os.LookupEnv(OwnerPIDEnv)
	if !present || raw == "" {
		return noop, false
	}
	owner, err := strconv.Atoi(raw)
	if err != nil || owner <= 0 {
		return noop, false
	}
	if interval <= 0 {
		interval = DefaultInterval
	}

	done := make(chan struct{})
	var once sync.Once
	stop := func() { once.Do(func() { close(done) }) }

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				if !alive(owner) {
					exit()
					return
				}
			}
		}
	}()

	return stop, true
}
