// reaper_test.go — Proves the reaper reclaims what is genuinely gone, kills only
// what is genuinely wedged or over budget, and never touches a healthy daemon.

package reaper_test

import (
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/instancegov"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/instancereg"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/reaper"
)

func rec(pid int, role instancereg.Role, startedAgo, beatAgo time.Duration, parallel bool, now time.Time) instancereg.Record {
	return instancereg.Record{
		PID: pid, Role: role, Parallel: parallel, Ports: []int{7890},
		StartedAt:   now.Add(-startedAgo).UTC().Format(time.RFC3339Nano),
		HeartbeatAt: now.Add(-beatAgo).UTC().Format(time.RFC3339Nano),
	}
}

func policy() instancegov.Policy {
	return instancegov.Policy{DaemonCap: 1, ParallelCap: 2, BridgeCap: 4}
}

// The single most important negative: a healthy production daemon must survive
// every reap. Killing it is worse than any leak the reaper exists to fix.
func TestHealthyDaemonIsNeverTouched(t *testing.T) {
	now := time.Now()
	healthy := rec(100, instancereg.RoleDaemon, 3*time.Hour, 5*time.Second, false, now)

	plan := reaper.Plan(reaper.Input{
		Live: []instancereg.Record{healthy}, All: []instancereg.Record{healthy},
		Policy: policy(), HeartbeatTTL: time.Minute, Now: now,
	})
	if len(plan.Actions) != 0 {
		t.Fatalf("Plan() = %+v, want no action against a healthy daemon", plan.Actions)
	}
	if len(plan.Keep) != 1 || plan.Keep[0].PID != 100 {
		t.Fatalf("Plan() kept %+v, want the healthy daemon", plan.Keep)
	}
}

// A record whose process is gone is garbage: remove the entry, kill nothing.
func TestDeadRecordIsPrunedNotKilled(t *testing.T) {
	now := time.Now()
	dead := rec(200, instancereg.RoleDaemon, time.Hour, time.Hour, false, now)

	plan := reaper.Plan(reaper.Input{
		Live: nil, All: []instancereg.Record{dead},
		Policy: policy(), HeartbeatTTL: time.Minute, Now: now,
	})
	if len(plan.Actions) != 1 {
		t.Fatalf("Plan() produced %d actions, want 1", len(plan.Actions))
	}
	if plan.Actions[0].Kind != reaper.ActionPrune {
		t.Fatalf("Plan() = %v for a dead record, want Prune", plan.Actions[0].Kind)
	}
}

// Alive but no longer heartbeating: it still holds its ports, so the entry cannot
// simply be forgotten -- the process has to be terminated.
func TestWedgedInstanceIsKilled(t *testing.T) {
	now := time.Now()
	wedged := rec(300, instancereg.RoleDaemon, 2*time.Hour, 30*time.Minute, false, now)

	plan := reaper.Plan(reaper.Input{
		Live: []instancereg.Record{wedged}, All: []instancereg.Record{wedged},
		Policy: policy(), HeartbeatTTL: time.Minute, Now: now,
	})
	if len(plan.Actions) != 1 || plan.Actions[0].Kind != reaper.ActionKill {
		t.Fatalf("Plan() = %+v, want a Kill for the wedged instance", plan.Actions)
	}
	if plan.Actions[0].Reason == "" {
		t.Error("Plan() gave no reason for killing")
	}
}

// Over the parallel cap, the oldest test daemons are killed and the newest kept.
func TestOverCapParallelDaemonsAreKilledOldestFirst(t *testing.T) {
	now := time.Now()
	a := rec(1, instancereg.RoleDaemon, 5*time.Hour, time.Second, true, now)
	b := rec(2, instancereg.RoleDaemon, 4*time.Hour, time.Second, true, now)
	c := rec(3, instancereg.RoleDaemon, 3*time.Hour, time.Second, true, now)
	d := rec(4, instancereg.RoleDaemon, 2*time.Hour, time.Second, true, now)
	live := []instancereg.Record{a, b, c, d}

	plan := reaper.Plan(reaper.Input{
		Live: live, All: live, Policy: policy(), HeartbeatTTL: time.Minute, Now: now,
	})
	killed := map[int]bool{}
	for _, action := range plan.Actions {
		if action.Kind == reaper.ActionKill {
			killed[action.Record.PID] = true
		}
	}
	if len(killed) != 2 || !killed[1] || !killed[2] {
		t.Fatalf("killed %v, want exactly pids 1 and 2 (oldest over a cap of 2)", killed)
	}
}

// A production daemon and test daemons coexist: the production one is not counted
// against the parallel cap and must never be chosen as an over-cap victim.
func TestProductionDaemonIsNotAnOverCapVictim(t *testing.T) {
	now := time.Now()
	production := rec(100, instancereg.RoleDaemon, 10*time.Hour, time.Second, false, now)
	live := []instancereg.Record{
		production,
		rec(1, instancereg.RoleDaemon, 5*time.Hour, time.Second, true, now),
		rec(2, instancereg.RoleDaemon, 4*time.Hour, time.Second, true, now),
		rec(3, instancereg.RoleDaemon, 3*time.Hour, time.Second, true, now),
	}
	plan := reaper.Plan(reaper.Input{
		Live: live, All: live, Policy: policy(), HeartbeatTTL: time.Minute, Now: now,
	})
	for _, action := range plan.Actions {
		if action.Record.PID == 100 {
			t.Fatalf("Plan() selected the production daemon for %v", action.Kind)
		}
	}
}

func TestDryRunPerformsNoSideEffects(t *testing.T) {
	now := time.Now()
	wedged := rec(300, instancereg.RoleDaemon, 2*time.Hour, time.Hour, false, now)
	plan := reaper.Plan(reaper.Input{
		Live: []instancereg.Record{wedged}, All: []instancereg.Record{wedged},
		Policy: policy(), HeartbeatTTL: time.Minute, Now: now,
	})

	var terminated []int
	result, err := reaper.Apply(plan, reaper.Deps{
		DryRun:    true,
		Terminate: func(pid int, force bool) error { terminated = append(terminated, pid); return nil },
		Remove:    func(path string) error { t.Error("dry run removed a record"); return nil },
	})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if len(terminated) != 0 {
		t.Fatalf("dry run terminated %v", terminated)
	}
	if result.Killed != 1 {
		t.Fatalf("Apply() reported %d planned kills, want 1", result.Killed)
	}
}

func TestApplyTerminatesAndRemoves(t *testing.T) {
	now := time.Now()
	wedged := rec(300, instancereg.RoleDaemon, 2*time.Hour, time.Hour, false, now)
	wedged.Path = "/tmp/registry/300.json"
	dead := rec(400, instancereg.RoleBridge, time.Hour, time.Hour, false, now)
	dead.Path = "/tmp/registry/400.json"

	plan := reaper.Plan(reaper.Input{
		Live: []instancereg.Record{wedged}, All: []instancereg.Record{wedged, dead},
		Policy: policy(), HeartbeatTTL: time.Minute, Now: now,
	})

	var terminated []int
	var removed []string
	result, err := reaper.Apply(plan, reaper.Deps{
		Terminate: func(pid int, force bool) error { terminated = append(terminated, pid); return nil },
		Remove:    func(path string) error { removed = append(removed, path); return nil },
	})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if len(terminated) != 1 || terminated[0] != 300 {
		t.Fatalf("terminated %v, want [300]", terminated)
	}
	if len(removed) != 2 {
		t.Fatalf("removed %v, want both records", removed)
	}
	if result.Killed != 1 || result.Pruned != 1 {
		t.Fatalf("Apply() = %+v, want 1 killed / 1 pruned", result)
	}
}

// A termination failure must be reported, never swallowed: a kill that silently
// failed leaves the port held while the census claims it was reclaimed.
func TestTerminationFailureIsReported(t *testing.T) {
	now := time.Now()
	wedged := rec(300, instancereg.RoleDaemon, 2*time.Hour, time.Hour, false, now)
	plan := reaper.Plan(reaper.Input{
		Live: []instancereg.Record{wedged}, All: []instancereg.Record{wedged},
		Policy: policy(), HeartbeatTTL: time.Minute, Now: now,
	})
	_, err := reaper.Apply(plan, reaper.Deps{
		Terminate: func(int, bool) error { return errStub },
		Remove:    func(string) error { return nil },
	})
	if err == nil {
		t.Fatal("Apply() swallowed a termination failure")
	}
}

var errStub = stubError("terminate refused")

type stubError string

func (e stubError) Error() string { return string(e) }
