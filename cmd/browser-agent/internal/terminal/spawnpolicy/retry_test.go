// spawn_retry_test.go -- Tests that a transient fork/exec EPERM is retried, and that
// nothing else is.
//
// Why this exists: a fork/exec EPERM is not always permanent. Under transient process
// pressure (fork limit, AV/EDR interposition, a security agent briefly holding the exec)
// the very next attempt succeeds. The old code surfaced the first EPERM straight to the
// user as "macOS sandbox restrictions — restart your daemon", which fixed nothing and
// blamed the wrong thing. A genuinely sandboxed daemon still fails every attempt and
// still gets the honest 503, just a few hundred milliseconds later.

package spawnpolicy

import (
	"errors"
	"fmt"
	"io/fs"
	"syscall"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/pty"
)

func forkExecErr(errno syscall.Errno) error {
	return fmt.Errorf("start /bin/zsh: %w", &fs.PathError{Op: "fork/exec", Path: "/bin/zsh", Err: errno})
}

func TestStartWithEPERMRetry_RetriesTransientEPERMThenSucceeds(t *testing.T) {
	t.Parallel()

	calls := 0
	var slept []time.Duration
	res, err := StartWithEPERMRetry(func() (*pty.StartResult, error) {
		calls++
		if calls < 3 {
			return nil, forkExecErr(syscall.EPERM)
		}
		return &pty.StartResult{SessionID: "ok"}, nil
	}, func(d time.Duration) { slept = append(slept, d) })

	if err != nil {
		t.Fatalf("a transient EPERM that later succeeds must not surface an error, got %v", err)
	}
	if res == nil || res.SessionID != "ok" {
		t.Fatalf("want the successful result, got %#v", res)
	}
	if calls != 3 {
		t.Fatalf("want 3 spawn attempts, got %d", calls)
	}
	if len(slept) != 2 {
		t.Fatalf("want a backoff between each retry (2), got %d: %v", len(slept), slept)
	}
	// Backoff must escalate but stay small — this runs inside a synchronous HTTP
	// handler, so the total added latency has to remain imperceptible.
	if slept[1] <= slept[0] {
		t.Errorf("backoff must escalate, got %v", slept)
	}
	var total time.Duration
	for _, d := range slept {
		total += d
	}
	if total > MaxTotalDelay {
		t.Errorf("total retry delay %v exceeds the %v budget for a synchronous handler", total, MaxTotalDelay)
	}
}

func TestStartWithEPERMRetry_GivesUpAfterBudgetAndReturnsRealError(t *testing.T) {
	t.Parallel()

	calls := 0
	sentinel := forkExecErr(syscall.EPERM)
	_, err := StartWithEPERMRetry(func() (*pty.StartResult, error) {
		calls++
		return nil, sentinel
	}, func(time.Duration) {})

	if calls != MaxAttempts {
		t.Fatalf("want exactly %d attempts, got %d", MaxAttempts, calls)
	}
	// A genuinely sandboxed daemon must still get the honest error — retrying must
	// never convert a real failure into a different or swallowed one.
	if !errors.Is(err, sentinel) {
		t.Fatalf("the final error must be the real spawn error, got %v", err)
	}
	if !IsSandboxError(err) {
		t.Fatal("the returned error must still classify as fork/exec EPERM so the 503 payload is unchanged")
	}
}

func TestStartWithEPERMRetry_DoesNotRetryOtherErrors(t *testing.T) {
	t.Parallel()

	// Retrying these is wrong AND slow: a session that already exists will never
	// stop existing, a bad cwd will never become good, and the session cap will not
	// clear inside a few hundred milliseconds. Each must fail on the first attempt.
	cases := map[string]error{
		"session exists": pty.ErrSessionExists,
		"max sessions":   pty.ErrMaxSessions,
		"bad cwd":        errors.New("chdir /nope: no such file or directory"),
		"missing shell":  forkExecErr(syscall.ENOENT),
		"bad perms":      forkExecErr(syscall.EACCES),
		"prose EPERM":    errors.New("fork/exec: operation not permitted"),
	}
	for name, want := range cases {
		t.Run(name, func(t *testing.T) {
			calls := 0
			slept := 0
			_, err := StartWithEPERMRetry(func() (*pty.StartResult, error) {
				calls++
				return nil, want
			}, func(time.Duration) { slept++ })

			if calls != 1 {
				t.Fatalf("%v must not be retried, got %d attempts", want, calls)
			}
			if slept != 0 {
				t.Fatalf("%v must not incur a backoff sleep, got %d", want, slept)
			}
			if !errors.Is(err, want) {
				t.Fatalf("error must pass through unchanged, got %v", err)
			}
		})
	}
}

func TestStartWithEPERMRetry_SucceedsFirstTryWithoutSleeping(t *testing.T) {
	t.Parallel()

	calls, slept := 0, 0
	_, err := StartWithEPERMRetry(func() (*pty.StartResult, error) {
		calls++
		return &pty.StartResult{SessionID: "ok"}, nil
	}, func(time.Duration) { slept++ })

	if err != nil || calls != 1 || slept != 0 {
		t.Fatalf("the happy path must spawn once and never sleep: err=%v calls=%d slept=%d", err, calls, slept)
	}
}
