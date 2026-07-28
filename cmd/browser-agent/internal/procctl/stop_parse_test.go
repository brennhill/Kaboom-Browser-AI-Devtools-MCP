// stop_parse_test.go — Tests lsof parsing used by the force-cleanup process sweep.

package procctl

import "testing"

// TestLsofListedPIDs_ExcludesSelfPID is the regression test for force-cleanup
// signaling itself: `lsof -c kaboom` prefix-matches the running binary's own
// command name, so its PID must be filtered out before SIGTERM/SIGKILL.
func TestLsofListedPIDs_ExcludesSelfPID(t *testing.T) {
	t.Parallel()

	output := "COMMAND   PID  USER   FD   TYPE DEVICE SIZE/OFF NODE NAME\n" +
		"kaboom    111 brenn  cwd    DIR    1,4      640  555 /\n" +
		"kaboom    222 brenn  cwd    DIR    1,4      640  555 /\n" +
		"kaboom    333 brenn  txt    REG    1,4    12345  556 /usr/local/bin/kaboom\n" +
		"garbage\n" +
		"kaboom    bad brenn  cwd    DIR    1,4      640  555 /\n" +
		"kaboom    -5  brenn  cwd    DIR    1,4      640  555 /\n"

	pids := lsofListedPIDs(output, 222)

	want := []int{111, 333}
	if len(pids) != len(want) {
		t.Fatalf("lsofListedPIDs = %v, want %v", pids, want)
	}
	for i, pid := range want {
		if pids[i] != pid {
			t.Fatalf("lsofListedPIDs = %v, want %v", pids, want)
		}
	}
	for _, pid := range pids {
		if pid == 222 {
			t.Fatal("self PID must be excluded from force-cleanup candidates")
		}
	}
}

func TestLsofListedPIDs_EmptyOutput(t *testing.T) {
	t.Parallel()

	if pids := lsofListedPIDs("", 1); len(pids) != 0 {
		t.Fatalf("lsofListedPIDs(empty) = %v, want none", pids)
	}
}
