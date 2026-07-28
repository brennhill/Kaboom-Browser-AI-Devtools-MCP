// response_builders.go — Assembles get_health payloads from runtime state, capture snapshots, and process metadata.
// Why: Keeps response composition logic cohesive and independent from mutation/handler paths.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package health

import (
	"os"
	"runtime"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
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
	ver string,
) MCPHealthResponse {
	serverInfo := hm.buildServerInfo(ver, getLaunchMode)
	if server != nil {
		serverInfo.TerminalPort = server.GetTerminalPort()
	}
	resp := MCPHealthResponse{
		Server:           serverInfo,
		Memory:           BuildMemoryInfo(cap),
		Buffers:          BuildBuffersInfo(cap, server),
		RateLimiting:     BuildRateLimitInfo(cap),
		Audit:            hm.BuildAuditInfo(),
		Pilot:            BuildPilotInfo(cap),
		CommandExecution: BuildCommandExecutionInfo(cap),
	}
	if info := BuildUpgradeInfo(upgrade); info != nil {
		resp.Upgrade = info
	}
	return resp
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
		h := cap.GetHealthSnapshot()
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
		h := cap.GetHealthSnapshot()
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

	statusMap := cap.GetPilotStatus()
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
