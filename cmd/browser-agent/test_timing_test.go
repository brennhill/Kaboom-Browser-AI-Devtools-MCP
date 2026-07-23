// Purpose: Shared timing budgets for tests that spawn the real binary.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// test_timing_test.go — One place for latency budgets and the liveness timeout,
// plus the invariant that keeps them from colliding.
//
// The flake this fixes: TestFastStart_ClientCompatibilityMatrix/claude_code read
// `initialize` with a 5s timeout while asserting a 4s budget elsewhere. Under
// full-suite load the first spawn of a freshly built 16 MB coverage binary costs
// ~520ms cold (page-in + code-signature validation) versus ~0ms warm, so the
// first subtest blew the 5s read and died as "timeout waiting for response" —
// a message that says nothing about what was slow. Two rules follow:
//
//  1. A liveness timeout is a hang guard, not an assertion. Keep it far above
//     every budget so a slow-but-alive process fails as a budget miss that
//     reports its measured elapsed time.
//  2. Budgets must measure steady state. buildTestBinary warms the binary once
//     after building so no measurement pays the one-time cold-exec cost.

package main

import (
	"testing"
	"time"
)

const (
	// testLivenessTimeout bounds how long to wait for a child that may be hung.
	// It is deliberately generous: exceeding it means "no response at all", which
	// is a different failure from "responded too slowly".
	testLivenessTimeout = 30 * time.Second

	// fastStartInitBudget bounds an `initialize` round-trip including process spawn.
	fastStartInitBudget = 4 * time.Second

	// fastStartResourceBudget bounds a resources/read on an initialized bridge.
	fastStartResourceBudget = 500 * time.Millisecond

	// fastStartWarmBudget bounds any request on an already-initialized bridge.
	fastStartWarmBudget = 100 * time.Millisecond
)

// TestLivenessTimeoutExceedsLatencyBudgets pins rule 1 above. Without this, a
// future tuning pass can quietly set a budget within a hair of the liveness
// timeout again and turn every slow run back into an uninformative timeout.
func TestLivenessTimeoutExceedsLatencyBudgets(t *testing.T) {
	budgets := map[string]time.Duration{
		"fastStartInitBudget":     fastStartInitBudget,
		"fastStartResourceBudget": fastStartResourceBudget,
		"fastStartWarmBudget":     fastStartWarmBudget,
	}
	// 3x margin: a run 3x over budget is still diagnosed as "too slow, took N"
	// rather than collapsing into "timeout".
	const minMargin = 3
	for name, budget := range budgets {
		if testLivenessTimeout < minMargin*budget {
			t.Errorf("testLivenessTimeout (%v) must be at least %dx %s (%v) so a slow "+
				"response fails as a budget miss with its elapsed time, not a bare timeout",
				testLivenessTimeout, minMargin, name, budget)
		}
	}
}
