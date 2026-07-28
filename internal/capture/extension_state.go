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
	"path/filepath"
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
	PilotSourceTestHelper     = "test_helper"

	SecurityModeNormal        = "normal"
	SecurityModeInsecureProxy = "insecure_proxy"
)

// ExtensionState tracks all extension-related state: connection, pilot, tracking, and test boundaries.
// Protected by parent Capture.mu (no separate lock) because activeTestIDs is read
// during hot-path event ingestion (AddWebSocketEvents, AddNetworkBodies, AddEnhancedActions).
//
// Invariants:
// - trackingEnabled implies trackedTabID > 0 for authoritative single-tab mode.
// - lastSyncSeen.IsZero() means extension has never synced in this process lifecycle.
// - missingInProgressByCorr tracks only commands currently pending in QueryDispatcher.
//
// Failure semantics:
//   - pilotStatusKnown=false intentionally defaults effective pilot access to enabled
//     until an authoritative sync/settings signal arrives.
type ExtensionState struct {
	// Connection tracking
	lastPollAt             time.Time // When extension last polled. Health endpoint uses 3s/5s thresholds.
	extSessionID           string    // Extension session ID (changes on reload/update).
	extSessionChangedAt    time.Time // When extSessionID last changed.
	lastExtensionConnected bool      // Previous connection state for transition detection.
	extensionVersion       string    // Last reported extension version from sync request.

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
	inProgress              []SyncInProgress // Last heartbeat snapshot of active commands.
	inProgressUpdated       time.Time        // When inProgress was last refreshed.
	missingInProgressByCorr map[string]int   // Consecutive missed heartbeats for started commands.

	// CSP detection: probed after each navigation to surface restrictions proactively.
	cspRestricted bool   // true if page CSP blocks execute_js (new Function).
	cspLevel      string // "none", "script_exec", or "page_blocked".

	// Last-resort altered-environment debug mode.
	securityMode     string   // "normal" (default) or "insecure_proxy".
	insecureRewrites []string // Rewrite set active in insecure mode (for transparent reporting).

	// Test boundaries
	activeTestIDs map[string]bool // Active test boundary IDs. Used to tag events during ingestion.
}

// ExtensionSnapshot contains a point-in-time view of extension state for health reporting.
type ExtensionSnapshot struct {
	LastPollAt          time.Time
	ExtSessionID        string
	ExtSessionChangedAt time.Time
	PilotEnabled        bool
	ActiveTestIDCount   int
}

type pilotStatusSnapshot struct {
	ConfiguredEnabled bool
	EffectiveEnabled  bool
	Authoritative     bool
	State             string
	Source            string
}

// WaitForExtensionConnected polls until the extension connects, the timeout elapses,
// or ctx is cancelled. Returns true if connected within the window, false otherwise.
// A zero or negative timeout returns false immediately (no polling).
// Poll interval is extensionReadinessPollInterval (200ms). ctx cancellation is
// honoured between polls.
func (c *Capture) WaitForExtensionConnected(ctx context.Context, timeout time.Duration) bool {
	if c.IsExtensionConnected() {
		return true
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ticker := time.NewTicker(extensionReadinessPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
			if c.IsExtensionConnected() {
				return true
			}
		}
	}
}

// IsExtensionConnected returns true if the extension has synced within the
// disconnect threshold (10s). Returns false if never synced or stale.
func (c *Capture) IsExtensionConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return !c.extensionState.lastSyncSeen.IsZero() && time.Since(c.extensionState.lastSyncSeen) < extensionDisconnectThreshold
}

// GetExtensionStatus returns a detached connection snapshot.
// Fields: connected (bool), last_seen (RFC3339 string), client_id (string).
//
// Failure semantics:
// - If extension has never synced, last_seen is empty and connected=false.
func (c *Capture) GetExtensionStatus() map[string]any {
	c.mu.RLock()
	defer c.mu.RUnlock()

	connected := !c.extensionState.lastSyncSeen.IsZero() && time.Since(c.extensionState.lastSyncSeen) < extensionDisconnectThreshold

	lastSeen := ""
	if !c.extensionState.lastSyncSeen.IsZero() {
		lastSeen = c.extensionState.lastSyncSeen.Format(time.RFC3339)
	}

	return map[string]any{
		"connected": connected,
		"last_seen": lastSeen,
		"client_id": c.extensionState.lastSyncClientID,
	}
}

// GetExtensionVersion returns the last reported extension version.
func (c *Capture) GetExtensionVersion() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.extensionState.extensionVersion
}

// GetVersionMismatch checks whether extension and server versions differ in major.minor.
// Returns the extension version, server version, and whether a mismatch exists.
// A mismatch is detected only when the extension has reported a version (non-empty)
// and the major.minor portions differ from the server version.
func (c *Capture) GetVersionMismatch() (extensionVersion string, serverVersion string, hasMismatch bool) {
	c.mu.RLock()
	extVer := c.extensionState.extensionVersion
	srvVer := c.serverVersion
	c.mu.RUnlock()

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
func (c *Capture) IsPilotEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.extensionState.pilotEnabled
}

// IsPilotActionAllowed returns whether pilot-gated actions should be allowed.
// Startup/reconnect uncertainty defaults to allowed until explicit disable arrives.
func (c *Capture) IsPilotActionAllowed() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	snap := pilotStatusSnapshotFromExtensionState(c.extensionState)
	return snap.EffectiveEnabled
}

// GetPilotStatus returns pilot and heartbeat command status.
// extension_connected uses the same threshold as IsExtensionConnected (10s on lastSyncSeen).
// extension_last_seen is the RFC3339 timestamp of the last /sync, empty if never synced.
//
// Invariants:
// - Returned in_progress slice is copied to prevent external mutation.
func (c *Capture) GetPilotStatus() any {
	c.mu.RLock()
	defer c.mu.RUnlock()
	snap := pilotStatusSnapshotFromExtensionState(c.extensionState)

	lastSeen := ""
	if !c.extensionState.lastSyncSeen.IsZero() {
		lastSeen = c.extensionState.lastSyncSeen.Format(time.RFC3339)
	}

	inProgress := make([]SyncInProgress, len(c.extensionState.inProgress))
	copy(inProgress, c.extensionState.inProgress)
	inProgressUpdated := ""
	if !c.extensionState.inProgressUpdated.IsZero() {
		inProgressUpdated = c.extensionState.inProgressUpdated.Format(time.RFC3339)
	}

	return map[string]any{
		"enabled":             snap.EffectiveEnabled,
		"configured_enabled":  snap.ConfiguredEnabled,
		"authoritative":       snap.Authoritative,
		"state":               snap.State,
		"source":              snap.Source,
		"extension_connected": !c.extensionState.lastSyncSeen.IsZero() && time.Since(c.extensionState.lastSyncSeen) < extensionDisconnectThreshold,
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
func (c *Capture) GetInProgressCommands() []SyncInProgress {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]SyncInProgress, len(c.extensionState.inProgress))
	copy(out, c.extensionState.inProgress)
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
func (c *Capture) GetTrackingStatus() (enabled bool, tabID int, tabURL string) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.extensionState.trackingEnabled, c.extensionState.trackedTabID, c.extensionState.trackedTabURL
}

// UpdateTrackedTab programmatically updates the tracked tab state.
// Used by switch_tab to retarget subsequent commands to the newly activated tab.
//
// Invariants:
// - tabID must be > 0; zero/negative values are silently ignored.
// - trackingEnabled is set to true when a valid tabID is provided.
func (c *Capture) UpdateTrackedTab(tabID int, tabURL string, tabTitle string) {
	if tabID <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.extensionState.trackingEnabled = true
	c.extensionState.trackedTabID = tabID
	c.extensionState.trackedTabURL = tabURL
	c.extensionState.trackedTabTitle = tabTitle
	c.extensionState.trackingUpdated = time.Now()
}

// GetTrackedTabTitle returns the tracked tab's title (may be stale).
func (c *Capture) GetTrackedTabTitle() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.extensionState.trackedTabTitle
}

// GetTabStatus returns the Chrome tab status ("loading", "complete", or empty if unknown).
func (c *Capture) GetTabStatus() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.extensionState.tabStatus
}

// IsTrackedTabActive returns whether the tracked tab is the foreground tab.
// Returns (active, known). known=false means the extension has not reported this yet.
func (c *Capture) IsTrackedTabActive() (bool, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.extensionState.trackedTabActive == nil {
		return false, false
	}
	return *c.extensionState.trackedTabActive, true
}

// SetTrackedTabActiveForTest sets the tracked tab active state for testing.
func (c *Capture) SetTrackedTabActiveForTest(active bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.extensionState.trackedTabActive = &active
}

// GetCSPStatus returns the last reported CSP restriction level for the tracked page.
func (c *Capture) GetCSPStatus() (restricted bool, level string) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.extensionState.cspRestricted, c.extensionState.cspLevel
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
func (c *Capture) SetSecurityMode(mode string, rewrites []string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	switch mode {
	case SecurityModeInsecureProxy:
		c.extensionState.securityMode = SecurityModeInsecureProxy
		c.extensionState.insecureRewrites = append([]string(nil), rewrites...)
	default:
		c.extensionState.securityMode = SecurityModeNormal
		c.extensionState.insecureRewrites = nil
	}
}

// GetSecurityMode returns current altered-environment mode and rewrite set.
// production_parity is true only in normal mode.
//
// Invariants:
// - Returned rewrite slice is copied and safe for caller mutation.
func (c *Capture) GetSecurityMode() (mode string, productionParity bool, rewrites []string) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	mode = c.extensionState.securityMode
	if mode == "" {
		mode = SecurityModeNormal
	}
	productionParity = mode == SecurityModeNormal
	rewrites = append([]string(nil), c.extensionState.insecureRewrites...)
	return mode, productionParity, rewrites
}

// GetActiveTestIDs returns the list of currently active test IDs.
func (c *Capture) GetActiveTestIDs() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]string, 0, len(c.extensionState.activeTestIDs))
	for testID := range c.extensionState.activeTestIDs {
		result = append(result, testID)
	}
	return result
}

// SetTestBoundaryStart marks a test boundary as active for future event tagging.
//
// Invariants:
// - activeTestIDs behaves as a set (idempotent insert).
func (c *Capture) SetTestBoundaryStart(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.extensionState.activeTestIDs[id] = true
}

// SetTestBoundaryEnd clears a test boundary marker.
//
// Failure semantics:
// - Deleting unknown IDs is a no-op.
func (c *Capture) SetTestBoundaryEnd(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.extensionState.activeTestIDs, id)
}

// ExtensionStatus is a legacy status envelope for non-/sync update paths.
//
// Invariants:
// - Tracking fields are merged directly into ExtensionState under c.mu.
type ExtensionStatus struct {
	Type               string `json:"type"`
	TrackingEnabled    bool   `json:"tracking_enabled"`
	TrackedTabID       int    `json:"tracked_tab_id"`
	TrackedTabURL      string `json:"tracked_tab_url"`
	Message            string `json:"message,omitempty"`
	ExtensionConnected bool   `json:"extension_connected"`
	Timestamp          string `json:"timestamp"`
}

// UpdateExtensionStatus applies legacy extension tracking updates.
//
// Failure semantics:
// - Timestamp parsing/validation is not enforced here; caller-provided fields are trusted.
func (c *Capture) UpdateExtensionStatus(status ExtensionStatus) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.extensionState.trackingEnabled = status.TrackingEnabled
	c.extensionState.trackedTabID = status.TrackedTabID
	c.extensionState.trackedTabURL = status.TrackedTabURL
	c.extensionState.trackingUpdated = time.Now()
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

func (c *Capture) LoadSettingsFromDisk() {
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

	c.mu.Lock()
	defer c.mu.Unlock()
	if time.Since(settings.Timestamp) > 5*time.Second {
		return
	}
	if settings.AIWebPilotEnabled != nil {
		c.extensionState.pilotEnabled = *settings.AIWebPilotEnabled
		c.extensionState.pilotStatusKnown = true
		c.extensionState.pilotUpdatedAt = settings.Timestamp
		c.extensionState.pilotSource = PilotSourceSettingsCache
	}
}

func (c *Capture) SaveSettingsToDisk() error {
	path, err := getSettingsPath()
	if err != nil {
		return err
	}

	c.mu.RLock()
	var pilotEnabled *bool
	if c.extensionState.pilotStatusKnown {
		value := c.extensionState.pilotEnabled
		pilotEnabled = &value
	}
	settings := PersistedSettings{
		AIWebPilotEnabled: pilotEnabled,
		Timestamp:         c.extensionState.pilotUpdatedAt,
		ExtSessionID:      c.extensionState.extSessionID,
	}
	c.mu.RUnlock()

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	// #nosec G301 -- runtime state directory is intentionally user-readable.
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmpPath := path + ".tmp"
	// #nosec G306 -- settings cache is owner-only.
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
