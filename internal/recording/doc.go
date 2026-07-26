// Purpose: Package recording — user flow recording, disk persistence, and storage quota management.
// Why: Preserves replayable execution history for later replay and before/after comparison.
// Docs: docs/features/feature/playback-engine/index.md

/*
Package recording captures user flows, persists them to disk, and tracks
recording storage usage.

The two consumers of a stored recording live in subpackages, so this package
stays about producing and storing recordings rather than interpreting them:

  - internal/recording/playback: replays a recording action by action.
  - internal/recording/logdiff: compares two recordings for regressions.

Both read recordings through a one-method interface that *Manager satisfies
(LookupRecording and GetRecording respectively), so neither needs the manager
type and both can be tested without disk.

Key types:
  - Manager (RecordingManager): recording lifecycle (start/stop), disk persistence, storage quotas.
  - Item (Recording): a captured user flow with actions, viewport info, and metadata.
  - Action (RecordingAction): a single user action (click, type, navigate) with selector and coordinates.

Key functions:
  - NewRecordingManager: creates a manager with initialized state.
  - Manager.StartRecording / StopRecording: begin and finalize a capture.
  - Manager.GetRecording / LookupRecording: load from disk / prefer in-memory then disk.
*/
package recording
