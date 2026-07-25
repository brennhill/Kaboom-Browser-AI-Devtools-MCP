// Purpose: Package queries — async command/query dispatch with correlation tracking and lifecycle tracing.
// Why: Coordinates async command flow between MCP tools and the browser extension under concurrency.
// Docs: docs/features/feature/query-service/index.md

/*
Package queries implements the asynchronous command dispatch system between the
Kaboom MCP server and the browser extension.

Key types:
  - QueryDispatcher: manages pending queries, results, expiration, and queue capacity.
  - PendingQuery: a command request queued for extension execution.
  - QueryResultEntry: a one-time consumable extension result with TTL cleanup.

Key functions:
  - NewQueryDispatcher: creates a dispatcher with configurable queue size and TTL.
  - CreateQuery: queues a new command for extension delivery.
  - TakeQueryResult: retrieves and consumes a one-time result by correlation ID.
  - normalizeCommandStatus: maps extension status values into canonical lifecycle states.
*/
package queries
