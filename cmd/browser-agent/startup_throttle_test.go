// startup_throttle_test.go -- Tests for crash-loop restart-storm self-defense.
package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestRecordRestartAndComputeDelay_ThrottlesRapidRestarts asserts the backoff engages
// only once the same install restarts MORE than restartThrottleMax times within the
// window, and that the delay escalates and is capped.
func TestRecordRestartAndComputeDelay_ThrottlesRapidRestarts(t *testing.T) {
	base := time.Unix(1_700_000_000, 0)
	h := restartHistory{}

	// The first restartThrottleMax restarts, all inside the window, are free.
	for i := 0; i < restartThrottleMax; i++ {
		var delay time.Duration
		h, delay = recordRestartAndComputeDelay(h, base.Add(time.Duration(i)*time.Second), "1.0.0", 100)
		if delay != 0 {
			t.Fatalf("restart %d within threshold: want delay 0, got %s", i+1, delay)
		}
	}

	// The (max+1)-th rapid restart engages the backoff: one step.
	var delay time.Duration
	h, delay = recordRestartAndComputeDelay(h, base.Add(time.Duration(restartThrottleMax)*time.Second), "1.0.0", 100)
	if delay != restartThrottleStep {
		t.Fatalf("first over-threshold restart: want delay %s, got %s", restartThrottleStep, delay)
	}

	// The next one escalates by another step.
	h, delay = recordRestartAndComputeDelay(h, base.Add(time.Duration(restartThrottleMax+1)*time.Second), "1.0.0", 100)
	if delay != 2*restartThrottleStep {
		t.Fatalf("second over-threshold restart: want delay %s, got %s", 2*restartThrottleStep, delay)
	}

	// Escalation is capped.
	for i := 0; i < 20; i++ {
		h, delay = recordRestartAndComputeDelay(h, base.Add(time.Duration(restartThrottleMax+2+i)*time.Second), "1.0.0", 100)
	}
	if delay != restartThrottleCap {
		t.Fatalf("escalating delay must be capped at %s, got %s", restartThrottleCap, delay)
	}
}

// TestRecordRestartAndComputeDelay_SpacedOutNeverThrottles asserts restarts spread
// further apart than the window never engage the backoff (old restarts are pruned).
func TestRecordRestartAndComputeDelay_SpacedOutNeverThrottles(t *testing.T) {
	base := time.Unix(1_700_000_000, 0)
	h := restartHistory{}
	for i := 0; i < 50; i++ {
		var delay time.Duration
		// Each restart is a full window + 1s after the previous, so pruning keeps
		// the count at 1 every time.
		now := base.Add(time.Duration(i) * (restartThrottleWindow + time.Second))
		h, delay = recordRestartAndComputeDelay(h, now, "1.0.0", 100)
		if delay != 0 {
			t.Fatalf("spaced-out restart %d must not throttle, got delay %s", i+1, delay)
		}
	}
}

// TestRecordRestartAndComputeDelay_UpgradeResetsCounter asserts an upgrade (new
// version) or install-epoch takeover (new epoch) is NOT counted as a crash-restart:
// the counter resets so a real deploy is never penalized.
func TestRecordRestartAndComputeDelay_UpgradeResetsCounter(t *testing.T) {
	base := time.Unix(1_700_000_000, 0)

	// Build a history that is deep in throttle territory for install (1.0.0, epoch 100).
	h := restartHistory{}
	for i := 0; i < restartThrottleMax+5; i++ {
		h, _ = recordRestartAndComputeDelay(h, base.Add(time.Duration(i)*time.Second), "1.0.0", 100)
	}

	// An upgrade to a new VERSION at the same instant must reset -> no throttle.
	next, delay := recordRestartAndComputeDelay(h, base.Add(time.Duration(restartThrottleMax+5)*time.Second), "1.1.0", 100)
	if delay != 0 {
		t.Fatalf("upgrade (new version) must not be throttled, got delay %s", delay)
	}
	if len(next.Timestamps) != 1 || next.Version != "1.1.0" {
		t.Fatalf("upgrade must reset history, got version=%s timestamps=%d", next.Version, len(next.Timestamps))
	}

	// A same-version but new-EPOCH takeover must likewise reset.
	_, delay = recordRestartAndComputeDelay(h, base.Add(time.Duration(restartThrottleMax+6)*time.Second), "1.0.0", 200)
	if delay != 0 {
		t.Fatalf("epoch takeover (new epoch) must not be throttled, got delay %s", delay)
	}
}

// TestRestartHistoryPersistenceRoundTrip asserts save/load preserves the history and
// that a missing/corrupt file yields an empty (fresh) history, never an error.
func TestRestartHistoryPersistenceRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := restartHistoryPath(dir)

	// Missing file -> empty.
	if got := loadRestartHistory(path); got.Version != "" || len(got.Timestamps) != 0 {
		t.Fatalf("missing file must load empty, got %+v", got)
	}

	want := restartHistory{Version: "2.0.0", InstallEpoch: 42, Timestamps: []int64{111, 222, 333}}
	if err := saveRestartHistory(path, want); err != nil {
		t.Fatalf("saveRestartHistory error = %v", err)
	}
	got := loadRestartHistory(path)
	if got.Version != want.Version || got.InstallEpoch != want.InstallEpoch || len(got.Timestamps) != len(want.Timestamps) {
		t.Fatalf("round-trip mismatch: got %+v want %+v", got, want)
	}

	// Corrupt file -> empty, no error.
	if err := os.WriteFile(path, []byte("{not valid json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := loadRestartHistory(path); got.Version != "" {
		t.Fatalf("corrupt file must load empty, got %+v", got)
	}
}

// TestApplyStartupRestartThrottle_EngagesPastThreshold drives the wired entrypoint
// with an injected clock + sleep + temp state dir: the delay engages only after the
// threshold and the injected sleep receives it (no real sleeping).
func TestApplyStartupRestartThrottle_EngagesPastThreshold(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("KABOOM_STATE_DIR", dir)

	oldNow, oldEpoch, oldSleep, oldVer := daemonNow, daemonInstallEpoch, daemonThrottleSleep, version
	defer func() {
		daemonNow = oldNow
		daemonInstallEpoch = oldEpoch
		daemonThrottleSleep = oldSleep
		version = oldVer
	}()
	version = "9.9.9"
	daemonInstallEpoch = func() int64 { return 777 }

	base := time.Unix(1_700_000_000, 0)
	fakeNow := base
	daemonNow = func() time.Time { return fakeNow }

	var slept time.Duration
	daemonThrottleSleep = func(d time.Duration) { slept += d }

	server, err := NewServer(filepath.Join(dir, "throttle.log"), 100)
	if err != nil {
		t.Fatalf("NewServer error = %v", err)
	}
	defer server.logs.shutdownAsyncLogger(2 * time.Second)

	// The first restartThrottleMax rapid starts do not throttle.
	for i := 0; i < restartThrottleMax; i++ {
		fakeNow = base.Add(time.Duration(i) * time.Second)
		if d := applyStartupRestartThrottle(server, 7890); d != 0 {
			t.Fatalf("start %d within threshold: want no delay, got %s", i+1, d)
		}
	}
	if slept != 0 {
		t.Fatalf("no delay should have been slept yet, got %s", slept)
	}

	// The next rapid start engages the backoff, and the injected sleep receives it.
	fakeNow = base.Add(time.Duration(restartThrottleMax) * time.Second)
	d := applyStartupRestartThrottle(server, 7890)
	if d != restartThrottleStep {
		t.Fatalf("over-threshold start: want delay %s, got %s", restartThrottleStep, d)
	}
	if slept != restartThrottleStep {
		t.Fatalf("injected sleep must receive the delay, got %s", slept)
	}
}

// TestCleanShutdownResetsRestartThrottle is the regression for the release-gate break:
// legitimate rapid stop/start cycles (which the test suite and real users perform) must
// NOT accumulate toward the throttle. A clean shutdown clears the history, so a rapid
// restart right after a clean stop is never throttled — only an uncleared (crash) run is.
func TestCleanShutdownResetsRestartThrottle(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("KABOOM_STATE_DIR", dir)

	oldNow, oldEpoch, oldSleep, oldVer := daemonNow, daemonInstallEpoch, daemonThrottleSleep, version
	defer func() {
		daemonNow = oldNow
		daemonInstallEpoch = oldEpoch
		daemonThrottleSleep = oldSleep
		version = oldVer
	}()
	version = "9.9.9"
	daemonInstallEpoch = func() int64 { return 777 }
	base := time.Unix(1_700_000_000, 0)
	fakeNow := base
	daemonNow = func() time.Time { return fakeNow }
	daemonThrottleSleep = func(time.Duration) {}

	server, err := NewServer(filepath.Join(dir, "throttle-reset.log"), 100)
	if err != nil {
		t.Fatalf("NewServer error = %v", err)
	}
	defer server.logs.shutdownAsyncLogger(2 * time.Second)

	// Drive rapid starts past the threshold so the backoff is engaged.
	for i := 0; i <= restartThrottleMax; i++ {
		fakeNow = base.Add(time.Duration(i) * time.Second)
		_ = applyStartupRestartThrottle(server, 7890)
	}
	fakeNow = base.Add(time.Duration(restartThrottleMax+1) * time.Second)
	if d := applyStartupRestartThrottle(server, 7890); d <= 0 {
		t.Fatalf("precondition: throttle should be engaged before the clean shutdown, got %s", d)
	}

	// A clean shutdown clears the history...
	clearRestartHistoryOnCleanShutdown(server, 7890)

	// ...so the very next rapid start is NOT throttled (counter reset).
	fakeNow = base.Add(time.Duration(restartThrottleMax+2) * time.Second)
	if d := applyStartupRestartThrottle(server, 7890); d != 0 {
		t.Fatalf("after a clean shutdown, a rapid restart must not be throttled, got %s", d)
	}
}

// TestClearRestartHistoryOnCleanShutdown asserts the clear removes the history file and
// is a no-op (no error/log) when the file is already absent.
func TestClearRestartHistoryOnCleanShutdown(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("KABOOM_STATE_DIR", dir)

	server, err := NewServer(filepath.Join(dir, "clear.log"), 100)
	if err != nil {
		t.Fatalf("NewServer error = %v", err)
	}
	defer server.logs.shutdownAsyncLogger(2 * time.Second)

	path := restartHistoryPath(dir)
	if err := saveRestartHistory(path, restartHistory{Version: "1.0.0", Timestamps: []int64{1, 2, 3}}); err != nil {
		t.Fatalf("saveRestartHistory error = %v", err)
	}
	clearRestartHistoryOnCleanShutdown(server, 7890)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("clear must remove the history file, stat err = %v", err)
	}
	// Idempotent: clearing an already-absent history is a no-op.
	clearRestartHistoryOnCleanShutdown(server, 7890)
}
