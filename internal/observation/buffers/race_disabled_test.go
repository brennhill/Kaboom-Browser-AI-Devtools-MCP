// Purpose: Concurrency tests for ring buffer without race detector overhead.
// Docs: docs/features/feature/ring-buffer/index.md

//go:build !race

package buffers

const raceDetectorEnabled = false
