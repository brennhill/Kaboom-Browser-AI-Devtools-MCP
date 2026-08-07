// cli_modes_subprocess_test.go — Covers user-facing CLI exit modes through the real binary.
// Docs: docs/features/feature/mcp-persistent-server/index.md

//go:build integration

package main

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestCLIEarlyExitModes(t *testing.T) {
	binary := buildTestBinary(t)
	tests := []struct {
		name       string
		args       []string
		wantOK     bool
		wantOutput string
	}{
		{name: "version", args: []string{"--version"}, wantOK: true, wantOutput: "kaboom v"},
		{name: "help", args: []string{"--help"}, wantOK: true, wantOutput: "Usage:"},
		{name: "invalid low port", args: []string{"--port", "0"}, wantOK: false, wantOutput: "invalid port"},
		{name: "invalid high port", args: []string{"--port", "65536"}, wantOK: false, wantOutput: "invalid port"},
		{name: "invalid install target", args: []string{"--install", "codxe"}, wantOK: false, wantOutput: "unknown install target"},
		{name: "stop unused port", args: []string{"--stop", "--port", "65431"}, wantOK: true, wantOutput: "No server found"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			cmd := startServerCmd(t, binary, tc.args...)
			cmd.Env = append(cmd.Env, "KABOOM_TELEMETRY_DISABLED=1")
			output, err := cmd.CombinedOutput()
			if tc.wantOK && err != nil {
				t.Fatalf("%v\n%s", err, output)
			}
			if !tc.wantOK && err == nil {
				t.Fatalf("expected failure\n%s", output)
			}
			if !strings.Contains(string(output), tc.wantOutput) {
				t.Fatalf("output missing %q:\n%s", tc.wantOutput, output)
			}
		})
	}
}

func TestCLIExplicitStateAndUploadConfiguration(t *testing.T) {
	binary := buildTestBinary(t)
	tests := []struct {
		name string
		args func(port int) []string
	}{
		{
			name: "explicit state and upload policy",
			args: func(port int) []string {
				return []string{
					"--daemon", "--port", strconv.Itoa(port),
					"--state-dir", t.TempDir(),
					"--upload-dir", t.TempDir(),
					"--upload-deny-pattern", "*.secret",
					"--ssrf-allow-host", "localhost:3000",
				}
			},
		},
		{
			name: "parallel mode generates isolated state",
			args: func(port int) []string {
				return []string{"--daemon", "--parallel", "--port", strconv.Itoa(port)}
			},
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			port := findFreePort(t)
			cmd := startServerCmd(t, binary, tc.args(port)...)
			cmd.Env = append(cmd.Env, "KABOOM_TELEMETRY_DISABLED=1")
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				_ = cmd.Process.Kill()
				_, _ = cmd.Process.Wait()
			})
			if bridgeRuntime().WaitForServer(port, 5*time.Second) {
				return
			}
			t.Fatalf("server did not start on port %d", port)
		})
	}
}
