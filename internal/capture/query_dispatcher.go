// Purpose: Delegates capture query and command lifecycle methods to the query dispatcher.
// Why: Keeps Capture's cohesive telemetry API while query state remains owned by internal/queries.
// Docs: docs/features/feature/query-service/index.md

package capture

import (
	"encoding/json"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

// ============================================================================
// Capture delegation methods.
// ============================================================================

// CreatePendingQuery delegates to QueryDispatcher.
func (c *Capture) CreatePendingQuery(query queries.PendingQuery) (string, error) {
	return c.queryDispatcher.CreatePendingQuery(query)
}

// CreatePendingQueryWithClient delegates to QueryDispatcher.
func (c *Capture) CreatePendingQueryWithClient(query queries.PendingQuery, clientID string) (string, error) {
	return c.queryDispatcher.CreatePendingQueryWithClient(query, clientID)
}

// CreatePendingQueryWithTimeout delegates to QueryDispatcher.
func (c *Capture) CreatePendingQueryWithTimeout(query queries.PendingQuery, timeout time.Duration, clientID string) (string, error) {
	return c.queryDispatcher.CreatePendingQueryWithTimeout(query, timeout, clientID)
}

// GetPendingQueries delegates to QueryDispatcher.
func (c *Capture) GetPendingQueries() []queries.PendingQueryResponse {
	return c.queryDispatcher.GetPendingQueries()
}

// GetPendingQueriesForClient delegates to QueryDispatcher.
func (c *Capture) GetPendingQueriesForClient(clientID string) []queries.PendingQueryResponse {
	return c.queryDispatcher.GetPendingQueriesForClient(clientID)
}

// WaitForPendingQueries delegates to QueryDispatcher.
func (c *Capture) WaitForPendingQueries(timeout time.Duration) {
	c.queryDispatcher.WaitForPendingQueries(timeout)
}

// AcknowledgePendingQuery delegates to QueryDispatcher.
func (c *Capture) AcknowledgePendingQuery(queryID string) {
	c.queryDispatcher.AcknowledgePendingQuery(queryID)
}

// GetPendingQueriesDisconnectAware returns pending queries with disconnect reconciliation.
// If the extension has not synced within extensionDisconnectThreshold (10s) and has
// synced at least once, all pending queries are expired with "extension_disconnected".
// This prevents queries from hanging indefinitely when the extension crashes or disconnects.
//
// Failure semantics:
// - On detected disconnect, pending commands are force-expired and nil is returned.
// - On healthy connection, behavior matches GetPendingQueries.
func (c *Capture) GetPendingQueriesDisconnectAware() []queries.PendingQueryResponse {
	c.mu.RLock()
	neverSynced := c.extensionState.lastSyncSeen.IsZero()
	disconnected := !neverSynced && time.Since(c.extensionState.lastSyncSeen) >= extensionDisconnectThreshold
	c.mu.RUnlock()

	// If extension was previously connected but is now stale, expire all pending queries
	if disconnected {
		c.queryDispatcher.ExpireAllPendingQueries("extension_disconnected")
		return nil
	}

	return c.queryDispatcher.GetPendingQueries()
}

// SetQueryResult delegates to QueryDispatcher.
func (c *Capture) SetQueryResult(id string, result json.RawMessage) {
	c.queryDispatcher.SetQueryResult(id, result)
}

// SetQueryResultWithClient delegates to QueryDispatcher.
func (c *Capture) SetQueryResultWithClient(id string, result json.RawMessage, clientID string) {
	c.queryDispatcher.SetQueryResultWithClient(id, result, clientID)
}

// SetQueryResultWithClientNoCommandComplete delegates to QueryDispatcher while
// preserving command lifecycle status (no implicit "complete" transition).
func (c *Capture) SetQueryResultWithClientNoCommandComplete(id string, result json.RawMessage, clientID string) {
	c.queryDispatcher.SetQueryResultWithClientNoCommandComplete(id, result, clientID)
}

// TakeQueryResult delegates to QueryDispatcher.
func (c *Capture) TakeQueryResult(id string) (json.RawMessage, bool) {
	return c.queryDispatcher.TakeQueryResult(id)
}

// TakeQueryResultForClient delegates to QueryDispatcher.
func (c *Capture) TakeQueryResultForClient(id string, clientID string) (json.RawMessage, bool) {
	return c.queryDispatcher.TakeQueryResultForClient(id, clientID)
}

// WaitForResult delegates to QueryDispatcher.
func (c *Capture) WaitForResult(id string, timeout time.Duration) (json.RawMessage, error) {
	return c.queryDispatcher.WaitForResult(id, timeout)
}

// WaitForResultWithClient delegates to QueryDispatcher.
func (c *Capture) WaitForResultWithClient(id string, timeout time.Duration, clientID string) (json.RawMessage, error) {
	return c.queryDispatcher.WaitForResultWithClient(id, timeout, clientID)
}

// SetQueryTimeout delegates to QueryDispatcher.
func (c *Capture) SetQueryTimeout(timeout time.Duration) {
	c.queryDispatcher.SetQueryTimeout(timeout)
}

// GetQueryTimeout delegates to QueryDispatcher.
func (c *Capture) GetQueryTimeout() time.Duration {
	return c.queryDispatcher.GetQueryTimeout()
}

// RegisterCommand delegates to QueryDispatcher.
func (c *Capture) RegisterCommand(correlationID string, queryID string, timeout time.Duration) {
	c.queryDispatcher.RegisterCommand(correlationID, queryID, timeout)
}

// ApplyCommandResult delegates status-aware command updates to QueryDispatcher.
func (c *Capture) ApplyCommandResult(correlationID string, status string, result json.RawMessage, err string) {
	c.queryDispatcher.ApplyCommandResult(correlationID, status, result, err)
}

// ExpireCommand delegates to QueryDispatcher.
func (c *Capture) ExpireCommand(correlationID string) {
	c.queryDispatcher.ExpireCommand(correlationID)
}

// WaitForCommand delegates to QueryDispatcher.
func (c *Capture) WaitForCommand(correlationID string, timeout time.Duration) (*queries.CommandResult, bool) {
	return c.queryDispatcher.WaitForCommand(correlationID, timeout)
}

// GetCommandResult delegates to QueryDispatcher.
func (c *Capture) GetCommandResult(correlationID string) (*queries.CommandResult, bool) {
	return c.queryDispatcher.GetCommandResult(correlationID)
}

// GetPendingCommands delegates to QueryDispatcher.
func (c *Capture) GetPendingCommands() []*queries.CommandResult {
	return c.queryDispatcher.GetPendingCommands()
}

// GetCompletedCommands delegates to QueryDispatcher.
func (c *Capture) GetCompletedCommands() []*queries.CommandResult {
	return c.queryDispatcher.GetCompletedCommands()
}

// GetFailedCommands delegates to QueryDispatcher.
func (c *Capture) GetFailedCommands() []*queries.CommandResult {
	return c.queryDispatcher.GetFailedCommands()
}

// GetRecentCommandTraces returns the latest command traces for diagnostics.
func (c *Capture) GetRecentCommandTraces(limit int) []*queries.CommandResult {
	return c.queryDispatcher.GetRecentCommandTraces(limit)
}

// QueuePosition delegates to QueryDispatcher.
func (c *Capture) QueuePosition(correlationID string) int {
	return c.queryDispatcher.QueuePosition(correlationID)
}

// QueueDepth delegates to QueryDispatcher.
func (c *Capture) QueueDepth() int {
	return c.queryDispatcher.QueueDepth()
}
