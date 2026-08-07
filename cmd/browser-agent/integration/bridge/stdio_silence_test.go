// Purpose: Tests for stdio silence enforcement in MCP mode.
// Docs: docs/features/feature/mcp-persistent-server/index.md

//go:build integration

package bridgeintegration

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/bridge"
	testprocess "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/integrationtest"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func waitForProcessExit(t *testing.T, command *exec.Cmd, timeout time.Duration) {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	select {
	case <-done:
	case <-time.After(timeout):
		_ = command.Process.Kill()
		t.Fatalf("process %d did not exit within %s", command.Process.Pid, timeout)
	}
}

// contentLengthFrame wraps a JSON payload in Content-Length framing.
func contentLengthFrame(payload string) string {
	return fmt.Sprintf("Content-Length: %d\r\nContent-Type: application/json\r\n\r\n%s", len(payload), payload)
}

// parseMCPResponses parses MCP responses from mixed line/content-length framed output.
func parseMCPResponses(t *testing.T, output string) []mcp.JSONRPCResponse {
	t.Helper()
	var responses []mcp.JSONRPCResponse
	reader := bufio.NewReader(strings.NewReader(output))
	for {
		line, err := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if line == "" {
			if err != nil {
				break
			}
			continue
		}
		if strings.HasPrefix(strings.ToLower(line), "content-length:") {
			// Read headers until empty line
			for {
				hdr, hErr := reader.ReadString('\n')
				if strings.TrimSpace(hdr) == "" || hErr != nil {
					break
				}
			}
			// Read body
			var body strings.Builder
			for {
				b, bErr := reader.ReadByte()
				if bErr != nil {
					break
				}
				body.WriteByte(b)
				if json.Valid([]byte(body.String())) {
					break
				}
			}
			var resp mcp.JSONRPCResponse
			if jErr := json.Unmarshal([]byte(body.String()), &resp); jErr == nil {
				responses = append(responses, resp)
			}
		} else {
			var resp mcp.JSONRPCResponse
			if jErr := json.Unmarshal([]byte(line), &resp); jErr == nil {
				responses = append(responses, resp)
			}
		}
		if err != nil {
			break
		}
	}
	return responses
}

// ⚠️ CRITICAL INVARIANT TEST - MCP STDIO SILENCE
//
// This test verifies that the wrapper and server produce ZERO non-JSON-RPC output
// on stdio during normal MCP operation. This is essential for MCP compliance.
//
// See: .claude/refs/mcp-stdio-invariant.md
//
// The wrapper and server MUST:
// 1. Output ONLY JSON-RPC messages to stdout
// 2. Output NOTHING to stderr during normal operation (silent connection)
// 3. Log all diagnostics/retries/debugging to log files
//
// DO NOT:
// - Remove or weaken this test
// - Allow any non-JSON-RPC output to stdio
// - Print progress messages, retry logs, or diagnostics to stderr

func TestStdioIsolation_StartupNoiseDoesNotPolluteMCPTransport(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	port := testprocess.FreePort(t)
	binary := testprocess.BuildBinary(t)
	stateDir := filepath.Join(t.TempDir(), "state")
	coverDir := filepath.Join(t.TempDir(), "cover")
	if err := os.MkdirAll(coverDir, 0o755); err != nil {
		t.Fatalf("Failed to create cover dir: %v", err)
	}

	cmd := testprocess.StartServer(t, binary, "--bridge", "--port", strconv.Itoa(port), "--state-dir", stateDir)
	cmd.Env = append(os.Environ(), "KABOOM_TEST_BRIDGE_NOISE=1", "GOCOVERDIR="+coverDir)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("Failed to create stdin pipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start bridge process: %v", err)
	}

	initRequest := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test-llm","version":"1.0"}}}`
	if _, err := stdin.Write([]byte(initRequest + "\n")); err != nil {
		t.Fatalf("Failed to write initialize request: %v", err)
	}
	_ = stdin.Close()

	waitForProcessExit(t, cmd, 8*time.Second)

	outStr := stdout.String()
	errStr := stderr.String()

	if strings.Contains(outStr, "KABOOM_TEST_NOISE_STDOUT") || strings.Contains(outStr, "KABOOM_TEST_NOISE_STDERR") {
		t.Fatalf("transport polluted by startup noise: %q", outStr)
	}
	_ = parseMCPResponses(t, outStr)

	if strings.TrimSpace(errStr) != "" {
		t.Fatalf("stderr must stay silent in bridge mode, got: %q", errStr)
	}

	wrapperLogPath := filepath.Join(stateDir, "logs", bridge.BridgeWrapperLogFileName)
	logBody, readErr := os.ReadFile(wrapperLogPath)
	if readErr != nil {
		t.Fatalf("read wrapper log: %v", readErr)
	}
	if !strings.Contains(string(logBody), "KABOOM_TEST_NOISE_STDOUT") {
		t.Fatalf("wrapper log missing redirected stdout noise: %s", wrapperLogPath)
	}
	if !strings.Contains(string(logBody), "KABOOM_TEST_NOISE_STDERR") {
		t.Fatalf("wrapper log missing redirected stderr noise: %s", wrapperLogPath)
	}
}

func TestStdioIsolation_ContentLengthFramingNotPollutedByStartupNoise(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	port := testprocess.FreePort(t)
	binary := testprocess.BuildBinary(t)
	stateDir := filepath.Join(t.TempDir(), "state")
	coverDir := filepath.Join(t.TempDir(), "cover")
	if err := os.MkdirAll(coverDir, 0o755); err != nil {
		t.Fatalf("Failed to create cover dir: %v", err)
	}

	cmd := testprocess.StartServer(t, binary, "--bridge", "--port", strconv.Itoa(port), "--state-dir", stateDir)
	cmd.Env = append(os.Environ(), "KABOOM_TEST_BRIDGE_NOISE=1", "GOCOVERDIR="+coverDir)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("Failed to create stdin pipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start bridge process: %v", err)
	}

	initPayload := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test-llm","version":"1.0"}}}`
	listPayload := `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`
	framed := contentLengthFrame(initPayload) + contentLengthFrame(listPayload)
	if _, err := stdin.Write([]byte(framed)); err != nil {
		t.Fatalf("Failed to write framed requests: %v", err)
	}
	_ = stdin.Close()

	waitForProcessExit(t, cmd, 8*time.Second)

	outStr := stdout.String()
	errStr := stderr.String()

	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(outStr)), "content-length:") {
		t.Fatalf("expected content-length framed output, got: %q", outStr)
	}
	if strings.Contains(outStr, "KABOOM_TEST_NOISE_STDOUT") || strings.Contains(outStr, "KABOOM_TEST_NOISE_STDERR") {
		t.Fatalf("framed transport polluted by startup noise: %q", outStr)
	}

	responses := parseMCPResponses(t, outStr)
	if len(responses) < 2 {
		t.Fatalf("expected at least 2 framed responses, got %d", len(responses))
	}
	for i, resp := range responses {
		if resp.JSONRPC != "2.0" {
			t.Fatalf("response %d missing jsonrpc 2.0: %+v", i, resp)
		}
	}

	if strings.TrimSpace(errStr) != "" {
		t.Fatalf("stderr must stay silent in bridge mode, got: %q", errStr)
	}
}

func TestStdioIsolation_BridgeExitsAfterStdinEOF(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	port := testprocess.FreePort(t)
	binary := testprocess.BuildBinary(t)
	stateDir := filepath.Join(t.TempDir(), "state")
	coverDir := filepath.Join(t.TempDir(), "cover")
	if err := os.MkdirAll(coverDir, 0o755); err != nil {
		t.Fatalf("Failed to create cover dir: %v", err)
	}

	cmd := testprocess.StartServer(t, binary, "--bridge", "--port", strconv.Itoa(port), "--state-dir", stateDir)
	cmd.Env = append(os.Environ(), "GOCOVERDIR="+coverDir)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("Failed to create stdin pipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start bridge process: %v", err)
	}

	initRequest := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test-llm","version":"1.0"}}}`
	if _, err := stdin.Write([]byte(initRequest + "\n")); err != nil {
		t.Fatalf("Failed to write initialize request: %v", err)
	}
	_ = stdin.Close()

	waitForProcessExit(t, cmd, 8*time.Second)

	outStr := stdout.String()
	errStr := stderr.String()

	if strings.TrimSpace(outStr) == "" {
		t.Fatalf("expected initialize JSON-RPC response, got empty stdout")
	}
	_ = parseMCPResponses(t, outStr)

	if strings.TrimSpace(errStr) != "" {
		t.Fatalf("stderr must stay silent in bridge mode, got: %q", errStr)
	}
}
