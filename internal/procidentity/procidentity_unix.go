// procidentity_unix.go — Process identity via a single `ps` listing on Unix.
// Why: `lstart` is an absolute wall-clock start time (stable for a process's whole
// life, unlike `etime`, which changes every second), and `comm` distinguishes a
// recycled pid's new owner. One listing covers every pid.

//go:build !windows

package procidentity

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// lstartFieldCount is the number of whitespace-separated fields `ps -o lstart=`
// emits under LC_ALL=C, e.g. "Thu Aug 27 18:58:54 2026".
const lstartFieldCount = 5

func snapshotProcesses() (map[int]Info, error) {
	cmd := exec.Command("ps", "-eo", "pid=,lstart=,comm=") // #nosec G204 -- fixed argv, no caller input
	// Force the C locale so the start-time rendering is stable across machines;
	// the value is only ever compared to itself, never parsed.
	cmd.Env = append(os.Environ(), "LC_ALL=C", "LANG=C")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return parseSnapshot(string(out)), nil
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

// parseProcessLine splits "<pid> <lstart(5 fields)> <comm...>". comm may itself
// contain spaces (a full executable path with a space in it), so it is taken as
// the remainder rather than a fixed field.
func parseProcessLine(line string) (int, Info, bool) {
	fields := strings.Fields(line)
	if len(fields) < lstartFieldCount+2 {
		return 0, Info{}, false
	}
	pid, err := strconv.Atoi(fields[0])
	if err != nil || pid <= 0 {
		return 0, Info{}, false
	}
	start := strings.Join(fields[1:1+lstartFieldCount], " ")
	command := strings.Join(fields[1+lstartFieldCount:], " ")
	if start == "" || command == "" {
		return 0, Info{}, false
	}
	return pid, Info{Start: start, Command: command}, true
}
