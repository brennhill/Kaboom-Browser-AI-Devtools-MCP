//go:build race

// race_on.go — Marks builds compiled with the race detector (-race).
// Used to skip wall-clock latency budgets, which race instrumentation invalidates.

package eval

const raceDetectorActive = true
