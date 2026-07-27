// checker.go — Polls the canonical release endpoint and caches available upgrades.
// Why: Keeps release-fetch state, deduplication, and scheduling in one explicit owner.

package versioncheck

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/daemonlife"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

const (
	DefaultReleaseURL = "https://api.github.com/repos/brennhill/Kaboom-Browser-AI-Devtools-MCP/releases/latest"
	defaultCacheTTL   = 6 * time.Hour
	defaultInterval   = 24 * time.Hour
)

type Options struct {
	CurrentVersion string
	ReleaseURL     string
	HTTPClient     *http.Client
	CacheTTL       time.Duration
	Interval       time.Duration
	Now            func() time.Time
}

type Checker struct {
	currentVersion string
	releaseURL     string
	httpClient     *http.Client
	cacheTTL       time.Duration
	interval       time.Duration
	now            func() time.Time

	mu               sync.Mutex
	availableVersion string
	lastCheck        time.Time
	fetchActive      bool
}

func New(options Options) *Checker {
	if options.ReleaseURL == "" {
		options.ReleaseURL = DefaultReleaseURL
	}
	if options.HTTPClient == nil {
		options.HTTPClient = &http.Client{Timeout: 10 * time.Second}
	}
	if options.CacheTTL <= 0 {
		options.CacheTTL = defaultCacheTTL
	}
	if options.Interval <= 0 {
		options.Interval = defaultInterval
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	return &Checker{
		currentVersion: options.CurrentVersion,
		releaseURL:     options.ReleaseURL,
		httpClient:     options.HTTPClient,
		cacheTTL:       options.CacheTTL,
		interval:       options.Interval,
		now:            options.Now,
	}
}

func (c *Checker) Available() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.availableVersion
}

func (c *Checker) Check() {
	now := c.now()
	if !c.beginFetch(now) {
		return
	}
	defer c.endFetch()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, c.releaseURL, nil)
	if err != nil {
		return
	}
	resp, err := c.httpClient.Do(req) // #nosec G704 -- release URL is explicit startup configuration.
	if err != nil {
		return
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		return
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if json.NewDecoder(resp.Body).Decode(&release) != nil {
		return
	}
	newVersion := strings.TrimPrefix(release.TagName, "v")
	if newVersion == "" {
		return
	}

	c.mu.Lock()
	if daemonlife.IsNewerVersion(newVersion, c.currentVersion) {
		c.availableVersion = newVersion
	} else {
		c.availableVersion = ""
	}
	c.lastCheck = now
	c.mu.Unlock()
}

func (c *Checker) Start(ctx context.Context) {
	util.SafeGo(func() {
		c.Check()
		ticker := time.NewTicker(c.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				c.Check()
			case <-ctx.Done():
				return
			}
		}
	})
}

func (c *Checker) beginFetch(now time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.fetchActive || (!c.lastCheck.IsZero() && now.Sub(c.lastCheck) < c.cacheTTL) {
		return false
	}
	c.fetchActive = true
	return true
}

func (c *Checker) endFetch() {
	c.mu.Lock()
	c.fetchActive = false
	c.mu.Unlock()
}
