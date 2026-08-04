// runtime.go — Owns per-application lifecycle collaborators and mutable state.
// Why: Daemon instances and tests must not share release, upgrade, or warning state through package globals.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package appruntime

import (
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/versioncheck"
)

type UpgradeInfoProvider interface {
	UpgradeInfo() (pending bool, version string, detectedAt time.Time)
}

type Runtime struct {
	version         string
	startedAt       time.Time
	releaseChecker  *versioncheck.Checker
	binaryUpgrade   UpgradeInfoProvider
	updateWarningMu sync.Mutex
	updateLastShown time.Time
}

func New(currentVersion string) *Runtime {
	return &Runtime{
		version:   currentVersion,
		startedAt: time.Now(),
		releaseChecker: versioncheck.New(versioncheck.Options{
			CurrentVersion: currentVersion,
			ReleaseURL:     os.Getenv("KABOOM_RELEASES_URL"),
			HTTPClient:     &http.Client{Timeout: 10 * time.Second},
		}),
	}
}

func (r *Runtime) Version() string                       { return r.version }
func (r *Runtime) StartedAt() time.Time                  { return r.startedAt }
func (r *Runtime) ReleaseChecker() *versioncheck.Checker { return r.releaseChecker }
func (r *Runtime) Upgrade() UpgradeInfoProvider          { return r.binaryUpgrade }
func (r *Runtime) SetUpgrade(value UpgradeInfoProvider)  { r.binaryUpgrade = value }

func (r *Runtime) SetReleaseChecker(checker *versioncheck.Checker) {
	r.releaseChecker = checker
}

func (r *Runtime) SetUpdateLastShown(value time.Time) {
	r.updateWarningMu.Lock()
	r.updateLastShown = value
	r.updateWarningMu.Unlock()
}

func (r *Runtime) ClaimUpdateWarning(now time.Time, cooldown time.Duration) bool {
	r.updateWarningMu.Lock()
	defer r.updateWarningMu.Unlock()
	if !r.updateLastShown.IsZero() && now.Sub(r.updateLastShown) < cooldown {
		return false
	}
	r.updateLastShown = now
	return true
}
