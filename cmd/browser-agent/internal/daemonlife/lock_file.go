// lock_file.go — Owns daemon lock-file persistence and ownership marker helpers.
// Why: Isolates filesystem metadata operations from startup/takeover policy logic.

package daemonlife

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statediag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statefile"
)

type lifecycleFilesystem interface {
	ReadFile(string) ([]byte, error)
	WriteFile(string, []byte) error
	Remove(string) error
}

type localLifecycleFilesystem struct{}

func (localLifecycleFilesystem) ReadFile(path string) ([]byte, error) { return os.ReadFile(path) }
func (localLifecycleFilesystem) WriteFile(path string, data []byte) error {
	return statefile.Write(path, data, 0o600)
}
func (localLifecycleFilesystem) Remove(path string) error { return os.Remove(path) }

var daemonLifecycleFiles lifecycleFilesystem = localLifecycleFilesystem{}

func daemonLockFilePath() (string, error) {
	return state.InRoot("run", "daemon.lock.json")
}

func daemonLockFilePathForError() string {
	path, err := daemonLockFilePath()
	if err != nil {
		return "<unknown>"
	}
	return path
}

func readDaemonLockFile() (*daemonLockRecord, error) {
	path, err := daemonLockFilePath()
	if err != nil {
		return nil, err
	}
	data, err := daemonLifecycleFiles.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var rec daemonLockRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, fmt.Errorf("parse daemon lock at %s: %w. Delete the stale lock file and retry", path, err)
	}
	return &rec, nil
}

func writeDaemonLockFile(rec daemonLockRecord) error {
	path, err := daemonLockFilePath()
	if err != nil {
		return err
	}
	if rec.UpdatedAt == "" {
		rec.UpdatedAt = daemonNow().UTC().Format(time.RFC3339)
	}
	data, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return daemonLifecycleFiles.WriteFile(path, data)
}

func removeDaemonLockFile() error {
	path, err := daemonLockFilePath()
	if err != nil {
		return err
	}
	if err := daemonLifecycleFiles.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// RemoveLockIfOwned deletes the daemon lock only when pid still owns it, so a
// shutting-down daemon never clears a successor's lock.
func RemoveLockIfOwned(pid int) error {
	rec, err := readDaemonLockFile()
	if err != nil {
		return err
	}
	if rec == nil {
		// EXPECTED_ABSENCE: no lock is the normal state after another clean shutdown;
		// logging it would misleadingly report idempotent cleanup as a failure.
		return nil
	}
	if rec.PID == pid {
		return removeDaemonLockFile()
	}
	return nil
}

// PersistCurrentLock registers this process as the owner of port, stamping the
// record with version and this install's epoch so later launches can classify it.
func PersistCurrentLock(port int, version string, diagnostics statediag.Reporter) error {
	stateDir, err := state.RootDir()
	if err != nil {
		return err
	}
	if err := writeDaemonLockFile(daemonLockRecord{
		PID:          os.Getpid(),
		Port:         port,
		StateDir:     stateDir,
		Version:      version,
		UpdatedAt:    daemonNow().UTC().Format(time.RFC3339),
		InstallEpoch: resolveInstallEpoch(diagnostics),
	}); err != nil {
		if diagnostics != nil {
			diagnostics.Report(statediag.Diagnostic{
				Name:   "daemon_lock_state",
				Detail: "Daemon ownership could not be persisted; startup cannot safely claim this state directory.",
				Fix:    "Check state-directory permissions and available disk space, then restart Kaboom.",
			})
		}
		return err
	}
	statediag.Resolve(diagnostics, "daemon_lock_state")
	return nil
}
