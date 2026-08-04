// runtime.go — Owns per-application lifecycle collaborators and mutable state.
// Why: Daemon instances and tests must not share release, upgrade, or warning state through package globals.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package appruntime

import (
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/exitdiag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/versioncheck"
)

type UpgradeInfoProvider interface {
	UpgradeInfo() (pending bool, version string, detectedAt time.Time)
}

type BridgeRunner interface {
	IsServerRunning(port int) bool
	WaitForServer(port int, timeout time.Duration) bool
	EnsureIOIsolation(logFileHint string) error
	LaunchFingerprint() map[string]any
	RunMode(port int, logFile string, maxEntries int)
}

type Runtime struct {
	version         string
	startedAt       time.Time
	releaseChecker  *versioncheck.Checker
	binaryUpgrade   UpgradeInfoProvider
	updateWarningMu sync.Mutex
	updateLastShown time.Time
	exitDiagnostics *exitdiag.Recorder
	bridgeRunner    BridgeRunner
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
		exitDiagnostics: exitdiag.New(exitdiag.Options{Version: currentVersion}),
	}
}

func (r *Runtime) Version() string                       { return r.version }
func (r *Runtime) StartedAt() time.Time                  { return r.startedAt }
func (r *Runtime) ReleaseChecker() *versioncheck.Checker { return r.releaseChecker }
func (r *Runtime) Upgrade() UpgradeInfoProvider          { return r.binaryUpgrade }
func (r *Runtime) SetUpgrade(value UpgradeInfoProvider)  { r.binaryUpgrade = value }
func (r *Runtime) ExitDiagnostics() *exitdiag.Recorder   { return r.exitDiagnostics }
func (r *Runtime) BridgeRunner() BridgeRunner            { return r.bridgeRunner }
func (r *Runtime) SetBridgeRunner(value BridgeRunner)    { r.bridgeRunner = value }

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
