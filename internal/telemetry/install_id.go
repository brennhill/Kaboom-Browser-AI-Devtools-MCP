// install_id.go — Random install ID for anonymous session correlation.

package telemetry

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// kaboomDir is the directory where install_id is persisted. Overridable for tests.
var kaboomDir = defaultKaboomDir()

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
func Warm() {
	GetInstallID()
	TouchSession()
}

// GetInstallID returns the persistent anonymous install ID.
// On first call, reads from ~/.kaboom/install_id or generates a new 12-char hex string.
// Thread-safe via sync.Once. Never returns an error — falls back to a fresh random ID.
func GetInstallID() string {
	installIDOnce.Do(func() {
		cachedInstallID = loadOrGenerateInstallID()
	})
	return cachedInstallID
}

func loadOrGenerateInstallID() string {
	idPath := filepath.Join(kaboomDir, "install_id")

	// Try to read existing file.
	data, err := os.ReadFile(idPath)
	if err == nil {
		id := strings.TrimSpace(string(data))
		if id != "" {
			return id
		}
	}

	// Generate a new random ID.
	id := generateRandomID()

	// Best-effort persist through a fully-written temporary file and an atomic
	// hard-link create. Concurrent processes can generate candidates, but only
	// one candidate can become install_id and every process returns that winner.
	if err := os.MkdirAll(kaboomDir, 0700); err == nil {
		tmp, createErr := os.CreateTemp(kaboomDir, ".install-id-*")
		if createErr == nil {
			tmpPath := tmp.Name()
			defer os.Remove(tmpPath)
			_ = tmp.Chmod(0600)
			_, writeErr := tmp.WriteString(id)
			closeErr := tmp.Close()
			if writeErr == nil && closeErr == nil {
				if linkErr := os.Link(tmpPath, idPath); linkErr == nil {
					return id
				}
				if winner, readErr := os.ReadFile(idPath); readErr == nil {
					if persisted := strings.TrimSpace(string(winner)); persisted != "" {
						return persisted
					}
				}
			}
		}
	}

	return id
}

func loadFirstToolCallInstallID() string {
	data, err := os.ReadFile(filepath.Join(kaboomDir, "first_tool_call_install_id"))
	if err != nil {
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
	if cachedFirstToolCallInstallID == installID {
		return false
	}

	cachedFirstToolCallInstallID = installID
	if err := os.MkdirAll(kaboomDir, 0700); err == nil {
		_ = os.WriteFile(
			filepath.Join(kaboomDir, "first_tool_call_install_id"),
			[]byte(installID),
			0600,
		)
	}

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
	installIDOnce = sync.Once{}
	cachedInstallID = ""
}

// resetFirstToolCallState clears the cached first-tool-call state for testing.
func resetFirstToolCallState() {
	firstToolCallOnce = sync.Once{}
	cachedFirstToolCallInstallID = ""
}
