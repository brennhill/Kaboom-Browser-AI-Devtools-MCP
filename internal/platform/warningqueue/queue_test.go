// queue_test.go — Verifies ordered one-shot warning delivery.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package warningqueue

import "testing"

func TestQueueDeduplicatesAndDrainsOnce(t *testing.T) {
	t.Parallel()
	queue := New()
	queue.Add("")
	queue.Add("disk unavailable")
	queue.Add("disk unavailable")
	queue.Add("telemetry disabled")

	warnings := queue.Drain()
	if len(warnings) != 2 {
		t.Fatalf("warnings = %#v", warnings)
	}
	if warnings[0] != "disk unavailable" || warnings[1] != "telemetry disabled" {
		t.Fatalf("warnings lost insertion order: %#v", warnings)
	}
	if secondDrain := queue.Drain(); secondDrain != nil {
		t.Fatalf("second drain = %#v, want nil", secondDrain)
	}
}
