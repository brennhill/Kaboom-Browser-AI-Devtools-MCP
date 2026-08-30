// policy_test.go — Pins the shared budget predicates. These replace the separate
// copies that admission and reclamation each used to carry; the wedged pair had
// already drifted apart before they were merged here.

package instancegov_test

import (
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/instancegov"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/instancereg"
)

func at(now time.Time, ago time.Duration) string {
	return now.Add(-ago).UTC().Format(time.RFC3339Nano)
}

func daemonRec(pid int, startedAgo time.Duration, parallel bool, now time.Time) instancereg.Record {
	return instancereg.Record{
		PID: pid, Role: instancereg.RoleDaemon, Parallel: parallel, Ports: []int{7890},
		StartedAt: at(now, startedAgo), HeartbeatAt: at(now, time.Second),
	}
}

func TestAutoParallelCapIsBoundedByCores(t *testing.T) {
	for _, tc := range []struct{ cpus, want int }{
		{1, 2}, {2, 2}, {4, 2}, {8, 2}, {12, 3}, {16, 4}, {32, 4}, {64, 4}, {0, 2}, {-1, 2},
	} {
		if got := instancegov.AutoParallelCapForTest(tc.cpus); got != tc.want {
			t.Errorf("AutoParallelCap(%d) = %d, want %d", tc.cpus, got, tc.want)
		}
	}
}

func TestDefaultPolicyAllowsExactlyOneProductionDaemon(t *testing.T) {
	if got := instancegov.DefaultPolicy().DaemonCap; got != 1 {
		t.Fatalf("DefaultPolicy().DaemonCap = %d, want 1", got)
	}
}

// The two callers differ only in whether a candidate is joining. incoming=1 is
// admission (the candidate counts); incoming=0 is reclamation (nothing joins).
func TestSurplusCountsTheIncomingCandidate(t *testing.T) {
	now := time.Now()
	members := []instancereg.Record{
		daemonRec(1, 3*time.Hour, true, now),
		daemonRec(2, 2*time.Hour, true, now),
	}
	if got := instancegov.Surplus(members, 2, 0); len(got) != 0 {
		t.Errorf("Surplus(2 members, cap 2, incoming 0) = %d victims, want 0", len(got))
	}
	got := instancegov.Surplus(members, 2, 1)
	if len(got) != 1 {
		t.Fatalf("Surplus(2 members, cap 2, incoming 1) = %d victims, want 1", len(got))
	}
	if got[0].PID != 1 {
		t.Errorf("evicted pid %d, want 1 (the oldest)", got[0].PID)
	}
}

func TestSurplusEvictsOldestFirstAndOnlyTheExcess(t *testing.T) {
	now := time.Now()
	members := []instancereg.Record{
		daemonRec(3, 1*time.Hour, true, now),
		daemonRec(1, 3*time.Hour, true, now),
		daemonRec(2, 2*time.Hour, true, now),
	}
	got := instancegov.Surplus(members, 2, 1)
	if len(got) != 2 {
		t.Fatalf("Surplus() = %d victims, want 2", len(got))
	}
	if got[0].PID != 1 || got[1].PID != 2 {
		t.Fatalf("evicted %d,%d, want 1,2 (oldest first)", got[0].PID, got[1].PID)
	}
}

// A cap below 1 is a misconfiguration, not permission to evict everything.
func TestSurplusClampsAnImpossibleCap(t *testing.T) {
	now := time.Now()
	members := []instancereg.Record{daemonRec(1, time.Hour, true, now)}
	if got := instancegov.Surplus(members, 0, 0); len(got) != 0 {
		t.Errorf("Surplus(1 member, cap 0, incoming 0) = %d victims, want 0 after clamping to 1", len(got))
	}
}

func TestSurplusNeverExceedsTheMemberCount(t *testing.T) {
	now := time.Now()
	members := []instancereg.Record{daemonRec(1, time.Hour, true, now)}
	if got := instancegov.Surplus(members, 1, 5); len(got) != 1 {
		t.Fatalf("Surplus() = %d victims, want at most the 1 member present", len(got))
	}
}

// A corrupt timestamp must not make an entry the automatic victim.
func TestOldestFirstSortsUnreadableStartTimesLast(t *testing.T) {
	now := time.Now()
	corrupt := daemonRec(100, 0, true, now)
	corrupt.StartedAt = "not-a-timestamp"
	old := daemonRec(200, 5*time.Hour, true, now)

	ordered := instancegov.OldestFirstForTest([]instancereg.Record{corrupt, old})
	if ordered[0].PID != 200 {
		t.Fatalf("OldestFirst()[0] = pid %d, want 200 (the genuinely oldest)", ordered[0].PID)
	}
}

func TestOldestFirstDoesNotMutateItsInput(t *testing.T) {
	now := time.Now()
	input := []instancereg.Record{daemonRec(1, time.Hour, true, now), daemonRec(2, 5*time.Hour, true, now)}
	_ = instancegov.OldestFirstForTest(input)
	if input[0].PID != 1 {
		t.Fatal("OldestFirst() reordered its caller's slice")
	}
}

func TestDaemonsAndBridgesSelectByKind(t *testing.T) {
	now := time.Now()
	records := []instancereg.Record{
		daemonRec(1, time.Hour, false, now),
		daemonRec(2, time.Hour, true, now),
		{PID: 3, Role: instancereg.RoleBridge, StartedAt: at(now, time.Hour)},
	}
	if got := instancegov.Daemons(records, false); len(got) != 1 || got[0].PID != 1 {
		t.Errorf("Daemons(production) = %+v, want only pid 1", got)
	}
	if got := instancegov.Daemons(records, true); len(got) != 1 || got[0].PID != 2 {
		t.Errorf("Daemons(parallel) = %+v, want only pid 2", got)
	}
	if got := instancegov.Bridges(records); len(got) != 1 || got[0].PID != 3 {
		t.Errorf("Bridges() = %+v, want only pid 3", got)
	}
}

// The predicate that had drifted. The reaper's copy returned false for an
// unreadable heartbeat while the registry's fell back to start time, so one
// record could be healthy to one caller and wedged to the other.
func TestIsWedgedFallsBackToStartTimeWhenTheHeartbeatIsUnreadable(t *testing.T) {
	now := time.Now()
	ttl := time.Minute

	fresh := daemonRec(1, 10*time.Second, false, now)
	if instancegov.IsWedged(fresh, now, ttl) {
		t.Error("a heartbeating instance was called wedged")
	}

	stale := daemonRec(2, time.Hour, false, now)
	stale.HeartbeatAt = at(now, 45*time.Minute)
	if !instancegov.IsWedged(stale, now, ttl) {
		t.Error("a 45-minute-old heartbeat was not called wedged")
	}

	unreadableOld := daemonRec(3, time.Hour, false, now)
	unreadableOld.HeartbeatAt = "garbage"
	if !instancegov.IsWedged(unreadableOld, now, ttl) {
		t.Error("an hour-old record with no readable heartbeat was not called wedged")
	}

	unreadableYoung := daemonRec(4, 5*time.Second, false, now)
	unreadableYoung.HeartbeatAt = "garbage"
	if instancegov.IsWedged(unreadableYoung, now, ttl) {
		t.Error("a five-second-old record was called wedged on an unreadable heartbeat")
	}

	unknown := instancereg.Record{PID: 5, Role: instancereg.RoleDaemon}
	if instancegov.IsWedged(unknown, now, ttl) {
		t.Error("a record with neither timestamp was called wedged")
	}
}
