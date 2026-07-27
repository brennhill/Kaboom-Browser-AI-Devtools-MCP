// doctor.go — Runs CLI setup checks for port/state/telemetry readiness before startup.
// Why: Keeps preflight setup diagnostics separate from live doctor check handlers.

package health

import (
	"fmt"
	"net"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/diag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

// IsLocalPortAvailable checks whether a local TCP port is available.
func IsLocalPortAvailable(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	_ = ln.Close() //nolint:errcheck // pre-flight check; port availability probe only
	return true
}

// SuggestAvailablePort finds an available port starting from startPort.
func SuggestAvailablePort(startPort, maxOffset int) (int, bool) {
	for offset := 0; offset <= maxOffset; offset++ {
		candidate := startPort + offset
		if candidate <= 0 {
			continue
		}
		if IsLocalPortAvailable(candidate) {
			return candidate, true
		}
	}
	return 0, false
}

// CheckPortAvailability prints port availability status.
func CheckPortAvailability(port int, portKillHint func(int) string) {
	diag.Print("Checking port availability... ")
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		diag.Println("FAILED")
		diag.Printf("  Port %d is already in use.\n", port)
		diag.Printf("  Fix: %s\n", portKillHint(port))
		diag.Printf("  Quick stop (Kaboom): kaboom --stop --port %d\n", port)
		if suggested, ok := SuggestAvailablePort(port+1, 25); ok {
			diag.Printf("  Suggested free port: --port %d\n", suggested)
		} else {
			diag.Printf("  Or use a different port: --port %d\n", port+1)
		}
	} else {
		_ = ln.Close() //nolint:errcheck // pre-flight check; port availability test only
		diag.Println("OK")
		diag.Printf("  Port %d is available.\n", port)
	}
	diag.Println()
}

// CheckStateDirectory prints runtime state directory status.
func CheckStateDirectory() {
	diag.Print("Checking runtime state directory... ")
	rootDir, err := state.RootDir()
	if err != nil {
		diag.Println("FAILED")
		diag.Printf("  Cannot determine runtime state directory: %v\n", err)
	} else {
		logFile, _ := state.DefaultLogFile()
		diag.Println("OK")
		diag.Printf("  State dir: %s\n", rootDir)
		diag.Printf("  Log file: %s\n", logFile)
	}
	diag.Println()
}

// RunSetupCheckWithOptions runs the full setup check and returns whether all thresholds pass.
func RunSetupCheckWithOptions(port int, options SetupCheckOptions, deps SetupDeps) bool {
	if options.MinSamples == 0 && options.MaxFailureRatio == 0 {
		options.MaxFailureRatio = -1
	}
	if options.MinSamples == 0 {
		options.MinSamples = 50
	}

	diag.Println()
	diag.Println("KABOOM SETUP CHECK")
	diag.Println("────────────────────────────────────────────────────────────────")
	diag.Println()
	diag.Printf("Version: %s\n", deps.Version)
	diag.Printf("Port:    %d\n", port)
	diag.Println()

	CheckPortAvailability(port, deps.PortKillHint)
	CheckStateDirectory()
	summary, _ := PrintFastPathTelemetryDiagnostics(200, deps.FastPathTelemetryLogPath)

	thresholdOK := true
	if options.MaxFailureRatio >= 0 {
		diag.Print("Checking fast-path failure threshold... ")
		if err := EvaluateFastPathFailureThreshold(summary, options.MinSamples, options.MaxFailureRatio); err != nil {
			diag.Println("FAILED")
			diag.Printf("  %v\n", err)
			diag.Println()
			thresholdOK = false
		} else {
			ratio := 0.0
			if summary.Total > 0 {
				ratio = float64(summary.Failure) / float64(summary.Total)
			}
			diag.Println("OK")
			diag.Printf("  Ratio %.4f within threshold %.4f (samples=%d)\n", ratio, options.MaxFailureRatio, summary.Total)
			diag.Println()
		}
	}

	diag.Println("────────────────────────────────────────────────────────────────")
	diag.Println()
	diag.Println("Next steps:")
	diag.Println("  1. Start server:    npx kaboom-agentic-browser")
	diag.Println("  2. Install extension:")
	diag.Println("     - Open chrome://extensions")
	diag.Println("     - Enable Developer mode")
	diag.Println("     - Click 'Load unpacked' → select extension/ folder")
	diag.Println("  3. Open any website")
	diag.Println("  4. Extension popup should show 'Connected'")
	diag.Println()
	diag.Printf("Verify:  curl http://localhost:%d/health\n", port)
	diag.Println()
	return thresholdOK
}
