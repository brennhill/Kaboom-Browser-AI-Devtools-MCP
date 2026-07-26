// deps.go — the explicit contract between this package and its host (package main).
// Why: daemonlife owns the single-instance POLICY (who defers, who takes over, when
// to back off). It deliberately does not own the process/port PRIMITIVES that policy
// acts through — those live in the host next to the rest of its process handling and
// are shared with the host's own port reclaim. Passing them in keeps the dependency
// arrow one-way (host -> daemonlife) and lets every decision here be tested with
// fakes and no real processes, ports, or HTTP.

package daemonlife

import (
	"context"
	"time"
)

// Logger receives structured daemon lifecycle events. The host adapts its own
// server logger to this; daemonlife never imports the host's server type.
type Logger interface {
	LogLifecycle(event string, port int, fields map[string]any)
}

// Deps are the host-owned seams daemonlife needs. Every field is required — the
// host builds this in one place so a missing seam cannot be introduced piecemeal.
//
// Seams that daemonlife can satisfy itself (its clock, its sleeps, its own install
// epoch) are NOT here; they are package-level vars swapped by this package's tests.
// The rule is: host-owned seams travel in Deps, daemonlife-owned seams stay local.
type Deps struct {
	// Log records lifecycle decisions (takeover, defer, reclaim, throttle).
	Log Logger
	// Version is the host binary's version, the primary takeover comparand.
	Version string
	// Warnf writes an operator-facing warning to the host's diagnostic sink.
	Warnf func(format string, args ...any)

	// IsProcessAlive reports whether a PID is still running.
	IsProcessAlive func(pid int) bool
	// IsServerRunning reports whether something is accepting on port.
	IsServerRunning func(port int) bool
	// TryShutdown asks the daemon on port to shut down over HTTP.
	TryShutdown func(port int) bool
	// WaitForPortRelease blocks until port is free or timeout elapses.
	WaitForPortRelease func(port int, timeout time.Duration) bool
	// TerminatePID signals a PID (SIGTERM, or SIGKILL when force is set).
	TerminatePID func(pid int, force bool)
	// FetchHealth performs ONE /health probe against port, reporting whether the
	// daemon answered, the version it reported, and whether the connection was
	// refused (nothing listening). It must never block past timeout and must
	// never return an error — an unreachable daemon is reachable=false.
	FetchHealth func(ctx context.Context, port int, timeout time.Duration) (reachable bool, version string, refused bool)
	// ReadPIDFile returns the PID recorded for port, or 0 when unknown.
	ReadPIDFile func(port int) int
	// RemovePIDFile deletes the PID file for port.
	RemovePIDFile func(port int)
}
