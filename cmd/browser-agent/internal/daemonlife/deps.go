// deps.go — the explicit contract between this package and its host (package main).
// Why: daemonlife owns crash-loop self-defense — counting how often THIS install has
// restarted and backing off before it can hammer launchd. It deliberately owns no
// process or port primitives: singleton admission moved to internal/instancegov,
// where a kernel-held flock decides who binds. Passing the remaining seams in keeps
// the dependency arrow one-way (host -> daemonlife) and lets every decision here be
// tested with fakes and no real processes, ports, or HTTP.

package daemonlife

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/incident"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statediag"
)

// Logger receives structured daemon lifecycle events. The host adapts its own
// server logger to this; daemonlife never imports the host's server type.
type Logger interface {
	LogLifecycle(event string, port int, fields map[string]any)
}

// Deps are the host-owned seams daemonlife needs. Every field is required — the
// host builds this in one place so a missing seam cannot be introduced piecemeal.
//
// This shrank from thirteen fields to five when singleton admission moved out.
// The process and port primitives (IsProcessAlive, TryShutdown, TerminatePID,
// FetchHealth, WaitForPortRelease, ReadPIDFile, RemovePIDFile, IsServerRunning)
// were removed rather than left declared-but-unused: an unused seam reads as a
// capability this package still has, and the pid-only IsProcessAlive in particular
// was the one that could signal a process whose pid had been recycled.
type Deps struct {
	// Log records lifecycle decisions (throttling, restart accounting).
	Log Logger
	// Version identifies this build in the restart-history identity key.
	Version string
	// Warnf writes an operator-facing warning to the host's diagnostic sink.
	Warnf func(format string, args ...any)
	// Recovery reports state-directory faults as Doctor diagnostics.
	Recovery statediag.Reporter
	// Incidents records an unclean prior exit for correlated diagnostics.
	Incidents *incident.Store
}
