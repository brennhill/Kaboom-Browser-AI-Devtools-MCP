---
doc_type: tech-spec
feature_id: feature-playback-engine
status: proposed
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Playback Engine Tech Spec

## Architecture
- Core types and orchestration: `internal/recording/playback.go`, `internal/recording/playback_engine_types.go`
- Action execution helpers: `internal/recording/playback_engine_actions.go`
- Session/runtime management: `internal/recording/playback_engine_session.go`
- CLI handler bridge: `cmd/browser-agent/recording_handlers_playback.go`

## Constraints
- Playback execution must remain bounded and interruptible.
- Error policy (`continue`, `stop`, dependency skip) must be explicit.
- Replay output should include enough per-step evidence for debugging.
