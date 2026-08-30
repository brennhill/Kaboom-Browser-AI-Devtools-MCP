// registry.go — The machine-wide census of live Kaboom processes.
// Why: every previous guard was scoped to a state directory, so --parallel,
// --state-dir, worktrees, and CI each created a private universe in which the
// singleton was trivially true and the machine-wide count was unbounded (880
// parallel state dirs accumulated on one developer machine). The registry is the
// one thing isolation must NOT isolate: its location ignores KABOOM_STATE_DIR so
// every instance, however isolated its data, is still counted here.
// Docs: docs/core/reliability/zombie-prevention.md

package instancereg

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/procidentity"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statefile"
)

// DirEnv overrides the registry location. It exists for tests and for sandboxed
// CI; production never sets it. It is deliberately NOT the state-dir variable.
const DirEnv = "KABOOM_REGISTRY_DIR"

// Role distinguishes the two process kinds that hold machine resources: a daemon
// (binds ports, serves the extension) and a bridge (one per MCP client, stdio).
type Role string

const (
	RoleDaemon Role = "daemon"
	RoleBridge Role = "bridge"
)

// Record is one live instance's registry entry. All JSON fields are snake_case
// per the repository's API field convention.
type Record struct {
	PID          int                `json:"pid"`
	PPID         int                `json:"ppid,omitempty"`
	Role         Role               `json:"role"`
	Ports        []int              `json:"ports,omitempty"`
	StateDir     string             `json:"state_dir,omitempty"`
	Version      string             `json:"version,omitempty"`
	InstallEpoch int64              `json:"install_epoch,omitempty"`
	Parallel     bool               `json:"parallel,omitempty"`
	StartedAt    string             `json:"started_at"`
	HeartbeatAt  string             `json:"heartbeat_at"`
	Identity     procidentity.Info  `json:"identity"`

	// Path is where this record was read from. It is not serialized: the registry
	// must not assume filename-to-pid coupling, because two records can name the
	// same pid (a live instance and a stale entry whose pid was recycled) and the
	// reaper has to be able to remove exactly one of them.
	Path string `json:"-"`
}

// Started reports the record's start time, and whether it could be read. Records
// are sorted and evicted by this, so an unparseable value must not silently sort
// as the zero time (which would make a corrupt record the first eviction victim).
func (r Record) Started() (time.Time, bool) {
	if r.StartedAt == "" {
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339Nano, r.StartedAt)
	if err != nil {
		return time.Time{}, false
	}
	return parsed, true
}

// HeartbeatAge reports how long ago this record last heartbeat, and whether that
// could be determined.
func (r Record) HeartbeatAge(now time.Time) (time.Duration, bool) {
	if r.HeartbeatAt == "" {
		return 0, false
	}
	parsed, err := time.Parse(time.RFC3339Nano, r.HeartbeatAt)
	if err != nil {
		return 0, false
	}
	return now.Sub(parsed), true
}

// Dir returns the machine-wide registry directory, creating it if needed.
//
// Resolution deliberately ignores KABOOM_STATE_DIR: an isolated run isolates its
// DATA, never its ACCOUNTING. Under `go test` an explicit DirEnv is REQUIRED —
// a test that forgets it fails loudly instead of writing into the developer's
// home, which is how 166,824 stray directories and 750MB accumulated in
// ~/.kaboom/projects before this guard existed.
func Dir() (string, error) {
	if override := strings.TrimSpace(os.Getenv(DirEnv)); override != "" {
		return ensureDir(filepath.Clean(override))
	}
	if testing.Testing() {
		return "", fmt.Errorf(
			"instancereg: refusing to use the real user registry from a test; set %s to a temp directory",
			DirEnv,
		)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("instancereg: cannot resolve home directory: %w", err)
	}
	return ensureDir(filepath.Join(home, ".kaboom", "registry"))
}

func ensureDir(dir string) (string, error) {
	// #nosec G301 -- runtime state directory: owner rwx, group rx for diagnostics
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("instancereg: create registry dir %s: %w", dir, err)
	}
	return dir, nil
}

func recordPath(dir string, pid int) string {
	return filepath.Join(dir, fmt.Sprintf("%d.json", pid))
}

// writeRecordTo persists a record at an explicit path. It is unexported: nothing
// outside this package has a reason to author a registry entry, and the only
// caller that ever did was a test planting fixtures (see export_test.go).
func writeRecordTo(path string, rec Record) error {
	data, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("instancereg: encode record: %w", err)
	}
	return statefile.Write(path, data, 0o600)
}

// Handle is an owned registry entry. The owner must Heartbeat periodically and
// Close on shutdown; a handle that stops heartbeating without closing is exactly
// what Wedged reports.
type Handle struct {
	path string
	rec  Record
}

// Record returns a copy of the handle's current record.
func (h *Handle) Record() Record {
	if h == nil {
		return Record{}
	}
	return h.rec
}

// Register stamps the caller's pid, parent, and process identity onto rec and
// publishes it. Callers supply role, ports, version, and state dir; identity and
// timestamps are owned here so no call site can register an unverifiable entry.
func Register(rec Record) (*Handle, error) {
	dir, err := Dir()
	if err != nil {
		return nil, err
	}
	identity, ok := procidentity.Self()
	if !ok {
		return nil, errors.New("instancereg: cannot determine this process's identity")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	rec.PID = os.Getpid()
	rec.PPID = os.Getppid()
	rec.Identity = identity
	rec.StartedAt = now
	rec.HeartbeatAt = now

	path := recordPath(dir, rec.PID)
	if err := writeRecordTo(path, rec); err != nil {
		return nil, err
	}
	return &Handle{path: path, rec: rec}, nil
}

// Heartbeat republishes the record with a fresh heartbeat timestamp, leaving the
// original start time intact. A stopped heartbeat is the wedged-process signal.
func (h *Handle) Heartbeat() error {
	if h == nil {
		return nil
	}
	h.rec.HeartbeatAt = time.Now().UTC().Format(time.RFC3339Nano)
	return writeRecordTo(h.path, h.rec)
}

// SetPorts records ports claimed after registration (the terminal port is bound
// after the HTTP port), so the census never under-reports what a process holds.
func (h *Handle) SetPorts(ports []int) error {
	if h == nil {
		return nil
	}
	h.rec.Ports = ports
	return h.Heartbeat()
}

// Close removes this instance's entry. Safe on a nil handle and idempotent.
func (h *Handle) Close() error {
	if h == nil || h.path == "" {
		return nil
	}
	err := os.Remove(h.path)
	h.path = ""
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("instancereg: deregister: %w", err)
	}
	return nil
}

// List returns every published record, newest start first. A corrupt or partially
// written entry is SKIPPED rather than failing the listing: the census must keep
// working during another process's write, and one bad file must never blind the
// cap to every good one.
func List() ([]Record, error) {
	dir, err := Dir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("instancereg: read registry dir: %w", err)
	}
	records := make([]Record, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(dir, entry.Name())) // #nosec G304 -- registry dir is owned by this package
		if readErr != nil {
			// EXPECTED_ABSENCE: a record removed by a concurrent Close between
			// ReadDir and ReadFile is normal deregistration, not a fault.
			continue
		}
		var rec Record
		if json.Unmarshal(data, &rec) != nil || rec.PID <= 0 {
			continue
		}
		rec.Path = filepath.Join(dir, entry.Name())
		records = append(records, rec)
	}
	sort.Slice(records, func(i, j int) bool {
		left, leftOK := records[i].Started()
		right, rightOK := records[j].Started()
		if leftOK != rightOK {
			return leftOK
		}
		if left.Equal(right) {
			return records[i].PID < records[j].PID
		}
		return left.After(right)
	})
	return records, nil
}

// RecordJSON encodes this handle's record, for publishing inside the singleton
// lock file so a census can name the lock holder from one source of truth.
func (h *Handle) RecordJSON() ([]byte, error) {
	if h == nil {
		return nil, errors.New("instancereg: RecordJSON on nil handle")
	}
	return json.Marshal(h.rec)
}

// DecodeRecord parses a record payload, reporting whether it was usable.
func DecodeRecord(data []byte) (Record, bool) {
	var rec Record
	if json.Unmarshal(data, &rec) != nil || rec.PID <= 0 {
		return Record{}, false
	}
	return rec, true
}

// SingletonLockPath is the machine-wide production-daemon lock. It lives beside
// the registry, so the lock and the census move together and can never end up in
// different directories describing different machines.
func SingletonLockPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "daemon.singleton.lock"), nil
}
