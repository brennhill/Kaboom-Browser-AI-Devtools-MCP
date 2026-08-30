// daemon_governance.go — Wires machine-wide instance governance into the daemon.
// Why: the admission gate, the heartbeat, the idle bound, and the client reaper
// are one lifecycle, and leaving any of them to an individual call site is how
// clientreg.ReapIdle came to be written, exported, and never once called.
// Docs: docs/core/reliability/zombie-prevention.md

package main

import (
	"context"
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/daemonlife"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/diag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/idlewatch"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/instancegov"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/instancereg"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

const (
	// productionIdleWindow is how long a daemon may serve nobody before releasing
	// its ports. Long enough to survive a developer's lunch, short enough that a
	// forgotten daemon does not hold two ports for days — one on a developer
	// machine had been up 2 days 13 hours with no client attached.
	productionIdleWindow = 2 * time.Hour
	// parallelIdleWindow is the equivalent for a test daemon, which belongs to a
	// single run and has no reason to outlive it by more than a few minutes.
	parallelIdleWindow = 5 * time.Minute
	// parallelMaxLifetime is a hard bound for test daemons: when a run dies without
	// stopping its daemon, this is the only thing that reclaims its two ports.
	parallelMaxLifetime = 30 * time.Minute
	// clientReapInterval is how often idle MCP client sessions are reaped.
	clientReapInterval = 5 * time.Minute
)

// busyInputs are the daemon's work signals. Each is optional at the call site and
// a nil probe means "cannot tell", which busyProbe always resolves as BUSY.
type busyInputs struct {
	Clients            func() int
	ExtensionConnected func() bool
	ActiveRecording    func() bool
	TerminalSessions   func() int
}

// busyProbe answers whether the daemon has work that must not be interrupted.
// It fails safe in every direction: any signal that cannot be read keeps the
// daemon alive, because shutting down mid-recording is far worse than a daemon
// that lingers one extra window.
func busyProbe(in busyInputs) idlewatch.BusyProbe {
	return func() (bool, string) {
		if in.Clients == nil || in.ExtensionConnected == nil ||
			in.ActiveRecording == nil || in.TerminalSessions == nil {
			return true, "a work signal is unavailable; assuming work in progress"
		}
		if count := in.Clients(); count > 0 {
			return true, "MCP clients connected"
		}
		if in.ExtensionConnected() {
			return true, "the browser extension is attached"
		}
		if in.ActiveRecording() {
			return true, "a recording is in progress"
		}
		if count := in.TerminalSessions(); count > 0 {
			return true, "terminal sessions are live"
		}
		return false, ""
	}
}

// idleConfigFor returns the lifetime budget for this daemon kind. Only a parallel
// daemon gets a hard lifetime: bounding a production daemon that someone is
// actively using would shut it down under them.
func idleConfigFor(parallel bool, probe idlewatch.BusyProbe) idlewatch.Config {
	if parallel {
		return idlewatch.Config{
			IdleAfter: parallelIdleWindow, MaxLifetime: parallelMaxLifetime,
			Poll: 30 * time.Second, Busy: probe,
		}
	}
	return idlewatch.Config{IdleAfter: productionIdleWindow, Poll: time.Minute, Busy: probe}
}

// admitDaemon runs the machine-wide admission gate. A Defer outcome means a daemon
// is already serving somewhere on this machine and this process must exit 0.
func admitDaemon(server *Server, port, terminalPort int, opts daemonlife.LaunchOptions) (instancegov.Result, error) {
	lockPath, err := instancereg.SingletonLockPath()
	if err != nil {
		return instancegov.Result{}, err
	}
	stateDir, _ := state.RootDir()
	ports := []int{port}
	if terminalPort > 0 {
		ports = append(ports, terminalPort)
	}
	// Reuse the host's existing process/HTTP primitives rather than introducing a
	// second set: daemonrecovery already owns shutdown and termination, and two
	// implementations of "stop that daemon" would eventually disagree.
	lifecycle := server.daemonRecovery.LifecycleDeps()
	return instancegov.Admit(instancegov.Config{
		Role:     instancereg.RoleDaemon,
		Ports:    ports,
		StateDir: stateDir,
		Version:  version,
		Parallel: opts.Parallel,
		LockPath: lockPath,
		Policy:   instancereg.DefaultPolicy(),
		// An upgrade asks the incumbent to stand down over HTTP and waits for the
		// kernel lock to be released, rather than racing it for the port.
		RequestShutdown: func(rec instancereg.Record) error {
			incumbentPort := port
			if len(rec.Ports) > 0 {
				incumbentPort = rec.Ports[0]
			}
			if lifecycle.TryShutdown(incumbentPort) {
				return nil
			}
			return fmt.Errorf("incumbent daemon pid %d on port %d did not accept a shutdown request",
				rec.PID, incumbentPort)
		},
		Terminate: func(pid int, force bool) error {
			lifecycle.TerminatePID(pid, force)
			return nil
		},
		Log: func(event string, fields map[string]any) {
			server.logLifecycle(event, port, fields)
		},
	})
}

// governanceLoops is the parameter set for the daemon's background lifetime loops.
// It is a struct rather than a parameter list because these values travel together
// and always will: a caller that can pass five of them and forget the sixth is a
// caller that can start a heartbeat with no idle bound.
type governanceLoops struct {
	Server    *Server
	Admission *instancegov.Result
	Port      int
	Inputs    busyInputs
	Parallel  bool
	ReapIdleClients func() int
	Shutdown  func(reason string)
}

// startGovernanceLoops keeps the registry entry fresh, reaps idle MCP clients, and
// shuts the daemon down once it has nothing left to serve.
func startGovernanceLoops(ctx context.Context, loops governanceLoops) {
	server, admission, port := loops.Server, loops.Admission, loops.Port
	inputs, parallel := loops.Inputs, loops.Parallel

	admission.StartHeartbeat(ctx, instancegov.DefaultHeartbeatInterval, func(err error) {
		// A silent heartbeat failure makes a healthy daemon look wedged and gets
		// it killed by the reaper, so it is reported, never swallowed.
		server.logLifecycle("instance_heartbeat_failed", port, map[string]any{"error": err.Error()})
	})

	startClientReaper(ctx, server, port, loops.ReapIdleClients)

	admission.StartIdleWatch(ctx, idleConfigFor(parallel, busyProbe(inputs)), func(reason string) {
		server.logLifecycle("daemon_idle_shutdown", port, map[string]any{
			"reason": reason, "parallel": parallel,
		})
		loops.Shutdown(reason)
	})
}

// startClientReaper drops MCP client sessions that have stopped polling. The
// registry has had a reaper since it was written; nothing ever ran it, so client
// state accumulated for the daemon's whole life.
func startClientReaper(ctx context.Context, server *Server, port int, reapIdleClients func() int) {
	if server == nil || reapIdleClients == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(clientReapInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if reaped := reapIdleClients(); reaped > 0 {
					server.logLifecycle("idle_clients_reaped", port, map[string]any{"count": reaped})
				}
			}
		}
	}()
}

// daemonBusyInputs binds the daemon's real work signals. Every probe is a closure
// over live state rather than a snapshot, so the idle watcher always reads the
// current value instead of one captured at boot.
func daemonBusyInputs(server *Server, cap *capture.Capture) busyInputs {
	return busyInputs{
		Clients: func() int {
			if cap == nil || cap.Clients() == nil || cap.Clients().Registry() == nil {
				return 0
			}
			return cap.Clients().Registry().Count()
		},
		ExtensionConnected: func() bool {
			return cap != nil && cap.Extension() != nil && cap.Extension().IsExtensionConnected()
		},
		ActiveRecording: func() bool {
			return cap != nil && cap.Recordings() != nil && cap.Recordings().ActiveRecordingID() != ""
		},
		TerminalSessions: func() int {
			if server == nil || server.ptyManager == nil {
				return 0
			}
			return server.ptyManager.Count()
		},
	}
}

// reapIdleClients drops MCP client sessions that stopped polling. clientreg has
// had this reaper since it was written and nothing ever called it, so per-client
// state accumulated for the daemon's entire life.
func reapIdleClients(cap *capture.Capture) int {
	if cap == nil || cap.Clients() == nil || cap.Clients().Registry() == nil {
		return 0
	}
	return cap.Clients().Registry().ReapIdle()
}

// requestSelfShutdown ends this daemon through the ordinary signal path, so idle
// shutdown runs exactly the same cleanup as Ctrl+C or --stop rather than a second
// teardown sequence that could drift from it.
func requestSelfShutdown(server *Server, port int, reason string) {
	diag.Printf("[Kaboom] Shutting down: %s\n", reason)
	process, err := os.FindProcess(os.Getpid())
	if err != nil {
		server.logLifecycle("idle_shutdown_failed", port, map[string]any{"error": err.Error()})
		return
	}
	if err := process.Signal(syscall.SIGTERM); err != nil {
		server.logLifecycle("idle_shutdown_failed", port, map[string]any{"error": err.Error()})
	}
}

// admitDaemonOrDefer runs the machine-wide gate and reports whether this process
// must exit cleanly because a daemon is already serving. Deferring is a normal
// outcome — a second launch is how MCP clients start Kaboom — so it exits 0 and
// says which daemon it is deferring to rather than the port it was asked for.
func admitDaemonOrDefer(server *Server, port int, opts daemonlife.LaunchOptions) (instancegov.Result, bool, error) {
	admission, err := admitDaemon(server, port, 0, opts)
	if err != nil {
		return instancegov.Result{}, false, err
	}
	if admission.Outcome != instancegov.OutcomeDefer {
		return admission, false, nil
	}
	server.logLifecycle("daemon_deferred_exit", port, map[string]any{"reason": admission.Reason})
	if admission.DeferTo != nil {
		diag.Printf("[Kaboom] A daemon is already serving this machine (pid=%d, ports=%v, version=%s); this instance is exiting.\n",
			admission.DeferTo.PID, admission.DeferTo.Ports, admission.DeferTo.Version)
	} else {
		diag.Printf("[Kaboom] A daemon is already serving this machine; this instance is exiting.\n")
	}
	return admission, true, nil
}

// startDaemonGovernance publishes this daemon's final port set and starts every
// background loop that bounds its lifetime: heartbeat, idle-client reaping, disk
// retention, and idle shutdown. They start together because they are one policy;
// starting them separately is how a heartbeat ends up running without the reaper
// that gives it meaning.
func startDaemonGovernance(ctx context.Context, gov daemonGovernance) {
	server, admission, port := gov.Server, gov.Admission, gov.Port
	cap := gov.Capture

	if terminalPort := gov.TerminalPort; terminalPort > 0 {
		if err := admission.Handle.SetPorts([]int{port, terminalPort}); err != nil {
			server.logLifecycle("instance_ports_publish_failed", port, map[string]any{"error": err.Error()})
		}
	}
	startRetentionSweeper(ctx, server, port)
	startGovernanceLoops(ctx, governanceLoops{
		Server: server, Admission: admission, Port: port,
		Inputs: daemonBusyInputs(server, cap), Parallel: gov.Options.Parallel,
		ReapIdleClients: func() int { return reapIdleClients(cap) },
		Shutdown:        func(reason string) { requestSelfShutdown(server, port, reason) },
	})
}

// daemonGovernance is the parameter set for starting a daemon's governance.
type daemonGovernance struct {
	Server       *Server
	Admission    *instancegov.Result
	Port         int
	TerminalPort int
	Capture      *capture.Capture
	Options      daemonlife.LaunchOptions
}
