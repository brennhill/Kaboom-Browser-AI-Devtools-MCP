// bridge.go -- Orchestrates bridge-mode transport forwarding between MCP stdio and daemon HTTP.
// Why: Keeps request/response forwarding resilient across daemon restarts and transport disruptions.
// The read loop and the per-request forwarder are one unit: the loop's only job
// is to hand each line to bridgeForwardRequest, and the forwarder's respawn+retry
// is what makes the loop survive a daemon death. Splitting them only hid that.
// Docs: docs/features/feature/bridge-restart/index.md

package bridge

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/bridge/stdioisolate"
	internbridge "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/bridge"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/schema"
	statecfg "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/telemetry"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// BridgeWrapperLogFileName is the name of the bridge wrapper log file.
const BridgeWrapperLogFileName = stdioisolate.WrapperLogFileName

// ActiveMCPTransportWriter returns the file used for MCP JSON-RPC transport.
func ActiveMCPTransportWriter() *os.File { return stdioisolate.ActiveMCPTransportWriter() }

// EnsureIOIsolation configures bridge mode so stdout/stderr noise cannot
// corrupt MCP JSON-RPC framing on stdout. Diagnostic seams come from this
// runner's transport owner.
func (r *Runner) EnsureIOIsolation(logFileHint string) error {
	return stdioisolate.Ensure(logFileHint,
		func(w io.Writer) { r.transport.SetStderr(w) },
		func(format string, args ...any) { r.transport.Stderrf(format, args...) },
	)
}

// toolCallTimeout delegates to internal/bridge for per-request timeout logic.
func toolCallTimeout(req mcp.JSONRPCRequest) time.Duration {
	return internbridge.ToolCallTimeout(req.Method, req.Params)
}

// buildDaemonCmd resolves the current executable and builds an exec.Cmd for the
// daemon process with the appropriate flags and detached-process settings.
func (s *daemonState) buildDaemonCmd() (*exec.Cmd, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("find executable: %w. Verify kaboom is installed correctly", err)
	}

	args := []string{"--daemon", "--port", fmt.Sprintf("%d", s.port)}
	if stateDir := os.Getenv(statecfg.StateDirEnv); stateDir != "" {
		args = append(args, "--state-dir", stateDir)
	}
	if s.logFile != "" {
		args = append(args, "--log-file", s.logFile)
	}
	if s.maxEntries > 0 {
		args = append(args, "--max-entries", fmt.Sprintf("%d", s.maxEntries))
	}
	cmd := exec.Command(exe, args...) // #nosec G702 -- exe is our own binary path from os.Executable with fixed flags // nosemgrep: go.lang.security.audit.dangerous-exec-command.dangerous-exec-command, go_subproc_rule-subproc -- bridge spawns own daemon
	cmd.Args[0] = s.runner.lifecycle.ProcessArgv0(exe)
	// Detach the daemon's standard streams. nil => os/exec connects the fd to
	// os.DevNull (/dev/null). We must NOT use io.Discard here: os/exec routes any
	// non-*os.File writer through an OS pipe whose read-end lives in THIS bridge
	// process. Setsid detaches the daemon's session/process group but NOT the
	// inherited pipe fds. When the bridge exits on stdin_eof, those pipe read-ends
	// close, and the daemon's next stderr write dies with SIGPIPE on fd 2 (Go
	// terminates on a broken pipe to fd 1/2). /dev/null never breaks, so the
	// spawned daemon stays persistent across the bridge's exit. This mirrors the
	// installer's startDaemonSilently (internal/nativeinstall/installer.go), which already uses nil.
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	util.SetDetachedProcess(cmd)
	return cmd, nil
}

// isConnectionError delegates to internal/bridge for connection error detection.
func isConnectionError(err error) bool {
	return internbridge.IsConnectionError(err)
}

// FlushStdout syncs stdout and logs any errors (best-effort)
func FlushStdout() {
	SyncStdoutBestEffort()
}

// StdioToHTTPFast forwards JSON-RPC with fast-start: responds to initialize/tools/list
// immediately while daemon starts in background. Only blocks on tools/call.
// #lizard forgives
func (r *Runner) StdioToHTTPFast(endpoint string, state *daemonState, port int) {
	defer FlushFastPathTelemetry()
	reader := bufio.NewReaderSize(os.Stdin, 64*1024)

	client := &http.Client{} // per-request timeouts via context

	// Start push relay goroutine to poll daemon inbox and relay to Claude via stdio.
	pushRelayDone := make(chan struct{})
	r.startBridgePushRelay(client, endpoint, pushRelayDone)

	var wg sync.WaitGroup
	responseSent := make(chan bool, 1)
	var responseOnce sync.Once
	stats := &bridgeSessionStats{}
	signalResponseSent := func() {
		responseOnce.Do(func() { responseSent <- true })
	}

	toolsList := schema.AllTools()

	var readErr error
	for {
		line, framing, err := r.readMCPStdioMessage(reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			r.transport.Debugf("stdin read error: %v", err)
			readErr = err
			break
		}
		if len(line) == 0 {
			continue
		}
		stats.requests++
		if framing == internbridge.StdioFramingContentLength {
			stats.contentLengthFraming++
		} else {
			stats.lineFraming++
		}

		var req mcp.JSONRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			stats.parseErrors++
			r.sendBridgeParseError(line, err, framing)
			signalResponseSent()
			continue
		}
		if req.HasInvalidID() {
			stats.invalidIDs++
			r.sendBridgeError(nil, -32600, "Invalid Request: id must be string or number when present", framing)
			signalResponseSent()
			continue
		}
		r.transport.Debugf("request method=%s id=%v", req.Method, req.ID)
		stats.lastMethod = req.Method

		// FAST PATH: Handle initialize and tools/list directly (no daemon needed)
		if r.handleFastPath(req, toolsList, framing) {
			stats.fastPath++
			signalResponseSent()
			continue
		}

		// RESTART FAST PATH: configure(action="restart") handled in bridge, not daemon
		if r.handleBridgeRestart(req, state, port, framing) {
			stats.fastPath++
			signalResponseSent()
			continue
		}

		// SLOW PATH: Check daemon status for tools/call and other methods.
		shouldForward := true
		if status := checkDaemonStatus(state, req, port); status != "" {
			if status == "method_not_found" {
				stats.methodNotFound++
			}
			if status == "starting" {
				stats.starting++
				// During startup, tools/call should wait-and-forward rather than
				// immediately returning a retry envelope to stdio clients.
				if req.Method == "tools/call" {
					shouldForward = true
				} else {
					shouldForward = false
				}
			} else {
				shouldForward = false
			}
			if !shouldForward {
				r.handleDaemonNotReady(req, status, signalResponseSent, framing)
				continue
			}
		}

		// Forward to HTTP server concurrently
		timeout := toolCallTimeout(req)
		reqCopy, lineCopy := req, append([]byte(nil), line...)
		stats.forwarded++
		wg.Add(1)
		util.SafeGo(func() {
			defer wg.Done()
			r.bridgeForwardRequest(client, endpoint, reqCopy, lineCopy, timeout, state, signalResponseSent, framing)
		})
	}

	close(pushRelayDone)
	r.bridgeShutdown(&wg, readErr, responseSent, stats)
}

// bridgeDoHTTP delegates to internal/bridge for HTTP forwarding.
func bridgeDoHTTP(ctx context.Context, client *http.Client, endpoint string, line []byte) (*http.Response, error) {
	return internbridge.DoHTTP(ctx, client, endpoint, line)
}

// bridgeForwardRequest forwards a JSON-RPC request to the HTTP server and writes the response.
// If state is non-nil and the daemon is unreachable, attempts a single respawn + retry.
// #lizard forgives
func (r *Runner) bridgeForwardRequest(client *http.Client, endpoint string, req mcp.JSONRPCRequest, line []byte, timeout time.Duration, state *daemonState, signal func(), framing internbridge.StdioFraming) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	activeCancel := cancel
	fallbackUsed := false

	resp, err := bridgeDoHTTP(ctx, client, endpoint, line)
	if err != nil && isConnectionError(err) && state != nil {
		fallbackUsed = true
		// Daemon died — attempt respawn and retry with fresh context
		// (original context may have little time left after respawn delay).
		if state.respawnIfNeeded() {
			cancel()
			retryCtx, retryCancel := context.WithTimeout(context.Background(), timeout)
			resp, err = bridgeDoHTTP(retryCtx, client, endpoint, line)
			activeCancel = retryCancel
		}
	}
	defer activeCancel()
	if err != nil {
		telemetry.AppError("bridge_connection_error", nil)
		message := "Server connection error: " + err.Error()
		if req.Method == "tools/call" {
			r.sendToolErrorWithOptions(req.ID, message, framing, bridgeToolErrorOptions{
				ErrorCode:    "bridge_connection_error",
				Subsystem:    "bridge_http_forwarder",
				Reason:       "http_forward_failed",
				Retryable:    true,
				RetryAfterMs: 2000,
				FallbackUsed: fallbackUsed,
				Detail:       err.Error(),
			})
		} else {
			r.sendBridgeError(req.ID, -32603, message, framing)
		}
		signal()
		return
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, r.transport.MaxBodySize))
	_ = resp.Body.Close() //nolint:errcheck // best-effort cleanup
	if err != nil {
		message := "Failed to read response: " + err.Error()
		if req.Method == "tools/call" {
			r.sendToolErrorWithOptions(req.ID, message, framing, bridgeToolErrorOptions{
				ErrorCode:    "bridge_response_read_error",
				Subsystem:    "bridge_http_forwarder",
				Reason:       "response_read_failed",
				Retryable:    true,
				RetryAfterMs: 1000,
				FallbackUsed: fallbackUsed,
				Detail:       err.Error(),
			})
		} else {
			r.sendBridgeError(req.ID, -32603, message, framing)
		}
		signal()
		return
	}

	if resp.StatusCode == 204 {
		if req.HasID() {
			message := "Server returned no content for request with an id"
			if req.Method == "tools/call" {
				r.sendToolErrorWithOptions(req.ID, message, framing, bridgeToolErrorOptions{
					ErrorCode:    "bridge_unexpected_no_content",
					Subsystem:    "bridge_http_forwarder",
					Reason:       "unexpected_no_content",
					Retryable:    true,
					RetryAfterMs: 500,
					FallbackUsed: fallbackUsed,
				})
			} else {
				r.sendBridgeError(req.ID, -32603, message, framing)
			}
		}
		signal()
		return
	}

	if resp.StatusCode != 200 {
		message := fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body))
		if req.Method == "tools/call" {
			retryable := resp.StatusCode >= 500
			retryAfter := 0
			if retryable {
				retryAfter = 1000
			}
			r.sendToolErrorWithOptions(req.ID, message, framing, bridgeToolErrorOptions{
				ErrorCode:    "bridge_http_status_error",
				Subsystem:    "bridge_http_forwarder",
				Reason:       "http_status_error",
				Retryable:    retryable,
				RetryAfterMs: retryAfter,
				FallbackUsed: fallbackUsed,
				Detail:       fmt.Sprintf("status_code=%d", resp.StatusCode),
			})
		} else {
			r.sendBridgeError(req.ID, -32603, message, framing)
		}
		signal()
		return
	}

	if req.HasID() && len(bytes.TrimSpace(body)) == 0 {
		message := "Server returned an empty body for request with an id"
		if req.Method == "tools/call" {
			r.sendToolErrorWithOptions(req.ID, message, framing, bridgeToolErrorOptions{
				ErrorCode:    "bridge_empty_response",
				Subsystem:    "bridge_http_forwarder",
				Reason:       "empty_response",
				Retryable:    true,
				RetryAfterMs: 500,
				FallbackUsed: fallbackUsed,
			})
		} else {
			r.sendBridgeError(req.ID, -32603, message, framing)
		}
		signal()
		return
	}

	if req.HasID() && !json.Valid(body) {
		message := "Server returned invalid JSON response"
		if req.Method == "tools/call" {
			r.sendToolErrorWithOptions(req.ID, message, framing, bridgeToolErrorOptions{
				ErrorCode:    "bridge_invalid_response",
				Subsystem:    "bridge_http_forwarder",
				Reason:       "invalid_json_response",
				Retryable:    true,
				RetryAfterMs: 1000,
				FallbackUsed: fallbackUsed,
			})
		} else {
			r.sendBridgeError(req.ID, -32603, message, framing)
		}
		signal()
		return
	}

	r.transport.Write(body, framing)
	signal()
}
