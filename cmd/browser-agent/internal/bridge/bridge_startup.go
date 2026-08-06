// bridge_startup.go — Coordinates daemon discovery, startup, and readiness.
// Why: health/version probes and startup policy change together. Cross-process
// lock persistence is owned separately by startuplock; the respawn state machine
// remains in bridge_startup_state.go.
// Docs: docs/features/feature/lazy-server-start/index.md

package bridge

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/bridge/startuplock"
	internbridge "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/bridge"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/incident"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/telemetry"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// daemonStartupGracePeriod is a short wait window for first tool calls so
// clients don't fail on daemon boot races.
var daemonStartupGracePeriod = 2 * time.Second

type healthMetadata struct {
	Version string `json:"version"`
	Name    string `json:"name"`
}

func decodeHealthMetadata(body []byte) (healthMetadata, bool) {
	var metadata healthMetadata
	if json.Unmarshal(body, &metadata) != nil {
		return healthMetadata{}, false
	}
	return metadata, true
}

func (m healthMetadata) serviceName() string {
	return strings.TrimSpace(m.Name)
}

func versionsMatch(left, right string) bool {
	normalize := func(version string) string {
		return strings.TrimPrefix(strings.TrimSpace(version), "v")
	}
	return normalize(left) == normalize(right)
}

// daemonStartupReadyTimeout bounds how long a bridge waits for a spawned daemon
// to report healthy before treating the attempt as failed.
var daemonStartupReadyTimeout = 2 * time.Second

// daemonPeerWaitTimeout is the follower wait budget while another bridge is
// expected to finish daemon startup under contention.
var daemonPeerWaitTimeout = 2 * time.Second

// daemonPeerPollInterval controls peer readiness polling cadence.
var daemonPeerPollInterval = 100 * time.Millisecond

// daemonPeerFallbackWaitTimeout adds a final short wait when another bridge
// still owns the startup lock but has not surfaced readiness yet.
var daemonPeerFallbackWaitTimeout = 250 * time.Millisecond

// daemonStartupLockStaleAfter defines when a startup lock is considered stale
// and can be reclaimed by another bridge.
var daemonStartupLockStaleAfter = 2 * time.Second

// IsServerRunning checks the daemon health endpoint.
func (r *Runner) IsServerRunning(port int) bool {
	return internbridge.IsServerRunning(port)
}

func (r *Runner) runningServerVersionCompatible(port int) (bool, string, string) {
	client := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/health", port)) // #nosec G704 -- localhost-only health probe
	if err != nil {
		return false, "", ""
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return false, "", ""
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 16*1024))
	if err != nil {
		return false, "", ""
	}

	meta, ok := decodeHealthMetadata(body)
	if !ok {
		return false, "", ""
	}

	serviceName := meta.serviceName()
	if serviceName != r.identity.ServerName {
		return false, strings.TrimSpace(meta.Version), serviceName
	}

	runningVersion := strings.TrimSpace(meta.Version)
	if runningVersion == "" {
		return false, "<missing>", serviceName
	}
	return versionsMatch(runningVersion, r.identity.Version), runningVersion, serviceName
}

// WaitForServer waits until the daemon health endpoint responds.
func (r *Runner) WaitForServer(port int, timeout time.Duration) bool {
	return internbridge.WaitForServer(port, timeout)
}

func daemonStartupSuggestion(failErr string, port int) string {
	suggestion := fmt.Sprintf("Server failed to start: %s. ", failErr)
	if strings.Contains(failErr, "port") || strings.Contains(failErr, "bind") || strings.Contains(failErr, "address") {
		suggestion += fmt.Sprintf("Port may be in use. Try: npx kaboom-agentic-browser --port %d", port+1)
	} else {
		suggestion += "Try: npx kaboom-agentic-browser --doctor"
	}
	return suggestion
}

func daemonStatusSnapshot(state *daemonState) (ready bool, failed bool, err string) {
	state.mu.Lock()
	defer state.mu.Unlock()
	return state.ready, state.failed, state.err
}

// DaemonFailureErr returns the current error message from daemon state.
func DaemonFailureErr(state *daemonState) string {
	state.mu.Lock()
	defer state.mu.Unlock()
	return state.err
}

func healDaemonReadyStateIfRunning(state *daemonState, isReady bool, isFailed bool) bool {
	// Only run this check when daemon state has a concrete port (state.port > 0)
	// to avoid test and fast-path false positives from unrelated local daemons.
	if state.port <= 0 || !state.runner.IsServerRunning(state.port) {
		return false
	}
	// Heal stale bridge state: daemon is up but local ready flag drifted.
	if !isReady || isFailed {
		state.mu.Lock()
		defer state.mu.Unlock()
		state.ready = true
		state.failed = false
		state.err = ""
	}
	return true
}

// checkDaemonStatus returns an error string if the daemon is not ready, or "" if ready.
func checkDaemonStatus(state *daemonState, req mcp.JSONRPCRequest, port int) string {
	// Validate method requires daemon
	if req.Method != "tools/call" && !strings.HasPrefix(req.Method, "tools/") && !strings.HasPrefix(req.Method, "resources/") {
		return "method_not_found"
	}

	isReady, isFailed, failErr := daemonStatusSnapshot(state)

	if healDaemonReadyStateIfRunning(state, isReady, isFailed) {
		return ""
	}

	if isFailed {
		// Previous spawn failed — try again before giving up.
		if state.respawnIfNeeded() {
			return ""
		}
		return daemonStartupSuggestion(failErr, port)
	}

	if !isReady {
		readySignal, failedSignal := waitForDaemonReadinessSignal(state, daemonStartupGracePeriod)
		if readySignal {
			return ""
		}
		if failedSignal {
			failErr = DaemonFailureErr(state)
			if state.respawnIfNeeded() {
				return ""
			}
			return daemonStartupSuggestion(failErr, port)
		}

		// Grace period elapsed: re-check daemon health once before returning startup retry.
		if state.port > 0 && state.runner.IsServerRunning(state.port) {
			state.markReady()
			return ""
		}
		return "starting"
	}
	return ""
}

// RunMode bridges stdio (from MCP client) to HTTP (to persistent server)
// Uses fast-start: responds to initialize/tools/list immediately while spawning daemon async.
// #lizard forgives
func (r *Runner) RunMode(port int, logFile string, maxEntries int) {
	serverURL := fmt.Sprintf("http://127.0.0.1:%d", port)

	// Track daemon state with proper failure handling
	state := &daemonState{runner: r,
		readyCh:    make(chan struct{}),
		failedCh:   make(chan struct{}),
		port:       port,
		logFile:    logFile,
		maxEntries: maxEntries,
	}

	shouldSpawn := true

	// Phase 1: Check if a compatible server is already running.
	if r.tryConnectToExisting(state, port) {
		shouldSpawn = false
	}

	// Phase 2: No server found. Wait for a peer bridge to finish spawning
	// before we start our own daemon (avoids multi-bridge spawn races).
	//
	// IMPORTANT: This wait must not block MCP stdio startup. The bridge read loop
	// must begin immediately so initialize/tools/list fast-path responses are not
	// delayed during cold start.
	if shouldSpawn {
		r.startDaemonSpawnCoordinator(state, port)
	}

	// Bridge stdio <-> HTTP with fast-start support
	r.StdioToHTTPFast(serverURL+"/mcp", state, port)
}

// startDaemonSpawnCoordinator runs peer-wait/spawn policy asynchronously so MCP
// stdio handling can start immediately.
func (r *Runner) startDaemonSpawnCoordinator(state *daemonState, port int) {
	util.SafeGo(func() {
		if r.coordinateDaemonStartup(state, port) {
			return
		}
		spawnDaemonAsync(state)
	})
}

func (r *Runner) coordinateDaemonStartup(state *daemonState, port int) bool {
	locks := startuplock.NewManager(r.identity.Version, r.lifecycle.IsProcessAlive)
	lock, acquired, err := locks.Acquire(port)
	if err != nil {
		// Coordination failed (state dir/lock issue). Fall back to local spawn.
		return false
	}
	if acquired {
		r.startAsStartupLeader(state, port, lock)
		return true
	}

	// Another bridge owns startup leadership. Give it time to bring the daemon up.
	if r.waitForPeerDaemon(state, port) {
		return true
	}

	// Leader appears stalled. Reclaim stale/dead lock and try to take over.
	_ = locks.ClearStale(port, daemonStartupLockStaleAfter)
	lock, acquired, err = locks.Acquire(port)
	if err != nil {
		return false
	}
	if acquired {
		r.startAsStartupLeader(state, port, lock)
		return true
	}

	// Last short wait in case leader just completed while lock handoff converges.
	return r.waitForPeerDaemonWithin(state, port, daemonPeerFallbackWaitTimeout)
}

func (r *Runner) startAsStartupLeader(state *daemonState, port int, lock *startuplock.Lock) {
	if lock != nil {
		defer lock.Release()
	}
	if r.tryConnectToExisting(state, port) {
		return
	}
	spawnDaemonAsync(state)
	// Hold leadership until this spawn attempt resolves so followers don't stampede.
	if ready, failed := waitForDaemonReadinessSignal(state, daemonStartupReadyTimeout+daemonPeerPollInterval); ready || failed {
		return
	}
	if r.IsServerRunning(state.port) {
		state.markReady()
	}
}

// tryConnectToExisting checks for a running server and validates compatibility.
// Returns true if connected (markReady), or fatally blocked (markFailed) — no point retrying.
// Returns false if no server is running or the port was freed for a new spawn.
func (r *Runner) tryConnectToExisting(state *daemonState, port int) bool {
	if !r.IsServerRunning(port) {
		return false
	}
	compatible, runningVersion, serviceName := r.runningServerVersionCompatible(port)
	if compatible {
		state.markReady()
		return true
	}
	if serviceName == r.identity.ServerName {
		// Version mismatch — stop old server, let caller spawn new one.
		if !r.lifecycle.StopServerForUpgrade(port) {
			state.markFailed(fmt.Sprintf("found running daemon version %s but could not recycle it", runningVersion))
			return true // fatally blocked, don't retry/spawn
		}
		return false // port freed, caller should spawn
	}
	// Non-kaboom service occupies the port.
	if serviceName == "" {
		serviceName = "unknown"
	}
	telemetry.AppError(incident.CodeBridgePortBlocked)
	state.markFailed(fmt.Sprintf("port %d is occupied by non-kaboom service %q", port, serviceName))
	return true // fatally blocked
}

// waitForPeerDaemon retries connecting to a server that another bridge may be spawning.
// Returns true if a compatible server appeared before the follower wait budget expires.
func (r *Runner) waitForPeerDaemon(state *daemonState, port int) bool {
	return r.waitForPeerDaemonWithin(state, port, daemonPeerWaitTimeout)
}

func (r *Runner) waitForPeerDaemonWithin(state *daemonState, port int, timeout time.Duration) bool {
	if timeout <= 0 {
		return r.tryConnectToExisting(state, port)
	}
	deadline := time.Now().Add(timeout)
	for {
		if r.tryConnectToExisting(state, port) {
			return true
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return false
		}
		sleepFor := daemonPeerPollInterval
		if remaining < sleepFor {
			sleepFor = remaining
		}
		r.sleep(sleepFor)
	}
}
