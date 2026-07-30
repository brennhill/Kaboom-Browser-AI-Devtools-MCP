// install_epoch.go — resolves a monotonic "which install is newer" stamp so that,
// at the SAME version, the LATEST install always wins the single-instance takeover.
// Two same-version daemons (e.g. a ~/.kaboom/bin install and an npm-global copy)
// otherwise have no tiebreaker and thrash — each defers to or kills the other. The
// epoch gives a deterministic total order: newer install epoch supersedes.
//
// Resolution order (per install, so two installs get distinct values):
//  1. an installer-written stamp file next to the binary (authoritative), else
//  2. the binary's own mtime (a manually-copied/run binary still gets a sane,
//     comparable value), else
//  3. 0 (unknown — treated as "oldest", so a stamped install always wins).

package daemonlife

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statediag"
)

// installEpochStampName is the per-install stamp file the installer writes next to
// the binary. Kept next to the binary (not in the shared state dir) so each install
// reports its OWN install time — a shared stamp would collapse to one value and
// defeat the tiebreaker.
const installEpochStampName = ".kaboom-install-epoch"

var (
	installEpochOnce sync.Once
	installEpoch     int64
)

// resolveInstallEpoch returns this binary's install epoch, computing it once.
func resolveInstallEpoch(diagnostics statediag.Reporter) int64 {
	installEpochOnce.Do(func() {
		installEpoch = computeInstallEpoch(os.Executable, os.Stat, os.ReadFile, diagnostics)
	})
	return installEpoch
}

// computeInstallEpoch is the pure, injectable core. Returns a unix-nanosecond stamp.
func computeInstallEpoch(
	execPath func() (string, error),
	stat func(string) (os.FileInfo, error),
	readFile func(string) ([]byte, error),
	diagnostics statediag.Reporter,
) int64 {
	exe, err := execPath()
	if err != nil || exe == "" {
		reportInstallEpochRecovery(diagnostics, "The daemon executable path could not be resolved; install ordering is unavailable.")
		return 0
	}
	// 1. Installer stamp next to the binary wins — it records the actual install
	//    moment, which mtime can't when a copy preserves timestamps.
	stamp := filepath.Join(filepath.Dir(exe), installEpochStampName)
	if b, e := readFile(stamp); e == nil {
		if v, e2 := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64); e2 == nil && v > 0 {
			return v
		}
		reportInstallEpochRecovery(diagnostics, "The install epoch stamp was malformed; the executable modification time is active.")
	} else if !os.IsNotExist(e) {
		reportInstallEpochRecovery(diagnostics, "The install epoch stamp could not be read; the executable modification time is active.")
	}
	// 2. Fall back to the binary's mtime.
	if fi, e := stat(exe); e == nil {
		return fi.ModTime().UnixNano()
	}
	reportInstallEpochRecovery(diagnostics, "Neither install epoch nor executable modification time could be read; this install has the lowest takeover priority.")
	return 0
}

func reportInstallEpochRecovery(diagnostics statediag.Reporter, detail string) {
	if diagnostics == nil {
		return
	}
	diagnostics.Report(statediag.Diagnostic{
		Name:   "install_epoch_state",
		Detail: detail,
		Fix:    "Reinstall Kaboom to recreate a valid install epoch stamp.",
	})
}
