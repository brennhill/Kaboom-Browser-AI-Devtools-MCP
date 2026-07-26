// Purpose: Periodically fetches the latest GitHub release to detect available upgrades, with TTL caching and dedup.
// Why: Surfaces upgrade-available warnings in health and tool responses without blocking normal operation.

package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/daemonlife"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

const (
	versionCheckCacheTTL = 6 * time.Hour
	versionCheckInterval = 24 * time.Hour
	httpClientTimeout    = 10 * time.Second
)

var (
	availableVersion   string
	lastVersionCheck   time.Time
	versionFetchActive bool // prevents duplicate concurrent fetches
	versionCheckMu     sync.Mutex
)

var (
	// githubAPIURL can be overridden via KABOOM_RELEASES_URL env var for forked repos.
	// Access must be protected by versionCheckMu (read via getGitHubAPIURL).
	githubAPIURL = getEnvOrDefault("KABOOM_RELEASES_URL",
		"https://api.github.com/repos/brennhill/Kaboom-Browser-AI-Devtools-MCP/releases/latest")
)

// getGitHubAPIURL returns the current GitHub API URL (test use only).
func getGitHubAPIURL() string {
	versionCheckMu.Lock()
	defer versionCheckMu.Unlock()
	return githubAPIURL
}

// setGitHubAPIURL overrides the GitHub API URL (test use only).
func setGitHubAPIURL(url string) {
	versionCheckMu.Lock()
	defer versionCheckMu.Unlock()
	githubAPIURL = url
}

// getAvailableVersion returns the currently known newer release version, if any.
func getAvailableVersion() string {
	versionCheckMu.Lock()
	defer versionCheckMu.Unlock()
	return availableVersion
}

// getEnvOrDefault returns the environment variable value or a default if not set
func getEnvOrDefault(key, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultValue
}

func beginVersionFetch(now time.Time) (fetchURL string, shouldFetch bool) {
	versionCheckMu.Lock()
	defer versionCheckMu.Unlock()

	// Check if cache is still valid (6 hour TTL) or fetch already in progress.
	if (!lastVersionCheck.IsZero() && now.Sub(lastVersionCheck) < versionCheckCacheTTL) || versionFetchActive {
		return "", false
	}

	// Mark fetch as active to prevent duplicate concurrent fetches.
	versionFetchActive = true
	return githubAPIURL, true
}

func endVersionFetch() {
	versionCheckMu.Lock()
	defer versionCheckMu.Unlock()
	versionFetchActive = false
}

func updateAvailableVersion(newVersion string, now time.Time) {
	versionCheckMu.Lock()
	defer versionCheckMu.Unlock()

	if daemonlife.IsNewerVersion(newVersion, version) {
		availableVersion = newVersion
	} else {
		// Do not advertise older/equal releases as available updates.
		availableVersion = ""
	}
	lastVersionCheck = now
}

// checkGitHubVersion fetches the latest version from GitHub
// Returns early if cache is still valid (within 6 hours)
// Used to determine if a newer version is available to notify users
func checkGitHubVersion() {
	now := time.Now()
	fetchURL, shouldFetch := beginVersionFetch(now)
	if !shouldFetch {
		return
	}
	defer endVersionFetch()

	// Fetch from GitHub (silent on errors - version check is optional/non-critical)
	client := &http.Client{Timeout: httpClientTimeout}
	resp, err := client.Get(fetchURL) // #nosec G107 -- constant GitHub API URL
	if err != nil {
		// Silent: network errors are common, version check is non-critical
		return
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		// Silent: GitHub rate limits (403) and other errors are expected
		// Version info is available via get_health tool when it succeeds
		return
	}

	var releaseInfo struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&releaseInfo); err != nil {
		// Silent: parse errors don't affect core functionality
		return
	}

	// Extract version from tag (e.g., "v6.0.3" -> "6.0.3")
	newVersion := strings.TrimPrefix(releaseInfo.TagName, "v")
	if newVersion == "" {
		// Silent: invalid tag doesn't affect core functionality
		return
	}

	updateAvailableVersion(newVersion, now)

	// Quiet mode: Version check results are available via get_health tool
	// No need to spam stderr - LLMs don't care about version updates
}

// startVersionCheckLoop starts a periodic check for new versions on GitHub (daily)
// Checks immediately on startup if no cached value, then periodically
// The goroutine stops cleanly when ctx is cancelled
func startVersionCheckLoop(ctx context.Context) {
	util.SafeGo(func() {
		// Check immediately on startup
		checkGitHubVersion()

		// Then check periodically
		ticker := time.NewTicker(versionCheckInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				checkGitHubVersion()
			case <-ctx.Done():
				// Context cancelled - clean shutdown
				return
			}
		}
	})
}
