// Purpose: Implements running-binary change detection and upgrade-pending state tracking.
// Why: Detects on-disk binary upgrades so long-lived daemons can surface restart guidance safely.
// Docs: docs/features/feature/deployment-watchdog/index.md

package binarywatch

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/semver"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

const (
	defaultBinaryWatchInterval  = 30 * time.Second
	defaultUpgradeGracePeriod   = 5 * time.Second
	defaultVersionVerifyTimeout = 5 * time.Second
)

type binaryVersionVerifier func(path string, timeout time.Duration) (string, error)

type binaryWatcherConfig struct {
	resolveExecutablePath func() (string, error)
	watchInterval         time.Duration
	upgradeGracePeriod    time.Duration
	versionCheckTimeout   time.Duration
	verifyVersion         binaryVersionVerifier
	now                   func() time.Time
	ticks                 <-chan time.Time
	after                 func(time.Duration) <-chan time.Time
	onBaselineCached      func()
	onStopped             func()
}

func normalizedBinaryWatcherConfig(cfg binaryWatcherConfig) binaryWatcherConfig {
	if cfg.resolveExecutablePath == nil {
		cfg.resolveExecutablePath = os.Executable
	}
	if cfg.watchInterval <= 0 {
		cfg.watchInterval = defaultBinaryWatchInterval
	}
	if cfg.upgradeGracePeriod <= 0 {
		cfg.upgradeGracePeriod = defaultUpgradeGracePeriod
	}
	if cfg.versionCheckTimeout <= 0 {
		cfg.versionCheckTimeout = defaultVersionVerifyTimeout
	}
	if cfg.verifyVersion == nil {
		cfg.verifyVersion = verifyBinaryVersionWithTimeout
	}
	if cfg.now == nil {
		cfg.now = time.Now
	}
	if cfg.after == nil {
		cfg.after = time.After
	}
	return cfg
}

// State tracks the on-disk binary state for upgrade detection.
type State struct {
	mu                  sync.Mutex
	execPath            string
	lastModTime         time.Time
	lastSize            int64
	upgradePending      bool
	detectedVersion     string
	detectedAt          time.Time
	versionCheckTimeout time.Duration
	verifyVersion       binaryVersionVerifier
	now                 func() time.Time
}

// UpgradeInfo returns the current upgrade detection state (thread-safe).
func (s *State) UpgradeInfo() (pending bool, version string, detectedAt time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.upgradePending, s.detectedVersion, s.detectedAt
}

// binaryChanged checks if the binary at execPath has changed since the last check.
// The first call always returns false and caches the initial file state.
func (s *State) binaryChanged() (bool, error) {
	fi, err := os.Stat(s.execPath)
	if err != nil {
		return false, fmt.Errorf("stat binary: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	modTime := fi.ModTime()
	size := fi.Size()

	if s.lastModTime.IsZero() {
		// First call: cache initial state
		s.lastModTime = modTime
		s.lastSize = size
		return false, nil
	}

	if modTime != s.lastModTime || size != s.lastSize {
		s.lastModTime = modTime
		s.lastSize = size
		return true, nil
	}
	return false, nil
}

// checkForUpgrade verifies whether the binary reports a newer version than current.
// Returns true if an upgrade is detected and sets the upgrade-pending state.
func (s *State) checkForUpgrade(currentVersion string) bool {
	verifyVersion := s.verifyVersion
	if verifyVersion == nil {
		verifyVersion = verifyBinaryVersionWithTimeout
	}
	newVer, err := verifyVersion(s.execPath, s.versionCheckTimeout)
	if err != nil {
		return false
	}

	if !semver.IsNewer(newVer, currentVersion) {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.upgradePending = true
	s.detectedVersion = newVer
	now := s.now
	if now == nil {
		now = time.Now
	}
	s.detectedAt = now()
	return true
}

// verifyBinaryVersionWithTimeout executes the binary with --version and parses the output.
// Timeout is injected for deterministic tests that should not mutate package globals.
func verifyBinaryVersionWithTimeout(path string, timeout time.Duration) (string, error) {
	if timeout <= 0 {
		timeout = defaultVersionVerifyTimeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, path, "--version") // #nosec G204 -- path is a verified binary from resolveCanonicalBinary
	cmd.Env = append(os.Environ(), "KABOOM_VERSION_CHECK=1")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("exec --version: %w", err)
	}

	return parseVersionCommandOutput(stdout.String(), stderr.String())
}

// parseVersionOutput extracts a version string from --version output.
// Handles "kaboom v0.8.0", "kaboom 0.8.0", "v0.8.0", and "0.8.0".
func parseVersionOutput(output string) (string, error) {
	// Try "kaboom v0.8.0" or "kaboom 0.8.0"
	if strings.HasPrefix(output, "kaboom ") {
		output = strings.TrimPrefix(output, "kaboom ")
	}
	output = strings.TrimPrefix(output, "v")

	parts := semver.Parts(output)
	if parts == nil {
		return "", fmt.Errorf("invalid version output: %q", output)
	}
	return output, nil
}

func parseVersionCommandOutput(stdout string, stderr string) (string, error) {
	candidates := make([]string, 0, 6)
	appendLines := func(raw string) {
		for _, line := range strings.Split(raw, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				candidates = append(candidates, trimmed)
			}
		}
	}

	appendLines(stdout)
	appendLines(stderr)

	for _, candidate := range candidates {
		if versionValue, err := parseVersionOutput(candidate); err == nil {
			return versionValue, nil
		}
	}

	trimmedStdout := strings.TrimSpace(stdout)
	trimmedStderr := strings.TrimSpace(stderr)
	if trimmedStdout == "" && trimmedStderr == "" {
		return "", fmt.Errorf("empty version output")
	}
	return "", fmt.Errorf("invalid version output: stdout=%q stderr=%q", trimmedStdout, trimmedStderr)
}

// Start starts a background goroutine that watches the daemon binary for changes.
// Returns nil if auto-upgrade is disabled via KABOOM_NO_AUTO_UPGRADE=1.
//
// Detection loop (every binaryWatchInterval):
//  1. Stat the executable, compare modtime+size
//  2. If changed: run --version, parse output
//  3. If newer: set upgrade_pending, call onUpgrade
//  4. After grace period: call triggerShutdown
func Start(ctx context.Context, currentVersion string, onUpgrade func(string), triggerShutdown func()) *State {
	return startBinaryWatcherWithConfig(ctx, currentVersion, onUpgrade, triggerShutdown, binaryWatcherConfig{})
}

func startBinaryWatcherWithConfig(
	ctx context.Context,
	currentVersion string,
	onUpgrade func(string),
	triggerShutdown func(),
	cfg binaryWatcherConfig,
) *State {
	if os.Getenv("KABOOM_NO_AUTO_UPGRADE") == "1" {
		return nil
	}

	cfg = normalizedBinaryWatcherConfig(cfg)

	execPath, err := cfg.resolveExecutablePath()
	if err != nil {
		return nil
	}

	state := &State{
		execPath:            execPath,
		versionCheckTimeout: cfg.versionCheckTimeout,
		verifyVersion:       cfg.verifyVersion,
		now:                 cfg.now,
	}

	util.SafeGo(func() {
		if cfg.onStopped != nil {
			defer cfg.onStopped()
		}
		// Cache initial binary state
		if _, err := state.binaryChanged(); err != nil {
			return
		}
		if cfg.onBaselineCached != nil {
			cfg.onBaselineCached()
		}

		ticks := cfg.ticks
		var ticker *time.Ticker
		if ticks == nil {
			ticker = time.NewTicker(cfg.watchInterval)
			ticks = ticker.C
			defer ticker.Stop()
		}

		for {
			select {
			case <-ticks:
				changed, err := state.binaryChanged()
				if err != nil || !changed {
					continue
				}

				if !state.checkForUpgrade(currentVersion) {
					continue
				}

				_, newVer, _ := state.UpgradeInfo()
				onUpgrade(newVer)

				// Grace period before shutdown
				select {
				case <-cfg.after(cfg.upgradeGracePeriod):
					triggerShutdown()
					return
				case <-ctx.Done():
					return
				}

			case <-ctx.Done():
				return
			}
		}
	})

	return state
}

// upgradeMarker persists the completed version transition across daemon restart.
type Marker struct {
	FromVersion string `json:"from_version"`
	ToVersion   string `json:"to_version"`
	Timestamp   string `json:"timestamp"`
}

func WriteMarker(fromVersion, toVersion, path string) error {
	marker := Marker{
		FromVersion: fromVersion,
		ToVersion:   toVersion,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
	}
	data, err := json.Marshal(marker)
	if err != nil {
		return fmt.Errorf("marshal upgrade marker: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create marker dir: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}

// ReadAndClearMarker consumes the marker exactly once. Invalid marker
// contents are cleared and treated as absent so they cannot poison startup.
func ReadAndClearMarker(path string) (*Marker, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read upgrade marker: %w", err)
	}
	_ = os.Remove(path)

	var marker Marker
	if err := json.Unmarshal(data, &marker); err != nil {
		return nil, nil
	}
	if marker.FromVersion == "" || marker.ToVersion == "" {
		return nil, nil
	}
	return &marker, nil
}
