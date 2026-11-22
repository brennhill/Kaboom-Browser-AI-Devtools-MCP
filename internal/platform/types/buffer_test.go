// Purpose: Tests for typed buffer operations and capacity management.
// Docs: docs/features/feature/normalized-event-schema/index.md

package types

import "testing"

func TestBufferClearCountsTotal(t *testing.T) {
	t.Parallel()

	counts := &BufferClearCounts{
		NetworkWaterfall: 2,
		NetworkBodies:    3,
		WebSocketEvents:  4,
		WebSocketStatus:  5,
		Actions:          6,
		Logs:             7,
		ExtensionLogs:    8,
	}
	if got := counts.Total(); got != 35 {
		t.Fatalf("Total() = %d, want 35", got)
	}
}
