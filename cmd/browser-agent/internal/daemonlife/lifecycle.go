// lifecycle.go — Signal naming and launch options for the daemon lifecycle.
// Why: singleton enforcement moved to internal/instancegov, where a kernel-held
// flock provides mutual exclusion the OS releases even on SIGKILL. The PID-based
// lock record, its liveness classification, and its takeover policy were deleted
// with it rather than left standing beside the new authority — two systems
// answering "may this daemon bind" is how they drift, and the PID-only liveness
// here would still have signalled a process whose pid had been recycled.

package daemonlife

import (
	"os"
	"syscall"
	"time"

)

// SignalSource describes a process signal for correlated shutdown diagnostics.
func SignalSource(signal os.Signal) string {
	switch signal {
	case os.Interrupt:
		return "Ctrl+C (SIGINT)"
	case syscall.SIGTERM:
		return "SIGTERM (likely --stop or kill)"
	case syscall.SIGHUP:
		return "SIGHUP (terminal closed)"
	default:
		return signal.String()
	}
}

// LaunchOptions describes how this daemon instance was launched.
type LaunchOptions struct {
	Parallel bool
}

// This package's OWN seams (see deps.go for the rule): daemonlife can satisfy
// these itself, so they stay package-local and are swapped by its own tests.
var (
	daemonNow   = time.Now
	daemonSleep = time.Sleep
	// daemonInstallEpoch reports THIS daemon's install epoch (the takeover
	// tiebreaker at equal versions). Injectable for tests.
	daemonInstallEpoch = resolveInstallEpoch
)

