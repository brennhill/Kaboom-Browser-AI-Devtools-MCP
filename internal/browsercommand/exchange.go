// exchange.go — Generation-gated application of one /sync heartbeat's command work.
// Why: superseded (old-generation) sync work must never mutate commands owned
// by the current connection; gating every mutation of an exchange through the
// connection owner makes that one atomic rule.
// Docs: docs/features/feature/query-service/index.md

package browsercommand

import (
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

// GenerationGate is supplied by the connection owner. It runs apply only while
// the exchange's generation is still current and must hold off generation
// changes until apply returns. It reports the current generation either way.
type GenerationGate func(apply func()) (current uint64, applied bool)

// Progress is one in-progress entry reported by the extension heartbeat.
type Progress struct {
	CorrelationID string
	Generation    uint64 // 0 inherits the exchange generation
}

// SyncExchange is the command work carried by one /sync heartbeat.
type SyncExchange struct {
	Generation uint64
	ClientID   string
	Results    []Result // Source is forced to SourceSync
	LastAck    string
	// Disconnected reports that the connection owner observed a liveness gap
	// before this heartbeat; undelivered and pending work is expired.
	Disconnected bool
	// InProgress is nil when the heartbeat carried no progress report.
	InProgress []Progress
	Now        time.Time
}

// StaleWork identifies a result or progress entry from a superseded generation.
type StaleWork struct {
	CorrelationID string
	Generation    uint64
}

// LostCommand is a started command the extension stopped reporting.
type LostCommand struct {
	CorrelationID string
	QueryID       string
}

// ExchangeReport describes what ApplySyncExchange did, for transport telemetry.
type ExchangeReport struct {
	Applied           bool
	CurrentGeneration uint64
	StaleResults      []StaleWork
	StaleProgress     []StaleWork
	Lost              []LostCommand
}

// MissingProgressGrace bounds how long a started command may be absent from
// in-progress heartbeats before it is failed as lost.
const MissingProgressGrace = 2 * time.Second

// LostCommandError is the terminal error recorded for a lost command.
const LostCommandError = "extension_lost_command: command acknowledged by extension but missing from in_progress heartbeats"

// ApplySyncExchange applies results, acknowledgment, disconnect expiry and
// missing-progress reconciliation for one heartbeat, all inside gate. If the
// generation was superseded nothing is mutated and Applied is false.
func (o *Owner) ApplySyncExchange(gate GenerationGate, exchange SyncExchange) ExchangeReport {
	var report ExchangeReport
	current, applied := gate(func() {
		report.StaleResults = o.applyExchangeResults(exchange)
		if exchange.LastAck != "" {
			o.queue.AcknowledgePendingQuery(exchange.LastAck)
		}
		if exchange.Disconnected {
			o.ExpireAll("extension_disconnected")
		}
		report.StaleProgress, report.Lost = o.reconcileProgress(exchange)
	})
	report.Applied = applied
	report.CurrentGeneration = current
	return report
}

func (o *Owner) applyExchangeResults(exchange SyncExchange) []StaleWork {
	var stale []StaleWork
	for _, result := range exchange.Results {
		result.Source = SourceSync
		result.ClientID = exchange.ClientID
		// EXPECTED_ABSENCE: generation 0 inherits the already-gated exchange
		// generation, so only an explicit mismatch is stale.
		if result.Generation != 0 && result.Generation != exchange.Generation {
			stale = append(stale, StaleWork{CorrelationID: result.CorrelationID, Generation: result.Generation})
			continue
		}
		o.apply(result)
	}
	return stale
}

// reconcileProgress fails started commands that disappear from heartbeat
// in_progress for longer than MissingProgressGrace without a terminal result.
func (o *Owner) reconcileProgress(exchange SyncExchange) ([]StaleWork, []LostCommand) {
	if exchange.InProgress == nil {
		return nil, nil
	}
	var stale []StaleWork
	active := make(map[string]struct{}, len(exchange.InProgress))
	for _, entry := range exchange.InProgress {
		if entry.Generation != 0 && entry.Generation != exchange.Generation {
			stale = append(stale, StaleWork{CorrelationID: entry.CorrelationID, Generation: entry.Generation})
			continue
		}
		if id := strings.TrimSpace(entry.CorrelationID); id != "" {
			active[id] = struct{}{}
		}
	}
	lost := o.collectLost(o.queue.GetPendingCommands(), active, exchange.Now)
	for _, command := range lost {
		o.apply(Result{Source: SourceDaemon, CorrelationID: command.CorrelationID, Status: "error", Error: LostCommandError})
	}
	return stale, lost
}

func (o *Owner) collectLost(pending []*queries.CommandResult, active map[string]struct{}, now time.Time) []LostCommand {
	o.mu.Lock()
	defer o.mu.Unlock()
	var lost []LostCommand
	stillPending := make(map[string]struct{}, len(pending))
	for _, command := range pending {
		if command == nil || command.CorrelationID == "" {
			continue
		}
		id := command.CorrelationID
		stillPending[id] = struct{}{}
		if _, ok := active[id]; ok {
			delete(o.missingSince, id)
			continue
		}
		if !commandHasStarted(command) {
			continue
		}
		since, seen := o.missingSince[id]
		if !seen {
			o.missingSince[id] = now
			continue
		}
		if now.Sub(since) >= MissingProgressGrace {
			lost = append(lost, LostCommand{CorrelationID: id, QueryID: command.QueryID})
			delete(o.missingSince, id)
		}
	}
	for id := range o.missingSince {
		if _, ok := stillPending[id]; !ok {
			delete(o.missingSince, id)
		}
	}
	return lost
}

func commandHasStarted(command *queries.CommandResult) bool {
	for _, event := range command.TraceEvents {
		switch event.Stage {
		case "started", "resolved", "errored", "timed_out":
			return true
		}
	}
	return strings.Contains(command.TraceTimeline, "started")
}
