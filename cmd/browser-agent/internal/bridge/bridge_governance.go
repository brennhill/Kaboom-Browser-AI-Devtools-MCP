// bridge_governance.go — Registers a bridge in the machine census and ends it when
// its MCP client is gone.
// Why: bridges outnumber daemons roughly 11:1 in this project's lifecycle logs, one
// per client session, and nothing counted or bounded them. stdin EOF ends the clean
// case; a client that is SIGKILLed never closes the pipe, so the bridge also watches
// for its parent disappearing.
// Docs: docs/core/reliability/zombie-prevention.md

package bridge

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/instancegov"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/instancereg"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/procwatch"
)

// bridgeGovernance describes the bridge seeking admission. The pid seams exist so
// the parent-death path is testable without spawning and killing real processes.
type bridgeGovernance struct {
	Version      string
	Port         int
	OriginalPPID int
	ParentPoll   time.Duration
	CurrentPPID  func() int
}

// governBridge registers this bridge, keeps its heartbeat fresh, and watches for
// its client's death. It returns a release function the caller must invoke on exit.
//
// The returned function is idempotent, and governBridge calls it ITSELF before
// invoking standDown. Handing the caller a release it must remember to call from
// the stand-down path would make that a shared variable written on one goroutine
// and read on another — a data race, and one call site away from a bridge that
// exits while still listed in the census.
//
// A registry problem is never fatal here: the census is bookkeeping, and refusing
// to serve an MCP session because a bookkeeping file could not be written would
// turn a diagnostic failure into an outage.
func governBridge(ctx context.Context, cfg bridgeGovernance, standDown func(reason string)) func() {
	admission, err := instancegov.Admit(instancegov.Config{
		Role:    instancereg.RoleBridge,
		Ports:   []int{cfg.Port},
		Version: cfg.Version,
		Policy:  instancereg.DefaultPolicy(),
		// A bridge over the cap is evicted by the reaper, not by its successor:
		// killing another editor's live session to start your own would be worse
		// than the leak this bounds.
		Terminate: func(int, bool) error { return nil },
	})
	if err != nil {
		return func() {}
	}
	if admission.Outcome != instancegov.OutcomeProceed {
		return func() { _ = admission.Release() }
	}

	admission.StartHeartbeat(ctx, instancegov.DefaultHeartbeatInterval, nil)

	var once sync.Once
	release := func() { once.Do(func() { _ = admission.Release() }) }

	ppid := cfg.OriginalPPID
	if ppid == 0 {
		ppid = os.Getppid()
	}
	go procwatch.Watch(ctx, procwatch.Config{
		OriginalPPID: ppid,
		Poll:         cfg.ParentPoll,
		CurrentPPID:  cfg.CurrentPPID,
	}, func(reason string) {
		// Leave the census before standing down, so a bridge is never listed as
		// live while it is on its way out.
		release()
		if standDown != nil {
			standDown(reason)
		}
	})

	return release
}
