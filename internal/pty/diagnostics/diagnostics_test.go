// diagnostics_test.go — Verifies the platform-neutral PTY diagnostic boundary.
package diagnostics

import (
	"sync"
	"testing"
)

func TestHookReceivesStructuredEventAndCanBeCleared(t *testing.T) {
	var mu sync.Mutex
	var gotEvent string
	var gotSession any
	SetHook(func(event string, fields map[string]any) {
		mu.Lock()
		gotEvent = event
		gotSession = fields["session_id"]
		mu.Unlock()
	})
	t.Cleanup(func() { SetHook(nil) })

	Emit(EventSessionCloseFailed, map[string]any{"session_id": "cross-platform"})
	mu.Lock()
	if gotEvent != EventSessionCloseFailed || gotSession != "cross-platform" {
		t.Fatalf("diagnostic = %q/%v, want %q/cross-platform", gotEvent, gotSession, EventSessionCloseFailed)
	}
	mu.Unlock()

	SetHook(nil)
	Emit(EventSessionCloseFailed, map[string]any{"session_id": "ignored"})
	mu.Lock()
	defer mu.Unlock()
	if gotSession != "cross-platform" {
		t.Fatalf("cleared hook observed a later event: %v", gotSession)
	}
}
