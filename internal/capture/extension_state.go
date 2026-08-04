// extension_state.go — Live extension state and its persisted pilot-settings cache.
// Purpose: Owns connection readiness, pilot gating, tab tracking, CSP posture, and persistence.
// Why: These fields share one lock and settings persistence snapshots the same state.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

const (
	PilotStateAssumedEnabled     = "assumed_enabled"
	PilotStateEnabled            = "enabled"
	PilotStateExplicitlyDisabled = "explicitly_disabled"

	PilotSourceAssumedStartup = "assumed_startup"
	PilotSourceExtensionSync  = "extension_sync"
	PilotSourceSettingsCache  = "settings_cache"

	SecurityModeNormal        = "normal"
	SecurityModeInsecureProxy = "insecure_proxy"
)

// ExtensionState tracks all extension-related state: connection, pilot, tracking, and test boundaries.
// It is private storage protected by ExtensionRuntime.mu.
//
// Invariants:
// - trackingEnabled implies trackedTabID > 0 for authoritative single-tab mode.
// - lastSyncSeen.IsZero() means extension has never synced in this process lifecycle.
// - missingInProgressSince tracks only commands currently pending in QueryDispatcher.
//
// Failure semantics:
//   - pilotStatusKnown=false intentionally defaults effective pilot access to enabled
//     until an authoritative sync/settings signal arrives.
type ExtensionState struct {
	// Connection tracking
	lastPollAt             time.Time // When extension last polled. Health endpoint uses 3s/5s thresholds.
	extSessionID           string    // Extension session ID (changes on reload/update).
	extSessionChangedAt    time.Time // When extSessionID last changed.
	connectionGeneration   uint64    // Monotonic daemon-owned generation for the active extension runtime.
	lastExtensionConnected bool      // Previous connection state for transition detection.
	extensionVersion       string    // Last reported extension version from sync request.
	serverVersion          string    // Daemon version used for extension compatibility checks.

	// Disconnect detection (P0-1 hardening)
	lastSyncSeen     time.Time // When last /sync request was received. Zero = never synced.
	lastSyncClientID string    // Client ID from most recent /sync request.

	// AI Web Pilot
	pilotEnabled     bool      // Last known pilot toggle from sync/settings cache.
	pilotStatusKnown bool      // False until authoritative pilot status is observed.
	pilotUpdatedAt   time.Time // When pilotEnabled was last updated.
	pilotSource      string    // Source of last authoritative pilot signal (sync/cache/test helper).

	// Tab tracking
	trackingEnabled  bool      // Single-tab mode active. true=specific tab, false=all tabs.
	trackedTabID     int       // Browser tab ID (0=none). Invariant: trackingEnabled → trackedTabID>0.
	trackedTabURL    string    // Tracked tab URL (informational, may be stale).
	trackedTabTitle  string    // Tracked tab title (informational, may be stale).
	tabStatus        string    // Chrome tab status: "loading" or "complete". Empty if unknown.
	trackedTabActive *bool     // Whether the tracked tab is the active (foreground) tab. nil=unknown.
	trackingUpdated  time.Time // When tracking status last refreshed.

	// Extension-reported active command execution state from /sync heartbeats.
	inProgress             []SyncInProgress     // Last heartbeat snapshot of active commands.
	inProgressUpdated      time.Time            // When inProgress was last refreshed.
	missingInProgressSince map[string]time.Time // First heartbeat that omitted a started command.

	// CSP detection: probed after each navigation to surface restrictions proactively.
	cspRestricted bool   // true if page CSP blocks execute_js (new Function).
	cspLevel      string // "none", "script_exec", or "page_blocked".

	// Last-resort altered-environment debug mode.
	securityMode     string   // "normal" (default) or "insecure_proxy".
	insecureRewrites []string // Rewrite set active in insecure mode (for transparent reporting).

	// Test boundaries
	activeTestIDs map[string]bool // Active test boundary IDs. Used to tag events during ingestion.
}

// ExtensionRuntime owns live browser-extension state and its synchronization.
type ExtensionRuntime struct {
	mu               sync.RWMutex
	state            ExtensionState
	connectionNotify chan struct{}
}

func newExtensionRuntime() *ExtensionRuntime {
	return &ExtensionRuntime{
		connectionNotify: make(chan struct{}),
		state: ExtensionState{
			activeTestIDs:          make(map[string]bool),
			missingInProgressSince: make(map[string]time.Time),
			pilotSource:            PilotSourceAssumedStartup,
			securityMode:           SecurityModeNormal,
		},
	}
}

// ExtensionSnapshot contains a point-in-time view of extension state for health reporting.
type ExtensionSnapshot struct {
	LastPollAt           time.Time
	ExtSessionID         string
	ExtSessionChangedAt  time.Time
	ConnectionGeneration uint64
	PilotEnabled         bool
	ActiveTestIDCount    int
}

func (r *ExtensionRuntime) Snapshot() ExtensionSnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return ExtensionSnapshot{
		LastPollAt:           r.state.lastPollAt,
		ExtSessionID:         r.state.extSessionID,
		ExtSessionChangedAt:  r.state.extSessionChangedAt,
		ConnectionGeneration: r.state.connectionGeneration,
		PilotEnabled:         r.state.pilotEnabled,
		ActiveTestIDCount:    len(r.state.activeTestIDs),
	}
}

func (r *ExtensionRuntime) ClearTestBoundaries() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.state.activeTestIDs = make(map[string]bool)
}

func (r *ExtensionRuntime) SetExtensionVersion(version string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.state.extensionVersion = version
}

// SetServerVersion stores the daemon version used for extension compatibility checks.
func (r *ExtensionRuntime) SetServerVersion(version string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.state.serverVersion = version
}

// ServerVersion returns the daemon version used for sync responses.
func (r *ExtensionRuntime) ServerVersion() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.state.serverVersion
}

func (r *ExtensionRuntime) Disconnected() (neverSynced bool, disconnected bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	neverSynced = r.state.lastSyncSeen.IsZero()
	return neverSynced, !neverSynced && !extensionStateConnected(r.state, time.Now())
}

func extensionStateConnected(state ExtensionState, now time.Time) bool {
	return state.lastExtensionConnected &&
		!state.lastSyncSeen.IsZero() &&
		now.Sub(state.lastSyncSeen) < extensionDisconnectThreshold
}

type pilotStatusSnapshot struct {
	ConfiguredEnabled bool
	EffectiveEnabled  bool
	Authoritative     bool
	State             string
	Source            string
}

// WaitForExtensionConnected blocks until the extension connects, the timeout
// elapses, or ctx is cancelled. Connection transitions close and rotate a
// generation channel under the state lock, preventing missed wakeups.
// A zero or negative timeout performs one state check without waiting.
func (r *ExtensionRuntime) WaitForExtensionConnected(ctx context.Context, timeout time.Duration) bool {
	connected, notify := r.connectionReadinessSnapshot()
	if connected {
		return true
	}
	if timeout <= 0 {
		return false
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return false
		case <-timer.C:
			return false
		case <-notify:
			connected, notify = r.connectionReadinessSnapshot()
			if connected {
				return true
			}
		}
	}
}

func (r *ExtensionRuntime) connectionReadinessSnapshot() (bool, <-chan struct{}) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return extensionStateConnected(r.state, time.Now()), r.connectionNotify
}

func (r *ExtensionRuntime) signalConnectionChangeLocked() {
	close(r.connectionNotify)
	r.connectionNotify = make(chan struct{})
}

// IsExtensionConnected returns true if the extension has synced within the
// disconnect threshold (10s). Returns false if never synced or stale.
func (r *ExtensionRuntime) IsExtensionConnected() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return extensionStateConnected(r.state, time.Now())
}

// MarkDisconnected records an authoritative transport loss while preserving
// the last settings snapshot for diagnostics and reconnect recovery.
func (r *ExtensionRuntime) MarkDisconnected() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.state.lastExtensionConnected {
		return
	}
	r.state.lastExtensionConnected = false
	r.signalConnectionChangeLocked()
}

// GetExtensionStatus returns a detached connection snapshot.
// Fields: connected (bool), last_seen (RFC3339 string), client_id (string).
//
// Failure semantics:
// - If extension has never synced, last_seen is empty and connected=false.
func (r *ExtensionRuntime) GetExtensionStatus() map[string]any {
	r.mu.RLock()
	defer r.mu.RUnlock()

	connected := extensionStateConnected(r.state, time.Now())

	lastSeen := ""
	if !r.state.lastSyncSeen.IsZero() {
		lastSeen = r.state.lastSyncSeen.Format(time.RFC3339)
	}

	return map[string]any{
		"connected": connected,
		"last_seen": lastSeen,
		"client_id": r.state.lastSyncClientID,
	}
}

// GetVersionMismatch checks whether extension and server versions differ in major.minor.
// Returns the extension version, server version, and whether a mismatch exists.
// A mismatch is detected only when the extension has reported a version (non-empty)
// and the major.minor portions differ from the server version.
func (r *ExtensionRuntime) VersionMismatch() (extensionVersion string, serverVersion string, hasMismatch bool) {
	r.mu.RLock()
	extVer := r.state.extensionVersion
	srvVer := r.state.serverVersion
	r.mu.RUnlock()

	if extVer == "" || srvVer == "" {
		return extVer, srvVer, false
	}

	extMajorMinor := majorMinor(extVer)
	srvMajorMinor := majorMinor(srvVer)
	if extMajorMinor == "" || srvMajorMinor == "" {
		return extVer, srvVer, false
	}

	return extVer, srvVer, extMajorMinor != srvMajorMinor
}

func (r *ExtensionRuntime) ExtensionVersion() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.state.extensionVersion
}

// majorMinor extracts "X.Y" from a semver string "X.Y.Z".
// Returns empty string if the version is not in a recognized format.
func majorMinor(v string) string {
	firstDot := -1
	for i := 0; i < len(v); i++ {
		if v[i] == '.' {
			if firstDot == -1 {
				firstDot = i
			} else {
				// Found second dot — return up to (but not including) it
				return v[:i]
			}
		}
	}
	// No second dot found — not a valid semver with patch
	if firstDot != -1 {
		return v // "X.Y" format, return as-is
	}
	return ""
}

// IsPilotEnabled returns whether AI Web Pilot is currently enabled.
func (r *ExtensionRuntime) IsPilotEnabled() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.state.pilotEnabled
}

// IsPilotActionAllowed returns whether pilot-gated actions should be allowed.
// Startup/reconnect uncertainty defaults to allowed until explicit disable arrives.
func (r *ExtensionRuntime) IsPilotActionAllowed() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	snap := pilotStatusSnapshotFromExtensionState(r.state)
	return snap.EffectiveEnabled
}

// GetPilotStatus returns pilot and heartbeat command status.
// extension_connected uses the same threshold as IsExtensionConnected (10s on lastSyncSeen).
// extension_last_seen is the RFC3339 timestamp of the last /sync, empty if never synced.
//
// Invariants:
// - Returned in_progress slice is copied to prevent external mutation.
func (r *ExtensionRuntime) GetPilotStatus() any {
	r.mu.RLock()
	defer r.mu.RUnlock()
	snap := pilotStatusSnapshotFromExtensionState(r.state)

	lastSeen := ""
	if !r.state.lastSyncSeen.IsZero() {
		lastSeen = r.state.lastSyncSeen.Format(time.RFC3339)
	}

	inProgress := make([]SyncInProgress, len(r.state.inProgress))
	copy(inProgress, r.state.inProgress)
	inProgressUpdated := ""
	if !r.state.inProgressUpdated.IsZero() {
		inProgressUpdated = r.state.inProgressUpdated.Format(time.RFC3339)
	}

	return map[string]any{
		"enabled":             snap.EffectiveEnabled,
		"configured_enabled":  snap.ConfiguredEnabled,
		"authoritative":       snap.Authoritative,
		"state":               snap.State,
		"source":              snap.Source,
		"extension_connected": extensionStateConnected(r.state, time.Now()),
		"extension_last_seen": lastSeen,
		"in_progress_count":   len(inProgress),
		"in_progress":         inProgress,
		"in_progress_updated": inProgressUpdated,
	}
}

// GetInProgressCommands returns a copy of latest extension-reported active commands.
//
// Failure semantics:
// - Returns empty slice when no heartbeat data is available.
func (r *ExtensionRuntime) GetInProgressCommands() []SyncInProgress {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]SyncInProgress, len(r.state.inProgress))
	copy(out, r.state.inProgress)
	return out
}

// pilotStatusSnapshotFromExtensionState converts raw extension state to API-level pilot semantics.
//
// Failure semantics:
// - Unknown/unset source fields fall back to conservative defaults.
func pilotStatusSnapshotFromExtensionState(ext ExtensionState) pilotStatusSnapshot {
	snap := pilotStatusSnapshot{
		ConfiguredEnabled: ext.pilotEnabled,
		Authoritative:     ext.pilotStatusKnown,
	}

	if !ext.pilotStatusKnown {
		snap.EffectiveEnabled = true
		snap.State = PilotStateAssumedEnabled
		snap.Source = PilotSourceAssumedStartup
		return snap
	}

	if ext.pilotEnabled {
		snap.EffectiveEnabled = true
		snap.State = PilotStateEnabled
		if ext.pilotSource != "" {
			snap.Source = ext.pilotSource
		} else {
			snap.Source = PilotSourceExtensionSync
		}
		return snap
	}

	snap.EffectiveEnabled = false
	snap.State = PilotStateExplicitlyDisabled
	if ext.pilotSource != "" {
		snap.Source = ext.pilotSource
	} else {
		snap.Source = PilotSourceExtensionSync
	}
	return snap
}

// GetTrackingStatus returns the current tab tracking state.
func (r *ExtensionRuntime) GetTrackingStatus() (enabled bool, tabID int, tabURL string) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.state.trackingEnabled, r.state.trackedTabID, r.state.trackedTabURL
}

// UpdateTrackedTab programmatically updates the tracked tab state.
// Used by switch_tab to retarget subsequent commands to the newly activated tab.
//
// Invariants:
// - tabID must be > 0; zero/negative values are silently ignored.
// - trackingEnabled is set to true when a valid tabID is provided.
func (r *ExtensionRuntime) UpdateTrackedTab(tabID int, tabURL string, tabTitle string) {
	if tabID <= 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.state.trackingEnabled = true
	r.state.trackedTabID = tabID
	r.state.trackedTabURL = tabURL
	r.state.trackedTabTitle = tabTitle
	r.state.trackingUpdated = time.Now()
}

// GetTrackedTabTitle returns the tracked tab's title (may be stale).
func (r *ExtensionRuntime) GetTrackedTabTitle() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.state.trackedTabTitle
}

// GetTabStatus returns the Chrome tab status ("loading", "complete", or empty if unknown).
func (r *ExtensionRuntime) GetTabStatus() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.state.tabStatus
}

// IsTrackedTabActive returns whether the tracked tab is the foreground tab.
// Returns (active, known). known=false means the extension has not reported this yet.
func (r *ExtensionRuntime) IsTrackedTabActive() (bool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.state.trackedTabActive == nil {
		return false, false
	}
	return *r.state.trackedTabActive, true
}

// GetCSPStatus returns the last reported CSP restriction level for the tracked page.
func (r *ExtensionRuntime) GetCSPStatus() (restricted bool, level string) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.state.cspRestricted, r.state.cspLevel
}

// CSPBlockedActions returns the actions blocked by the given CSP level and a
// human-readable reason string. When the level is "none" or unrecognized, both
// return values are nil/"" — callers should omit them from the response entirely
// to avoid wasting tokens on normal pages.
func CSPBlockedActions(level string) (actions []string, reason string) {
	switch level {
	case "script_exec":
		return []string{"execute_js"},
			"Page CSP blocks dynamic script execution. Use dom, get_readable, or list_interactive instead."
	case "page_blocked":
		return []string{
				"execute_js", "click", "type", "select", "check", "scroll_to", "focus",
				"get_text", "get_value", "get_attribute", "set_attribute",
				"list_interactive", "get_readable", "get_markdown",
				"fill_form", "fill_form_and_submit",
			},
			"Page blocks all script injection. Only navigate, screenshot, and network observation available."
	default:
		return nil, ""
	}
}

// SetSecurityMode updates altered-environment mode reported to callers.
// mode values: normal (default), insecure_proxy.
//
// Invariants:
// - Any non-insecure mode value normalizes to SecurityModeNormal.
// - Rewrite slice is copied on write to avoid external aliasing.
func (r *ExtensionRuntime) SetSecurityMode(mode string, rewrites []string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	switch mode {
	case SecurityModeInsecureProxy:
		r.state.securityMode = SecurityModeInsecureProxy
		r.state.insecureRewrites = append([]string(nil), rewrites...)
	default:
		r.state.securityMode = SecurityModeNormal
		r.state.insecureRewrites = nil
	}
}

// GetSecurityMode returns current altered-environment mode and rewrite set.
// production_parity is true only in normal mode.
//
// Invariants:
// - Returned rewrite slice is copied and safe for caller mutation.
func (r *ExtensionRuntime) GetSecurityMode() (mode string, productionParity bool, rewrites []string) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	mode = r.state.securityMode
	if mode == "" {
		mode = SecurityModeNormal
	}
	productionParity = mode == SecurityModeNormal
	rewrites = append([]string(nil), r.state.insecureRewrites...)
	return mode, productionParity, rewrites
}

// GetActiveTestIDs returns the list of currently active test IDs.
func (r *ExtensionRuntime) GetActiveTestIDs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]string, 0, len(r.state.activeTestIDs))
	for testID := range r.state.activeTestIDs {
		result = append(result, testID)
	}
	return result
}

// SetTestBoundaryStart marks a test boundary as active for future event tagging.
//
// Invariants:
// - activeTestIDs behaves as a set (idempotent insert).
func (r *ExtensionRuntime) SetTestBoundaryStart(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.state.activeTestIDs[id] = true
}

// SetTestBoundaryEnd clears a test boundary marker.
//
// Failure semantics:
// - Deleting unknown IDs is a no-op.
func (r *ExtensionRuntime) SetTestBoundaryEnd(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.state.activeTestIDs, id)
}

type PersistedSettings struct {
	AIWebPilotEnabled *bool     `json:"ai_web_pilot_enabled,omitempty"`
	Timestamp         time.Time `json:"timestamp"`
	ExtSessionID      string    `json:"ext_session_id"`
}

func getSettingsPath() (string, error) {
	return state.SettingsFile()
}

func readSettingsData() ([]byte, error) {
	path, err := getSettingsPath()
	if err != nil {
		return nil, fmt.Errorf("could not determine settings path: %w", err)
	}

	// #nosec G304 -- path is resolved from trusted runtime state, not user input.
	data, err := os.ReadFile(path)
	if err == nil {
		return data, nil
	}
	if os.IsNotExist(err) {
		return nil, nil
	}
	return nil, fmt.Errorf("could not read settings file: %w", err)
}

// LoadSettingsFromDisk refreshes recent pilot state from the canonical settings file.
func (r *ExtensionRuntime) LoadSettingsFromDisk() {
	data, err := readSettingsData()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[Kaboom] %v\n", err)
		return
	}
	if data == nil {
		return
	}

	var settings PersistedSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		fmt.Fprintf(os.Stderr, "[Kaboom] Could not parse settings file: %v\n", err)
		return
	}

	if time.Since(settings.Timestamp) > 5*time.Second {
		return
	}
	if settings.AIWebPilotEnabled != nil {
		r.ApplyCachedPilot(*settings.AIWebPilotEnabled, settings.Timestamp)
	}
}

func (r *ExtensionRuntime) ApplyCachedPilot(enabled bool, updatedAt time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.state.pilotEnabled = enabled
	r.state.pilotStatusKnown = true
	r.state.pilotUpdatedAt = updatedAt
	r.state.pilotSource = PilotSourceSettingsCache
}
