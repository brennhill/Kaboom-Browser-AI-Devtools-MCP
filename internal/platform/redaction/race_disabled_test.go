// race_disabled_test.go — Build-tagged constant for non-race builds.

//go:build !race

package redaction

const raceEnabled = false
