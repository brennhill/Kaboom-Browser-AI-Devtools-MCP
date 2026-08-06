// lock.go — Cross-process bridge startup leadership and stale-owner recovery.
package startuplock

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	statecfg "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

// Record is the durable ownership claim shared by competing bridge processes.
type Record struct {
	PID       int    `json:"pid"`
	Port      int    `json:"port"`
	Version   string `json:"version,omitempty"`
	CreatedAt string `json:"created_at"`
}

// Lock is an acquired startup leadership claim.
type Lock struct {
	path string
	pid  int
}

// Manager owns lock acquisition and stale-owner classification policy.
type Manager struct {
	Version        string
	IsProcessAlive func(int) bool
	Now            func() time.Time
	PID            func() int
}

// NewManager constructs startup lock ownership with explicit process seams.
func NewManager(version string, isProcessAlive func(int) bool) Manager {
	return Manager{Version: version, IsProcessAlive: isProcessAlive, Now: time.Now, PID: os.Getpid}
}

// Path returns the deterministic local startup-lock path for a daemon port.
func Path(port int) (string, error) {
	return statecfg.InRoot("run", fmt.Sprintf("bridge-startup-%d.lock.json", port))
}

// Acquire atomically claims startup leadership when no claim exists.
func (m Manager) Acquire(port int) (*Lock, bool, error) {
	path, err := Path(port)
	if err != nil {
		return nil, false, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, false, err
	}
	pid := m.PID()
	payload, err := json.Marshal(Record{PID: pid, Port: port, Version: m.Version, CreatedAt: m.Now().UTC().Format(time.RFC3339Nano)})
	if err != nil {
		return nil, false, err
	}
	// #nosec G304 -- deterministic lock path rooted in the local state directory.
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if os.IsExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if _, err := file.Write(payload); err != nil {
		_ = file.Close()    //nolint:errcheck // cleanup preserves the original write error
		_ = os.Remove(path) //nolint:errcheck // cleanup preserves the original write error
		return nil, false, err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path) //nolint:errcheck // cleanup preserves the original close error
		return nil, false, err
	}
	return &Lock{path: path, pid: pid}, true, nil
}

// Release relinquishes leadership only when the lock is still owned by this process.
func (l *Lock) Release() {
	if l == nil || l.path == "" {
		return
	}
	record, err := Read(l.path)
	if err == nil && record != nil && record.PID != l.pid {
		return
	}
	_ = os.Remove(l.path) //nolint:errcheck // ownership release is best effort
}

// Read decodes one startup claim; a missing claim is not an error.
func Read(path string) (*Record, error) {
	// #nosec G304 -- callers provide a deterministic local startup-lock path.
	payload, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var record Record
	if err := json.Unmarshal(payload, &record); err != nil {
		return nil, err
	}
	return &record, nil
}

// ClearStale removes malformed, dead-owner, or expired startup claims.
func (m Manager) ClearStale(port int, staleAfter time.Duration) bool {
	path, err := Path(port)
	if err != nil {
		return false
	}
	record, err := Read(path)
	if err != nil {
		_ = os.Remove(path) //nolint:errcheck // malformed claims must not block recovery
		return true
	}
	if record == nil {
		return false
	}
	if record.PID <= 0 || !m.IsProcessAlive(record.PID) {
		_ = os.Remove(path) //nolint:errcheck // dead owners cannot retain leadership
		return true
	}
	createdAt, err := ParseTime(record.CreatedAt)
	if err != nil || staleAfter > 0 && m.Now().Sub(createdAt) > staleAfter {
		_ = os.Remove(path) //nolint:errcheck // invalid or expired claims must not block recovery
		return true
	}
	return false
}

// ParseTime accepts the two timestamp precisions used by existing lock records.
func ParseTime(raw string) (time.Time, error) {
	if parsed, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return parsed, nil
	}
	return time.Parse(time.RFC3339, raw)
}
