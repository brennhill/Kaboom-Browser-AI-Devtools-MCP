// Purpose: Package streaming — the MCP notification stream: filtering, throttling, dedup and emission.
// Why: Enables real-time notification of browser events without flooding consumers with duplicate noise.
// Docs: docs/features/feature/push-alerts/index.md

/*
Package streaming owns the outbound MCP notification stream: what an alert must
look like to be pushed, how often, and in what format.

Key types:
  - StreamState: configuration and runtime state for streaming (enabled, events, throttle).
  - StreamConfig: user-configurable streaming parameters (events, throttle, severity).
  - MCPNotification: the notifications/message envelope written to the stream writer.

Key functions:
  - NewStreamState: creates a stream state with default configuration.
  - EmitAlert: filters, rate-limits, deduplicates, and writes an alert notification.
  - SeverityRank: severity ordering used by the severity_min filter.

The alert producer side — the bounded buffer, CI materialization, anomaly
detection and alert post-processing — lives in the alertbuf subpackage, which
imports this one. The arrow is one-way: streaming never imports alertbuf.
*/
package streaming
