// Purpose: MCP health entrypoint and main-package adapters for internal/health.
// Why: Transport wiring and response dependency adaptation change together.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package main

import (
	"fmt"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/bridge"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/health"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/procctl"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

func isLocalPortAvailable(port int) bool {
	return health.IsLocalPortAvailable(port)
}

func suggestAvailablePort(startPort, maxOffset int) (int, bool) {
	return health.SuggestAvailablePort(startPort, maxOffset)
}

func checkPortAvailability(port int) {
	health.CheckPortAvailability(port, procctl.PortKillHint)
}

func checkStateDirectory() {
	health.CheckStateDirectory()
}

func runSetupCheckWithOptions(port int, options setupCheckOptions) bool {
	return health.RunSetupCheckWithOptions(port, health.SetupCheckOptions{
		MinSamples: options.minSamples, MaxFailureRatio: options.maxFailureRatio,
	}, health.SetupDeps{
		Version: version, PortKillHint: procctl.PortKillHint,
		FastPathTelemetryLogPath: bridge.FastPathTelemetryLogPath,
	})
}

// toolGetHealth is the MCP tool handler for get_health.
// It returns comprehensive server health metrics.
func (h *ToolHandler) toolGetHealth(req JSONRPCRequest) JSONRPCResponse {
	if h.healthMetrics == nil {
		return fail(req, ErrInternal, "Health metrics not initialized", "Internal server error — do not retry")
	}

	response := getHealthResponse(h.healthMetrics, h.capture, h.server, version)
	return succeed(req, "Server health", response)
}

func (h *ToolHandler) toolDoctor(req JSONRPCRequest) JSONRPCResponse {
	checks := health.RunDoctorChecks(h.capture)
	if h.healthMetrics != nil {
		uptime := h.healthMetrics.GetUptime()
		checks = append(checks, health.DoctorCheck{
			Name: "server_uptime", Status: "pass",
			Detail: fmt.Sprintf("Server running for %s (version %s)", uptime.Round(time.Second), version),
		})
	}
	overallStatus := "healthy"
	readyForInteraction := true
	for _, check := range checks {
		if check.Status == "fail" {
			overallStatus = "unhealthy"
			readyForInteraction = false
		} else if check.Status == "warn" && overallStatus != "unhealthy" {
			overallStatus = "degraded"
			readyForInteraction = false
		}
	}
	return succeed(req, "Doctor: "+overallStatus, map[string]any{
		"status": overallStatus, "ready_for_interaction": readyForInteraction,
		"checks": checks, "hint": h.DiagnosticHintString(),
	})
}

type serverDepsAdapter struct {
	s *Server
}

func (a *serverDepsAdapter) GetTerminalPort() int {
	if a.s == nil {
		return 0
	}
	return a.s.getTerminalPort()
}

func (a *serverDepsAdapter) GetConsoleStats() (int, int, int64) {
	if a.s == nil || a.s.logs == nil {
		return 0, defaultMaxEntries, 0
	}
	return a.s.logs.EntryCount(), a.s.logs.MaxEntries(), a.s.logs.DropCount()
}

// defaultMaxEntries is also the --max-entries default used by config.go.
const defaultMaxEntries = 1000

func getHealthResponse(hm *health.Metrics, cap *capture.Store, server *Server, ver string) health.MCPHealthResponse {
	var serverDeps health.ServerDeps
	if server != nil {
		serverDeps = &serverDepsAdapter{s: server}
	}
	var upgrade health.UpgradeProvider
	if binaryUpgradeState != nil {
		upgrade = binaryUpgradeState
	}
	return hm.GetHealth(cap, serverDeps, upgrade, getLaunchModeInfo, ver)
}

func getLaunchModeInfo() health.LaunchModeInfo {
	lm := getCurrentLaunchMode()
	return health.LaunchModeInfo{
		Mode:          lm.Mode,
		Reason:        lm.Reason,
		ParentProcess: lm.ParentProcess,
	}
}

func buildUpgradeInfo() *health.UpgradeInfo {
	if binaryUpgradeState == nil {
		return nil
	}
	return health.BuildUpgradeInfo(binaryUpgradeState)
}

func buildCommandExecutionInfo(cap *capture.Store) health.CommandExecutionInfo {
	return health.BuildCommandExecutionInfo(cap)
}

func buildCommandExecutionInfoAt(cap *capture.Store, now time.Time) health.CommandExecutionInfo {
	return health.BuildCommandExecutionInfoAt(cap, now)
}

func calcUtilization(entries, capacity int) float64 {
	return health.CalcUtilization(entries, capacity)
}

func buildPilotInfo(cap *capture.Store) health.PilotInfo {
	return health.BuildPilotInfo(cap)
}
