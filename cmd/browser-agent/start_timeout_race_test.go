//go:build race

// start_timeout_race_test.go — Server-startup poll budget under the race detector.
// Why: `go test -race ./cmd/browser-agent/... ./internal/...` runs package test
// binaries in parallel; race instrumentation plus that cross-package contention can
// push a spawned server's health-ready time well past the normal budget. WaitForServer
// returns as soon as the server is up, so a larger ceiling only affects genuine
// failures — never the happy path.

package main

import "time"

const (
	serverStartTimeout       = 30 * time.Second
	coverageInstrumentedTest = true
)

func integrationResponseTimeout(time.Duration) time.Duration {
	return 30 * time.Second
}
