// noise_first_connect_test.go — Tests for automatic noise detection on first extension connection.
package main

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capturefixture"
)

// ============================================
// Issue #264: Auto-detect noise on first connection
// ============================================

// simulateExtensionConnect triggers the "extension_connected" lifecycle event
// by directly calling the capture connection state API, avoiding the 5-second
// long-poll in HandleSync. This makes tests fast and deterministic.
func simulateExtensionConnect(cap *capture.Capture) {
	capturefixture.Connect(cap)
}

func awaitNoiseDetection(t *testing.T, detected <-chan struct{}) {
	t.Helper()
	select {
	case <-detected:
	case <-time.After(time.Second):
		t.Fatal("noise first-connect callback did not run")
	}
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
	capturefixture.SetPilot(cap, false)
	mcpHandler := NewToolHandler(server, cap)
	handler := mcpHandler.tools.Executor.(*ToolHandler)

	// Override first-connect detection to count invocations.
	detected := make(chan struct{}, 1)
	handler.noiseFirstConnectFn = func() {
		detectCount.Add(1)
		detected <- struct{}{}
	}

	// Simulate first extension connection
	simulateExtensionConnect(cap)

	awaitNoiseDetection(t, detected)

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
	capturefixture.SetPilot(cap, false)
	mcpHandler := NewToolHandler(server, cap)
	handler := mcpHandler.tools.Executor.(*ToolHandler)

	// Override first-connect detection to count invocations.
	detected := make(chan struct{}, 1)
	handler.noiseFirstConnectFn = func() {
		detectCount.Add(1)
		detected <- struct{}{}
	}

	// Simulate multiple connections (extension polls repeatedly)
	for i := 0; i < 5; i++ {
		simulateExtensionConnect(cap)
	}

	awaitNoiseDetection(t, detected)

	if got := detectCount.Load(); got != 1 {
		t.Errorf("noise auto-detect should run exactly once across multiple syncs, got %d", got)
	}
}

func TestNoiseAutoDetectOnFirstSync_ManualAutoDetectStillWorks(t *testing.T) {
	t.Parallel()

	env := newConfigureTestEnv(t)

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

// parseResponseJSON already defined in contract_helpers_test.go — reused here.
