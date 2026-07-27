---
doc_type: tech-spec
feature_id: feature-playback-engine
status: proposed
last_reviewed: 2026-07-27
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Playback Engine Tech Spec

## Architecture
- Recording data types and the manager that produces/persists them: `internal/recording/`
- Replay session/result types and the `RecordingSource` seam: `internal/recording/playback/types.go`
- Action execution helpers: `internal/recording/playback/actions.go`
- Session/runtime management: `internal/recording/playback/session.go`
- Selector fragility analysis: `internal/recording/playback/fragile.go`
- Recording comparison and reports: `internal/recording/logdiff/`
- Delegation surface: `internal/capture/handlers.go`
- MCP configure/observe bridges: `cmd/browser-agent/tools_configure.go` and `cmd/browser-agent/internal/toolobserve/dispatcher.go`
- Runtime construction seams: `cmd/browser-agent/tools_core_constructor.go`
- Recording and playback handler implementation: `cmd/browser-agent/internal/toolrecording/handler.go`

## Constraints
- Playback execution must remain bounded and interruptible.
- Error policy (`continue`, `stop`, dependency skip) must be explicit.
- Replay output should include enough per-step evidence for debugging.
