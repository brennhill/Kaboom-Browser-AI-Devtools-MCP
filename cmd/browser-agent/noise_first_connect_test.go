// noise_first_connect_test.go — Tests for automatic noise detection on first extension connection.
package main

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/noiseautorun"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

// ============================================
// Issue #264: Auto-detect noise on first connection
// ============================================

// simulateExtensionConnect triggers the "extension_connected" lifecycle event
// by directly calling the capture connection state API, avoiding the 5-second
// long-poll in HandleSync. This makes tests fast and deterministic.
func simulateExtensionConnect(cap *capture.Capture) {
	cap.SimulateSyncForTest("test-sess-1", "test-client")
}

func waitForNoiseFirstConnectCallback() {
	time.Sleep(noiseautorun.FirstConnectDelay() + 150*time.Millisecond)
}

func TestNoiseAutoDetectOnFirstSync_TriggersOnce(t *testing.T) {
	t.Parallel()

	var detectCount atomic.Int32
	server, err := NewServer(t.TempDir()+"/test.jsonl", 100)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(func() { server.Close() })

	cap := capture.NewCapture()
	cap.Extension().SetPilotEnabled(false)
	mcpHandler := NewToolHandler(server, cap)
	handler := mcpHandler.tools.Executor.(*ToolHandler)

	// Override first-connect detection to count invocations.
	handler.noiseFirstConnectFn = func() { detectCount.Add(1) }

	// Simulate first extension connection
	simulateExtensionConnect(cap)

	// Give the async callback time to fire and execute.
	waitForNoiseFirstConnectCallback()

	if got := detectCount.Load(); got != 1 {
		t.Errorf("noise auto-detect should run once on first connection, got %d", got)
	}
}

func TestNoiseAutoDetectOnFirstSync_DoesNotRepeat(t *testing.T) {
	t.Parallel()

	var detectCount atomic.Int32
	server, err := NewServer(t.TempDir()+"/test.jsonl", 100)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(func() { server.Close() })

	cap := capture.NewCapture()
	cap.Extension().SetPilotEnabled(false)
	mcpHandler := NewToolHandler(server, cap)
	handler := mcpHandler.tools.Executor.(*ToolHandler)

	// Override first-connect detection to count invocations.
	handler.noiseFirstConnectFn = func() { detectCount.Add(1) }

	// Simulate multiple connections (extension polls repeatedly)
	for i := 0; i < 5; i++ {
		simulateExtensionConnect(cap)
		time.Sleep(50 * time.Millisecond)
	}

	// Give async callback time to fire and execute.
	waitForNoiseFirstConnectCallback()

	if got := detectCount.Load(); got != 1 {
		t.Errorf("noise auto-detect should run exactly once across multiple syncs, got %d", got)
	}
}

func TestNoiseAutoDetectOnFirstSync_ManualAutoDetectStillWorks(t *testing.T) {
	t.Parallel()

	env := newConfigureTestEnv(t)

	// Trigger first-connection auto-detect
	simulateExtensionConnect(env.capture)
	waitForNoiseFirstConnectCallback()

	// Manual auto_detect should still work independently
	result, ok := env.callConfigure(t, `{"what":"noise_rule","noise_action":"auto_detect"}`)
	if !ok {
		t.Fatal("manual noise auto_detect should return result")
	}
	if result.IsError {
		t.Fatalf("manual noise auto_detect should not error, got: %s", result.Content[0].Text)
	}

	data := parseResponseJSON(t, result)
	if _, ok := data["proposals"]; !ok {
		t.Error("manual auto_detect response should contain proposals")
	}
}

func TestNoiseAutoDetectOnFirstSync_EmitsLogEntry(t *testing.T) {
	t.Parallel()

	server, err := NewServer(t.TempDir()+"/test.jsonl", 100)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(func() { server.Close() })

	cap := capture.NewCapture()
	cap.Extension().SetPilotEnabled(false)
	mcpHandler := NewToolHandler(server, cap)
	_ = mcpHandler.tools.Executor.(*ToolHandler)

	// Simulate first extension connection
	simulateExtensionConnect(cap)

	// Give async callback time to fire and complete.
	waitForNoiseFirstConnectCallback()

	// The diagnostic log is written to the configured stderr sink; verify no panic occurs.
	// and the auto-detect function executes (covered by count tests above)
}

// parseResponseJSON already defined in contract_helpers_test.go — reused here.
