// bridge_respawn_backoff_test.go -- Regression tests for bounded respawn backoff.
// A repeatedly-dying daemon must not become a spawn storm: respawnIfNeeded throttles
// the real cmd.Start() to at most one per (escalating, capped) interval.
package bridge

import (
	"net"
	"testing"
	"time"
)

// closedLoopbackPort returns a loopback port with nothing listening, so
// testRunner.IsServerRunning(port) is deterministically false (connection refused).
func closedLoopbackPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen(:0) error = %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return port
}

// TestRespawnIfNeeded_ThrottlesRapidSpawns asserts N rapid respawnIfNeeded calls
// produce at most ONE real spawn within the throttle interval, and that a spawn is
// permitted again once the interval elapses (fake clock advanced).
func TestRespawnIfNeeded_ThrottlesRapidSpawns(t *testing.T) {
	oldNow, oldSpawn := bridgeNow, respawnSpawnFn
	defer func() { bridgeNow = oldNow; respawnSpawnFn = oldSpawn }()

	fakeNow := time.Unix(1_700_000_000, 0)
	bridgeNow = func() time.Time { return fakeNow }

	spawns := 0
	respawnSpawnFn = func(s *daemonState) bool {
		spawns++
		// Simulate a daemon that never comes up, so the state stays "failed" and
		// every subsequent respawnIfNeeded re-enters the spawn section (and is
		// therefore subject to the throttle gate rather than short-circuiting ready).
		s.markFailed("simulated: respawned daemon never became healthy")
		return false
	}

	port := closedLoopbackPort(t)
	s := &daemonState{runner: testRunner,
		port:     port,
		readyCh:  make(chan struct{}),
		failedCh: make(chan struct{}),
	}
	// Put the state into "failed" so planRespawnAttempt takes the spawn path
	// immediately (rather than waiting on peer signals).
	s.markFailed("initial: force spawn path")

	// N rapid calls at the same instant: only the first should actually spawn.
	const rapid = 6
	for i := 0; i < rapid; i++ {
		s.respawnIfNeeded()
		s.markFailed("keep failed for next iteration") // ensure spawn path re-entry
	}
	if spawns != 1 {
		t.Fatalf("rapid respawns within the interval: want exactly 1 actual spawn, got %d", spawns)
	}

	// Advance past the throttle interval: the next respawn is permitted.
	fakeNow = fakeNow.Add(respawnMinInterval + time.Millisecond)
	s.respawnIfNeeded()
	if spawns != 2 {
		t.Fatalf("after the interval elapsed: want a 2nd real spawn, got %d", spawns)
	}

	// Immediately after, still within the (now escalated) interval: throttled again.
	s.markFailed("keep failed")
	s.respawnIfNeeded()
	if spawns != 2 {
		t.Fatalf("second rapid burst must be throttled: want spawns still 2, got %d", spawns)
	}
}

// TestReserveRespawnSlot_BackoffEscalatesAndCools unit-tests the throttle gate:
// escalating (capped) interval under a storm, reset after a cool-down gap.
func TestReserveRespawnSlot_BackoffEscalatesAndCools(t *testing.T) {
	s := &daemonState{runner: testRunner}
	base := time.Unix(1_700_000_000, 0)

	// First reservation always granted (immediate respawn after a death).
	if !s.reserveRespawnSlot(base) {
		t.Fatal("first reserveRespawnSlot must be granted")
	}
	// Same instant -> throttled.
	if s.reserveRespawnSlot(base) {
		t.Fatal("second reservation at the same instant must be throttled")
	}
	// Exactly at the min interval -> granted, and the interval doubles.
	if !s.reserveRespawnSlot(base.Add(respawnMinInterval)) {
		t.Fatal("reservation at the min interval must be granted")
	}
	// The interval has now escalated beyond min: a gap of min is no longer enough.
	if s.reserveRespawnSlot(base.Add(respawnMinInterval + respawnMinInterval)) {
		t.Fatal("after escalation, a min-sized gap must still be throttled")
	}
	// A cool-down gap >= respawnMaxBackoff resets the escalation and is granted.
	coolStart := base.Add(respawnMinInterval + respawnMaxBackoff + time.Second)
	if !s.reserveRespawnSlot(coolStart) {
		t.Fatal("a respawn after a full cool-down must be granted")
	}
	// And escalation is reset: the very next min-interval gap is granted again.
	if !s.reserveRespawnSlot(coolStart.Add(respawnMinInterval)) {
		t.Fatal("after cool-down reset, a min-interval gap must be granted again")
	}
}
