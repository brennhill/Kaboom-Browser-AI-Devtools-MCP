// Purpose: Unit tests for browser-agent health logic.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// health_unit_test.go — Unit tests for HealthMetrics counters.
package main

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/health"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/launchmode"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/logstore"
	terminalstatus "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/terminal/status"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	capturelogstore "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/logstore"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/syncruntime"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capturefixture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statediag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/streaming/alertbuf"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
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
		terminalStatus: terminalstatus.New(),
		logs: logstore.New(logstore.Config{
			MaxEntries: 100,
			ChanSize:   1,
			AddWarning: func(string) {},
		}),
	}

	// Fill queue (no worker draining it), then trigger two drops
	_ = srv.logs.AppendToFile([]types.LogEntry{{"level": "info", "message": "fill"}})
	_ = srv.logs.AppendToFile([]types.LogEntry{{"level": "info", "message": "drop1"}})
	_ = srv.logs.AppendToFile([]types.LogEntry{{"level": "info", "message": "drop2"}})

	resp := getHealthResponse(hm, nil, srv, nil, nil, "test")

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
	srv := newTestServerForHandlers(t)

	resp := getHealthResponse(hm, nil, srv, nil, nil, "test")

	if resp.Buffers.Console.DroppedCount != 0 {
		t.Fatalf("Console.DroppedCount = %d, want 0 for fresh server", resp.Buffers.Console.DroppedCount)
	}

}

func TestHealthResponseExposesMachineReadableResourcePressure(t *testing.T) {
	hm := health.NewMetrics()
	cap := capture.NewCapture()
	cap.Telemetry().AddNetworkBodies(make([]types.NetworkBody, cap.Telemetry().Pressure().Network.Capacity+2))
	cap.ExtensionLogs().Add(make([]types.ExtensionLog, capturelogstore.ExtensionCapacity+1))
	alerts := alertbuf.NewAlertBuffer()
	for i := 0; i < alertbuf.AlertBufferCap+3; i++ {
		alerts.AddAlert(types.Alert{Title: fmt.Sprintf("alert-%d", i)})
	}

	resp := getHealthResponse(hm, cap, nil, alerts, nil, "test")
	for name, wantDropped := range map[string]int64{"network": 2, "extension_diagnostics": 1, "alerts": 3} {
		got, ok := resp.ResourcePressure[name]
		if !ok {
			t.Fatalf("resource_pressure missing %q: %#v", name, resp.ResourcePressure)
		}
		if got.DroppedCount != wantDropped || got.Entries > got.Capacity {
			t.Fatalf("resource_pressure[%s] = %#v, want dropped=%d and bounded", name, got, wantDropped)
		}
	}
	commands := resp.ResourcePressure["pending_commands"]
	if commands.Capacity != queries.MaxPendingQueries || commands.ActiveEntries != 0 {
		t.Fatalf("pending command pressure = %#v", commands)
	}
	if recordings := resp.ResourcePressure["recordings"]; recordings.ByteCapacity == 0 {
		t.Fatalf("recording storage byte budget missing: %#v", recordings)
	}
}

func TestHealthResponseExposesDoctorTimelinePressure(t *testing.T) {
	recovery := statediag.NewCollector()
	recovery.Report(statediag.Diagnostic{Name: "restart", CorrelationID: "corr-1", Detail: "recovering", Fix: "wait"})
	recovery.Resolve("restart")

	resp := getHealthResponse(health.NewMetrics(), nil, nil, nil, recovery, "test")
	pressure := resp.ResourcePressure["doctor_timeline"]
	if pressure.Entries != 1 || pressure.ActiveEntries != 0 || pressure.Capacity == 0 {
		t.Fatalf("doctor timeline pressure = %#v", pressure)
	}
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
	capturefixture.SetPilot(cap, false)

	info := health.BuildPilotInfo(cap)
	if info.Enabled {
		t.Fatalf("enabled = true, want false for explicit disable")
	}
	if info.Source != syncruntime.PilotSourceSettingsCache {
		t.Fatalf("source = %q, want settings_cache", info.Source)
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
	resp := getHealthResponse(hm, nil, nil, nil, nil, "test-version")
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
