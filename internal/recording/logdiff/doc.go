// Purpose: Package logdiff — compares two recordings to surface regressions, fixes, and changed values.
// Why: Diffing is a consumer of recordings, not part of recording capture; separating it keeps the manager focused.
// Docs: docs/features/feature/playback-engine/index.md

/*
Package logdiff compares an original recording against a replay recording and
reports what changed between the two runs.

It reads recordings through the one-method RecordingSource interface (satisfied
by *recording.Manager) so diffing can be exercised without touching disk.

Layout:
  - types.go: Result and its constituent entry types
  - compare.go: diff orchestration and the regression/fix/value-change detectors
  - helpers.go: action-count and typed-value helpers shared by the detectors
  - report.go: human-readable regression report rendering

Key functions:
  - Compare: loads both recordings and produces the diff Result.
  - Result.GetRegressionReport: renders the diff as report text.
*/
package logdiff
