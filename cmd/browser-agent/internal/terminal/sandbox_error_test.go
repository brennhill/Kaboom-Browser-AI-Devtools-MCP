// sandbox_error_test.go -- Tests that sandbox attribution is narrow and never swallows the real error.
//
// Regression: spawnpolicy.IsSandboxError used to substring-match "not permitted" anywhere in
// the error and then replace it with a confident "the daemon was started by an
// MCP client" story plus a restart instruction. Every syscall in the PTY spawn
// path can return EPERM (open /dev/ptmx, TIOCPTYGRANT, TIOCPTYUNLK, opening the
// slave, TIOCSWINSZ), so unrelated failures were reported as sandboxing and the
// real cause was discarded — undiagnosable from either the panel or the logs.

package terminal

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"syscall"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/terminal/spawnpolicy"
)

// forkExecErr builds the error shape os/exec produces when the child fails to
// launch: a *fs.PathError with Op "fork/exec" wrapping the errno.
func forkExecErr(errno syscall.Errno) error {
	return fmt.Errorf("start /bin/zsh: %w", &fs.PathError{
		Op:   "fork/exec",
		Path: "/bin/zsh",
		Err:  errno,
	})
}

func TestIsSandboxErrorMatchesForkExecEPERM(t *testing.T) {
	if !spawnpolicy.IsSandboxError(forkExecErr(syscall.EPERM)) {
		t.Fatal("fork/exec EPERM is the sandbox signature and must be detected")
	}
}

func TestIsSandboxErrorIgnoresNonForkExecEPERM(t *testing.T) {
	// grantpt/unlockpt/ptmx failures are EPERM but are not sandboxing. Attributing
	// them to the sandbox sends the user to restart a daemon that was never at fault.
	cases := map[string]error{
		"grantpt":  fmt.Errorf("grantpt: %w", syscall.EPERM),
		"unlockpt": fmt.Errorf("unlockpt: %w", syscall.EPERM),
		"ptmx":     fmt.Errorf("open /dev/ptmx: %w", syscall.EPERM),
		"winsize":  fmt.Errorf("set winsize: %w", syscall.EPERM),
	}
	for name, err := range cases {
		t.Run(name, func(t *testing.T) {
			if spawnpolicy.IsSandboxError(err) {
				t.Fatalf("%v must not be attributed to the sandbox", err)
			}
		})
	}
}

func TestIsSandboxErrorIgnoresUnrelatedFailures(t *testing.T) {
	cases := map[string]error{
		"nil":            nil,
		"session exists": errors.New("pty: session already exists: default"),
		"max sessions":   errors.New("pty: maximum concurrent sessions reached: limit 10"),
		"missing shell":  forkExecErr(syscall.ENOENT),
		"bad perms":      forkExecErr(syscall.EACCES),
		// A message that merely contains the words must not trigger detection.
		"prose": errors.New("the operation was not permitted by policy"),
	}
	for name, err := range cases {
		t.Run(name, func(t *testing.T) {
			if spawnpolicy.IsSandboxError(err) {
				t.Fatalf("%v must not be attributed to the sandbox", err)
			}
		})
	}
}

func TestSandboxPayloadCarriesUnderlyingError(t *testing.T) {
	// The diagnosis is a guess; the underlying error is the fact. The payload must
	// always carry the fact so the panel and the logs can show what actually failed.
	payload := sandboxPayload(forkExecErr(syscall.EPERM))
	detail, ok := payload["detail"].(string)
	if !ok || detail == "" {
		t.Fatalf("sandbox payload must include a non-empty detail, got %#v", payload["detail"])
	}
	if !strings.Contains(detail, "fork/exec") {
		t.Fatalf("detail must preserve the underlying error, got %q", detail)
	}
	if payload["error"] != "sandbox_restricted" {
		t.Fatalf("error code changed: %v", payload["error"])
	}
}
