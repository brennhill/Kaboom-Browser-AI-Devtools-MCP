//go:build !race

// start_timeout_norace_test.go — Server-startup poll budget without the race detector.
// Keeps failure feedback fast in normal runs; the race build uses a larger ceiling.

package main

import "time"

const serverStartTimeout = 5 * time.Second
