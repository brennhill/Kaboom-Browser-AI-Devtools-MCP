//go:build !windows

// detach_unix_test.go — Tests detached child-process setup on Unix.
package detach

import (
	"os/exec"
	"testing"
)

func TestSetDetachedProcess(t *testing.T) {
	t.Parallel()

	cmd := exec.Command("echo", "hi")
	SetDetachedProcess(cmd)
	if cmd.SysProcAttr == nil || !cmd.SysProcAttr.Setsid {
		t.Fatalf("SysProcAttr = %+v, expected Setsid=true", cmd.SysProcAttr)
	}
}
