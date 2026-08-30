// policy.go — The machine's instance budget and the shared predicates that apply it.
// Why: admission and reclamation ask the same three questions — is this instance
// wedged, which peers are oldest, how many exceed the cap — and each previously
// answered them separately. The wedged answers actually DIFFERED: one fell back to
// start time when a heartbeat was unparseable and the other did not, so the same
// record could be healthy to the reaper and wedged to the census. One definition
// each, used by every caller.

package instancegov

import (
	"sort"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/instancereg"
)

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
	return Policy{DaemonCap: 1, ParallelCap: autoParallelCap(numCPU()), BridgeCap: 8}
}

// autoParallelCap derives the concurrent test-daemon allowance from core count.
// Bounded at both ends: at least 2 so a two-shard suite never deadlocks against
// its own cap, and at most 4 because each daemon holds two ports and a full
// browser-facing runtime — a 64-core CI box must not be permitted 16 of them.
func autoParallelCap(cpus int) int {
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

// IsWedged reports whether an instance is alive but no longer updating the
// registry, which means it still holds its ports while nothing is driving it.
//
// A record whose heartbeat cannot be read falls back to its start time: an entry
// with no readable liveness that has existed longer than the TTL is wedged by any
// useful definition, and treating it as healthy is how one would survive forever.
func IsWedged(rec instancereg.Record, now time.Time, ttl time.Duration) bool {
	if age, ok := rec.HeartbeatAge(now); ok {
		return age > ttl
	}
	started, ok := rec.Started()
	return ok && now.Sub(started) > ttl
}

// oldestFirst orders instances for eviction. Unexported: callers ask Surplus for
// WHO must go, which is the decision; handing out the ordering invites a second
// caller to re-derive that decision and disagree about the cap arithmetic.
//
// A record whose start time cannot be parsed sorts LAST: a corrupt timestamp must
// not make an entry the automatic victim, because the one thing worse than an
// over-cap machine is killing the wrong process on unreadable evidence.
func oldestFirst(records []instancereg.Record) []instancereg.Record {
	ordered := make([]instancereg.Record, len(records))
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

// Surplus returns the instances that must go for `members` to fit within cap,
// oldest first. `incoming` is 1 when a candidate is seeking admission alongside
// these members and 0 when merely reclaiming what is already running — the only
// difference between the admission and reaper cases, which is why they share this.
func Surplus(members []instancereg.Record, cap, incoming int) []instancereg.Record {
	if cap < 1 {
		cap = 1
	}
	over := len(members) + incoming - cap
	if over <= 0 {
		return nil
	}
	ordered := oldestFirst(members)
	if over > len(ordered) {
		over = len(ordered)
	}
	return ordered[:over]
}

// Daemons selects daemon records of one kind (production or parallel).
func Daemons(records []instancereg.Record, parallel bool) []instancereg.Record {
	out := make([]instancereg.Record, 0, len(records))
	for _, rec := range records {
		if rec.Role == instancereg.RoleDaemon && rec.Parallel == parallel {
			out = append(out, rec)
		}
	}
	return out
}

// Bridges selects bridge records.
func Bridges(records []instancereg.Record) []instancereg.Record {
	out := make([]instancereg.Record, 0, len(records))
	for _, rec := range records {
		if rec.Role == instancereg.RoleBridge {
			out = append(out, rec)
		}
	}
	return out
}

func peersOf(live []instancereg.Record, candidate instancereg.Record) []instancereg.Record {
	peers := make([]instancereg.Record, 0, len(live))
	for _, rec := range live {
		if rec.PID == candidate.PID {
			continue
		}
		peers = append(peers, rec)
	}
	return peers
}
