// Purpose: Package playback — replays a stored recording action by action and reports per-action outcomes.
// Why: Replay is a consumer of recordings, not part of recording capture; keeping it separate keeps the manager focused.
// Docs: docs/features/feature/playback-engine/index.md

/*
Package playback replays recorded user flows.

It reads recordings through the one-method RecordingSource interface (satisfied
by *recording.RecordingManager) so replay can be exercised without touching disk.

Layout:
  - types.go: Session/Result/Coordinates
  - session.go: session lifecycle (Start, Execute) and Status projection
  - actions.go: per-action execution and click selector-healing strategies
  - fragile.go: cross-session selector fragility analysis

Key functions:
  - Start: opens a session for a recording, rejecting missing or empty recordings.
  - Execute: opens a session and replays every action into it.
  - Status: summarizes a finished session for the MCP response.
  - DetectFragileSelectors: flags selectors that fail in more than half their runs.
*/
package playback
