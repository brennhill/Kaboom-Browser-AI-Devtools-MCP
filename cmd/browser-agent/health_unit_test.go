// Purpose: Unit tests for browser-agent health logic.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// health_unit_test.go — Unit tests for HealthMetrics counters.
package main

import (
	"sync"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/health"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/launchmode"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/logstore"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

func TestHealthMetrics_IncrementAndGet(t *testing.T) {
	t.Parallel()

	hm := health.NewMetrics()

	hm.IncrementRequest("observe")
	hm.IncrementRequest("observe")
	hm.IncrementRequest("configure")
	hm.IncrementError("observe")

	if got := hm.GetRequestCount("observe"); got != 2 {
		t.Fatalf("GetRequestCount(observe) = %d, want 2", got)
	}
	if got := hm.GetRequestCount("configure"); got != 1 {
		t.Fatalf("GetRequestCount(configure) = %d, want 1", got)
	}
	if got := hm.GetRequestCount("unknown"); got != 0 {
		t.Fatalf("GetRequestCount(unknown) = %d, want 0", got)
	}
	if got := hm.GetErrorCount("observe"); got != 1 {
		t.Fatalf("GetErrorCount(observe) = %d, want 1", got)
	}
	if got := hm.GetErrorCount("configure"); got != 0 {
		t.Fatalf("GetErrorCount(configure) = %d, want 0", got)
	}
}

func TestHealthMetrics_Totals(t *testing.T) {
	t.Parallel()

	hm := health.NewMetrics()

	hm.IncrementRequest("observe")
	hm.IncrementRequest("configure")
	hm.IncrementRequest("interact")
	hm.IncrementError("observe")
	hm.IncrementError("interact")

	if got := hm.GetTotalRequests(); got != 3 {
		t.Fatalf("GetTotalRequests() = %d, want 3", got)
	}
	if got := hm.GetTotalErrors(); got != 2 {
		t.Fatalf("GetTotalErrors() = %d, want 2", got)
	}
}

func TestHealthMetrics_EmptyTotals(t *testing.T) {
	t.Parallel()

	hm := health.NewMetrics()

	if got := hm.GetTotalRequests(); got != 0 {
		t.Fatalf("GetTotalRequests() on empty = %d, want 0", got)
	}
	if got := hm.GetTotalErrors(); got != 0 {
		t.Fatalf("GetTotalErrors() on empty = %d, want 0", got)
	}
}

func TestHealthMetrics_Uptime(t *testing.T) {
	t.Parallel()

	hm := health.NewMetrics()
	uptime := hm.GetUptime()
	if uptime < 0 {
		t.Fatalf("GetUptime() = %v, expected positive", uptime)
	}
}

func TestHealthResponseIncludesDroppedCount(t *testing.T) {
	t.Parallel()

	hm := health.NewMetrics()

	// Create a server with a channel of size 1 and NO async worker,
	// so the channel stays full when we manually fill it.
	srv := &Server{
		logs: logstore.New(logstore.Config{
			MaxEntries: 100,
			ChanSize:   1,
			AddWarning: func(string) {},
		}),
	}

	// Fill queue (no worker draining it), then trigger two drops
	_ = srv.logs.AppendToFile([]LogEntry{{"level": "info", "message": "fill"}})
	_ = srv.logs.AppendToFile([]LogEntry{{"level": "info", "message": "drop1"}})
	_ = srv.logs.AppendToFile([]LogEntry{{"level": "info", "message": "drop2"}})

	resp := getHealthResponse(hm, nil, srv, "test")

	if resp.Buffers.Console.DroppedCount != 2 {
		t.Fatalf("Console.DroppedCount = %d, want 2", resp.Buffers.Console.DroppedCount)
	}

	// Other buffers should have zero dropped count
	if resp.Buffers.Network.DroppedCount != 0 {
		t.Fatalf("Network.DroppedCount = %d, want 0", resp.Buffers.Network.DroppedCount)
	}
	if resp.Buffers.WebSocket.DroppedCount != 0 {
		t.Fatalf("WebSocket.DroppedCount = %d, want 0", resp.Buffers.WebSocket.DroppedCount)
	}
	if resp.Buffers.Actions.DroppedCount != 0 {
		t.Fatalf("Actions.DroppedCount = %d, want 0", resp.Buffers.Actions.DroppedCount)
	}

	// Clean up: no worker was started, so Shutdown returns after a short timeout
	srv.logs.Shutdown(10 * time.Millisecond)
}

func TestHealthResponseZeroDroppedCount(t *testing.T) {
	t.Parallel()

	hm := health.NewMetrics()
	srv := &Server{
		logs: logstore.New(logstore.Config{
			MaxEntries: 100,
			ChanSize:   10,
			AddWarning: func(string) {},
		}),
	}

	resp := getHealthResponse(hm, nil, srv, "test")

	if resp.Buffers.Console.DroppedCount != 0 {
		t.Fatalf("Console.DroppedCount = %d, want 0 for fresh server", resp.Buffers.Console.DroppedCount)
	}

	srv.logs.Shutdown(10 * time.Millisecond)
}

func TestBuildPilotInfo_AssumedEnabledStartupState(t *testing.T) {
	t.Parallel()

	cap := capture.NewCapture()
	info := health.BuildPilotInfo(cap)

	if !info.Enabled {
		t.Fatalf("enabled = false, want true during startup uncertainty")
	}
	if info.Source != "assumed_startup" {
		t.Fatalf("source = %q, want assumed_startup", info.Source)
	}
}

func TestBuildPilotInfo_ExplicitDisableState(t *testing.T) {
	t.Parallel()

	cap := capture.NewCapture()
	cap.SetPilotEnabled(false)

	info := health.BuildPilotInfo(cap)
	if info.Enabled {
		t.Fatalf("enabled = true, want false for explicit disable")
	}
	if info.Source != "test_helper" {
		t.Fatalf("source = %q, want test_helper", info.Source)
	}
}

func TestCalcUtilization_Normal(t *testing.T) {
	t.Parallel()

	if got := health.CalcUtilization(50, 100); got != 50.0 {
		t.Fatalf("health.CalcUtilization(50, 100) = %v, want 50.0", got)
	}
	if got := health.CalcUtilization(0, 100); got != 0.0 {
		t.Fatalf("health.CalcUtilization(0, 100) = %v, want 0.0", got)
	}
	if got := health.CalcUtilization(100, 100); got != 100.0 {
		t.Fatalf("health.CalcUtilization(100, 100) = %v, want 100.0", got)
	}
}

func TestCalcUtilization_ZeroCapacity(t *testing.T) {
	t.Parallel()

	if got := health.CalcUtilization(50, 0); got != 0.0 {
		t.Fatalf("health.CalcUtilization(50, 0) = %v, want 0.0", got)
	}
	if got := health.CalcUtilization(0, 0); got != 0.0 {
		t.Fatalf("health.CalcUtilization(0, 0) = %v, want 0.0", got)
	}
}

func TestCalcUtilization_NegativeCapacity(t *testing.T) {
	t.Parallel()

	if got := health.CalcUtilization(50, -1); got != 0.0 {
		t.Fatalf("health.CalcUtilization(50, -1) = %v, want 0.0", got)
	}
}

func TestHealthMetrics_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	hm := health.NewMetrics()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			hm.IncrementRequest("observe")
		}()
		go func() {
			defer wg.Done()
			hm.IncrementError("observe")
		}()
	}
	wg.Wait()

	if got := hm.GetTotalRequests(); got != 100 {
		t.Fatalf("GetTotalRequests() after 100 concurrent increments = %d, want 100", got)
	}
	if got := hm.GetTotalErrors(); got != 100 {
		t.Fatalf("GetTotalErrors() after 100 concurrent increments = %d, want 100", got)
	}
}

func TestBuildServerInfo_IncludesLaunchModeMetadata(t *testing.T) {
	previous := launchmode.Current()
	launchmode.SetCurrent(launchmode.Info{
		Mode:          launchmode.LikelyTransient,
		Reason:        "interactive_shell_parent",
		ParentProcess: "zsh",
	})
	t.Cleanup(func() { launchmode.SetCurrent(previous) })

	hm := health.NewMetrics()
	resp := getHealthResponse(hm, nil, nil, "test-version")
	info := resp.Server
	if info.LaunchMode != launchmode.LikelyTransient {
		t.Fatalf("launch_mode = %q, want %q", info.LaunchMode, launchmode.LikelyTransient)
	}
	if info.LaunchModeReason != "interactive_shell_parent" {
		t.Fatalf("launch_mode_reason = %q, want interactive_shell_parent", info.LaunchModeReason)
	}
	if info.ParentProcess != "zsh" {
		t.Fatalf("parent_process = %q, want zsh", info.ParentProcess)
	}
}
