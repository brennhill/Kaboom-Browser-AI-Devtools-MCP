// procidentity_unix.go — Process identity via a single `ps` listing on Unix.
// Why: `lstart` is an absolute wall-clock start time (stable for a process's whole
// life, unlike `etime`, which changes every second), and `comm` distinguishes a
// recycled pid's new owner. One listing covers every pid.

//go:build !windows

package procidentity

import (
	"strconv"
	"strings"
)

// lstartFieldCount is the number of whitespace-separated fields `ps -o lstart=`
// emits under LC_ALL=C, e.g. "Thu Aug 27 18:58:54 2026".
const lstartFieldCount = 5

func listProcesses() (string, error) {
	return runProcessLister("ps", "-eo", "pid=,lstart=,comm=")
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
