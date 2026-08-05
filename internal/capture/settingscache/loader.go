// loader.go — Loads the bounded extension settings cache with Doctor recovery.
// Docs: docs/features/feature/backend-log-streaming/index.md

package settingscache

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statediag"
)

const (
	diagnosticName = "extension_settings_state"
	maximumAge     = 5 * time.Second
	maximumFuture  = time.Second
)

type settings struct {
	AIWebPilotEnabled *bool     `json:"ai_web_pilot_enabled,omitempty"`
	Timestamp         time.Time `json:"timestamp"`
	ExtSessionID      string    `json:"ext_session_id"`
}

// Load refreshes recent pilot state from the canonical extension settings file.
func Load(apply func(bool, time.Time), diagnostics statediag.Reporter) error {
	if apply == nil {
		return errors.New("extension settings apply boundary is required")
	}
	if diagnostics == nil {
		return errors.New("extension settings Doctor reporter is required")
	}
	load(time.Now().UTC(), read, apply, diagnostics)
	return nil
}

func read() ([]byte, error) {
	path, err := state.SettingsFile()
	if err != nil {
		return nil, fmt.Errorf("resolve extension settings path: %w", err)
	}
	// #nosec G304 -- path is resolved from the canonical local state root.
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		// EXPECTED_ABSENCE: a first run has no settings cache; defaults are authoritative.
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read extension settings: %w", err)
	}
	return data, nil
}

func load(
	now time.Time,
	readData func() ([]byte, error),
	apply func(bool, time.Time),
	diagnostics statediag.Reporter,
) {
	data, err := readData()
	if err != nil {
		reportFailure(diagnostics, "The extension settings cache could not be read; safe defaults remain active.")
		return
	}
	if data == nil {
		// EXPECTED_ABSENCE: missing first-run cache is normal and safe defaults need no warning.
		statediag.Resolve(diagnostics, diagnosticName)
		return
	}

	var persisted settings
	if err := json.Unmarshal(data, &persisted); err != nil {
		reportFailure(diagnostics, "The extension settings cache was invalid; safe defaults remain active.")
		return
	}
	age := now.Sub(persisted.Timestamp)
	if persisted.Timestamp.IsZero() || age < -maximumFuture {
		reportFailure(diagnostics, "The extension settings cache timestamp was invalid; safe defaults remain active.")
		return
	}
	statediag.Resolve(diagnostics, diagnosticName)
	if age > maximumAge {
		// EXPECTED_ABSENCE: stale cache data is advisory and authoritative sync will replace it.
		return
	}
	if persisted.AIWebPilotEnabled == nil {
		// EXPECTED_ABSENCE: an omitted optional pilot preference leaves the current default unchanged.
		return
	}
	apply(*persisted.AIWebPilotEnabled, persisted.Timestamp)
}

func reportFailure(diagnostics statediag.Reporter, detail string) {
	diagnostics.Report(statediag.Diagnostic{
		Name:   diagnosticName,
		Detail: detail,
		Fix:    "Reload the extension; if this repeats, check the Kaboom state directory permissions and submit a Doctor report.",
	})
}
