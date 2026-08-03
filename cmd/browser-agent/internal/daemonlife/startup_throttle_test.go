// startup_throttle_test.go -- Tests for crash-loop restart-storm self-defense.
package daemonlife

import (
	"io/fs"
	"os"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statediag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statefault"
)

type faultLifecycleFilesystem struct {
	readData  []byte
	readErr   error
	writeErr  error
	removeErr error
}

func (f faultLifecycleFilesystem) ReadFile(string) ([]byte, error) {
	if f.readErr != nil {
		return nil, f.readErr
	}
	if f.readData == nil {
		return nil, fs.ErrNotExist
	}
	return append([]byte(nil), f.readData...), nil
}
func (f faultLifecycleFilesystem) WriteFile(string, []byte) error { return f.writeErr }
func (f faultLifecycleFilesystem) Remove(string) error            { return f.removeErr }

func installLifecycleFault(t *testing.T, files lifecycleFilesystem) {
	t.Helper()
	daemonLifecycleFiles = files
	t.Cleanup(func() { daemonLifecycleFiles = localLifecycleFilesystem{} })
}

func TestStartupThrottleCanonicalStateFaultsFailOpenWithDiagnostics(t *testing.T) {
	for _, kind := range []statefault.Kind{statefault.Read, statefault.Write, statefault.Quota, statefault.Cancellation} {
		t.Run(string(kind), func(t *testing.T) {
			t.Setenv("KABOOM_STATE_DIR", t.TempDir())
			scenario := statefault.New(kind, "private-restart-history")
			files := faultLifecycleFilesystem{}
			if kind == statefault.Read {
				files.readErr = scenario.Error()
			} else {
				files.writeErr = scenario.Error()
			}
			installLifecycleFault(t, files)
			deps, logger := newTestDeps(t)
			diagnostics := statediag.NewCollector()
			deps.Recovery = diagnostics

			if delay := ApplyStartupRestartThrottle(deps, throttleTestPort); delay != 0 {
				t.Fatalf("fault fallback delay = %s, want zero", delay)
			}
			if got := diagnostics.Snapshot(); len(got) == 0 || got[0].Name != "restart_history_state" {
				t.Fatalf("diagnostics = %#v, want restart history incident", got)
			}
			for _, event := range logger.events {
				if value := event.Fields["error"]; value != nil {
					t.Fatalf("lifecycle event leaked raw error: %#v", event)
				}
			}
		})
	}
}

// TestRecordRestartAndComputeDelay_ThrottlesRapidRestarts asserts the backoff engages
// only once the same install restarts MORE than restartThrottleMax times within the
// window, and that the delay escalates and is capped.
func TestRecordRestartAndComputeDelay_ThrottlesRapidRestarts(t *testing.T) {
	base := time.Unix(1_700_000_000, 0)
	h := restartHistory{}

	// The first restartThrottleMax restarts, all inside the window, are free.
	for i := 0; i < restartThrottleMax; i++ {
		var delay time.Duration
		h, delay = recordRestartAndComputeDelay(h, base.Add(time.Duration(i)*time.Second), "1.0.0", 100, throttleTestPort)
		if delay != 0 {
			t.Fatalf("restart %d within threshold: want delay 0, got %s", i+1, delay)
		}
	}

	// The (max+1)-th rapid restart engages the backoff: one step.
	var delay time.Duration
	h, delay = recordRestartAndComputeDelay(h, base.Add(time.Duration(restartThrottleMax)*time.Second), "1.0.0", 100, throttleTestPort)
	if delay != restartThrottleStep {
		t.Fatalf("first over-threshold restart: want delay %s, got %s", restartThrottleStep, delay)
	}

	// The next one escalates by another step.
	h, delay = recordRestartAndComputeDelay(h, base.Add(time.Duration(restartThrottleMax+1)*time.Second), "1.0.0", 100, throttleTestPort)
	if delay != 2*restartThrottleStep {
		t.Fatalf("second over-threshold restart: want delay %s, got %s", 2*restartThrottleStep, delay)
	}

	// Escalation is capped.
	for i := 0; i < 20; i++ {
		h, delay = recordRestartAndComputeDelay(h, base.Add(time.Duration(restartThrottleMax+2+i)*time.Second), "1.0.0", 100, throttleTestPort)
	}
	if delay != restartThrottleCap {
		t.Fatalf("escalating delay must be capped at %s, got %s", restartThrottleCap, delay)
	}
}

func TestApplyStartupRestartThrottleReportsCorruptHistory(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("KABOOM_STATE_DIR", stateDir)
	path := restartHistoryPath(stateDir)
	if err := os.WriteFile(path, []byte(`{"token":"secret"`), 0o600); err != nil {
		t.Fatal(err)
	}

	deps, _ := newTestDeps(t)
	diagnostics := statediag.NewCollector()
	deps.Recovery = diagnostics
	if delay := ApplyStartupRestartThrottle(deps, throttleTestPort); delay != 0 {
		t.Fatalf("corrupt history fallback delay = %s, want none", delay)
	}
	got := diagnostics.Snapshot()
	if len(got) != 1 || got[0].Name != "restart_history_state" || got[0].Fix == "" {
		t.Fatalf("diagnostics = %#v, want actionable restart-history warning", got)
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
		h, delay = recordRestartAndComputeDelay(h, now, "1.0.0", 100, throttleTestPort)
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
		h, _ = recordRestartAndComputeDelay(h, base.Add(time.Duration(i)*time.Second), "1.0.0", 100, throttleTestPort)
	}

	// An upgrade to a new VERSION at the same instant must reset -> no throttle.
	next, delay := recordRestartAndComputeDelay(h, base.Add(time.Duration(restartThrottleMax+5)*time.Second), "1.1.0", 100, throttleTestPort)
	if delay != 0 {
		t.Fatalf("upgrade (new version) must not be throttled, got delay %s", delay)
	}
	if len(next.Timestamps) != 1 || next.Version != "1.1.0" {
		t.Fatalf("upgrade must reset history, got version=%s timestamps=%d", next.Version, len(next.Timestamps))
	}

	// A same-version but new-EPOCH takeover must likewise reset.
	_, delay = recordRestartAndComputeDelay(h, base.Add(time.Duration(restartThrottleMax+6)*time.Second), "1.0.0", 200, throttleTestPort)
	if delay != 0 {
		t.Fatalf("epoch takeover (new epoch) must not be throttled, got delay %s", delay)
	}
}

// throttleTestPort is the port used by the pure-function tests. The throttle is
// scoped per listening port, so tests that are not specifically about port scoping
// must all use the same one.
const throttleTestPort = 7890

// TestRecordRestartAndComputeDelay_DifferentPortIsADifferentInstance is the
// regression for the CI break this scoping fixes.
//
// A restart storm is about ONE endpoint being restarted over and over — that is
// what launchd throttles. Counting every daemon that happens to share a state dir
// conflated unrelated instances: the Go test suite spawns many short-lived daemons
// on distinct random ports inside one state dir, crossed the threshold, and then
// every subsequent start slept up to the 3s cap — so ~25 integration tests failed
// with "Server failed to start" on CI while passing locally (the -race suite is
// slow enough to spread restarts past the 30s window; the coverage run is not).
//
// Production is unaffected: the daemon always uses the same port, so a genuine
// crash loop still accumulates and still backs off.
func TestRecordRestartAndComputeDelay_DifferentPortIsADifferentInstance(t *testing.T) {
	base := time.Unix(1_700_000_000, 0)

	// Drive one port deep into throttle territory.
	h := restartHistory{}
	for i := 0; i < restartThrottleMax+5; i++ {
		h, _ = recordRestartAndComputeDelay(h, base.Add(time.Duration(i)*time.Second), "1.0.0", 100, 7890)
	}

	// A start on a DIFFERENT port at the same instant is a different daemon
	// instance, not a restart of the throttled one — it must not be penalized.
	next, delay := recordRestartAndComputeDelay(h, base.Add(time.Duration(restartThrottleMax+5)*time.Second), "1.0.0", 100, 45291)
	if delay != 0 {
		t.Fatalf("a different port is a different instance and must not be throttled, got delay %s", delay)
	}
	if next.Port != 45291 || len(next.Timestamps) != 1 {
		t.Fatalf("a different port must start a fresh history, got port=%d timestamps=%d", next.Port, len(next.Timestamps))
	}

	// And the same port keeps accumulating — the guard must not disable the defense.
	_, delay = recordRestartAndComputeDelay(h, base.Add(time.Duration(restartThrottleMax+6)*time.Second), "1.0.0", 100, 7890)
	if delay == 0 {
		t.Fatal("the same port must still accumulate toward the storm threshold")
	}
}

// TestRestartHistoryPersistenceRoundTrip asserts save/load preserves the history and
// that a missing/corrupt file yields an empty (fresh) history, never an error.
func TestRestartHistoryPersistenceRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := restartHistoryPath(dir)

	// Missing file -> empty.
	if got, err := loadRestartHistory(path); err != nil || got.Version != "" || len(got.Timestamps) != 0 {
		t.Fatalf("missing file must load empty, got %+v", got)
	}

	want := restartHistory{Version: "2.0.0", InstallEpoch: 42, Timestamps: []int64{111, 222, 333}}
	if err := saveRestartHistory(path, want); err != nil {
		t.Fatalf("saveRestartHistory error = %v", err)
	}
	got, err := loadRestartHistory(path)
	if err != nil {
		t.Fatalf("loadRestartHistory error = %v", err)
	}
	if got.Version != want.Version || got.InstallEpoch != want.InstallEpoch || len(got.Timestamps) != len(want.Timestamps) {
		t.Fatalf("round-trip mismatch: got %+v want %+v", got, want)
	}

	// Corrupt file -> empty fallback plus explicit recovery error.
	if err := os.WriteFile(path, []byte("{not valid json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := loadRestartHistory(path); err == nil || got.Version != "" {
		t.Fatalf("corrupt file = %+v, %v; want empty plus recovery error", got, err)
	}
}

// TestApplyStartupRestartThrottle_EngagesPastThreshold drives the wired entrypoint
// with an injected clock + sleep + temp state dir: the delay engages only after the
// threshold and the injected sleep receives it (no real sleeping).
func TestApplyStartupRestartThrottle_EngagesPastThreshold(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("KABOOM_STATE_DIR", dir)

	base := time.Unix(1_700_000_000, 0)
	fakeNow := freezeClock(t, base)
	stubInstallEpoch(t, 777)

	oldSleep := daemonThrottleSleep
	defer func() { daemonThrottleSleep = oldSleep }()
	var slept time.Duration
	daemonThrottleSleep = func(d time.Duration) { slept += d }

	deps, _ := newTestDeps(t)
	deps.Version = "9.9.9"

	// The first restartThrottleMax rapid starts do not throttle.
	for i := 0; i < restartThrottleMax; i++ {
		*fakeNow = base.Add(time.Duration(i) * time.Second)
		if d := ApplyStartupRestartThrottle(deps, 7890); d != 0 {
			t.Fatalf("start %d within threshold: want no delay, got %s", i+1, d)
		}
	}
	if slept != 0 {
		t.Fatalf("no delay should have been slept yet, got %s", slept)
	}

	// The next rapid start engages the backoff, and the injected sleep receives it.
	*fakeNow = base.Add(time.Duration(restartThrottleMax) * time.Second)
	d := ApplyStartupRestartThrottle(deps, 7890)
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

	base := time.Unix(1_700_000_000, 0)
	fakeNow := freezeClock(t, base)
	stubInstallEpoch(t, 777)

	oldSleep := daemonThrottleSleep
	defer func() { daemonThrottleSleep = oldSleep }()
	daemonThrottleSleep = func(time.Duration) {}

	deps, _ := newTestDeps(t)
	deps.Version = "9.9.9"

	// Drive rapid starts past the threshold so the backoff is engaged.
	for i := 0; i <= restartThrottleMax; i++ {
		*fakeNow = base.Add(time.Duration(i) * time.Second)
		_ = ApplyStartupRestartThrottle(deps, 7890)
	}
	*fakeNow = base.Add(time.Duration(restartThrottleMax+1) * time.Second)
	if d := ApplyStartupRestartThrottle(deps, 7890); d <= 0 {
		t.Fatalf("precondition: throttle should be engaged before the clean shutdown, got %s", d)
	}

	// A clean shutdown clears the history...
	ClearRestartHistoryOnCleanShutdown(deps, 7890)

	// ...so the very next rapid start is NOT throttled (counter reset).
	*fakeNow = base.Add(time.Duration(restartThrottleMax+2) * time.Second)
	if d := ApplyStartupRestartThrottle(deps, 7890); d != 0 {
		t.Fatalf("after a clean shutdown, a rapid restart must not be throttled, got %s", d)
	}
}

// TestClearRestartHistoryOnCleanShutdown asserts the clear removes the history file and
// is a no-op (no error/log) when the file is already absent.
func TestClearRestartHistoryOnCleanShutdown(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("KABOOM_STATE_DIR", dir)

	deps, log := newTestDeps(t)

	path := restartHistoryPath(dir)
	if err := saveRestartHistory(path, restartHistory{Version: "1.0.0", Timestamps: []int64{1, 2, 3}}); err != nil {
		t.Fatalf("saveRestartHistory error = %v", err)
	}
	ClearRestartHistoryOnCleanShutdown(deps, 7890)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("clear must remove the history file, stat err = %v", err)
	}
	// Idempotent: clearing an already-absent history is a no-op — and silent.
	ClearRestartHistoryOnCleanShutdown(deps, 7890)
	if evt := log.find("restart_history_clear_failed"); evt != nil {
		t.Fatalf("clearing an absent history must not log a failure, got %+v", *evt)
	}
}
