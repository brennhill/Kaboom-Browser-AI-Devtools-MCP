// runtime_test.go — Verifies deterministic startup configuration assembly.

package startupconfig

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/runtimeflags"
)

func TestBuildRuntimeValidatesAndResolvesProcessConfiguration(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), "state")
	uploadDir := t.TempDir()
	config, err := BuildRuntime(runtimeflags.Values{
		Port: 8123, MaxEntries: 42, APIKey: "local", StateDir: stateDir,
		UploadDir: uploadDir, EnableOSUpload: true, BridgeMode: true,
	}, time.Date(2026, 8, 7, 10, 0, 0, 0, time.UTC), 99)
	if err != nil {
		t.Fatalf("BuildRuntime() error = %v", err)
	}
	if config.Port != 8123 || config.MaxEntries != 42 || config.APIKey != "local" || !config.BridgeMode {
		t.Fatalf("config = %#v", config)
	}
	if config.LogFile == "" || config.UploadSecurity == nil || !config.UploadAutomation {
		t.Fatalf("resolved config = %#v", config)
	}
}

func TestBuildRuntimeRejectsInvalidPortWithoutExiting(t *testing.T) {
	_, err := BuildRuntime(runtimeflags.Values{Port: 70000}, time.Time{}, 1)
	if err == nil || !strings.Contains(err.Error(), "port") {
		t.Fatalf("BuildRuntime() error = %v, want port validation", err)
	}
}
