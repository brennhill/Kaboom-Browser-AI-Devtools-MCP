# Testing Guidelines — Determinism

How to write tests that fail only when the code is wrong. Every rule here comes
from a flake that actually shipped in this repo, named so the pattern is
recognisable next time.

Canonical helper: [`internal/testsync`](../../internal/testsync/testsync.go).
Timing budgets for spawned-binary tests: [`cmd/browser-agent/test_timing_test.go`](../../cmd/browser-agent/test_timing_test.go).

---

## 1. Poll for a condition; never sleep before an assertion

A fixed `time.Sleep` before an assertion encodes a guess about how long async
work takes. It is wrong in both directions: too short and the test flakes under
load, too long and every run pays the worst case.

```go
// NO — a guess about scheduler timing
c.AddEnhancedActions(...)
time.Sleep(50 * time.Millisecond)
if got := called.Load(); got != 1 { t.Errorf(...) }

// YES — returns as soon as it is true, fails only if it never is
c.AddEnhancedActions(...)
testsync.Eventually(t, testsync.DefaultTimeout, "the navigation callback to fire", func() bool {
    return called.Load() == 1
})
```

Polling is also usually *faster*: a condition that becomes true in 3ms costs 3ms
instead of the full 50ms sleep.

### When a sleep is still correct

Not every sleep is a bug. Keep it, and say why in a comment, when:

| Case | Why polling cannot replace it |
|------|-------------------------------|
| **Negative assertion** ("the callback must NOT fire") | there is no event to wait for; a settle window is the only way to observe "never" |
| **Duration assertion** ("the server must stay alive 30s") | elapsed wall time *is* the thing under test |
| **Fixture latency** (a fake handler that simulates a slow peer) | the sleep is part of the fixture, not the wait |

## 2. Budgets measure steady state; warm the cache first

A latency budget that includes one-time initialisation is measuring the wrong
thing, and it fails on whichever test happens to run first.

Two instances of this, same shape:

- **Cold binary exec.** The first exec of a just-built 16 MB coverage-instrumented
  binary costs **~520ms** (page-in + macOS code-signature validation); every exec
  after is **~0ms**. `TestFastStart_ClientCompatibilityMatrix/claude_code` — the
  *first* subtest — blew its read timeout under load while subtests 2–4 passed.
  Fix: `buildTestBinary` runs the binary once after building.
- **Cold discovery cache.** The quality-gate hook scans the repo and caches per
  (project root, extension) with a 5-minute TTL. `TestEval_AllFixtures` runs its
  fixtures with `t.Parallel()`, so every quality-gate fixture missed the cache at
  once and a random 1–3 of them blew the 500ms budget on ~2 runs in 3. Fix: a
  serial warm-up pass before the timed run.

If cold-start cost matters, give it **its own** assertion with its own budget.
Do not let it contaminate the steady-state one.

## 3. A liveness timeout is a hang guard, not an assertion

Keep them far apart — at least 3x. The fast-start tests read `initialize` with a
5s timeout while asserting a 4s budget, so a loaded machine produced
`timeout waiting for response after 5s` instead of `took 6.2s, want < 4s`: the
uninformative failure instead of the useful one.

`TestLivenessTimeoutExceedsLatencyBudgets` enforces the margin.

## 4. Tests must not touch the network

`TestUploadInteg_FormSubmit_FilePermissionDenied` posted to
`https://example.com/upload`. It never intended to make a request — an unreadable
file should fail first — but `ValidateFormActionURL` resolves the host for its
SSRF check with a **5s** timeout. When DNS stalled the test took exactly 5.00s and
failed on a DNS error that mentions neither "open" nor "permission".

Use `httptest.NewServer` plus `uploadhandler.SetSkipSSRFCheck(true)` (the SSRF
guard rejects 127.0.0.1 by design). Where the contract is "no request should
happen", record it and assert it:

```go
var contacted atomic.Bool
srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
    contacted.Store(true)
}))
defer srv.Close()
// ...
if contacted.Load() { t.Error("must reject before making any request") }
```

## 5. Goroutine-leak checks poll, they do not sleep

`sleep; GC; sleep; assert count` is a perennial flake — teardown is not
synchronous with `Close()`. Use `testsync.EventuallyGoroutines`, and take the
baseline with `testsync.SettledGoroutines()` so it is not sampled while a
previous test is still winding down.

```go
before := testsync.SettledGoroutines()
c := NewCapture()
c.Close()
testsync.EventuallyGoroutines(t, before+1, "capture goroutines to exit after Close")
```

## 6. Shared state crossing a goroutine boundary must be atomic

`TestStartBinaryWatcher_ContextCancellation` used a plain `bool` written by the
watcher goroutine and read by the test. It is a data race by construction even
when the callback never fires. Use `atomic.Bool`.

Related: tests that swap package globals (`skipSSRFCheck`, `uploadSecurityConfig`)
must **not** use `t.Parallel()`. Several upload test files carry that warning in
their header — keep it.

---

## Reference: `internal/testsync`

| Helper | Use for |
|--------|---------|
| `Eventually(tb, timeout, msg, cond)` | wait for a condition, fail with `msg` on timeout |
| `EventuallyNoFail(timeout, cond)` | wait and report success as a bool, when the caller asserts something more specific |
| `Value(tb, timeout, msg, get)` | wait for a value to become available and return it |
| `EventuallyGoroutines(tb, max, msg)` | wait for the goroutine count to settle at or below `max` |
| `SettledGoroutines()` | sample a leak baseline once the count stops moving |
| `DefaultTimeout` | 5s — bounds a hang, does not assert latency |

`testsync` is test-only. Nothing in production code may import it: it pulls in
`testing`, which registers test flags on any binary that links it.
