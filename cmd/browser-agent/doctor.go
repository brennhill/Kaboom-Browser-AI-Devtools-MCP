// Purpose: Thin adapter for CLI setup checks, delegating to health sub-package.
// Why: Keeps preflight setup diagnostics separate from live doctor check handlers.

package main

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/bridge"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/health"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/procctl"
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
		MinSamples:      options.minSamples,
		MaxFailureRatio: options.maxFailureRatio,
	}, health.SetupDeps{
		Version:                  version,
		PortKillHint:             procctl.PortKillHint,
		FastPathTelemetryLogPath: bridge.FastPathTelemetryLogPath,
	})
}
