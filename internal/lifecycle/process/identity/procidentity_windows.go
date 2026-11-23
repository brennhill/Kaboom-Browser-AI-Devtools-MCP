// procidentity_windows.go — Process identity via the Win32 toolhelp + process-times APIs.
// Why: the previous implementation shelled out to `wmic`, which Microsoft removed
// from current Windows images (windows-2025 CI runners ship without it). Every
// daemon start then failed with "cannot determine this process's identity". The
// stdlib syscall package exposes CreateToolhelp32Snapshot, OpenProcess, and
// GetProcessTimes, so identity needs no external binary and no new dependency.

//go:build windows

package procidentity

import (
	"errors"
	"syscall"
	"unsafe"
)

// processQueryLimitedInformation is PROCESS_QUERY_LIMITED_INFORMATION: the minimum
// right GetProcessTimes needs, and grantable for other users' processes.
const processQueryLimitedInformation = 0x1000

// errNoMoreFiles is ERROR_NO_MORE_FILES, the normal end of a toolhelp walk.
const errNoMoreFiles syscall.Errno = 18

func snapshotProcesses() (map[int]Info, error) {
	handle, err := syscall.CreateToolhelp32Snapshot(syscall.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer syscall.CloseHandle(handle) //nolint:errcheck // read-only snapshot handle

	var entry syscall.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	snap := make(map[int]Info)
	for err = syscall.Process32First(handle, &entry); err == nil; err = syscall.Process32Next(handle, &entry) {
		pid := int(entry.ProcessID)
		creation, ok := processCreationTime(entry.ProcessID)
		if !ok {
			// EXPECTED_ABSENCE: protected/system processes refuse even limited
			// query access, and a process can exit mid-walk. Neither can be one of
			// our records' owners being verified (we can always query our own
			// user's processes), so omitting them is correct, not a lost failure.
			continue
		}
		if info, ok := buildInfo(pid, creation, syscall.UTF16ToString(entry.ExeFile[:])); ok {
			snap[pid] = info
		}
	}
	if !errors.Is(err, errNoMoreFiles) {
		return nil, err
	}
	return snap, nil
}

// processCreationTime returns the process creation FILETIME as 100ns ticks.
func processCreationTime(pid uint32) (int64, bool) {
	h, err := syscall.OpenProcess(processQueryLimitedInformation, false, pid)
	if err != nil {
		return 0, false
	}
	defer syscall.CloseHandle(h) //nolint:errcheck // query-only process handle
	var creation, exit, kernel, user syscall.Filetime
	if err := syscall.GetProcessTimes(h, &creation, &exit, &kernel, &user); err != nil {
		return 0, false
	}
	return int64(creation.HighDateTime)<<32 | int64(creation.LowDateTime), true
}
