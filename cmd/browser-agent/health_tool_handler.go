// Purpose: MCP health entrypoint and main-package adapters for internal/health.
// Why: Transport wiring and response dependency adaptation change together.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package main

import (
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/health"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

// toolGetHealth is the MCP tool handler for get_health.
// It returns comprehensive server health metrics.
func (h *ToolHandler) toolGetHealth(req JSONRPCRequest) JSONRPCResponse {
	if h.healthMetrics == nil {
		return fail(req, ErrInternal, "Health metrics not initialized", "Internal server error — do not retry")
	}

	response := getHealthResponse(h.healthMetrics, h.capture, h.server, version)
	return succeed(req, "Server health", response)
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
