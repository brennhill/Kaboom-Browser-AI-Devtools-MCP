// screenrec_wiring_test.go — Guards every screen-recording dependency seam.

package main

import "testing"

func TestScreenrecDeps_AllSeamsWired(t *testing.T) {
	t.Parallel()
	handler, _, _ := makeToolHandler(t)
	deps := handler.screenrecDeps()
	if deps.EnqueuePendingQuery == nil || deps.RequirePilot == nil ||
		deps.RequireExtension == nil || deps.RecordAIAction == nil ||
		deps.DiagnosticHint == nil || deps.GetCommandResult == nil {
		t.Fatalf("screenrec deps contain nil seam: %#v", deps)
	}
}
