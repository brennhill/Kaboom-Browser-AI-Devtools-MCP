// bridge_transport.go -- Everything the stdio<->HTTP loop needs that is not the
// loop itself: stdio framing, the JSON-RPC error envelopes the bridge emits on
// its own behalf, the configure(action="restart") fast path, and shutdown
// accounting. These were three files split by topic, but they are one layer —
// every one of them exists only to serve StdioToHTTPFast, and none is reachable
// from anywhere else.
// Docs: docs/features/feature/bridge-restart/index.md

package bridge

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	internbridge "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/bridge"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/telemetry"
)

const (
	// bridgeShutdownResponseDrain is the maximum time to wait for the last response
	// to be sent before closing the bridge stdio connection.
	bridgeShutdownResponseDrain = 5 * time.Second

	// bridgeShutdownFlushDelay is the pause after flushing stdout to let the
	// parent process read the final bytes before the bridge process exits.
	bridgeShutdownFlushDelay = 100 * time.Millisecond
)

type bridgeSessionStats struct {
	requests             int
	parseErrors          int
	invalidIDs           int
	fastPath             int
	forwarded            int
	methodNotFound       int
	starting             int
	lineFraming          int
	contentLengthFraming int
	lastMethod           string
}

// readMCPStdioMessage delegates to internal/bridge for stdio message parsing.
func readMCPStdioMessage(reader *bufio.Reader) ([]byte, internbridge.StdioFraming, error) {
	return internbridge.ReadStdioMessageWithMode(reader, int(deps.MaxPostBodySize))
}

// bridgeShutdown waits for in-flight requests and performs clean shutdown.
func bridgeShutdown(wg *sync.WaitGroup, readErr error, responseSent chan bool, stats *bridgeSessionStats) {
	wg.Wait()
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		deps.Stderrf("[kaboom-bridge] ERROR: stdin read error: %v\n", readErr)
	}

	select {
	case <-responseSent:
	case <-time.After(bridgeShutdownResponseDrain):
	}
	close(responseSent)

	FlushStdout()
	time.Sleep(bridgeShutdownFlushDelay)

	if stats != nil {
		// PRIVACY: beacon props must NOT include readErr or any error messages.
		// The extra map below (for the exit diagnostic recorder) intentionally includes
		// more detail because it writes to a local file, not to telemetry.
		if stats.parseErrors > 0 || stats.methodNotFound > 0 || (readErr != nil && !errors.Is(readErr, io.EOF)) {
			telemetry.AppError("bridge_exit_error", nil)
		}
		reason := "stdin_eof"
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			reason = "stdin_read_error"
		}
		extra := map[string]any{
			"reason":                 reason,
			"requests":               stats.requests,
			"parse_errors":           stats.parseErrors,
			"invalid_ids":            stats.invalidIDs,
			"fast_path":              stats.fastPath,
			"forwarded":              stats.forwarded,
			"method_not_found":       stats.methodNotFound,
			"starting_retries":       stats.starting,
			"line_framing":           stats.lineFraming,
			"content_length_framing": stats.contentLengthFraming,
			"last_method":            stats.lastMethod,
		}
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			extra["read_error"] = readErr.Error()
		}
		_ = deps.AppendExitDiagnostic("bridge_exit", extra)
	}
}

type bridgeToolErrorOptions struct {
	ErrorCode     string
	Subsystem     string
	Reason        string
	Retryable     bool
	RetryAfterMs  int
	FallbackUsed  bool
	CorrelationID string
	Detail        string
}

// handleDaemonNotReady sends appropriate error responses when the daemon is not available.
func handleDaemonNotReady(req mcp.JSONRPCRequest, status string, signal func(), framing internbridge.StdioFraming) {
	switch status {
	case "method_not_found":
		sendBridgeError(req.ID, -32601, "Method not found: "+req.Method, framing)
	case "starting":
		sendToolErrorWithOptions(req.ID, "Server is starting up. Please retry this tool call in 2 seconds.", framing, bridgeToolErrorOptions{
			ErrorCode:    "daemon_starting",
			Subsystem:    "bridge_startup",
			Reason:       "daemon_starting",
			Retryable:    true,
			RetryAfterMs: 2000,
		})
	default:
		sendToolErrorWithOptions(req.ID, status, framing, bridgeToolErrorOptions{
			ErrorCode:    "daemon_not_ready",
			Subsystem:    "bridge_startup",
			Reason:       "daemon_not_ready",
			Retryable:    true,
			RetryAfterMs: 2000,
		})
	}
	signal()
}

// sendBridgeParseError sends a JSON-RPC parse error (id must be null per spec).
func sendBridgeParseError(_ []byte, err error, framing internbridge.StdioFraming) {
	errResp := mcp.JSONRPCResponse{
		JSONRPC: mcp.JSONRPCVersion,
		ID:      nil, // JSON-RPC: parse errors must have null id
		Error:   &mcp.JSONRPCError{Code: -32700, Message: "Parse error: " + err.Error()},
	}
	// Error impossible: simple struct with no circular refs or unsupported types
	respJSON, _ := json.Marshal(errResp)
	deps.WriteMCPPayload(respJSON, framing)
}

// sendBridgeError sends a JSON-RPC error response to stdout.
func sendBridgeError(id any, code int, message string, framing internbridge.StdioFraming) {
	errResp := mcp.JSONRPCResponse{
		JSONRPC: mcp.JSONRPCVersion,
		ID:      id,
		Error: &mcp.JSONRPCError{
			Code:    code,
			Message: message,
		},
	}
	// Error impossible: simple struct with no circular refs or unsupported types
	respJSON, _ := json.Marshal(errResp)
	deps.WriteMCPPayload(respJSON, framing)
}

func sendToolErrorWithOptions(id any, message string, framing internbridge.StdioFraming, opts bridgeToolErrorOptions) {
	errorCode := opts.ErrorCode
	if errorCode == "" {
		errorCode = "bridge_tool_error"
	}
	subsystem := opts.Subsystem
	if subsystem == "" {
		subsystem = "bridge"
	}
	reason := opts.Reason
	if reason == "" {
		reason = "tool_error"
	}
	correlationID := opts.CorrelationID
	if correlationID == "" {
		correlationID = bridgeRequestIDString(id)
	}

	result := map[string]any{
		"content": []map[string]any{
			{"type": "text", "text": message},
		},
		"isError":        true,
		"status":         "error",
		"error_code":     errorCode,
		"subsystem":      subsystem,
		"reason":         reason,
		"retryable":      opts.Retryable,
		"fallback_used":  opts.FallbackUsed,
		"correlation_id": correlationID,
	}
	if opts.Retryable && opts.RetryAfterMs > 0 {
		result["retry_after_ms"] = opts.RetryAfterMs
	}
	if opts.Detail != "" {
		result["detail"] = opts.Detail
	}
	// Error impossible: map contains only primitive types and nested maps
	resultJSON, _ := json.Marshal(result)
	resp := mcp.JSONRPCResponse{
		JSONRPC: mcp.JSONRPCVersion,
		ID:      id,
		Result:  resultJSON,
	}
	// Error impossible: simple struct with no circular refs or unsupported types
	respJSON, _ := json.Marshal(resp)
	deps.WriteMCPPayload(respJSON, framing)
}

func bridgeRequestIDString(id any) string {
	switch v := id.(type) {
	case nil:
		return ""
	case string:
		return v
	case json.RawMessage:
		if len(v) == 0 {
			return ""
		}
		var s string
		if err := json.Unmarshal(v, &s); err == nil {
			return s
		}
		var n float64
		if err := json.Unmarshal(v, &n); err == nil {
			return strconv.FormatFloat(n, 'f', -1, 64)
		}
		return strings.TrimSpace(string(v))
	case fmt.Stringer:
		return v.String()
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	default:
		return fmt.Sprintf("%v", v)
	}
}

const (
	// restartGracePeriod is the maximum time to wait for the daemon to become ready
	// after a bridge-initiated restart before reporting a timeout.
	restartGracePeriod = 6 * time.Second
)

// ExtractToolAction delegates to internal/bridge for tool action extraction.
func ExtractToolAction(req mcp.JSONRPCRequest) (toolName, action string) {
	return internbridge.ExtractToolAction(req.Method, req.Params)
}

// forceKillOnPort sends SIGCONT then SIGKILL to any process on the given port.
// This handles edge cases where the daemon is frozen (SIGSTOP) and can't process
// SIGTERM from stopServerForUpgrade's normal escalation path.
func forceKillOnPort(port int) {
	pids, err := deps.FindProcessOnPort(port)
	if err != nil {
		return
	}
	self := os.Getpid()
	for _, pid := range pids {
		if pid == self {
			continue
		}
		p, err := os.FindProcess(pid)
		if err != nil {
			continue
		}
		// On Unix this sends SIGCONT; on Windows this is a no-op.
		signalResumeProcess(p)
	}
}

// handleBridgeRestart handles configure(action="restart") in the bridge layer.
// This is a fast path that works even when the daemon is unresponsive.
// Returns true if the request was handled.
func handleBridgeRestart(req mcp.JSONRPCRequest, state *daemonState, port int, framing internbridge.StdioFraming) bool {
	tool, action := ExtractToolAction(req)
	if tool != "configure" || action != "restart" {
		return false
	}

	deps.Stderrf("[Kaboom] bridge restart requested, stopping daemon on port %d\n", port)

	// Kill the daemon (3-layer: HTTP -> PID -> lsof).
	// First send SIGCONT to unfreeze any SIGSTOP'd process so signals can be delivered.
	forceKillOnPort(port)
	stopped := deps.StopServerForUpgrade(port)

	// Reset bridge state for fresh spawn.
	readyCh, failedCh := func() (chan struct{}, chan struct{}) {
		state.mu.Lock()
		defer state.mu.Unlock()
		state.ready = false
		state.failed = false
		state.err = ""
		state.resetSignalsLocked()
		return state.readyCh, state.failedCh
	}()

	// Spawn fresh daemon.
	spawnDaemonAsync(state)

	// Wait for daemon to become ready (6s timeout).
	var status, message string
	select {
	case <-readyCh:
		status = "ok"
		message = "Daemon restarted successfully"
		deps.Stderrf("[Kaboom] daemon restarted successfully on port %d\n", port)
	case <-failedCh:
		errMsg := DaemonFailureErr(state)
		status = "error"
		message = "Daemon restart failed: " + errMsg
		deps.Stderrf("[Kaboom] daemon restart failed: %s\n", errMsg)
	case <-time.After(restartGracePeriod):
		status = "error"
		message = "Daemon restart timed out after 6s"
		deps.Stderrf("[Kaboom] daemon restart timed out\n")
	}

	result := map[string]any{
		"status":           status,
		"restarted":        status == "ok",
		"message":          message,
		"previous_stopped": stopped,
	}
	resultJSON, _ := json.Marshal(result)
	toolResult := map[string]any{
		"content": []map[string]any{
			{"type": "text", "text": string(resultJSON)},
		},
	}
	if status != "ok" {
		toolResult["isError"] = true
	}
	toolResultJSON, _ := json.Marshal(toolResult)
	sendFastResponse(req.ID, toolResultJSON, framing)
	return true
}
