//go:build !race

// race_disabled.go — Marks ordinary integration timing policy as non-race-instrumented.

package integrationtest

const raceEnabled = false
