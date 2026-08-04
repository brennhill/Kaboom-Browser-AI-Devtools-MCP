// response_builders.go — Assembles get_health payloads from runtime state, capture snapshots, and process metadata.
// Why: Keeps response composition logic cohesive and independent from mutation/handler paths.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package health

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/streaming/alertbuf"
)

const (
	defaultMaxEntries = 1000
)

// GetHealth computes and returns the current health metrics.
// This is called on-demand when the get_health tool is invoked.
func (hm *Metrics) GetHealth(
	cap *capture.Capture,
	server ServerDeps,
	upgrade UpgradeProvider,
	getLaunchMode func() LaunchModeInfo,
	alerts *alertbuf.AlertBuffer,
	ver string,
) MCPHealthResponse {
	serverInfo := hm.buildServerInfo(ver, getLaunchMode)
	if server != nil {
		serverInfo.TerminalPort = server.GetTerminalPort()
	}
	resourcePressure := BuildResourcePressure(cap, alerts)
	consoleEntries, consoleCapacity, consoleDropped := getConsoleStats(server)
	resourcePressure["console"] = ResourcePressure{Entries: consoleEntries, Capacity: consoleCapacity,
		DroppedCount: consoleDropped, RecoverableEntries: consoleEntries}
	resp := MCPHealthResponse{
		Server:           serverInfo,
		Memory:           BuildMemoryInfo(cap),
		Buffers:          BuildBuffersInfo(cap, server),
		RateLimiting:     BuildRateLimitInfo(cap),
		Audit:            hm.BuildAuditInfo(),
		Pilot:            BuildPilotInfo(cap),
		CommandExecution: BuildCommandExecutionInfo(cap),
		ResourcePressure: resourcePressure,
	}
	if info := BuildUpgradeInfo(upgrade); info != nil {
		resp.Upgrade = info
	}
	return resp
}

// BuildResourcePressureChecks explains saturation without treating bounded historical eviction as failure.
func BuildResourcePressureChecks(cap *capture.Capture, alerts *alertbuf.AlertBuffer) []DoctorCheck {
	pressure := BuildResourcePressure(cap, alerts)
	checks := make([]DoctorCheck, 0, len(pressure))
	for name, resource := range pressure {
		countBelowLimit := resource.Capacity == 0 || resource.Entries < resource.Capacity
		bytesBelowLimit := resource.ByteCapacity == 0 || resource.RetainedBytes < resource.ByteCapacity
		if resource.DroppedCount == 0 && countBelowLimit && bytesBelowLimit {
			continue
		}
		status := "pass"
		fix := "No action required; disposable history was evicted and active obligations were preserved."
		if (resource.ActiveEntries >= resource.Capacity && resource.Capacity > 0) || !bytesBelowLimit {
			status = "warn"
			fix = "Wait for active commands to complete before dispatching more work."
		}
		checks = append(checks, DoctorCheck{Name: "resource_pressure_" + name, Status: status,
			Detail: fmt.Sprintf("%s retains %d/%d entries, dropped %d, oldest age %dms.", name,
				resource.Entries, resource.Capacity, resource.DroppedCount, resource.OldestAgeMs),
			Fix: fix, Occurrences: int(resource.DroppedCount)})
	}
	return checks
}

// BuildResourcePressure exposes common resource budgets without merging ownership.
func BuildResourcePressure(cap *capture.Capture, alerts *alertbuf.AlertBuffer) map[string]ResourcePressure {
	result := make(map[string]ResourcePressure)
	if cap != nil {
		telemetry := cap.Telemetry().Pressure()
		for name, pressure := range map[string]capture.PressureStats{
			"network": telemetry.Network, "network_waterfall": telemetry.NetworkWaterfall,
			"websocket": telemetry.WebSocket, "actions": telemetry.Actions,
			"extension_diagnostics": cap.ExtensionLogs().Pressure(),
		} {
			result[name] = ResourcePressure{Entries: pressure.Size, Capacity: pressure.Capacity,
				DroppedCount: pressure.Dropped, OldestAgeMs: pressure.OldestAge.Milliseconds(), RecoverableEntries: pressure.Size}
		}
		queries := cap.Queries().GetSnapshot()
		result["pending_commands"] = ResourcePressure{Entries: queries.PendingQueryCount,
			Capacity: queries.PendingCapacity, OldestAgeMs: queries.OldestPendingAge.Milliseconds(), ActiveEntries: queries.PendingQueryCount}
		performance := cap.Performance().Pressure()
		for name, pressure := range map[string]capture.PressureStats{
			"performance_snapshots": performance.Snapshots, "performance_samples": performance.Samples,
			"performance_baselines": performance.BeforeSnapshots,
		} {
			result[name] = ResourcePressure{Entries: pressure.Size, Capacity: pressure.Capacity,
				DroppedCount: pressure.Dropped, OldestAgeMs: pressure.OldestAge.Milliseconds(), RecoverableEntries: pressure.Size}
		}
		recordings := cap.Recordings().Pressure()
		result["recordings"] = ResourcePressure{Entries: recordings.RecordingCount,
			ActiveEntries: recordings.ActiveCount, RecoverableEntries: recordings.RecordingCount,
			RetainedBytes: recordings.UsedBytes, ByteCapacity: recordings.CapacityBytes}
	}
	if alerts != nil {
		pressure := alerts.Pressure()
		result["alerts"] = ResourcePressure{Entries: pressure.Size, Capacity: pressure.Capacity,
			DroppedCount: pressure.Dropped, OldestAgeMs: pressure.OldestAge.Milliseconds(), RecoverableEntries: pressure.Size}
		stream := alerts.Stream.Pressure()
		result["notification_queue"] = ResourcePressure{Entries: stream.Size, Capacity: stream.Capacity,
			DroppedCount: stream.Dropped, OldestAgeMs: stream.OldestAge.Milliseconds(), RecoverableEntries: stream.Size}
	}
	return result
}

// BuildUpgradeInfo returns upgrade detection state, or nil if no upgrade is pending.
func BuildUpgradeInfo(upgrade UpgradeProvider) *UpgradeInfo {
	if upgrade == nil {
		return nil
	}
	pending, newVer, detectedAt := upgrade.UpgradeInfo()
	if !pending {
		return nil
	}
	return &UpgradeInfo{
		Pending:    true,
		NewVersion: newVer,
		DetectedAt: detectedAt.UTC().Format(time.RFC3339),
	}
}

// buildServerInfo returns server identification and uptime.
func (hm *Metrics) buildServerInfo(ver string, getLaunchMode func() LaunchModeInfo) ServerInfo {
	launch := getLaunchMode()
	return ServerInfo{
		Version:          ver,
		UptimeSeconds:    hm.GetUptime().Seconds(),
		PID:              os.Getpid(),
		Platform:         runtime.GOOS + "/" + runtime.GOARCH,
		GoVersion:        runtime.Version(),
		LaunchMode:       launch.Mode,
		LaunchModeReason: launch.Reason,
		ParentProcess:    launch.ParentProcess,
	}
}

// BuildMemoryInfo returns runtime memory statistics.
func BuildMemoryInfo(cap *capture.Capture) MemoryInfo {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	return MemoryInfo{
		CurrentMB: float64(memStats.Alloc) / (1024 * 1024),
		AllocMB:   float64(memStats.Alloc) / (1024 * 1024),
		SysMB:     float64(memStats.Sys) / (1024 * 1024),
		BufferBreakdown: BufferMemoryBreakdown{
			WebSocketBytes: 0,
			NetworkBytes:   0,
			ActionsBytes:   0,
		},
	}
}

// BuildBuffersInfo returns buffer utilization stats from capture and server.
func BuildBuffersInfo(cap *capture.Capture, server ServerDeps) BuffersInfo {
	var networkEntries, wsEntries, actionEntries int
	if cap != nil {
		h := capture.NewHealthReader(cap).Snapshot()
		networkEntries = h.NetworkBodyCount
		wsEntries = h.WebSocketCount
		actionEntries = h.ActionCount
	}

	consoleEntries, consoleCapacity, consoleDropped := getConsoleStats(server)

	return BuffersInfo{
		Console: BufferStats{
			Entries:        consoleEntries,
			Capacity:       consoleCapacity,
			UtilizationPct: CalcUtilization(consoleEntries, consoleCapacity),
			DroppedCount:   consoleDropped,
		},
		Network: BufferStats{
			Entries:        networkEntries,
			Capacity:       capture.MaxNetworkBodies,
			UtilizationPct: CalcUtilization(networkEntries, capture.MaxNetworkBodies),
		},
		WebSocket: BufferStats{
			Entries:        wsEntries,
			Capacity:       capture.MaxWSEvents,
			UtilizationPct: CalcUtilization(wsEntries, capture.MaxWSEvents),
		},
		Actions: BufferStats{
			Entries:        actionEntries,
			Capacity:       capture.MaxEnhancedActions,
			UtilizationPct: CalcUtilization(actionEntries, capture.MaxEnhancedActions),
		},
	}
}

// getConsoleStats returns console buffer entries, capacity, and drop count from the server.
func getConsoleStats(server ServerDeps) (int, int, int64) {
	if server == nil {
		return 0, defaultMaxEntries, 0
	}
	return server.GetConsoleStats()
}

// BuildRateLimitInfo returns rate limiting state from capture.
func BuildRateLimitInfo(cap *capture.Capture) RateLimitingInfo {
	info := RateLimitingInfo{Threshold: capture.RateLimitThreshold}
	if cap != nil {
		h := capture.NewHealthReader(cap).Snapshot()
		info.CurrentRate = h.WindowEventCount
		info.CircuitOpen = h.CircuitOpen
		info.CircuitReason = h.CircuitReason
	}
	return info
}

// BuildAuditInfo returns tool invocation statistics.
func (hm *Metrics) BuildAuditInfo() AuditInfo {
	hm.mu.RLock()
	callsPerTool := make(map[string]int64, len(hm.requestCounts))
	var totalCalls, totalErrors int64
	for tool, count := range hm.requestCounts {
		callsPerTool[tool] = count
		totalCalls += count
	}
	for _, count := range hm.errorCounts {
		totalErrors += count
	}
	hm.mu.RUnlock()

	var errorRate float64
	if totalCalls > 0 {
		errorRate = float64(totalErrors) / float64(totalCalls) * 100
	}

	return AuditInfo{
		TotalCalls:   totalCalls,
		TotalErrors:  totalErrors,
		ErrorRatePct: errorRate,
		CallsPerTool: callsPerTool,
	}
}

// BuildPilotInfo returns AI Web Pilot status from capture.
func BuildPilotInfo(cap *capture.Capture) PilotInfo {
	defaultStatus := PilotInfo{Source: "never_connected"}
	if cap == nil {
		return defaultStatus
	}

	statusMap := cap.Extension().GetPilotStatus()
	m, ok := statusMap.(map[string]any)
	if !ok {
		return defaultStatus
	}

	enabled, _ := m["enabled"].(bool)
	source, _ := m["source"].(string)
	extConn, _ := m["extension_connected"].(bool)

	return PilotInfo{
		Enabled:            enabled,
		Source:             source,
		ExtensionConnected: extConn,
	}
}

// CalcUtilization calculates buffer utilization percentage.
func CalcUtilization(entries, capacity int) float64 {
	if capacity <= 0 {
		return 0
	}
	return float64(entries) / float64(capacity) * 100
}
