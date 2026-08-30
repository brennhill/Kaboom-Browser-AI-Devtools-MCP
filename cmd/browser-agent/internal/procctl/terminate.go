// terminate.go — The one way this codebase ends another process.
// Why: SIGTERM-then-escalate was implemented separately in daemonrecovery and
// needed again by the reaper. Two implementations of "stop that process" drift,
// and the one that drifts is the one that leaves a port held.

package procctl

import (
	"fmt"
	"os"
	"runtime"
	"syscall"
	"time"
)

// TerminateGracePeriod is how long a process is given to exit after SIGTERM
// before SIGKILL. Short, because every caller is already reclaiming a resource
// the process has stopped using correctly.
const TerminateGracePeriod = 500 * time.Millisecond

// TerminatePID ends a process, escalating from SIGTERM to SIGKILL. On Windows,
// which has no SIGTERM, it kills directly.
//
// It returns an error when the process could not be ended, rather than reporting
// success it cannot verify: a kill that silently failed leaves the port held while
// the caller reports the resource reclaimed.
func TerminatePID(pid int, force bool) error {
	if pid <= 0 {
		return fmt.Errorf("procctl: refusing to signal invalid pid %d", pid)
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("procctl: find process %d: %w", pid, err)
	}
	if force || runtime.GOOS == "windows" {
		if err := process.Kill(); err != nil {
			return fmt.Errorf("procctl: kill %d: %w", pid, err)
		}
		return nil
	}
	if err := process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("procctl: SIGTERM %d: %w", pid, err)
	}
	time.Sleep(TerminateGracePeriod)
	if !IsProcessAlive(pid) {
		return nil
	}
	if err := process.Kill(); err != nil {
		return fmt.Errorf("procctl: SIGKILL %d after it ignored SIGTERM: %w", pid, err)
	}
	return nil
}
