// bridge_startup_state.go -- Daemon startup state and respawn synchronization for bridge fast-start.

package bridge

import (
	"fmt"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/telemetry"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// daemonState tracks the state of daemon startup for fast-start mode.
// Supports respawning: if the daemon dies mid-session, the bridge detects
// connection errors and re-launches the daemon transparently.
type daemonState struct {
	runner    *Runner
	ready     bool
	failed    bool
	err       string
	mu        sync.Mutex
	readyCh   chan struct{}
	failedCh  chan struct{}
	readySig  bool
	failedSig bool

	// Spawn config — set once at startup, read-only after.
	port       int
	logFile    string
	maxEntries int

	// Respawn throttle state. Bounds how often the real spawn (cmd.Start) can fire
	// so a repeatedly-dying daemon cannot become a launchd-throttling spawn storm.
	// Guarded by respawnMu (independent of mu, which markReady/markFailed take).
	respawnMu       sync.Mutex
	lastRespawnAt   time.Time
	respawnInterval time.Duration
}

// bridgeNow is the injectable clock for respawn throttling (mirrors daemonNow in
// the main package). Overridden in tests for deterministic backoff assertions.
var bridgeNow = time.Now

const (
	// respawnMinInterval is the minimum gap between real daemon spawns. The first
	// respawn after a death is immediate; only rapid repeats are throttled.
	respawnMinInterval = 1 * time.Second
	// respawnMaxBackoff caps the exponential backoff between rapid respawns and
	// also marks when a storm has "cooled" (a respawn this long after the last is
	// treated as isolated and resets the escalation).
	respawnMaxBackoff = 8 * time.Second
)

// respawnSpawnFn performs the actual daemon (re)spawn: build the command, start it
// detached, and wait for readiness. Injectable so tests can count/stub real spawns
// without launching processes.
var respawnSpawnFn = performDaemonRespawn

// reserveRespawnSlot reports whether a real daemon spawn is permitted at `now`, and
// if so records the attempt and advances the (capped, exponential) backoff. It never
// blocks: a throttled caller is told "no" and backs off instead of hammering
// cmd.Start(). The first respawn after a death is always granted; only rapid repeats
// are throttled. A respawn arriving after a full cool-down (>= respawnMaxBackoff since
// the last) is treated as isolated and resets the escalation, so a lone respawn is
// never penalized.
func (s *daemonState) reserveRespawnSlot(now time.Time) bool {
	s.respawnMu.Lock()
	defer s.respawnMu.Unlock()

	if s.lastRespawnAt.IsZero() {
		s.lastRespawnAt = now
		s.respawnInterval = respawnMinInterval
		return true
	}
	elapsed := now.Sub(s.lastRespawnAt)
	if elapsed >= respawnMaxBackoff {
		// Storm has cooled — treat as a fresh, isolated respawn.
		s.lastRespawnAt = now
		s.respawnInterval = respawnMinInterval
		return true
	}
	if elapsed < s.respawnInterval {
		return false // too soon; throttle
	}
	// Interval satisfied but still inside the storm window: permit and escalate.
	s.lastRespawnAt = now
	s.respawnInterval *= 2
	if s.respawnInterval > respawnMaxBackoff {
		s.respawnInterval = respawnMaxBackoff
	}
	return true
}

// performDaemonRespawn is the default (real) spawn implementation behind respawnSpawnFn.
func performDaemonRespawn(s *daemonState) bool {
	cmd, err := s.buildDaemonCmd()
	if err != nil {
		s.markFailed(err.Error())
		return false
	}
	if err := cmd.Start(); err != nil {
		s.markFailed("Failed to start daemon: " + err.Error())
		return false
	}
	if s.runner.WaitForServer(s.port, daemonStartupReadyTimeout) {
		s.markReady()
		s.runner.transport.Stderrf("[Kaboom] daemon respawned successfully on port %d\n", s.port)
		return true
	}
	s.markFailed(fmt.Sprintf("Daemon respawned but not responding on port %d after %s", s.port, daemonStartupReadyTimeout))
	return false
}

type respawnPlan struct {
	alreadyReady bool
	waitForPeer  bool
	readyCh      <-chan struct{}
	failedCh     <-chan struct{}
}

type peerSignalWaitResult struct {
	ready    bool
	failed   bool
	timedOut bool
}

// resetSignalsLocked replaces readiness/failure channels for a fresh spawn cycle.
// Caller must hold s.mu.
func (s *daemonState) resetSignalsLocked() {
	s.readyCh = make(chan struct{})
	s.failedCh = make(chan struct{})
	s.readySig = false
	s.failedSig = false
}

// markReady atomically marks the daemon as ready and closes readyCh once.
func (s *daemonState) markReady() {
	readyCh, shouldClose := s.setReadyState()
	if shouldClose {
		close(readyCh)
	}
}

func (s *daemonState) setReadyState() (readyCh chan struct{}, shouldClose bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ready = true
	s.failed = false
	s.err = ""
	readyCh = s.readyCh
	shouldClose = !s.readySig
	if shouldClose {
		s.readySig = true
	}
	return readyCh, shouldClose
}

// markFailed atomically marks the daemon state as failed and closes failedCh once.
func (s *daemonState) markFailed(errMsg string) {
	failedCh, shouldClose := s.setFailedState(errMsg)
	if shouldClose {
		close(failedCh)
	}
}

func (s *daemonState) setFailedState(errMsg string) (failedCh chan struct{}, shouldClose bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ready = false
	s.failed = true
	s.err = errMsg
	failedCh = s.failedCh
	shouldClose = !s.failedSig
	if shouldClose {
		s.failedSig = true
	}
	return failedCh, shouldClose
}

func (s *daemonState) planRespawnAttempt() respawnPlan {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Already responsive? Quick health check to confirm.
	if s.ready && s.runner.IsServerRunning(s.port) {
		return respawnPlan{alreadyReady: true}
	}

	// Already respawning (channels still open from a concurrent call)?
	// Wait on readyCh/failedCh without spawning again.
	if !s.ready && !s.failed {
		return respawnPlan{
			waitForPeer: true,
			readyCh:     s.readyCh,
			failedCh:    s.failedCh,
		}
	}

	// Reset state for new spawn attempt.
	s.ready = false
	s.failed = false
	s.err = ""
	s.resetSignalsLocked()
	return respawnPlan{}
}

func waitForRespawnPeerSignals(plan respawnPlan, timeout time.Duration) peerSignalWaitResult {
	if timeout <= 0 {
		timeout = daemonStartupReadyTimeout
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-plan.readyCh:
		return peerSignalWaitResult{ready: true}
	case <-plan.failedCh:
		return peerSignalWaitResult{failed: true}
	case <-timer.C:
		return peerSignalWaitResult{timedOut: true}
	}
}

func (s *daemonState) reclaimRespawnLeadership(expectedReady <-chan struct{}, expectedFailed <-chan struct{}) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ready {
		return false
	}
	if !s.failed && (s.readyCh != expectedReady || s.failedCh != expectedFailed) {
		return false
	}

	s.ready = false
	s.failed = false
	s.err = ""
	s.resetSignalsLocked()
	return true
}

// respawnIfNeeded re-launches the daemon if it's not responding.
// Safe to call from multiple goroutines — only one respawn runs at a time.
// Returns true if the daemon is ready after the respawn attempt.
func (s *daemonState) respawnIfNeeded() bool {
	for {
		plan := s.planRespawnAttempt()
		if plan.alreadyReady {
			return true
		}
		if !plan.waitForPeer {
			break
		}

		waitResult := waitForRespawnPeerSignals(plan, daemonStartupReadyTimeout)
		if waitResult.ready {
			return true
		}
		if waitResult.failed {
			return false
		}
		if waitResult.timedOut {
			if s.reclaimRespawnLeadership(plan.readyCh, plan.failedCh) {
				break
			}
			// Another goroutine changed state while this caller was waiting; re-plan.
			continue
		}
	}

	if s.port <= 0 {
		s.markFailed("respawn requested without a valid daemon port")
		return false
	}

	// Bounded respawn backoff: never fire cmd.Start() faster than the (capped,
	// exponential) throttle interval, so a repeatedly-dying daemon cannot become a
	// spawn storm that launchd throttles into oblivion. Fail loud when throttled —
	// never silently drop the request. If a prior spawn actually did come up while
	// we were being throttled, adopt it as ready instead of reporting failure.
	if !s.reserveRespawnSlot(bridgeNow()) {
		if s.runner.IsServerRunning(s.port) {
			s.markReady()
			return true
		}
		s.markFailed(fmt.Sprintf("daemon respawn throttled on port %d; backing off to avoid a restart storm", s.port))
		return false
	}

	s.runner.transport.Stderrf("[Kaboom] daemon not responding, respawning on port %d\n", s.port)
	return respawnSpawnFn(s)
}

func spawnDaemonAsync(state *daemonState) {
	// Spawn daemon in background (don't block on it)
	util.SafeGo(func() {
		cmd, err := state.buildDaemonCmd()
		if err != nil {
			telemetry.AppError("bridge_spawn_build_error")
			state.markFailed(err.Error())
			return
		}
		if err := cmd.Start(); err != nil {
			telemetry.AppError("bridge_spawn_start_error")
			state.markFailed("Failed to start daemon: " + err.Error())
			return
		}

		// Wait for server to be ready (bounded startup budget).
		if state.runner.WaitForServer(state.port, daemonStartupReadyTimeout) {
			state.markReady()
		} else {
			telemetry.AppError("bridge_spawn_timeout")
			state.markFailed(fmt.Sprintf("Daemon started but not responding on port %d after %s", state.port, daemonStartupReadyTimeout))
		}
	})
}

func waitForDaemonReadinessSignal(state *daemonState, timeout time.Duration) (ready bool, failed bool) {
	if timeout <= 0 {
		return false, false
	}
	readyCh, failedCh := func() (chan struct{}, chan struct{}) {
		state.mu.Lock()
		defer state.mu.Unlock()
		return state.readyCh, state.failedCh
	}()
	if readyCh == nil || failedCh == nil {
		return false, false
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-readyCh:
		return true, false
	case <-failedCh:
		return false, true
	case <-timer.C:
		return false, false
	}
}
