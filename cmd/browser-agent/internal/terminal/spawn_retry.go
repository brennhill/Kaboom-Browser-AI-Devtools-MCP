// spawn_retry.go — bounded retry for a transient fork/exec EPERM on PTY spawn.
// Why: a fork/exec EPERM is not always permanent. Under momentary process pressure
// (fork limit, AV/EDR interposition, a security agent briefly holding the exec) the
// next attempt succeeds. Surfacing the first EPERM straight to the user produced a
// "macOS sandbox restrictions — restart your daemon" dead end for a failure that
// would have cleared on its own. A genuinely sandboxed daemon fails every attempt
// and still gets the honest 503, only a few hundred milliseconds later.
// Docs: docs/features/feature/terminal/index.md

package terminal

import (
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/pty"
)

const (
	// epermSpawnAttempts is the total number of spawn attempts (1 initial + retries).
	epermSpawnAttempts = 3
	// epermRetryBaseDelay is the first backoff; it doubles per retry.
	epermRetryBaseDelay = 75 * time.Millisecond
	// epermRetryMaxTotalDelay bounds the cumulative sleep. This runs inside a
	// synchronous HTTP handler, so the added latency must stay imperceptible: with
	// 3 attempts the real total is 75ms + 150ms = 225ms.
	epermRetryMaxTotalDelay = 500 * time.Millisecond
)

// startWithEPERMRetry runs start, retrying ONLY a transient fork/exec EPERM with a
// small escalating backoff. Any other error — session exists, session cap, bad cwd,
// missing shell — is returned on the first attempt: retrying those is both wrong and
// slow, since none of them can resolve inside a few hundred milliseconds.
//
// The final error is the real spawn error, unwrapped and unmodified, so
// classifyStartError still produces the same honest payload it would have without
// the retry. sleep is injected so tests are deterministic and never actually wait.
func startWithEPERMRetry(start func() (*pty.StartResult, error), sleep func(time.Duration)) (*pty.StartResult, error) {
	var (
		res   *pty.StartResult
		err   error
		delay = epermRetryBaseDelay
	)
	for attempt := 1; attempt <= epermSpawnAttempts; attempt++ {
		res, err = start()
		if err == nil {
			return res, nil
		}
		// Only a typed fork/exec EPERM is worth another try, and only if we have
		// an attempt left — never sleep after the final one.
		if !IsSandboxError(err) || attempt == epermSpawnAttempts {
			return res, err
		}
		sleep(delay)
		delay *= 2
	}
	return res, err
}
