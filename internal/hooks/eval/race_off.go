//go:build !race

// race_off.go — Normal (non-race) builds enforce latency budgets as written.

package eval

const raceDetectorActive = false
