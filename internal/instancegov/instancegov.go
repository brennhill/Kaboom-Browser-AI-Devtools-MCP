// instancegov.go — The machine-wide admission sequence every instance runs before
// it binds anything.
// Why: the previous gate was a lock FILE scoped to a state directory, so it could
// neither see an instance in another state dir nor survive its own read-decide-write
// race. Here a kernel-held lock elects the single production daemon (no timestamps,
// no liveness heuristics, released automatically even on SIGKILL), an upgrade is a
// HANDOFF rather than a race, and test daemons are bounded by a counted cap that
// evicts the oldest instead of multiplying.
// Docs: docs/core/reliability/zombie-prevention.md

package instancegov

import (
	"errors"
	"fmt"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/instancereg"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/proclock"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/semver"
)

// Outcome is what the starting instance must do next.
type Outcome int

const (
	// OutcomeProceed: this instance holds its admission and may bind.
	OutcomeProceed Outcome = iota
	// OutcomeDefer: an incumbent is serving; exit 0 without binding.
	OutcomeDefer
)

func (o Outcome) String() string {
	if o == OutcomeDefer {
		return "defer"
	}
	return "proceed"
}

const (
	defaultHandoffTimeout = 5 * time.Second
	handoffPollInterval   = 50 * time.Millisecond
	// incumbentPublishWait bounds how long a losing instance waits for the winner
	// to publish who it is. The winner acquires the kernel lock and only then
	// registers and writes its payload, so an instance that loses by microseconds
	// reads an empty lock file. Waiting briefly turns "a daemon is already serving"
	// into a message that names the pid, ports, and version an operator can act on.
	incumbentPublishWait = 500 * time.Millisecond
	incumbentPollInterval = 25 * time.Millisecond
)

// Config describes the instance seeking admission.
type Config struct {
	Role         instancereg.Role
	Ports        []int
	StateDir     string
	Version      string
	InstallEpoch int64
	// Parallel marks an isolated test instance. Parallel instances never contend
	// for the production singleton lock — they are bounded by count, not by mutual
	// exclusion — so a test run can never lock a developer out of their daemon.
	Parallel bool
	// LockPath is the machine-wide production singleton lock.
	LockPath string
	Policy   Policy

	// HandoffTimeout bounds the wait for an incumbent to release the lock after
	// being asked to stand down.
	HandoffTimeout time.Duration
	// RequestShutdown asks an incumbent to exit for an upgrade. When nil, an
	// upgrade defers rather than taking over: without a way to ask politely, the
	// only alternative is killing a daemon that is serving someone.
	RequestShutdown func(instancereg.Record) error
	// Terminate ends an over-cap instance. Required only when a cap can be exceeded.
	Terminate func(pid int, force bool) error
	// Now supplies the clock. Defaults to time.Now.
	Now func() time.Time
	// Log records admission decisions.
	Log func(event string, fields map[string]any)
}

// Result is the admission outcome plus the resources it acquired.
type Result struct {
	Outcome Outcome
	DeferTo *instancereg.Record
	Handle  *instancereg.Handle
	Lock    *proclock.Lock
	Evicted []instancereg.Record
	Reason  string
}

// Release surrenders the registry entry and the singleton lock. Safe to call more
// than once and on a zero Result, so a caller can defer it unconditionally.
func (r *Result) Release() error {
	if r == nil {
		return nil
	}
	var failures []error
	if err := r.Handle.Close(); err != nil {
		failures = append(failures, err)
	}
	r.Handle = nil
	if r.Lock != nil {
		if err := r.Lock.Release(); err != nil {
			failures = append(failures, err)
		}
		r.Lock = nil
	}
	return errors.Join(failures...)
}

// Heartbeat republishes this instance's liveness. Callers must run it on a ticker;
// a stopped heartbeat is what marks an instance wedged for the reaper.
func (r *Result) Heartbeat() error {
	if r == nil {
		return nil
	}
	return r.Handle.Heartbeat()
}

func (c Config) now() time.Time {
	if c.Now == nil {
		return time.Now()
	}
	return c.Now()
}

func (c Config) log(event string, fields map[string]any) {
	if c.Log != nil {
		c.Log(event, fields)
	}
}

// Admit runs the full admission sequence and, on OutcomeProceed, returns a
// registered, lock-holding Result the caller must Release on shutdown.
func Admit(cfg Config) (Result, error) {
	if cfg.Parallel || cfg.Role == instancereg.RoleBridge {
		return admitCapped(cfg)
	}
	return admitSingleton(cfg)
}

// admitSingleton elects the one production daemon via a kernel-held lock.
func admitSingleton(cfg Config) (Result, error) {
	lock, err := proclock.Acquire(cfg.LockPath)
	if err == nil {
		return finish(cfg, lock, nil, "acquired the machine singleton lock")
	}
	if !errors.Is(err, proclock.ErrLocked) {
		return Result{}, err
	}

	incumbent := findIncumbent(cfg)
	if !shouldRequestHandoff(cfg, incumbent) {
		cfg.log("admission_defer", map[string]any{
			"reason": "a production daemon already holds the machine singleton lock",
		})
		return Result{Outcome: OutcomeDefer, DeferTo: incumbent,
			Reason: "a production daemon is already serving on this machine"}, nil
	}

	lock, err = requestHandoff(cfg, incumbent)
	if err != nil {
		return Result{Outcome: OutcomeDefer, DeferTo: incumbent,
			Reason: "upgrade handoff did not complete"}, err
	}
	return finish(cfg, lock, nil, "took over from an older daemon after a clean handoff")
}

// admitCapped admits an instance governed by a counted cap rather than by mutual
// exclusion, evicting the oldest peers when the machine is over budget. A capped
// candidate is never refused outright: a test run must always be able to start.
func admitCapped(cfg Config) (Result, error) {
	live, err := instancereg.Live()
	if err != nil {
		return Result{}, err
	}
	peers := peersOf(live, cfg.candidate())

	var members []instancereg.Record
	var cap int
	var kind string
	if cfg.Role == instancereg.RoleBridge {
		members, cap, kind = Bridges(peers), cfg.Policy.BridgeCap, "bridge"
	} else {
		members, cap, kind = Daemons(peers, true), cfg.Policy.ParallelCap, "parallel daemon"
	}

	// incoming=1: this candidate is joining, so it counts against the cap.
	victims := Surplus(members, cap, 1)
	if len(victims) == 0 {
		return finish(cfg, nil, nil, kind+" is within the machine cap")
	}
	if err := evict(cfg, victims); err != nil {
		return Result{}, err
	}
	return finish(cfg, nil, victims, kind+" count would exceed the machine cap")
}

func evict(cfg Config, victims []instancereg.Record) error {
	if len(victims) == 0 {
		return nil
	}
	if cfg.Terminate == nil {
		return fmt.Errorf("instancegov: %d instances exceed the machine cap but no Terminate function was provided", len(victims))
	}
	var failures []error
	for _, victim := range victims {
		cfg.log("admission_evict", map[string]any{
			"pid": victim.PID, "role": string(victim.Role), "ports": victim.Ports,
		})
		if err := cfg.Terminate(victim.PID, false); err != nil {
			failures = append(failures, fmt.Errorf("evict pid %d: %w", victim.PID, err))
		}
	}
	return errors.Join(failures...)
}

// finish registers the admitted instance. A registration failure releases the lock
// rather than proceeding unregistered: an instance the census cannot see is exactly
// the uncounted daemon this package exists to prevent.
func finish(cfg Config, lock *proclock.Lock, evicted []instancereg.Record, reason string) (Result, error) {
	handle, err := instancereg.Register(cfg.candidate())
	if err != nil {
		_ = lock.Release()
		return Result{}, fmt.Errorf("instancegov: register instance: %w", err)
	}
	if lock != nil {
		// Publish who holds the lock inside the lock file itself, so a census can
		// name the holder without a second file that could disagree with it.
		if payload, marshalErr := handle.RecordJSON(); marshalErr == nil {
			_ = lock.Write(payload)
		}
	}
	cfg.log("admission_proceed", map[string]any{"reason": reason, "evicted": len(evicted)})
	return Result{
		Outcome: OutcomeProceed, Handle: handle, Lock: lock,
		Evicted: evicted, Reason: reason,
	}, nil
}

func (c Config) candidate() instancereg.Record {
	return instancereg.Record{
		Role: c.Role, Ports: c.Ports, StateDir: c.StateDir,
		Version: c.Version, InstallEpoch: c.InstallEpoch, Parallel: c.Parallel,
	}
}

// findIncumbent reads who currently holds the singleton lock, waiting briefly for
// the holder to publish itself. It is diagnostic only: the lock, not this record,
// is the authority, so failing to identify the holder never changes the decision.
func findIncumbent(cfg Config) *instancereg.Record {
	deadline := cfg.now().Add(incumbentPublishWait)
	for {
		if rec := readIncumbentOnce(cfg); rec != nil {
			return rec
		}
		if !cfg.now().Before(deadline) {
			return nil
		}
		time.Sleep(incumbentPollInterval)
	}
}

// readIncumbentOnce prefers the lock's own payload — one file holding both the
// lock and the holder's identity cannot disagree with itself — and falls back to
// the registry for a holder that has registered but not yet written the payload.
func readIncumbentOnce(cfg Config) *instancereg.Record {
	if payload, err := proclock.ReadUnlocked(cfg.LockPath); err == nil {
		if rec, ok := instancereg.DecodeRecord(payload); ok {
			return &rec
		}
	}
	live, err := instancereg.Live()
	if err != nil {
		return nil
	}
	for _, rec := range live {
		if rec.Role == instancereg.RoleDaemon && !rec.Parallel {
			candidate := rec
			return &candidate
		}
	}
	return nil
}

// shouldRequestHandoff reports whether this build supersedes the incumbent.
//
// Two things supersede: a strictly newer VERSION, and — at the same version — a
// strictly newer INSTALL. The install tiebreaker exists because two same-version
// installs (an npm-global copy and ~/.kaboom/bin) otherwise have no way to pick a
// winner, so a fresh install could never displace the daemon its predecessor left
// running. Both comparisons are STRICT, which is what stops two daemons taking
// turns evicting each other: an equal or older build always defers.
func shouldRequestHandoff(cfg Config, incumbent *instancereg.Record) bool {
	if cfg.RequestShutdown == nil || incumbent == nil {
		return false
	}
	if incumbent.Version != "" && semver.IsNewer(cfg.Version, incumbent.Version) {
		return true
	}
	if !semver.Same(cfg.Version, incumbent.Version) {
		return false
	}
	// An unknown epoch on either side is not evidence of being newer.
	return cfg.InstallEpoch > 0 && cfg.InstallEpoch > incumbent.InstallEpoch
}

// requestHandoff asks the incumbent to stand down, then waits for the kernel lock
// to become free and takes it.
func requestHandoff(cfg Config, incumbent *instancereg.Record) (*proclock.Lock, error) {
	timeout := cfg.HandoffTimeout
	if timeout <= 0 {
		timeout = defaultHandoffTimeout
	}
	cfg.log("admission_handoff_requested", map[string]any{
		"incumbent_pid": incumbent.PID, "incumbent_version": incumbent.Version,
		"our_version": cfg.Version,
	})
	if err := cfg.RequestShutdown(*incumbent); err != nil {
		return nil, fmt.Errorf("instancegov: incumbent pid %d did not stand down: %w", incumbent.PID, err)
	}
	deadline := cfg.now().Add(timeout)
	for {
		lock, err := proclock.Acquire(cfg.LockPath)
		if err == nil {
			return lock, nil
		}
		if !errors.Is(err, proclock.ErrLocked) {
			return nil, err
		}
		if cfg.now().After(deadline) {
			return nil, fmt.Errorf(
				"instancegov: incumbent pid %d still holds the singleton lock %s after %s",
				incumbent.PID, cfg.LockPath, timeout)
		}
		time.Sleep(handoffPollInterval)
	}
}
