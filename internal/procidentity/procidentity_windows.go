// procidentity_windows.go — Process identity via WMIC on Windows.
// Why: Windows has no `ps`. CreationDate is the start-time analogue of lstart and
// is likewise compared as an opaque string.

//go:build windows

package procidentity

import (
	"strconv"
	"strings"
)

func listProcesses() (string, error) {
	return runProcessLister("wmic", "process", "get", "ProcessId,CreationDate,Name", "/format:csv")
}

// parseProcessLine reads WMIC CSV rows: "<node>,<CreationDate>,<Name>,<ProcessId>".
func parseProcessLine(line string) (int, Info, bool) {
	parts := strings.Split(strings.TrimSpace(line), ",")
	if len(parts) < 4 {
		return 0, Info{}, false
	}
	creation := strings.TrimSpace(parts[len(parts)-3])
	name := strings.TrimSpace(parts[len(parts)-2])
	pid, err := strconv.Atoi(strings.TrimSpace(parts[len(parts)-1]))
	if err != nil || pid <= 0 {
		return 0, Info{}, false
	}
	if creation == "" || name == "" {
		return 0, Info{}, false
	}
	return pid, Info{Start: creation, Command: name}, true
}
