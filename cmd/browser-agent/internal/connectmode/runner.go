// runner.go — Connects an MCP stdio client to an existing Kaboom daemon.
// Why: Owns multi-client registration and HTTP forwarding as one explicit transport boundary.

package connectmode

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/bridge"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

const (
	healthTimeout   = 5 * time.Second
	registerTimeout = 5 * time.Second
	forwardTimeout  = 30 * time.Second
	maxScanToken    = 10 * 1024 * 1024
)

type Deps struct {
	Input       io.Reader
	HTTPClient  *http.Client
	Diagnosticf func(string, ...any)
	WriteMCP    func([]byte, bridge.StdioFraming)
	Exit        func(int)
}

type Runner struct {
	deps Deps
}

func New(deps Deps) *Runner {
	if deps.Input == nil {
		deps.Input = strings.NewReader("")
	}
	if deps.HTTPClient == nil {
		deps.HTTPClient = http.DefaultClient
	}
	if deps.Diagnosticf == nil {
		deps.Diagnosticf = func(string, ...any) {}
	}
	if deps.WriteMCP == nil {
		deps.WriteMCP = func([]byte, bridge.StdioFraming) {}
	}
	if deps.Exit == nil {
		deps.Exit = func(int) {}
	}
	return &Runner{deps: deps}
}

func (r *Runner) Run(port int, clientID, cwd string) {
	serverURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	if !r.checkHealth(serverURL, port) {
		return
	}
	r.registerClient(serverURL, clientID, cwd)
	r.deps.Diagnosticf("[Kaboom] Connected to %s (client: %s)\n", serverURL, clientID)
	r.forwardLoop(serverURL+"/mcp", clientID)
	r.unregisterClient(serverURL, clientID)
	r.deps.Diagnosticf("[Kaboom] Disconnected from %s\n", serverURL)
}

func (r *Runner) checkHealth(serverURL string, port int) bool {
	ctx, cancel := context.WithTimeout(context.Background(), healthTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, serverURL+"/health", nil)
	if err != nil {
		r.deps.Diagnosticf("[Kaboom] Failed to create health check request: %v\n", err)
		r.deps.Exit(1)
		return false
	}
	resp, err := r.deps.HTTPClient.Do(req) // #nosec G704 -- localhost URL constructed from trusted port flag
	if err != nil {
		r.deps.Diagnosticf("[Kaboom] Cannot connect to server at %s: %v\n", serverURL, err)
		r.deps.Diagnosticf("[Kaboom] Start a server first: kaboom --server --port %d\n", port)
		r.deps.Exit(1)
		return false
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		r.deps.Diagnosticf("[Kaboom] Server health check failed: %d\n", resp.StatusCode)
		r.deps.Exit(1)
		return false
	}
	return true
}

func (r *Runner) registerClient(serverURL, clientID, cwd string) {
	regBody, _ := json.Marshal(map[string]string{"cwd": cwd})
	ctx, cancel := context.WithTimeout(context.Background(), registerTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, serverURL+"/clients", strings.NewReader(string(regBody)))
	if err != nil {
		r.deps.Diagnosticf("[Kaboom] Warning: could not create registration request: %v\n", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Kaboom-Client", clientID)
	resp, err := r.deps.HTTPClient.Do(req) // #nosec G704 -- localhost-only serverURL
	if err != nil {
		r.deps.Diagnosticf("[Kaboom] Warning: could not register client: %v\n", err)
		return
	}
	_ = resp.Body.Close() // lint:body-close-ok -- no response body is consumed.
}

func (r *Runner) forwardLoop(mcpURL, clientID string) {
	scanner := bufio.NewScanner(r.deps.Input)
	scanner.Buffer(make([]byte, maxScanToken), maxScanToken)
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			r.forwardRequest(mcpURL, clientID, line)
		}
	}
}

func (r *Runner) forwardRequest(mcpURL, clientID, line string) {
	ctx, cancel := context.WithTimeout(context.Background(), forwardTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, mcpURL, strings.NewReader(line))
	if err != nil {
		r.sendMCPError(nil, -32603, "Internal error: "+err.Error())
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Kaboom-Client", clientID)
	resp, err := r.deps.HTTPClient.Do(req) // #nosec G704 -- localhost-only serverURL
	if err != nil {
		r.sendMCPError(extractRequestID(line), -32603, "Server connection error: "+err.Error())
		return
	}
	defer resp.Body.Close() //nolint:errcheck

	var respData json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		if id := extractRequestID(line); id != nil {
			r.sendMCPError(id, -32603, "Invalid server response")
		}
		return
	}
	r.deps.WriteMCP(respData, bridge.StdioFramingLine)
}

func extractRequestID(line string) any {
	var req mcp.JSONRPCRequest
	if json.Unmarshal([]byte(line), &req) == nil {
		return req.ID
	}
	return nil
}

func (r *Runner) unregisterClient(serverURL, clientID string) {
	if clientID == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), registerTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, serverURL+"/clients/"+clientID, nil)
	if err != nil {
		return
	}
	req.Header.Set("X-Kaboom-Client", clientID)
	if resp, err := r.deps.HTTPClient.Do(req); err == nil {
		_ = resp.Body.Close() // lint:body-close-ok -- no response body is consumed.
	}
}

func (r *Runner) sendMCPError(id any, code int, message string) {
	resp := mcp.JSONRPCResponse{
		JSONRPC: mcp.JSONRPCVersion,
		ID:      id,
		Error:   &mcp.JSONRPCError{Code: code, Message: message},
	}
	respJSON, _ := json.Marshal(resp)
	r.deps.WriteMCP(respJSON, bridge.StdioFramingLine)
}
