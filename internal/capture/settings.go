// Purpose: Persists and restores the extension pilot-settings cache to/from disk.
// Why: Lets pilot enablement survive a daemon restart without waiting for the next /sync.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

// PersistedSettings is the on-disk cache schema for extension pilot status.
//
// Invariants:
// - Timestamp is used as freshness guard; stale cache is ignored.
type PersistedSettings struct {
	AIWebPilotEnabled *bool     `json:"ai_web_pilot_enabled,omitempty"`
	Timestamp         time.Time `json:"timestamp"`
	ExtSessionID      string    `json:"ext_session_id"`
}

// getSettingsPath returns the path to the settings cache file
func getSettingsPath() (string, error) {
	return state.SettingsFile()
}

func getLegacySettingsPath() (string, error) {
	return state.LegacySettingsFile()
}

// readSettingsData reads settings from primary path with legacy fallback.
//
// Failure semantics:
// - Missing files return (nil,nil) to indicate "no cache" without hard failure.
func readSettingsData() ([]byte, error) {
	path, err := getSettingsPath()
	if err != nil {
		return nil, fmt.Errorf("could not determine settings path: %w", err)
	}

	// #nosec G304 -- path is resolved from trusted runtime state directory, not user input
	data, err := os.ReadFile(path)
	if err == nil {
		return data, nil
	}
	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("could not read settings file: %w", err)
	}

	// Compatibility fallback for older locations.
	legacyPath, legacyErr := getLegacySettingsPath()
	if legacyErr != nil {
		return nil, nil // no legacy path available, not an error
	}
	// #nosec G304 -- legacy path is deterministic, not user input
	legacyData, readErr := os.ReadFile(legacyPath)
	if readErr != nil {
		if !os.IsNotExist(readErr) {
			return nil, fmt.Errorf("could not read legacy settings file: %w", readErr)
		}
		return nil, nil
	}
	return legacyData, nil
}

// LoadSettingsFromDisk hydrates pilot state from recent persisted cache.
//
// Invariants:
// - Cache older than 5s is intentionally ignored to avoid stale startup state overriding live sync.
//
// Failure semantics:
// - Read/parse failures are logged and ignored; capture remains operational.
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

// SaveSettingsToDisk persists authoritative pilot status snapshot.
//
// Invariants:
// - Snapshot fields are read under c.mu and written atomically via temp-file rename.
//
// Failure semantics:
// - Any filesystem error aborts write and returns error; previous settings file remains intact.
func (c *Capture) SaveSettingsToDisk() error {
	path, err := getSettingsPath()
	if err != nil {
		return err
	}

	c.mu.RLock()
	var pilotEnabled *bool
	if c.extensionState.pilotStatusKnown {
		v := c.extensionState.pilotEnabled
		pilotEnabled = &v
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

	// Write atomically via temp file
	// #nosec G306 -- settings file is owner-only readable (0600) for privacy
	// #nosec G301 -- runtime state directory should be user-readable for diagnostics
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
