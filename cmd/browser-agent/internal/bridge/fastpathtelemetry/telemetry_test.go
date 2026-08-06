// telemetry_test.go — Verifies fast-path diagnostics persistence and isolation.
package fastpathtelemetry

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	statecfg "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

func TestMethodAndResourceDiagnosticsRemainIndependent(t *testing.T) {
	t.Setenv(statecfg.StateDirEnv, t.TempDir())
	ResetMethodCounters()
	ResetResourceReadCounters()

	RecordMethod("0.9.0", "initialize", true, 0)
	RecordResourceRead("0.9.0", "kaboom://capabilities", false, -32002)
	Flush()

	assertEvent(t, MethodLogPath, "bridge_fastpath_method", "version", "0.9.0")
	assertEvent(t, ResourceReadLogPath, "bridge_fastpath_resources_read", "bridge_version", "0.9.0")
	if success, failure := SnapshotResourceReadCounters(); success != 0 || failure != 1 {
		t.Fatalf("resource counters = %d/%d, want 0/1", success, failure)
	}
}

func TestResetResourceReadCounters(t *testing.T) {
	t.Setenv(statecfg.StateDirEnv, t.TempDir())
	ResetResourceReadCounters()
	RecordResourceRead("test", "kaboom://capabilities", true, 0)
	Flush()
	ResetResourceReadCounters()
	if success, failure := SnapshotResourceReadCounters(); success != 0 || failure != 0 {
		t.Fatalf("resource counters after reset = %d/%d, want 0/0", success, failure)
	}
}

func assertEvent(t *testing.T, pathFn func() (string, error), event, versionField, version string) {
	t.Helper()
	path, err := pathFn()
	if err != nil {
		t.Fatalf("log path: %v", err)
	}
	payload, err := os.ReadFile(path) //nolint:gosec // test-owned state directory
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	var entry map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(payload))), &entry); err != nil {
		t.Fatalf("decode log: %v", err)
	}
	if entry["event"] != event || entry[versionField] != version {
		t.Fatalf("event = %#v, want event %q and %s %q", entry, event, versionField, version)
	}
}
