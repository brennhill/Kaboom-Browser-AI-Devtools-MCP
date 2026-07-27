// Purpose: Implements multi-client connect mode, forwarding MCP stdin/stdout to an existing daemon via HTTP.
// Why: Enables multiple Claude Code sessions to share a single Kaboom server with client ID isolation.

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/bridge"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/diag"
)

const (
	// connectModeHealthTimeout is the deadline for the initial health check when
	// connecting to an existing daemon.
	connectModeHealthTimeout = 5 * time.Second

	// connectModeRegisterTimeout is the deadline for client registration and
	// unregistration requests in connect mode.
	connectModeRegisterTimeout = 5 * time.Second

	// connectModeForwardTimeout is the deadline for forwarding individual JSON-RPC
	// requests from the connect-mode client to the daemon.
	connectModeForwardTimeout = 30 * time.Second
)

// runConnectMode connects to an existing Kaboom server as an MCP client.
// This enables multiple Claude Code sessions to share a single server.
// The client ID is sent via X-Kaboom-Client header for state isolation.
func runConnectMode(port int, clientID string, cwd string) {
	serverURL := fmt.Sprintf("http://127.0.0.1:%d", port)

	connectCheckHealth(serverURL, port)
	connectRegisterClient(serverURL, clientID, cwd)

	diag.Printf("[Kaboom] Connected to %s (client: %s)\n", serverURL, clientID)

	connectForwardLoop(serverURL+"/mcp", clientID)

	connectUnregisterClient(serverURL, clientID)

	diag.Printf("[Kaboom] Disconnected from %s\n", serverURL)
}

// connectCheckHealth verifies the server is running. Exits on failure.
func connectCheckHealth(serverURL string, port int) {
	ctx, cancel := context.WithTimeout(context.Background(), connectModeHealthTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", serverURL+"/health", nil)
	if err != nil {
		diag.Printf("[Kaboom] Failed to create health check request: %v\n", err)
		os.Exit(1)
	}

	resp, err := http.DefaultClient.Do(req) // #nosec G107,G704 -- localhost URL constructed from trusted port flag
	if err != nil {
		diag.Printf("[Kaboom] Cannot connect to server at %s: %v\n", serverURL, err)
		diag.Printf("[Kaboom] Start a server first: kaboom --server --port %d\n", port)
		os.Exit(1)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		diag.Printf("[Kaboom] Server health check failed: %d\n", resp.StatusCode)
		os.Exit(1)
	}
}

// connectRegisterClient registers this client with the server (best-effort).
func connectRegisterClient(serverURL, clientID, cwd string) {
	// Error impossible: map contains only string values
	regBody, _ := json.Marshal(map[string]string{"cwd": cwd})

	ctx, cancel := context.WithTimeout(context.Background(), connectModeRegisterTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", serverURL+"/clients", strings.NewReader(string(regBody)))
	if err != nil {
		diag.Printf("[Kaboom] Warning: could not create registration request: %v\n", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Kaboom-Client", clientID)

	resp, err := http.DefaultClient.Do(req) // #nosec G704 -- request targets localhost-only serverURL
	if err != nil {
		diag.Printf("[Kaboom] Warning: could not register client: %v\n", err)
		return
	}
	_ = resp.Body.Close() //nolint:errcheck // best-effort cleanup after client registration
}

// connectForwardLoop reads JSON-RPC from stdin and forwards to the server.
func connectForwardLoop(mcpURL, clientID string) {
	scanner := bufio.NewScanner(os.Stdin)
	const maxScanTokenSize = 10 * 1024 * 1024
	buf := make([]byte, maxScanTokenSize)
	scanner.Buffer(buf, maxScanTokenSize)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		connectForwardRequest(mcpURL, clientID, line)
	}
}

// connectForwardRequest forwards a single JSON-RPC request to the server.
func connectForwardRequest(mcpURL, clientID, line string) {
	ctx, cancel := context.WithTimeout(context.Background(), connectModeForwardTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", mcpURL, strings.NewReader(line)) // #nosec G601 -- URL from localhost-only serverURL
	if err != nil {
		sendMCPError(nil, -32603, "Internal error: "+err.Error())
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Kaboom-Client", clientID)

	resp, err := http.DefaultClient.Do(req) // #nosec G704 -- request targets localhost-only serverURL
	if err != nil {
		id := extractRequestID(line)
		sendMCPError(id, -32603, "Server connection error: "+err.Error())
		return
	}

	var respData json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		_ = resp.Body.Close() //nolint:errcheck // best-effort cleanup after decode error
		id := extractRequestID(line)
		if id != nil {
			sendMCPError(id, -32603, "Invalid server response")
		}
		return
	}
	_ = resp.Body.Close() //nolint:errcheck // best-effort cleanup after successful decode

	writeMCPPayload(respData, bridge.StdioFramingLine)
}

// extractRequestID attempts to extract the JSON-RPC ID from a request string.
func extractRequestID(line string) any {
	var jsonReq JSONRPCRequest
	if json.Unmarshal([]byte(line), &jsonReq) == nil {
		return jsonReq.ID
	}
	return nil
}

// connectUnregisterClient unregisters this client from the server (best-effort).
func connectUnregisterClient(serverURL, clientID string) {
	if clientID == "" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), connectModeRegisterTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "DELETE", serverURL+"/clients/"+clientID, nil)
	if err != nil {
		return
	}
	req.Header.Set("X-Kaboom-Client", clientID)
	resp, err := http.DefaultClient.Do(req) // #nosec G704 -- request targets localhost-only serverURL
	if err == nil {
		_ = resp.Body.Close() //nolint:errcheck // best-effort cleanup after unregister
	}
}

// sendMCPError sends a framed JSON-RPC error response in connect mode.
func sendMCPError(id any, code int, message string) {
	resp := JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Error: &JSONRPCError{
			Code:    code,
			Message: message,
		},
	}
	// Error impossible: simple struct with no circular refs or unsupported types
	respJSON, _ := json.Marshal(resp)
	writeMCPPayload(respJSON, bridge.StdioFramingLine)
}
