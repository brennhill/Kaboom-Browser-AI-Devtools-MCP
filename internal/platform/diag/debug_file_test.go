// debug_file_test.go — Verifies append-only debug diagnostics and environment configuration.

package diag

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDebugFileAppendsTimestampedEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "debug.jsonl")
	logger := NewDebugFile(path, true)
	logger.Printf("first %d", 1)
	logger.Printf("second")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read debug file: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, " first 1\n") || !strings.Contains(text, " second\n") {
		t.Fatalf("unexpected debug log: %q", text)
	}
}

func TestDebugFileDisabledDoesNotCreateFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "disabled.jsonl")
	NewDebugFile(path, false).Printf("ignored")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("disabled logger created file: %v", err)
	}
}

func TestNewDebugFileFromEnvUsesOverrideAndOffSwitch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "override.jsonl")
	t.Setenv("KABOOM_MCP_DEBUG_FILE", path)
	t.Setenv("KABOOM_DEBUG", "")
	logger := NewDebugFileFromEnv()
	if logger.path != path || !logger.enabled {
		t.Fatalf("override not applied: %#v", logger)
	}

	t.Setenv("KABOOM_DEBUG", "off")
	disabled := NewDebugFileFromEnv()
	if disabled.enabled {
		t.Fatal("KABOOM_DEBUG=off should disable debug file logging")
	}
}
