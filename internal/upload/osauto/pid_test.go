// pid_test.go — Tests browser PID output parsing and failure contracts.

package osauto

import (
	"context"
	"errors"
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
	run := fakePIDCommandOutput(map[string]pidCommandResult{
		"pgrep -x Google Chrome":                           {output: "111\n"},
		"pgrep -x chrome":                                  {err: errors.New("not found")},
		"pgrep -x chromium":                                {output: "bad\n"},
		"pgrep -x google-chrome":                           {output: "222\n"},
		"tasklist /FI IMAGENAME eq chrome.exe /FO CSV /NH": {output: "\"chrome.exe\",\"333\",\"Console\",\"1\",\"1 K\"\n"},
	})

	if got, err := detectBrowserPIDDarwin(run); err != nil || got != 111 {
		t.Fatalf("darwin detector = %d, %v", got, err)
	}
	if got, err := detectBrowserPIDLinux(run); err != nil || got != 222 {
		t.Fatalf("linux detector = %d, %v", got, err)
	}
	if got, err := detectBrowserPIDWindows(run); err != nil || got != 333 {
		t.Fatalf("windows detector = %d, %v", got, err)
	}
}

func TestBrowserPIDDetectorFailuresAreActionable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test command fixtures use POSIX shell scripts")
	}
	run := fakePIDCommandOutput(map[string]pidCommandResult{
		"pgrep -x Google Chrome":                           {output: "nope\n"},
		"tasklist /FI IMAGENAME eq chrome.exe /FO CSV /NH": {output: "No tasks are running\n"},
	})

	if _, err := detectBrowserPIDDarwin(run); err == nil || !strings.Contains(err.Error(), "non-numeric") {
		t.Fatalf("darwin error = %v", err)
	}
	if _, err := detectBrowserPIDLinux(run); err == nil || !strings.Contains(err.Error(), "pgrep found none") {
		t.Fatalf("linux error = %v", err)
	}
	if _, err := detectBrowserPIDWindows(run); err == nil || !strings.Contains(err.Error(), "tasklist found no") {
		t.Fatalf("windows error = %v", err)
	}
}

func TestWindowsPIDDetectorRejectsMalformedRows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test command fixtures use POSIX shell scripts")
	}
	run := fakePIDCommandOutput(map[string]pidCommandResult{
		"tasklist /FI IMAGENAME eq chrome.exe /FO CSV /NH": {output: "malformed\n"},
	})
	if _, err := detectBrowserPIDWindows(run); err == nil || !strings.Contains(err.Error(), "unexpected format") {
		t.Fatalf("format error = %v", err)
	}

	run = fakePIDCommandOutput(map[string]pidCommandResult{
		"tasklist /FI IMAGENAME eq chrome.exe /FO CSV /NH": {output: "\"chrome.exe\",\"not-a-pid\"\n"},
	})
	if _, err := detectBrowserPIDWindows(run); err == nil || !strings.Contains(err.Error(), "non-numeric") {
		t.Fatalf("PID error = %v", err)
	}
}

type pidCommandResult struct {
	output string
	err    error
}

func fakePIDCommandOutput(results map[string]pidCommandResult) pidCommandOutput {
	return func(_ context.Context, name string, args ...string) ([]byte, error) {
		result, ok := results[strings.Join(append([]string{name}, args...), " ")]
		if !ok {
			return nil, errors.New("fixture command not found")
		}
		return []byte(result.output), result.err
	}
}
