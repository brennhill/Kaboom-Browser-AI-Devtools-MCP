// Purpose: Core QueryDispatcher struct: manages pending queries, results, expiration, and queue capacity.
// Docs: docs/features/feature/query-service/index.md

// Package dispatch stores queued browser commands, query results and async
// command records. It is internal to internal/browsercommand: Owner is the only
// caller, so every state transition goes through the owner's policies.
package dispatch

import (
	"encoding/json"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/browsercommand/query"
	"sync"
	"time"
)

// PendingQueryEntry tracks a queued extension command.
//
// Invariants:
// - Query.ID is unique within this process and remains stable until acknowledged/expired.
// - Expires is an absolute deadline; once passed, queue cleanup treats the entry as non-deliverable.
// - ClientID is empty for single-client mode, non-empty for multi-client isolation.
type PendingQueryEntry struct {
	Query     queries.PendingQueryResponse
	CreatedAt time.Time
	Expires   time.Time
	ClientID  string // owning client for multi-client isolation
}

// QueryResultEntry stores a one-time consumable extension result.
//
// Invariants:
// - Result is deleted on successful TakeQueryResult* read to avoid stale replay.
// - ClientID must match the originating request in multi-client mode.
// - CreatedAt drives TTL-based cleanup and must reflect first insertion time.
type QueryResultEntry struct {
	Result    json.RawMessage
	ClientID  string // owning client for multi-client isolation
	CreatedAt time.Time
}

// QueryDispatcher manages pending query queues, result storage, and async command tracking.
// Owns two locks:
//   - mu (sync.Mutex): protects pendingQueries, queryResults, queryCond, queryIDCounter, queryTimeout, queryNotify
//   - resultsMu (sync.RWMutex): protects activeCommands, terminalHistory
//
// Lock ordering: mu released BEFORE resultsMu acquired (never reverse).
//
// Invariants:
// - pendingQueries is FIFO and bounded by queries.MaxPendingQueries.
// - commandNotify is always non-nil; writers close-and-rotate it under resultsMu to signal waiters.
// - queryNotify is always non-nil; enqueuers close-and-rotate it under mu so ALL waiters wake.
// - terminalHistory is a five-entry ring shared by successful and failed terminal commands.
//
// Failure semantics:
// - Queue saturation rejects new work with queries.ErrQueueFull instead of dropping existing entries.
// - Terminal command states are monotonic; once non-pending, updates are ignored.
// - Cleanup can expire orphaned commands/results, favoring bounded memory over indefinite retention.
type QueryDispatcher struct {
	mu             sync.Mutex
	pendingQueries []PendingQueryEntry
	queryResults   map[string]QueryResultEntry
	queryCond      *sync.Cond
	queryIDCounter int
	queryIDPrefix  string
	queryTimeout   time.Duration

	resultsMu       sync.RWMutex
	activeCommands  map[string]*queries.CommandResult
	terminalHistory []*queries.CommandResult
	commandNotify   chan struct{} // closed on a terminal ApplyCommandResult, then recreated
	queryNotify     chan struct{} // closed when pending queries are enqueued, then recreated (protected by mu)

	cleanupStop     chan struct{}
	cleanupDone     chan struct{}
	cleanupStopOnce sync.Once
}

// NewQueryDispatcher creates a dispatcher with active cleanup lifecycle.
//
// Failure semantics:
// - If callers never invoke Close, periodic cleanup goroutine continues for process lifetime.
// - Constructor never returns a partially initialized dispatcher.
func NewQueryDispatcher() *QueryDispatcher {
	qd := &QueryDispatcher{
		pendingQueries:  make([]PendingQueryEntry, 0),
		queryResults:    make(map[string]QueryResultEntry),
		queryIDPrefix:   newQueryIDPrefix(),
		queryTimeout:    queries.DefaultQueryTimeout,
		activeCommands:  make(map[string]*queries.CommandResult),
		terminalHistory: make([]*queries.CommandResult, 0, terminalCommandHistoryLimit),
		commandNotify:   make(chan struct{}),
		queryNotify:     make(chan struct{}),
		cleanupStop:     make(chan struct{}),
		cleanupDone:     make(chan struct{}),
	}
	qd.queryCond = sync.NewCond(&qd.mu)
	qd.startResultCleanup()
	return qd
}

// Close stops cleanup background work.
//
// Invariants:
// - Idempotent: repeated calls after the first are no-ops.
//
// Failure semantics:
// - Does not clear in-memory queues/results; it only ends cleanup lifecycle.
// - Waiters continue using timeout semantics after close.
func (qd *QueryDispatcher) Close() {
	qd.cleanupStopOnce.Do(func() { close(qd.cleanupStop) })
	<-qd.cleanupDone
}

// GetSnapshot returns a thread-safe snapshot of query state.
func (qd *QueryDispatcher) GetSnapshot() queries.QuerySnapshot {
	qd.mu.Lock()
	defer qd.mu.Unlock()
	snapshot := queries.QuerySnapshot{
		PendingQueryCount: len(qd.pendingQueries),
		QueryResultCount:  len(qd.queryResults),
		PendingCapacity:   queries.MaxPendingQueries,
		QueryTimeout:      qd.queryTimeout,
	}
	if len(qd.pendingQueries) > 0 && !qd.pendingQueries[0].CreatedAt.IsZero() {
		snapshot.OldestPendingAge = time.Since(qd.pendingQueries[0].CreatedAt)
	}
	return snapshot
}
