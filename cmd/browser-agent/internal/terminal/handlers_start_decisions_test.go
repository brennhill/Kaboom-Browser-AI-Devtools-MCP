// handlers_start_decisions_test.go — unit tests for the pure start decisions
// extracted from HandleTerminalStart: CWD priority and error classification.
// Previously these lived inline in a 110-line HTTP handler and could only be
// exercised by making a real *pty.Manager return each error (real spawn / real
// session-limit setup). As pure functions they test directly.

package terminal

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"syscall"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/pty"
)

func TestResolveStartDir(t *testing.T) {
	t.Parallel()

	autoDetect := func() string { return "/auto" }

	tests := []struct {
		name           string
		reqDir         string
		activeCodebase string
		want           string
		wantAutoCalled bool
	}{
		{"request dir wins over everything", "/req", "/active", "/req", false},
		{"active codebase when no request dir", "", "/active", "/active", false},
		{"auto-detect when both empty", "", "", "/auto", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			autoCalled := false
			got := resolveStartDir(tc.reqDir, tc.activeCodebase, func() string {
				autoCalled = true
				return autoDetect()
			})
			if got != tc.want {
				t.Errorf("resolveStartDir(%q, %q) = %q, want %q", tc.reqDir, tc.activeCodebase, got, tc.want)
			}
			if autoCalled != tc.wantAutoCalled {
				t.Errorf("autoDetect called = %v, want %v (must be lazy — only when higher-priority sources are empty)", autoCalled, tc.wantAutoCalled)
			}
		})
	}
}

func TestClassifyStartError(t *testing.T) {
	t.Parallel()

	const sessionID, token = "sess-1", "tok-abc"

	t.Run("fork/exec EPERM -> 503 carrying the real error", func(t *testing.T) {
		status, body := classifyStartError(forkExecErr(syscall.EPERM), sessionID, token)
		if status != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want 503", status)
		}
		if body["error"] != "sandbox_restricted" {
			t.Errorf("error = %v, want sandbox_restricted", body["error"])
		}
		// The diagnosis is an inference; the underlying error is the fact and must
		// survive so the panel/logs can show what actually failed.
		if detail, _ := body["detail"].(string); !strings.Contains(detail, "fork/exec") {
			t.Errorf("detail must preserve the underlying error, got %q", detail)
		}
	})

	// Regression: the old classifier substring-matched "not permitted" anywhere, so a
	// PTY-layer EPERM (grantpt/unlockpt/ptmx/winsize) was reported as sandboxing and
	// the user was told to restart a daemon that was never at fault. These must fall
	// through to an ordinary error, not 503.
	t.Run("non-fork/exec 'not permitted' is NOT sandbox", func(t *testing.T) {
		for name, err := range map[string]error{
			"prose":   errors.New("fork/exec: operation not permitted"),
			"grantpt": fmt.Errorf("grantpt: %w", syscall.EPERM),
			"ptmx":    fmt.Errorf("open /dev/ptmx: %w", syscall.EPERM),
			"winsize": fmt.Errorf("set winsize: %w", syscall.EPERM),
		} {
			t.Run(name, func(t *testing.T) {
				status, body := classifyStartError(err, sessionID, token)
				if status == http.StatusServiceUnavailable {
					t.Fatalf("%v must not be classified as a sandbox restriction", err)
				}
				if body["error"] == "sandbox_restricted" {
					t.Fatalf("%v must not be labelled sandbox_restricted", err)
				}
			})
		}
	})

	t.Run("session already exists -> 409 with token (benign reconnect)", func(t *testing.T) {
		status, body := classifyStartError(pty.ErrSessionExists, sessionID, token)
		if status != http.StatusConflict {
			t.Fatalf("status = %d, want 409", status)
		}
		if body["token"] != token {
			t.Errorf("token = %v, want %q (client must attach to the live session)", body["token"], token)
		}
		if body["session_id"] != sessionID {
			t.Errorf("session_id = %v, want %q", body["session_id"], sessionID)
		}
	})

	t.Run("max sessions -> 429", func(t *testing.T) {
		status, body := classifyStartError(pty.ErrMaxSessions, sessionID, token)
		if status != http.StatusTooManyRequests {
			t.Fatalf("status = %d, want 429", status)
		}
		if _, hasToken := body["token"]; hasToken {
			t.Error("a real failure (429) must NOT hand back a token — that would reconnect to a phantom session")
		}
	})

	t.Run("generic failure (bad cwd / spawn) -> 400, no token", func(t *testing.T) {
		status, body := classifyStartError(errors.New("chdir /nope: no such file or directory"), sessionID, token)
		if status != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", status)
		}
		if _, hasToken := body["token"]; hasToken {
			t.Error("a real failure (400) must NOT hand back a token — bucketing it as 409-with-token silently reconnected to the old cwd")
		}
		if body["error"] == nil {
			t.Error("the real error message must surface, not be swallowed")
		}
	})
}
