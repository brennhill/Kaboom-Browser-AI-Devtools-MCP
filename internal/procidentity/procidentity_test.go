// procidentity_test.go — Proves a PID alone never establishes identity, so a
// recycled PID cannot resurrect a dead daemon or get an unrelated process killed.

package procidentity_test

import (
	"os"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/procidentity"
)

func TestSnapshotIncludesThisProcess(t *testing.T) {
	snap, err := procidentity.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	self, ok := snap[os.Getpid()]
	if !ok {
		t.Fatalf("Snapshot() is missing this process (pid %d)", os.Getpid())
	}
	if self.Start == "" {
		t.Error("Snapshot() returned an empty Start for this process")
	}
	if self.Command == "" {
		t.Error("Snapshot() returned an empty Command for this process")
	}
}

func TestSnapshotOmitsImpossiblePID(t *testing.T) {
	snap, err := procidentity.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if _, ok := snap[-1]; ok {
		t.Error("Snapshot() reported a process for pid -1")
	}
}

// The bug this package exists to prevent: on this machine, daemon lock records
// written on 8 Aug claimed pids 4240/4411/4642, which the OS had since reassigned
// to TextInputSwitcher and two Adobe Creative Cloud helpers. A kill(pid,0) check
// called all three alive, which both kept dead daemons "running" forever and put
// an unrelated user process one code path away from being SIGTERMed.
func TestMatchesRejectsRecycledPID(t *testing.T) {
	self := procidentity.Info{Start: "Thu Aug 27 18:58:54 2026", Command: "kaboom-agentic-browser"}
	recycled := procidentity.Info{Start: "Fri Aug 28 09:14:02 2026", Command: "TextInputSwitcher"}

	if !procidentity.MatchesForTest(self, self) {
		t.Error("Matches() rejected an identical identity")
	}
	if procidentity.MatchesForTest(self, recycled) {
		t.Error("Matches() accepted a recycled pid whose start time and command both differ")
	}
	if procidentity.MatchesForTest(self, procidentity.Info{Start: self.Start, Command: "TextInputSwitcher"}) {
		t.Error("Matches() accepted a differing command")
	}
	if procidentity.MatchesForTest(self, procidentity.Info{Start: "Fri Aug 28 09:14:02 2026", Command: self.Command}) {
		t.Error("Matches() accepted a differing start time")
	}
}

// A record written before identity tracking existed has no identity to compare.
// It must not be treated as a match, or the recycled-pid hole stays open.
func TestMatchesRejectsEmptyIdentity(t *testing.T) {
	live := procidentity.Info{Start: "Thu Aug 27 18:58:54 2026", Command: "kaboom"}
	if procidentity.MatchesForTest(procidentity.Info{}, live) {
		t.Error("Matches() accepted an empty recorded identity")
	}
	if procidentity.MatchesForTest(live, procidentity.Info{}) {
		t.Error("Matches() accepted an empty observed identity")
	}
}

func TestLookupReturnsSelf(t *testing.T) {
	info, ok := procidentity.LookupForTest(os.Getpid())
	if !ok {
		t.Fatal("Lookup() could not find this process")
	}
	if info.Start == "" || info.Command == "" {
		t.Fatalf("Lookup() returned incomplete identity: %+v", info)
	}
}
