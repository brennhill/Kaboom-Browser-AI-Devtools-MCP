// procidentity_info.go — Platform-neutral construction of an Info from raw OS facts.
// Why: the Windows path reads numeric creation ticks from the Win32 API; keeping the
// validation and formatting here lets it be unit-tested on every OS, not only on a
// Windows runner.

package procidentity

import "strconv"

// buildInfo turns a pid, a creation timestamp (opaque ticks, never parsed back),
// and an executable name into an Info, rejecting anything that could not be
// compared reliably later.
func buildInfo(pid int, creationTicks int64, command string) (Info, bool) {
	if pid <= 0 || creationTicks <= 0 || command == "" {
		return Info{}, false
	}
	return Info{Start: strconv.FormatInt(creationTicks, 10), Command: command}, true
}
