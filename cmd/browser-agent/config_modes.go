// Purpose: Handles early-exit runtime modes from parsed CLI flags.
// Why: Keeps mode-dispatch side effects (check/stop/connect/install/etc.) isolated from flag registration.

package main

import (
	"os"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/session/clientreg"
)

// handleEarlyExitModes handles --version, --help, --force, --check/--doctor, --stop, --connect.
// Calls os.Exit for any matched mode; returns normally if none matched.
func handleEarlyExitModes(f *parsedFlags) {
	if *f.showVersion {
		stderrf("kaboom v%s\n", version)
		os.Exit(0)
	}
	if *f.showHelp {
		printHelp()
		os.Exit(0)
	}
	if *f.forceCleanup {
		runForceCleanup()
		os.Exit(0)
	}
	if *f.checkSetup || *f.doctorMode {
		ok := runSetupCheckWithOptions(*f.port, setupCheckOptions{
			minSamples:      *f.fastPathMinSamples,
			maxFailureRatio: *f.fastPathMaxFailureRatio,
		})
		if !ok {
			os.Exit(1)
		}
		os.Exit(0)
	}
	if *f.stopMode {
		runStopMode(*f.port)
		os.Exit(0)
	}
	if *f.installMode {
		runNativeInstall()
		os.Exit(0)
	}
	if *f.connectMode {
		cwd, _ := os.Getwd()
		id := *f.clientID
		if id == "" {
			id = clientreg.DeriveClientID(cwd)
		}
		runConnectMode(*f.port, id, cwd)
		os.Exit(0)
	}
}
