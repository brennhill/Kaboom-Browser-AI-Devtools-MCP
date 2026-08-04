// install_id.go — Random install ID for anonymous session correlation.

package telemetry

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/incident"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statefile"
)

type identityFilesystem interface {
	ReadFile(string) ([]byte, error)
	CreateExclusive(string, string, []byte) error
	Replace(string, string, []byte) error
	WriteFile(string, []byte) error
}

type localIdentityFilesystem struct{}

func (localIdentityFilesystem) ReadFile(path string) ([]byte, error) { return os.ReadFile(path) }

func (localIdentityFilesystem) CreateExclusive(dir, path string, data []byte) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".install-id-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Link(tmpPath, path)
}

func (localIdentityFilesystem) Replace(dir, path string, data []byte) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return statefile.Write(path, data, 0o600)
}

func (localIdentityFilesystem) WriteFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return statefile.Write(path, data, 0o600)
}

// kaboomDir is the directory where install_id is persisted. Overridable for tests.
var kaboomDir = defaultKaboomDir()
var installIdentityFiles identityFilesystem = localIdentityFilesystem{}

// cachedInstallID holds the in-memory cached value after first load.
var cachedInstallID string

// installIDOnce ensures the install ID is loaded/generated exactly once.
var installIDOnce sync.Once

// firstToolCallMu protects first-tool-call state across goroutines.
var firstToolCallMu sync.Mutex

// firstToolCallOnce ensures the persisted state is loaded once per process.
var firstToolCallOnce sync.Once

// cachedFirstToolCallInstallID is the install ID that has already emitted first_tool_call.
var cachedFirstToolCallInstallID string
var (
	stateRecovery              *incident.Store
	stateRecoveryMu            sync.Mutex
	installIdentityIncidentKey string
	installIdentityGeneration  uint64
)

// defaultKaboomDir resolves the installation root. Installation identity must
// not follow KABOOM_STATE_DIR or XDG_STATE_HOME: those roots isolate runtime
// data for projects and tests, while iid must survive every upgrade and launch.
func defaultKaboomDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), ".kaboom")
	}
	return filepath.Join(home, ".kaboom")
}

// Warm pre-loads install ID and session state so the first tool call
// doesn't incur filesystem I/O on the hot path. Call at daemon startup.
func Warm(diagnostics *incident.Store) {
	stateRecoveryMu.Lock()
	stateRecovery = diagnostics
	stateRecoveryMu.Unlock()
	GetInstallID()
	TouchSession()
}

// GetInstallID returns the persistent anonymous install ID.
// On first call, reads from ~/.kaboom/install_id or generates a new 12-char hex string.
// Thread-safe via sync.Once. Returns empty when identity cannot be read or
// persisted so telemetry is suppressed rather than fragmenting one install.
func GetInstallID() string {
	installIDOnce.Do(func() {
		cachedInstallID = loadOrGenerateInstallID()
	})
	return cachedInstallID
}

func loadOrGenerateInstallID() string {
	idPath := filepath.Join(kaboomDir, "install_id")

	// Try to read existing file.
	data, err := installIdentityFiles.ReadFile(idPath)
	if err == nil {
		id := strings.TrimSpace(string(data))
		if validInstallID(id) {
			resolveInstallIDRecovery()
			return id
		}
		reportInstallIDRecovery("Installation identity was malformed; a new stable identity replaced it.")
		return replaceInstallID(idPath)
	}
	if !os.IsNotExist(err) {
		reportInstallIDRecovery("Installation identity could not be read; anonymous telemetry is disabled for this process.")
		return ""
	}

	// Generate a new random ID.
	id := generateRandomID()

	// Best-effort persist through a fully-written temporary file and an atomic
	// hard-link create. Concurrent processes can generate candidates, but only
	// one candidate can become install_id and every process returns that winner.
	if createErr := installIdentityFiles.CreateExclusive(kaboomDir, idPath, []byte(id)); createErr == nil {
		resolveInstallIDRecovery()
		return id
	}
	if winner, readErr := installIdentityFiles.ReadFile(idPath); readErr == nil {
		if persisted := strings.TrimSpace(string(winner)); validInstallID(persisted) {
			resolveInstallIDRecovery()
			return persisted
		}
	}

	reportInstallIDRecovery("Installation identity could not be persisted; anonymous telemetry is disabled for this process.")
	return ""
}

func replaceInstallID(idPath string) string {
	id := generateRandomID()
	if err := installIdentityFiles.Replace(kaboomDir, idPath, []byte(id)); err != nil {
		reportInstallIDRecovery("Installation identity recovery could not be persisted; anonymous telemetry is disabled for this process.")
		return ""
	}
	resolveInstallIDRecovery()
	return id
}

func validInstallID(id string) bool {
	if len(id) != 12 {
		return false
	}
	_, err := hex.DecodeString(id)
	return err == nil
}

func reportInstallIDRecovery(detail string) {
	stateRecoveryMu.Lock()
	defer stateRecoveryMu.Unlock()
	if stateRecovery == nil {
		return
	}
	installIdentityGeneration++
	key, err := stateRecovery.Detect(incident.Report{
		Code: incident.CodeStateRecoveryFailed, CorrelationID: "install_identity",
		Generation: installIdentityGeneration, Evidence: incident.LocalEvidence{Detail: detail},
	})
	if err != nil {
		return
	}
	installIdentityIncidentKey = key
}

func resolveInstallIDRecovery() {
	stateRecoveryMu.Lock()
	defer stateRecoveryMu.Unlock()
	if stateRecovery == nil || installIdentityIncidentKey == "" {
		return
	}
	stateRecovery.Recover(installIdentityIncidentKey, installIdentityGeneration)
}

func loadFirstToolCallInstallID() string {
	data, err := installIdentityFiles.ReadFile(filepath.Join(kaboomDir, "first_tool_call_install_id"))
	if err != nil {
		if !os.IsNotExist(err) {
			reportInstallIDRecovery("The first-tool-call identity marker could not be read; first-call telemetry is suppressed for this process.")
		}
		return ""
	}
	return strings.TrimSpace(string(data))
}

// markFirstToolCallEmittedForInstall persists first-tool state and returns true
// only the first time it is called for the current install ID.
func markFirstToolCallEmittedForInstall() bool {
	firstToolCallMu.Lock()
	defer firstToolCallMu.Unlock()

	firstToolCallOnce.Do(func() {
		cachedFirstToolCallInstallID = loadFirstToolCallInstallID()
	})

	installID := GetInstallID()
	if installID == "" {
		return false
	}
	if cachedFirstToolCallInstallID == installID {
		return false
	}

	if err := installIdentityFiles.WriteFile(
		filepath.Join(kaboomDir, "first_tool_call_install_id"),
		[]byte(installID),
	); err != nil {
		reportInstallIDRecovery("The first-tool-call identity marker could not be persisted; first-call telemetry is suppressed for this process.")
		return false
	}
	cachedFirstToolCallInstallID = installID
	resolveInstallIDRecovery()
	return true
}

func generateRandomID() string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		// Fallback: return a zero-filled ID rather than failing.
		return "000000000000"
	}
	return hex.EncodeToString(b)
}

// overrideKaboomDir sets a custom directory for testing.
func overrideKaboomDir(dir string) {
	kaboomDir = dir
}

// resetKaboomDir restores the default Kaboom directory after testing.
func resetKaboomDir() {
	kaboomDir = defaultKaboomDir()
}

// resetInstallIDState clears the cached install ID and sync.Once for testing.
func resetInstallIDState() {
	reliabilityEvents.WaitIdle()
	installIDOnce = sync.Once{}
	cachedInstallID = ""
	stateRecovery = nil
	installIdentityIncidentKey = ""
	installIdentityGeneration = 0
	installIdentityFiles = localIdentityFilesystem{}
}

// resetFirstToolCallState clears the cached first-tool-call state for testing.
func resetFirstToolCallState() {
	firstToolCallOnce = sync.Once{}
	cachedFirstToolCallInstallID = ""
}

// sessionTimeout is the inactivity duration after which a new session starts.
const sessionTimeout = 30 * time.Minute

// session holds the current session state. Thread-safe via mu.
var session struct {
	mu              sync.Mutex
	id              string
	lastSeen        time.Time
	nextStartReason string // set by TouchSession on rotation, consumed by ConsumeSessionStartReason
}

// sessionEndCallback is called when a session rotates due to timeout.
// Set by UsageTracker to emit session_end beacons.
var sessionEndCallback func(reason string)

// SetSessionEndCallback registers a callback for session timeout rotation.
func SetSessionEndCallback(fn func(reason string)) {
	session.mu.Lock()
	sessionEndCallback = fn
	session.mu.Unlock()
}

// GetSessionID returns the current session ID, minting one if none exists.
// Does NOT detect timeout rotation — that happens in TouchSession, which
// is always called from RecordToolCall right after GetSessionID.
// This keeps GetSessionID simple and race-free (no unlock/relock).
func GetSessionID() string {
	session.mu.Lock()
	defer session.mu.Unlock()

	if session.id == "" {
		session.id = generateSessionID()
		session.lastSeen = time.Now()
	}
	return session.id
}

// TouchSession refreshes the session's last-seen timestamp.
// Detects timeout-based rotation: if lastSeen is older than sessionTimeout,
// fires the session_end callback and mints a new session.
// The callback is fired AFTER all state is updated and the lock is released,
// so it's safe for the callback to call GetSessionID (no deadlock, no TOCTOU).
func TouchSession() {
	session.mu.Lock()
	if session.id == "" {
		session.id = generateSessionID()
	}

	var cb func(string)
	if !session.lastSeen.IsZero() && time.Since(session.lastSeen) > sessionTimeout {
		cb = sessionEndCallback
		// Mint new session BEFORE releasing lock — no TOCTOU window.
		session.id = generateSessionID()
		session.nextStartReason = "post_timeout"
	}
	session.lastSeen = time.Now()
	session.mu.Unlock()

	// Fire callback outside the lock. State is already consistent.
	if cb != nil {
		cb("timeout")
	}
}

// ConsumeSessionStartReason returns and clears the pending session start reason.
// Returns "first_activity" when no specific reason was set (first session or default).
func ConsumeSessionStartReason() string {
	session.mu.Lock()
	reason := session.nextStartReason
	session.nextStartReason = ""
	session.mu.Unlock()
	if reason == "" {
		return "first_activity"
	}
	return reason
}

func generateSessionID() string {
	b := make([]byte, 8) // 16 hex chars
	if _, err := rand.Read(b); err != nil {
		return "0000000000000000"
	}
	return hex.EncodeToString(b)
}

// resetSessionState clears session state for testing.
func resetSessionState() {
	session.mu.Lock()
	session.id = ""
	session.lastSeen = time.Time{}
	session.nextStartReason = ""
	session.mu.Unlock()
}
