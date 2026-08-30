// admission_test.go — Proves the machine-wide cap: one production daemon ever, a
// bounded number of test daemons, and eviction of the oldest when over budget.

package instancereg_test

import (
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/instancereg"
)

func daemonAt(pid int, started time.Time, parallel bool) instancereg.Record {
	return instancereg.Record{
		PID: pid, Role: instancereg.RoleDaemon, Parallel: parallel,
		StartedAt:   started.UTC().Format(time.RFC3339Nano),
		HeartbeatAt: started.UTC().Format(time.RFC3339Nano),
		Ports:       []int{7890},
	}
}

func TestAutoParallelCapIsBoundedByCores(t *testing.T) {
	cases := []struct{ cpus, want int }{
		{1, 2}, {2, 2}, {4, 2}, {8, 2}, {12, 3}, {16, 4}, {32, 4}, {64, 4}, {0, 2}, {-1, 2},
	}
	for _, tc := range cases {
		if got := instancereg.AutoParallelCap(tc.cpus); got != tc.want {
			t.Errorf("AutoParallelCap(%d) = %d, want %d", tc.cpus, got, tc.want)
		}
	}
}

// The headline guarantee: never more than one production daemon on a machine,
// regardless of how many state directories exist.
func TestSecondProductionDaemonDefersEvenAcrossStateDirs(t *testing.T) {
	now := time.Now()
	incumbent := daemonAt(100, now.Add(-time.Hour), false)
	incumbent.StateDir = "/Users/dev/.kaboom"

	candidate := daemonAt(200, now, false)
	candidate.StateDir = "/Users/dev/worktree/.kaboom" // different state dir, same machine

	decision := instancereg.Decide([]instancereg.Record{incumbent}, candidate, instancereg.DefaultPolicy(), now)
	if decision.Outcome != instancereg.OutcomeDefer {
		t.Fatalf("Decide() = %v, want Defer (one production daemon per machine)", decision.Outcome)
	}
	if decision.DeferTo == nil || decision.DeferTo.PID != 100 {
		t.Fatalf("Decide() deferred to %+v, want the incumbent pid 100", decision.DeferTo)
	}
}

func TestFirstDaemonIsAdmitted(t *testing.T) {
	now := time.Now()
	decision := instancereg.Decide(nil, daemonAt(100, now, false), instancereg.DefaultPolicy(), now)
	if decision.Outcome != instancereg.OutcomeAdmit {
		t.Fatalf("Decide() = %v, want Admit for the first daemon", decision.Outcome)
	}
}

// A production daemon must never be evicted to make room for a test daemon, and a
// test daemon must never make a production daemon defer.
func TestParallelAndProductionDaemonsAreCountedSeparately(t *testing.T) {
	now := time.Now()
	production := daemonAt(100, now.Add(-time.Hour), false)

	parallel := daemonAt(200, now, true)
	decision := instancereg.Decide([]instancereg.Record{production}, parallel, instancereg.DefaultPolicy(), now)
	if decision.Outcome != instancereg.OutcomeAdmit {
		t.Fatalf("parallel candidate got %v, want Admit alongside a production daemon", decision.Outcome)
	}

	testDaemon := daemonAt(300, now.Add(-time.Minute), true)
	decision = instancereg.Decide([]instancereg.Record{testDaemon}, daemonAt(400, now, false), instancereg.DefaultPolicy(), now)
	if decision.Outcome != instancereg.OutcomeAdmit {
		t.Fatalf("production candidate got %v, want Admit despite a parallel daemon", decision.Outcome)
	}
}

// Over the test cap, the OLDEST test daemons are evicted so a run can always
// start. This is the "auto-detect and kill the extras" behavior.
func TestOverParallelCapEvictsOldestFirst(t *testing.T) {
	now := time.Now()
	policy := instancereg.Policy{DaemonCap: 1, ParallelCap: 2, BridgeCap: 8}

	oldest := daemonAt(100, now.Add(-3*time.Hour), true)
	middle := daemonAt(200, now.Add(-2*time.Hour), true)
	newest := daemonAt(300, now.Add(-1*time.Hour), true)

	decision := instancereg.Decide(
		[]instancereg.Record{newest, middle, oldest},
		daemonAt(400, now, true),
		policy, now,
	)
	if decision.Outcome != instancereg.OutcomeEvict {
		t.Fatalf("Decide() = %v, want Evict when over the parallel cap", decision.Outcome)
	}
	// 3 live + 1 candidate = 4, cap 2 -> evict 2, oldest first.
	if len(decision.Evict) != 2 {
		t.Fatalf("Decide() evicts %d, want 2", len(decision.Evict))
	}
	if decision.Evict[0].PID != 100 || decision.Evict[1].PID != 200 {
		t.Fatalf("Decide() evicts pids %d,%d, want 100,200 (oldest first)",
			decision.Evict[0].PID, decision.Evict[1].PID)
	}
}

func TestAtParallelCapExactlyStillAdmits(t *testing.T) {
	now := time.Now()
	policy := instancereg.Policy{DaemonCap: 1, ParallelCap: 3, BridgeCap: 8}
	live := []instancereg.Record{
		daemonAt(100, now.Add(-3*time.Hour), true),
		daemonAt(200, now.Add(-2*time.Hour), true),
	}
	if decision := instancereg.Decide(live, daemonAt(300, now, true), policy, now); decision.Outcome != instancereg.OutcomeAdmit {
		t.Fatalf("Decide() = %v with 2 live under cap 3, want Admit", decision.Outcome)
	}
}

func TestBridgesAreCappedAndEvictOldest(t *testing.T) {
	now := time.Now()
	policy := instancereg.Policy{DaemonCap: 1, ParallelCap: 2, BridgeCap: 2}
	bridge := func(pid int, ago time.Duration) instancereg.Record {
		return instancereg.Record{
			PID: pid, Role: instancereg.RoleBridge,
			StartedAt:   now.Add(-ago).UTC().Format(time.RFC3339Nano),
			HeartbeatAt: now.Add(-ago).UTC().Format(time.RFC3339Nano),
		}
	}
	live := []instancereg.Record{bridge(1, 3*time.Hour), bridge(2, 2*time.Hour)}
	decision := instancereg.Decide(live, bridge(3, 0), policy, now)
	if decision.Outcome != instancereg.OutcomeEvict {
		t.Fatalf("Decide() = %v, want Evict over the bridge cap", decision.Outcome)
	}
	if len(decision.Evict) != 1 || decision.Evict[0].PID != 1 {
		t.Fatalf("Decide() evicts %+v, want the oldest bridge (pid 1)", decision.Evict)
	}
}

// A candidate must never be told to evict itself, which would make a restarting
// daemon kill the entry it is about to replace and then defer to nothing.
func TestCandidateIsNeverItsOwnEvictionTarget(t *testing.T) {
	now := time.Now()
	policy := instancereg.Policy{DaemonCap: 1, ParallelCap: 1, BridgeCap: 8}
	self := daemonAt(100, now.Add(-time.Hour), true)
	decision := instancereg.Decide([]instancereg.Record{self}, self, policy, now)
	for _, victim := range decision.Evict {
		if victim.PID == self.PID {
			t.Fatal("Decide() selected the candidate itself for eviction")
		}
	}
	if decision.Outcome == instancereg.OutcomeDefer && decision.DeferTo != nil && decision.DeferTo.PID == self.PID {
		t.Fatal("Decide() told the candidate to defer to itself")
	}
}

// Records with an unreadable start time must sort LAST for eviction, never first:
// a corrupt timestamp should not make an entry the automatic victim.
func TestUnparseableStartTimeIsNotEvictedFirst(t *testing.T) {
	now := time.Now()
	policy := instancereg.Policy{DaemonCap: 1, ParallelCap: 1, BridgeCap: 8}
	corrupt := daemonAt(100, now, true)
	corrupt.StartedAt = "not-a-timestamp"
	old := daemonAt(200, now.Add(-5*time.Hour), true)

	decision := instancereg.Decide([]instancereg.Record{corrupt, old}, daemonAt(300, now, true), policy, now)
	if len(decision.Evict) == 0 {
		t.Fatal("Decide() evicted nothing while over cap")
	}
	if decision.Evict[0].PID != 200 {
		t.Fatalf("Decide() evicted pid %d first, want 200 (the genuinely oldest)", decision.Evict[0].PID)
	}
}
