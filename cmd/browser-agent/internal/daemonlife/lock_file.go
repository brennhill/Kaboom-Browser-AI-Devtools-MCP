// lock_file.go — Owns daemon lock-file persistence and ownership marker helpers.
// Why: Isolates filesystem metadata operations from startup/takeover policy logic.

package daemonlife

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

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
	data, err := os.ReadFile(path)
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
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	if rec.UpdatedAt == "" {
		rec.UpdatedAt = daemonNow().UTC().Format(time.RFC3339)
	}
	data, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func removeDaemonLockFile() error {
	path, err := daemonLockFilePath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// RemoveLockIfOwned deletes the daemon lock only when pid still owns it, so a
// shutting-down daemon never clears a successor's lock.
func RemoveLockIfOwned(pid int) {
	rec, err := readDaemonLockFile()
	if err != nil || rec == nil {
		return
	}
	if rec.PID == pid {
		_ = removeDaemonLockFile() //nolint:errcheck // best-effort ownership cleanup
	}
}

// PersistCurrentLock registers this process as the owner of port, stamping the
// record with version and this install's epoch so later launches can classify it.
func PersistCurrentLock(port int, version string) error {
	stateDir, err := state.RootDir()
	if err != nil {
		return err
	}
	return writeDaemonLockFile(daemonLockRecord{
		PID:          os.Getpid(),
		Port:         port,
		StateDir:     stateDir,
		Version:      version,
		UpdatedAt:    daemonNow().UTC().Format(time.RFC3339),
		InstallEpoch: resolveInstallEpoch(),
	})
}
