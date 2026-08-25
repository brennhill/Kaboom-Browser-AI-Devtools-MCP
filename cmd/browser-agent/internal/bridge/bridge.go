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
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/bridge/fastpathtelemetry"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/bridge/pushrelay"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/bridge/stdioisolate"
	internbridge "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/bridge"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/incident"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/push"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/schema"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/telemetry"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// Identity describes the stable MCP identity exposed by one runner.
type Identity struct {
	Version            string
	ServerName         string
	ServerInstructions string
}

// Transport owns diagnostic and MCP transport output.
type Transport struct {
	MaxBodySize int64
	Stderrf     func(string, ...any)
	Debugf      func(string, ...any)
	Write       func([]byte, internbridge.StdioFraming)
	Sync        func()
	SetStderr   func(io.Writer)
}

// Protocol owns negotiated MCP state and fast-path content.
type Protocol struct {
	GetFraming          func() internbridge.StdioFraming
	StoreFraming        func(internbridge.StdioFraming)
	SetCapabilities     func(push.ClientCapabilities)
	ExtractCapabilities func(json.RawMessage) push.ClientCapabilities
	NegotiateVersion    func(json.RawMessage) string
	Resources           func() []mcp.MCPResource
	ResourceTemplates   func() []any
	ResolveResource     func(string) (string, string, bool)
}

// Lifecycle owns process operations needed to start and recycle the daemon.
type Lifecycle struct {
	ProcessArgv0         func(string) string
	StopServerForUpgrade func(int) bool
	FindProcessOnPort    func(int) ([]int, error)
	IsProcessAlive       func(int) bool
	AppendExitDiagnostic func(string, map[string]any) string
}

// Runner owns a bridge session and its change-coupled collaborators.
type Runner struct {
	identity  Identity
	transport Transport
	protocol  Protocol
	lifecycle Lifecycle
	sleep     func(time.Duration)
}

// NewRunner constructs an independent bridge runtime.
func NewRunner(identity Identity, transport Transport, protocol Protocol, lifecycle Lifecycle) *Runner {
	return &Runner{identity: identity, transport: transport, protocol: protocol, lifecycle: lifecycle, sleep: time.Sleep}
}

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
	defer fastpathtelemetry.Flush()
	reader := bufio.NewReaderSize(os.Stdin, 64*1024)

	client := &http.Client{} // per-request timeouts via context

	// Start push relay goroutine to poll daemon inbox and relay to Claude via stdio.
	pushRelayDone := make(chan struct{})
	pushrelay.New(client, endpoint, pushrelay.Deps{
		Framing: r.protocol.GetFraming,
		Write:   r.transport.Write,
		Debugf:  r.transport.Debugf,
	}).Start(pushRelayDone)

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
			r.bridgeForwardRequest(forwardTarget{client: client, endpoint: endpoint, timeout: timeout}, reqCopy, lineCopy, state, signalResponseSent, framing)
		})
	}

	close(pushRelayDone)
	r.bridgeShutdown(&wg, readErr, responseSent, stats)
}

// bridgeDoHTTP delegates to internal/bridge for HTTP forwarding.
func bridgeDoHTTP(ctx context.Context, client *http.Client, endpoint string, line []byte) (*http.Response, error) {
	return internbridge.DoHTTP(ctx, client, endpoint, line)
}

// bridgeSendWithRespawn performs the initial HTTP forward and, on a connection
// error while state is non-nil, attempts a single respawn + retry with a fresh
// context (the original context may have little time left after respawn delay).
// The returned cancel func must be deferred by the caller.
func bridgeSendWithRespawn(client *http.Client, endpoint string, line []byte, timeout time.Duration, state *daemonState) (*http.Response, error, context.CancelFunc, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	activeCancel := cancel
	fallbackUsed := false

	resp, err := bridgeDoHTTP(ctx, client, endpoint, line)
	if err != nil && isConnectionError(err) && state != nil {
		fallbackUsed = true
		if state.respawnIfNeeded() {
			cancel()
			retryCtx, retryCancel := context.WithTimeout(context.Background(), timeout)
			resp, err = bridgeDoHTTP(retryCtx, client, endpoint, line)
			activeCancel = retryCancel
		}
	}
	return resp, err, activeCancel, fallbackUsed
}

// sendForwardFailure writes a forwarding failure as a structured tool error for
// tools/call requests and as a plain JSON-RPC error otherwise.
func (r *Runner) sendForwardFailure(req mcp.JSONRPCRequest, message string, framing internbridge.StdioFraming, fallbackUsed bool, toolOpts bridgeToolErrorOptions) {
	if req.Method == "tools/call" {
		toolOpts.FallbackUsed = fallbackUsed
		r.sendToolErrorWithOptions(req.ID, message, framing, toolOpts)
		return
	}
	r.sendBridgeError(req.ID, -32603, message, framing)
}

// forwardTarget names the daemon HTTP endpoint a forwarded request is sent to,
// bundling the client, URL, and per-request timeout the forwarder needs.
type forwardTarget struct {
	client   *http.Client
	endpoint string
	timeout  time.Duration
}

// bridgeForwardRequest forwards a JSON-RPC request to the HTTP server and writes the response.
// If state is non-nil and the daemon is unreachable, attempts a single respawn + retry.
func (r *Runner) bridgeForwardRequest(target forwardTarget, req mcp.JSONRPCRequest, line []byte, state *daemonState, signal func(), framing internbridge.StdioFraming) {
	resp, err, activeCancel, fallbackUsed := bridgeSendWithRespawn(target.client, target.endpoint, line, target.timeout, state)
	defer activeCancel()
	if err != nil {
		telemetry.AppError(incident.CodeBridgeConnectionError)
		r.sendForwardFailure(req, "Server connection error: "+err.Error(), framing, fallbackUsed, bridgeToolErrorOptions{
			ErrorCode:    "bridge_connection_error",
			Subsystem:    "bridge_http_forwarder",
			Reason:       "http_forward_failed",
			Retryable:    true,
			RetryAfterMs: 2000,
			Detail:       err.Error(),
		})
		signal()
		return
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, r.transport.MaxBodySize))
	_ = resp.Body.Close() //nolint:errcheck // best-effort cleanup
	if err != nil {
		r.sendForwardFailure(req, "Failed to read response: "+err.Error(), framing, fallbackUsed, bridgeToolErrorOptions{
			ErrorCode:    "bridge_response_read_error",
			Subsystem:    "bridge_http_forwarder",
			Reason:       "response_read_failed",
			Retryable:    true,
			RetryAfterMs: 1000,
			Detail:       err.Error(),
		})
		signal()
		return
	}

	if resp.StatusCode == 204 {
		if req.HasID() {
			r.sendForwardFailure(req, "Server returned no content for request with an id", framing, fallbackUsed, bridgeToolErrorOptions{
				ErrorCode:    "bridge_unexpected_no_content",
				Subsystem:    "bridge_http_forwarder",
				Reason:       "unexpected_no_content",
				Retryable:    true,
				RetryAfterMs: 500,
			})
		}
		signal()
		return
	}

	if resp.StatusCode != 200 {
		retryable := resp.StatusCode >= 500
		retryAfter := 0
		if retryable {
			retryAfter = 1000
		}
		r.sendForwardFailure(req, fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body)), framing, fallbackUsed, bridgeToolErrorOptions{
			ErrorCode:    "bridge_http_status_error",
			Subsystem:    "bridge_http_forwarder",
			Reason:       "http_status_error",
			Retryable:    retryable,
			RetryAfterMs: retryAfter,
			Detail:       fmt.Sprintf("status_code=%d", resp.StatusCode),
		})
		signal()
		return
	}

	if req.HasID() && len(bytes.TrimSpace(body)) == 0 {
		r.sendForwardFailure(req, "Server returned an empty body for request with an id", framing, fallbackUsed, bridgeToolErrorOptions{
			ErrorCode:    "bridge_empty_response",
			Subsystem:    "bridge_http_forwarder",
			Reason:       "empty_response",
			Retryable:    true,
			RetryAfterMs: 500,
		})
		signal()
		return
	}

	if req.HasID() && !json.Valid(body) {
		r.sendForwardFailure(req, "Server returned invalid JSON response", framing, fallbackUsed, bridgeToolErrorOptions{
			ErrorCode:    "bridge_invalid_response",
			Subsystem:    "bridge_http_forwarder",
			Reason:       "invalid_json_response",
			Retryable:    true,
			RetryAfterMs: 1000,
		})
		signal()
		return
	}

	r.transport.Write(body, framing)
	signal()
}
