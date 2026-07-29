// pid_test.go — Tests browser PID output parsing and failure contracts.

package osauto

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestParseFirstPIDLine(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		output string
		want   int
		ok     bool
	}{
		{name: "single", output: "123\n", want: 123, ok: true},
		{name: "first of several", output: "456\n789\n", want: 456, ok: true},
		{name: "whitespace", output: " 42 ", want: 42, ok: true},
		{name: "empty", output: "", ok: false},
		{name: "non numeric", output: "chrome", ok: false},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := parseFirstPIDLine(tc.output)
			if got != tc.want || ok != tc.ok {
				t.Fatalf("parseFirstPIDLine(%q) = %d, %v; want %d, %v", tc.output, got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestBrowserPIDDetectorsParseCommandOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test command fixtures use POSIX shell scripts")
	}
	bin := t.TempDir()
	writeCommandFixture(t, bin, "pgrep", "#!/bin/sh\ncase \"$2\" in\nGoogle\\ Chrome) echo 111;;\nchromium) echo bad;;\ngoogle-chrome) echo 222;;\n*) exit 1;;\nesac\n")
	writeCommandFixture(t, bin, "tasklist", "#!/bin/sh\nprintf '\"chrome.exe\",\"333\",\"Console\",\"1\",\"1 K\"\\n'\n")
	t.Setenv("PATH", bin)

	if got, err := detectBrowserPIDDarwin(); err != nil || got != 111 {
		t.Fatalf("darwin detector = %d, %v", got, err)
	}
	if got, err := detectBrowserPIDLinux(); err != nil || got != 222 {
		t.Fatalf("linux detector = %d, %v", got, err)
	}
	if got, err := detectBrowserPIDWindows(); err != nil || got != 333 {
		t.Fatalf("windows detector = %d, %v", got, err)
	}
}

func TestBrowserPIDDetectorFailuresAreActionable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test command fixtures use POSIX shell scripts")
	}
	bin := t.TempDir()
	writeCommandFixture(t, bin, "pgrep", "#!/bin/sh\nif [ \"$2\" = \"Google Chrome\" ]; then echo nope; exit 0; fi\nexit 1\n")
	writeCommandFixture(t, bin, "tasklist", "#!/bin/sh\necho 'No tasks are running'; exit 0\n")
	t.Setenv("PATH", bin)

	if _, err := detectBrowserPIDDarwin(); err == nil || !strings.Contains(err.Error(), "non-numeric") {
		t.Fatalf("darwin error = %v", err)
	}
	if _, err := detectBrowserPIDLinux(); err == nil || !strings.Contains(err.Error(), "pgrep found none") {
		t.Fatalf("linux error = %v", err)
	}
	if _, err := detectBrowserPIDWindows(); err == nil || !strings.Contains(err.Error(), "tasklist found no") {
		t.Fatalf("windows error = %v", err)
	}
}

func TestWindowsPIDDetectorRejectsMalformedRows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test command fixtures use POSIX shell scripts")
	}
	bin := t.TempDir()
	writeCommandFixture(t, bin, "tasklist", "#!/bin/sh\necho malformed\n")
	t.Setenv("PATH", bin)
	if _, err := detectBrowserPIDWindows(); err == nil || !strings.Contains(err.Error(), "unexpected format") {
		t.Fatalf("format error = %v", err)
	}

	writeCommandFixture(t, bin, "tasklist", "#!/bin/sh\nprintf '\"chrome.exe\",\"not-a-pid\"\\n'\n")
	if _, err := detectBrowserPIDWindows(); err == nil || !strings.Contains(err.Error(), "non-numeric") {
		t.Fatalf("PID error = %v", err)
	}
}

func writeCommandFixture(t *testing.T, dir, name, script string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}
