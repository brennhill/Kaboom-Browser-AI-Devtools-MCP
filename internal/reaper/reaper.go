// reaper.go — Decides and applies what to reclaim across the machine.
// Why: nothing enumerated Kaboom processes machine-wide, so the only global tool
// was `--force`, a blunt `pkill -f "kaboom.*--daemon"` that killed healthy daemons
// too and ran only at install time. Planning is a pure function of the census so
// every decision is testable without real processes, and so the one outcome that
// must never happen — terminating a healthy daemon — is provable rather than hoped.
// Docs: docs/core/reliability/zombie-prevention.md

package reaper

import (
	"errors"
	"fmt"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/instancegov"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/instancereg"
)

// ActionKind is what should happen to one registry entry.
type ActionKind int

const (
	// ActionPrune removes a registry entry whose process is already gone. It
	// terminates nothing.
	ActionPrune ActionKind = iota
	// ActionKill terminates a live process and then removes its entry.
	ActionKill
)

func (k ActionKind) String() string {
	if k == ActionKill {
		return "kill"
	}
	return "prune"
}

// Action is one planned reclamation.
type Action struct {
	Record instancereg.Record
	Kind   ActionKind
	Reason string
}

// Input is the census the plan is computed from. Live must be the subset of All
// whose processes are running as themselves (identity-checked, not pid-checked).
type Input struct {
	Live         []instancereg.Record
	All          []instancereg.Record
	Policy       instancegov.Policy
	HeartbeatTTL time.Duration
	Now          time.Time
}

// Report is the plan plus what it deliberately leaves alone.
type Report struct {
	Actions []Action
	Keep    []instancereg.Record
}

// Plan computes the reclamation set. It never plans anything against an instance
// that is both alive and heartbeating within the TTL and within its cap.
func Plan(in Input) Report {
	liveByPID := make(map[int]bool, len(in.Live))
	for _, rec := range in.Live {
		liveByPID[rec.PID] = true
	}

	planned := make(map[int]bool)
	report := Report{}

	// Dead entries: the process is gone, so only the bookkeeping remains.
	for _, rec := range in.All {
		if liveByPID[rec.PID] {
			continue
		}
		report.Actions = append(report.Actions, Action{
			Record: rec, Kind: ActionPrune,
			Reason: "the process that wrote this entry is no longer running",
		})
		planned[rec.PID] = true
	}

	// Wedged: alive, holding ports, no longer heartbeating. The predicate is
	// shared with the census so the two can never disagree about one record.
	for _, rec := range in.Live {
		if planned[rec.PID] {
			continue
		}
		if !instancegov.IsWedged(rec, in.Now, in.HeartbeatTTL) {
			continue
		}
		age, _ := rec.HeartbeatAge(in.Now)
		report.Actions = append(report.Actions, Action{
			Record: rec, Kind: ActionKill,
			Reason: fmt.Sprintf("alive but has not heartbeat for %s (limit %s); ports %v are still held",
				age.Round(time.Second), in.HeartbeatTTL, rec.Ports),
		})
		planned[rec.PID] = true
	}

	report.Actions = append(report.Actions, overCapActions(in, planned)...)

	for _, rec := range in.Live {
		if !planned[rec.PID] {
			report.Keep = append(report.Keep, rec)
		}
	}
	return report
}

// overCapActions selects the oldest instances to terminate in each capped class.
// Production daemons are excluded: the DaemonCap is enforced at admission, where
// the LATE arrival defers. Reclaiming it here instead would let a newly started
// daemon kill the incumbent a developer is actively using.
func overCapActions(in Input, planned map[int]bool) []Action {
	var actions []Action
	var unplanned []instancereg.Record
	for _, rec := range in.Live {
		if !planned[rec.PID] {
			unplanned = append(unplanned, rec)
		}
	}
	classes := []struct {
		name    string
		cap     int
		members []instancereg.Record
	}{
		{"parallel daemon", in.Policy.ParallelCap, instancegov.Daemons(unplanned, true)},
		{"bridge", in.Policy.BridgeCap, instancegov.Bridges(unplanned)},
	}
	for _, class := range classes {
		// incoming=0: nothing is joining here, we are reclaiming what already runs.
		// That single argument is the whole difference from the admission case,
		// which is why both now call one function instead of two near-copies.
		for _, victim := range instancegov.Surplus(class.members, class.cap, 0) {
			actions = append(actions, Action{
				Record: victim, Kind: ActionKill,
				Reason: fmt.Sprintf("%d %ss exceed the machine cap of %d; this is the oldest",
					len(class.members), class.name, class.cap),
			})
			planned[victim.PID] = true
		}
	}
	return actions
}

// Deps are the effects Apply performs. Both are required so that no call site can
// accidentally run a reap with a silent no-op terminator.
type Deps struct {
	// DryRun plans and reports without terminating or removing anything.
	DryRun bool
	// Terminate signals a pid. force selects SIGKILL over SIGTERM.
	Terminate func(pid int, force bool) error
	// Remove deletes a registry entry at path.
	Remove func(path string) error
}

// Result counts what Apply did (or, in a dry run, would do).
type Result struct {
	Killed int
	Pruned int
}

// Apply executes a plan. A termination failure is returned rather than swallowed:
// a kill that silently failed leaves the port held while the census reports it
// reclaimed, which is precisely the class of invisible failure that let twelve
// daemons survive twenty hours of green test runs.
func Apply(report Report, deps Deps) (Result, error) {
	var result Result
	var failures []error

	for _, action := range report.Actions {
		if action.Kind == ActionKill {
			if !deps.DryRun {
				if deps.Terminate == nil {
					return result, errors.New("reaper: Apply requires a Terminate function")
				}
				if err := deps.Terminate(action.Record.PID, false); err != nil {
					failures = append(failures, fmt.Errorf(
						"terminate pid %d (%s): %w", action.Record.PID, action.Record.Role, err))
					continue
				}
			}
			result.Killed++
		} else {
			result.Pruned++
		}
		if deps.DryRun || action.Record.Path == "" {
			continue
		}
		if deps.Remove == nil {
			return result, errors.New("reaper: Apply requires a Remove function")
		}
		if err := deps.Remove(action.Record.Path); err != nil {
			failures = append(failures, fmt.Errorf("remove record %s: %w", action.Record.Path, err))
		}
	}
	return result, errors.Join(failures...)
}
