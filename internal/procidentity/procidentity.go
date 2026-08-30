// procidentity.go — Identity-checked process liveness (pid + start time + command).
// Why: kill(pid, 0) answers "does SOME process hold this pid", not "is MY process
// still running". PIDs are recycled, so the two answers diverge. On this machine
// daemon locks written 8 Aug claimed pids 4240/4411/4642, which the OS had since
// reassigned to TextInputSwitcher and two Adobe Creative Cloud helpers: a pid-only
// check reported all three alive, permanently poisoning those state dirs and
// leaving an unrelated user process one branch away from being SIGTERMed.
// Docs: docs/core/reliability/zombie-prevention.md

package procidentity

import (
	"os"
	"os/exec"
	"strings"
)

// Info is the identity of a running process beyond its pid. Start is whatever the
// platform reports as the process start time, compared as an OPAQUE STRING: we
// never parse it, so no locale, timezone, or clock-format assumption can make two
// identical processes look different.
type Info struct {
	Start   string
	Command string
}

// Matches reports whether an observed process is the same process a record was
// written for. An empty identity on either side never matches — a record with no
// recorded identity cannot be proven live, and treating it as live is exactly the
// recycled-pid hole this package closes.
func Matches(recorded, observed Info) bool {
	if recorded.Start == "" || recorded.Command == "" {
		return false
	}
	if observed.Start == "" || observed.Command == "" {
		return false
	}
	return recorded.Start == observed.Start && recorded.Command == observed.Command
}

// Snapshot returns the identity of every visible process in ONE process listing,
// so pruning N registry records costs one lookup instead of N.
func Snapshot() (map[int]Info, error) {
	output, err := listProcesses()
	if err != nil {
		return nil, err
	}
	return parseSnapshot(output), nil
}

// Lookup returns the identity of a single pid. It is a Snapshot filter rather than
// a per-pid query so both paths share one parser and one platform contract.
func Lookup(pid int) (Info, bool) {
	snap, err := Snapshot()
	if err != nil {
		return Info{}, false
	}
	info, ok := snap[pid]
	return info, ok
}

// Self returns this process's identity, for stamping into a record at registration.
func Self() (Info, bool) {
	return Lookup(os.Getpid())
}

// IsAlive reports whether pid is running AS the process described by recorded.
// A live pid with a different identity is a recycled pid and is NOT alive.
func IsAlive(pid int, recorded Info) bool {
	if pid <= 0 {
		return false
	}
	observed, ok := Lookup(pid)
	if !ok {
		return false
	}
	return Matches(recorded, observed)
}

// AliveIn is IsAlive against an already-taken Snapshot, for bulk pruning.
func AliveIn(snap map[int]Info, pid int, recorded Info) bool {
	if pid <= 0 || snap == nil {
		return false
	}
	observed, ok := snap[pid]
	if !ok {
		return false
	}
	return Matches(recorded, observed)
}

func runProcessLister(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...) // #nosec G204 -- fixed argv, no caller input
	// Force the C locale so the platform's start-time rendering is stable across
	// machines; the value is only ever compared to itself, never parsed.
	cmd.Env = append(os.Environ(), "LC_ALL=C", "LANG=C")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func parseSnapshot(output string) map[int]Info {
	snap := make(map[int]Info)
	for _, line := range strings.Split(output, "\n") {
		pid, info, ok := parseProcessLine(line)
		if !ok {
			continue
		}
		snap[pid] = info
	}
	return snap
}
