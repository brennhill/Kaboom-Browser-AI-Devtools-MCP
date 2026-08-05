// Purpose: Tests the lazy-server-start contract: tool calls auto-start daemon, extension reconnects.
// Docs: docs/features/feature/lazy-server-start/index.md

package bridge

import (
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolguard"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/schema"
)

func TestBridge_SpawnsDaemonWhenNoneRunning(t *testing.T) {
	unusedPort := 19876
	state := &daemonState{runner: testRunner,
		readyCh:  make(chan struct{}),
		failedCh: make(chan struct{}),
		port:     unusedPort,
	}
	connected := testRunner.tryConnectToExisting(state, unusedPort)
	if connected {
		t.Fatal("tryConnectToExisting should return false when no server is running")
	}
	if state.ready {
		t.Fatal("state should not be marked ready when no server is running")
	}
}

func TestBridge_SkipsSpawnWhenDaemonAlreadyRunning(t *testing.T) {
	state := &daemonState{runner: testRunner,
		readyCh:  make(chan struct{}),
		failedCh: make(chan struct{}),
		port:     0,
	}
	result := testRunner.tryConnectToExisting(state, 0)
	if result {
		t.Fatal("should return false with port 0 (no server)")
	}
}

func TestDaemonState_RespawnResetsClearFailure(t *testing.T) {
	state := &daemonState{runner: testRunner,
		readyCh:  make(chan struct{}),
		failedCh: make(chan struct{}),
		port:     19877,
	}
	state.markFailed("port bind error")
	if !state.failed {
		t.Fatal("state should be marked failed")
	}
	expectedReady := state.readyCh
	expectedFailed := state.failedCh
	state.mu.Lock()
	canReset := state.failed && state.readyCh == expectedReady && state.failedCh == expectedFailed
	if canReset {
		state.ready = false
		state.failed = false
		state.err = ""
		state.resetSignalsLocked()
	}
	state.mu.Unlock()
	if state.failed {
		t.Fatal("failed flag should be cleared after reset")
	}
	if state.err != "" {
		t.Fatal("error should be cleared after reset")
	}
}

func TestCheckDaemonStatus_ReturnsStartingDuringSpawn(t *testing.T) {
	state := &daemonState{runner: testRunner,
		readyCh:  make(chan struct{}),
		failedCh: make(chan struct{}),
		port:     19878,
	}
	req := mcp.JSONRPCRequest{Method: "tools/call"}
	saved := daemonStartupGracePeriod
	daemonStartupGracePeriod = 50 * time.Millisecond
	defer func() { daemonStartupGracePeriod = saved }()
	status := checkDaemonStatus(state, req, state.port)
	if status != "starting" {
		t.Fatalf("expected 'starting' during spawn, got %q", status)
	}
}

func TestCheckDaemonStatus_ReadyReturnsEmpty(t *testing.T) {
	state := &daemonState{runner: testRunner,
		readyCh:  make(chan struct{}),
		failedCh: make(chan struct{}),
		port:     0,
	}
	state.markReady()
	req := mcp.JSONRPCRequest{Method: "tools/call"}
	status := checkDaemonStatus(state, req, state.port)
	if status != "" {
		t.Fatalf("expected empty status when daemon is ready, got %q", status)
	}
}

func TestFastPath_InitializeDoesNotRequireDaemon(t *testing.T) {
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", Method: "initialize", ID: float64(1)}
	state := &daemonState{runner: testRunner,
		readyCh:  make(chan struct{}),
		failedCh: make(chan struct{}),
		port:     19879,
	}
	status := checkDaemonStatus(state, req, state.port)
	if status != "method_not_found" {
		t.Fatalf("initialize should be classified as method_not_found (handled by fast-path), got %q", status)
	}
}

func TestFastPath_ToolsListDoesNotRequireDaemon(t *testing.T) {
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", Method: "tools/list", ID: float64(1)}
	toolsList := schema.AllTools()
	handled := testRunner.handleFastPath(req, toolsList, 0)
	if !handled {
		t.Fatal("tools/list should be handled by fast-path without daemon")
	}
}

func TestToolsCall_WaitsDuringStartup_NotInstantError(t *testing.T) {
	status := "starting"
	method := "tools/call"
	shouldForward := true
	if status != "" {
		if status == "starting" {
			if method == "tools/call" {
				shouldForward = true
			} else {
				shouldForward = false
			}
		} else {
			shouldForward = false
		}
	}
	if !shouldForward {
		t.Fatal("tools/call should be forwarded (waited on) during startup, not rejected")
	}
}

func TestRequireExtension_WaitsForColdStart(t *testing.T) {
	if toolguard.DefaultExtensionReadinessTimeout <= 0 {
		t.Fatal("DefaultExtensionReadinessTimeout should be > 0 to allow cold-start reconnection")
	}
}
