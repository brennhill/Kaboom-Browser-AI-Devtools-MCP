// admission.go — The machine-wide cap a starting instance must pass.
// Why: enforcing "one daemon" per state directory made the guarantee trivially
// true and globally meaningless — every isolated run was its own universe, so N
// runs meant N daemons and 2N ports with nothing counting them. Admission decides
// against the MACHINE census instead, and evicts the oldest test instances when a
// run would exceed the budget rather than letting the budget grow without bound.
// Docs: docs/core/reliability/zombie-prevention.md

package instancereg

import (
	"sort"
	"time"
)

// Outcome is what a starting instance must do.
type Outcome int

const (
	// OutcomeAdmit: proceed and bind.
	OutcomeAdmit Outcome = iota
	// OutcomeDefer: an incumbent is already serving; exit 0 without binding.
	OutcomeDefer
	// OutcomeEvict: terminate Decision.Evict, then proceed.
	OutcomeEvict
)

func (o Outcome) String() string {
	switch o {
	case OutcomeAdmit:
		return "admit"
	case OutcomeDefer:
		return "defer"
	case OutcomeEvict:
		return "evict"
	default:
		return "unknown"
	}
}

// Policy is the machine's instance budget.
type Policy struct {
	// DaemonCap is how many PRODUCTION daemons may run. It is 1 and is the whole
	// point of this package; it is a field only so tests can prove the arithmetic.
	DaemonCap int
	// ParallelCap is how many isolated test daemons may run concurrently.
	ParallelCap int
	// BridgeCap bounds stdio bridges, one of which exists per MCP client session.
	BridgeCap int
}

// DefaultPolicy is the production budget: exactly one daemon, a core-derived test
// allowance, and enough bridges for every plausible concurrent editor session.
func DefaultPolicy() Policy {
	return Policy{DaemonCap: 1, ParallelCap: AutoParallelCap(numCPU()), BridgeCap: 8}
}

// AutoParallelCap derives the concurrent test-daemon allowance from core count.
// Bounded at both ends: at least 2 so a two-shard suite never deadlocks against
// its own cap, and at most 4 because each daemon holds two ports and a full
// browser-facing runtime — a 64-core CI box must not be permitted 16 of them.
func AutoParallelCap(cpus int) int {
	const floor, ceiling = 2, 4
	derived := cpus / 4
	if derived < floor {
		derived = floor
	}
	if derived > ceiling {
		derived = ceiling
	}
	return derived
}

// Decision is the admission verdict. DeferTo is set for OutcomeDefer; Evict is
// set, oldest first, for OutcomeEvict.
type Decision struct {
	Outcome Outcome
	DeferTo *Record
	Evict   []Record
	Reason  string
}

// Decide applies the policy to the live census. The candidate is never selected
// as its own eviction target or defer target: a restarting instance that killed
// the entry it is about to replace would then have nothing to defer to.
func Decide(live []Record, candidate Record, policy Policy, now time.Time) Decision {
	peers := peersOf(live, candidate)

	if candidate.Role == RoleBridge {
		return capDecision(bridges(peers), policy.BridgeCap, "bridge")
	}
	if candidate.Parallel {
		return capDecision(daemons(peers, true), policy.ParallelCap, "parallel daemon")
	}

	// A production daemon defers to any live production daemon anywhere on the
	// machine — including one in a different state directory, which is precisely
	// the case the per-state-dir lock could not see.
	if incumbents := daemons(peers, false); len(incumbents) > 0 {
		oldest := oldestFirst(incumbents)[0]
		return Decision{
			Outcome: OutcomeDefer,
			DeferTo: &oldest,
			Reason:  "a production daemon is already serving on this machine",
		}
	}
	return Decision{Outcome: OutcomeAdmit, Reason: "no production daemon is running"}
}

// capDecision admits when the candidate fits within cap, and otherwise nominates
// the oldest peers for eviction so that admitting the candidate lands exactly at
// the cap.
func capDecision(peers []Record, cap int, kind string) Decision {
	if cap < 1 {
		cap = 1
	}
	if len(peers)+1 <= cap {
		return Decision{Outcome: OutcomeAdmit, Reason: kind + " is within the machine cap"}
	}
	ordered := oldestFirst(peers)
	surplus := len(peers) + 1 - cap
	if surplus > len(ordered) {
		surplus = len(ordered)
	}
	return Decision{
		Outcome: OutcomeEvict,
		Evict:   ordered[:surplus],
		Reason:  kind + " count would exceed the machine cap",
	}
}

func peersOf(live []Record, candidate Record) []Record {
	peers := make([]Record, 0, len(live))
	for _, rec := range live {
		if rec.PID == candidate.PID {
			continue
		}
		peers = append(peers, rec)
	}
	return peers
}

func daemons(records []Record, parallel bool) []Record {
	out := make([]Record, 0, len(records))
	for _, rec := range records {
		if rec.Role == RoleDaemon && rec.Parallel == parallel {
			out = append(out, rec)
		}
	}
	return out
}

func bridges(records []Record) []Record {
	out := make([]Record, 0, len(records))
	for _, rec := range records {
		if rec.Role == RoleBridge {
			out = append(out, rec)
		}
	}
	return out
}

// oldestFirst orders eviction candidates. A record whose start time cannot be
// parsed sorts LAST: a corrupt timestamp must not make an entry the automatic
// victim, because the one thing worse than an over-cap machine is killing the
// wrong process on unreadable evidence.
func oldestFirst(records []Record) []Record {
	ordered := make([]Record, len(records))
	copy(ordered, records)
	sort.SliceStable(ordered, func(i, j int) bool {
		left, leftOK := ordered[i].Started()
		right, rightOK := ordered[j].Started()
		if leftOK != rightOK {
			return leftOK
		}
		if !leftOK && !rightOK {
			return ordered[i].PID < ordered[j].PID
		}
		if left.Equal(right) {
			return ordered[i].PID < ordered[j].PID
		}
		return left.Before(right)
	})
	return ordered
}

// OldestFirst exposes the eviction ordering to the reaper so that admission and
// reclamation cannot disagree about which instance is the oldest.
func OldestFirst(records []Record) []Record { return oldestFirst(records) }
