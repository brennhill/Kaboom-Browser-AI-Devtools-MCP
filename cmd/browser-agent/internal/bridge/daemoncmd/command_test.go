// command_test.go — Verifies persistent daemon process construction.

package daemoncmd

import (
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	statecfg "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

func TestBuildDetachesStreamsAndIncludesRuntimeOptions(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv(statecfg.StateDirEnv, stateDir)
	cmd, err := Build(7890, "/tmp/kaboom.log", 25, func(string) string { return "kaboom-daemon" })
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	for name, stream := range map[string]io.Writer{"stdout": cmd.Stdout, "stderr": cmd.Stderr} {
		if stream != nil {
			if stream == io.Discard {
				t.Fatalf("%s = io.Discard; bridge-owned pipes can terminate the daemon", name)
			}
			if _, ok := stream.(*os.File); !ok {
				t.Fatalf("%s = %T, want nil or *os.File", name, stream)
			}
		}
	}
	if cmd.Stdin != nil {
		t.Fatalf("stdin = %T, want nil", cmd.Stdin)
	}
	if cmd.Args[0] != "kaboom-daemon" {
		t.Fatalf("argv[0] = %q, want daemon identity", cmd.Args[0])
	}
	for _, want := range []string{"--daemon", "--port", "7890", "--state-dir", stateDir, "--log-file", "/tmp/kaboom.log", "--max-entries", "25"} {
		if !slices.Contains(cmd.Args, want) {
			t.Fatalf("args %q missing %q", cmd.Args, want)
		}
	}
}

func TestBuildRetainsDetachedProcessBoundary(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("command.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(source), "util.SetDetachedProcess(cmd)") {
		t.Fatal("Build must detach the persistent daemon process")
	}
}
