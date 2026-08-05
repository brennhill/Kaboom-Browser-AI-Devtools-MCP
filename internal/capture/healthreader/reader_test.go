// reader_test.go — Verifies detached cross-owner health projection.

package healthreader

import (
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

func TestSnapshotProjectsCanonicalOwnerState(t *testing.T) {
	t.Parallel()
	captured := capture.NewCapture()
	t.Cleanup(captured.Close)
	captured.Extension().SetTestBoundaryStart("health-test")
	captured.Telemetry().AddNetworkBodies([]types.NetworkBody{{Status: 200}})
	captured.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{{Event: "open", ID: "ws-1"}})
	captured.Telemetry().AddEnhancedActions([]types.EnhancedAction{{Type: "click"}})

	snapshot := New(captured).Snapshot()
	if snapshot.NetworkBodyCount != 1 || snapshot.WebSocketCount != 1 || snapshot.ActionCount != 1 || snapshot.ActiveTestIDCount != 1 {
		t.Fatalf("health snapshot = %+v", snapshot)
	}
}
