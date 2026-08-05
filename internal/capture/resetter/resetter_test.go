// resetter_test.go — Verifies coordinated capture-state clearing.

package resetter

import (
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/logstore"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/perfstore"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/telemetrystore"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

type extensionFixture struct{ cleared bool }

func (f *extensionFixture) ClearTestBoundaries() { f.cleared = true }

func TestClearAllCoordinatesEveryCanonicalOwner(t *testing.T) {
	t.Parallel()
	extension := &extensionFixture{}
	telemetry := telemetrystore.New(telemetrystore.Dependencies{})
	telemetry.AddEnhancedActions([]types.EnhancedAction{{Type: "click"}})
	performance := perfstore.New()
	logs := logstore.NewExtension(func(value string) string { return value })
	logs.Add([]types.ExtensionLog{{Message: "diagnostic"}})

	clearedLogs := New(Dependencies{Extension: extension, Telemetry: telemetry, Performance: performance, ExtensionLogs: logs}).ClearAll()

	if !extension.cleared || telemetry.Actions().Stats().Count != 0 || clearedLogs != 1 {
		t.Fatalf("clear result: extension=%v actions=%d logs=%d", extension.cleared, telemetry.Actions().Stats().Count, clearedLogs)
	}
}
