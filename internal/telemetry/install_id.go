// install_id.go — Random install ID for anonymous session correlation.

package telemetry

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
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

// defaultKaboomDir resolves the runtime state root the same way every other
// Kaboom artifact does — via state.RootDir(), which honours KABOOM_STATE_DIR and
// XDG_STATE_HOME before falling back to ~/.kaboom. Deriving ~/.kaboom directly
// here (the old behaviour) scattered install_id/first_tool_call outside a
// configured state dir, so a project-isolated daemon lost its install identity.
// The no-env default is unchanged (~/.kaboom).
func defaultKaboomDir() string {
	root, err := state.RootDir()
	if err != nil {
		return filepath.Join(os.TempDir(), ".kaboom")
	}
	return root
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

	// Best-effort persist: create dir and write file.
	if err := os.MkdirAll(kaboomDir, 0700); err == nil {
		_ = os.WriteFile(idPath, []byte(id), 0600)
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
