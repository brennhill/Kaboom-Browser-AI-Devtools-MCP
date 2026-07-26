// bridge.go -- Orchestrates bridge-mode transport forwarding between MCP stdio and daemon HTTP.
// Why: Keeps request/response forwarding resilient across daemon restarts and transport disruptions.
// Docs: docs/features/feature/bridge-restart/index.md

package bridge

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/bridge/stdioisolate"
	internbridge "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/bridge"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/schema"
	statecfg "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// BridgeWrapperLogFileName is the name of the bridge wrapper log file.
const BridgeWrapperLogFileName = stdioisolate.WrapperLogFileName

// ActiveMCPTransportWriter returns the file used for MCP JSON-RPC transport.
func ActiveMCPTransportWriter() *os.File { return stdioisolate.ActiveMCPTransportWriter() }

// EnsureIOIsolation configures bridge mode so stdout/stderr noise cannot
// corrupt MCP JSON-RPC framing on stdout. The host's diagnostic seams are
// resolved from deps here, at call time, so a test that swaps deps still wins.
func EnsureIOIsolation(logFileHint string) error {
	return stdioisolate.Ensure(logFileHint,
		func(w io.Writer) { deps.SetStderrSink(w) },
		func(format string, args ...any) { deps.Stderrf(format, args...) },
	)
}

// IsKaboomService accepts canonical and legacy server names for compatibility.
func IsKaboomService(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	if n == deps.MCPServerName {
		return true
	}
	for _, legacy := range deps.LegacyMCPServerNames {
		if n == legacy {
			return true
		}
	}
	return false
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
	cmd.Args[0] = deps.DaemonProcessArgv0(exe)
	// Detach the daemon's standard streams. nil => os/exec connects the fd to
	// os.DevNull (/dev/null). We must NOT use io.Discard here: os/exec routes any
	// non-*os.File writer through an OS pipe whose read-end lives in THIS bridge
	// process. Setsid detaches the daemon's session/process group but NOT the
	// inherited pipe fds. When the bridge exits on stdin_eof, those pipe read-ends
	// close, and the daemon's next stderr write dies with SIGPIPE on fd 2 (Go
	// terminates on a broken pipe to fd 1/2). /dev/null never breaks, so the
	// spawned daemon stays persistent across the bridge's exit. This mirrors the
	// installer's startDaemonSilently (native_install.go), which already uses nil.
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
	deps.SyncStdoutBestEffort()
}

// StdioToHTTPFast forwards JSON-RPC with fast-start: responds to initialize/tools/list
// immediately while daemon starts in background. Only blocks on tools/call.
// #lizard forgives
func StdioToHTTPFast(endpoint string, state *daemonState, port int) {
	reader := bufio.NewReaderSize(os.Stdin, 64*1024)

	client := &http.Client{} // per-request timeouts via context

	// Start push relay goroutine to poll daemon inbox and relay to Claude via stdio.
	pushRelayDone := make(chan struct{})
	startBridgePushRelay(client, endpoint, pushRelayDone)

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
		line, framing, err := readMCPStdioMessage(reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			deps.Debugf("stdin read error: %v", err)
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
			sendBridgeParseError(line, err, framing)
			signalResponseSent()
			continue
		}
		if req.HasInvalidID() {
			stats.invalidIDs++
			sendBridgeError(nil, -32600, "Invalid Request: id must be string or number when present", framing)
			signalResponseSent()
			continue
		}
		deps.Debugf("request method=%s id=%v", req.Method, req.ID)
		stats.lastMethod = req.Method

		// FAST PATH: Handle initialize and tools/list directly (no daemon needed)
		if handleFastPath(req, toolsList, framing) {
			stats.fastPath++
			signalResponseSent()
			continue
		}

		// RESTART FAST PATH: configure(action="restart") handled in bridge, not daemon
		if handleBridgeRestart(req, state, port, framing) {
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
				handleDaemonNotReady(req, status, signalResponseSent, framing)
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
			bridgeForwardRequest(client, endpoint, reqCopy, lineCopy, timeout, state, signalResponseSent, framing)
		})
	}

	close(pushRelayDone)
	bridgeShutdown(&wg, readErr, responseSent, stats)
}

// Forwarding, error responses, and restart handling moved to bridge_forward.go
