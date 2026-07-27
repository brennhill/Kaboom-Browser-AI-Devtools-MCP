// Purpose: Serializes and writes MCP JSON-RPC payloads to stdout with content-length or line framing.
// Why: Guarantees stdout output invariants (valid JSON, single newline) required by the MCP transport spec.

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"

	cmbridge "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/bridge"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/bridge"
)

func syncStdoutBestEffort() {
	if err := cmbridge.ActiveMCPTransportWriter().Sync(); err != nil && !isIgnorableStdoutSyncError(err) {
		stderrf("[Kaboom] warning: stdout.Sync failed: %v\n", err)
	}
}

func isIgnorableStdoutSyncError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, syscall.EINVAL) || errors.Is(err, syscall.EBADF) {
		return true
	}
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return errors.Is(pathErr.Err, syscall.EINVAL) || errors.Is(pathErr.Err, syscall.EBADF)
	}
	return false
}

// writeMCPPayload is the only stdout emitter used by MCP wrapper responses.
func writeMCPPayload(payload []byte, framing bridge.StdioFraming) {
	normalized := normalizeMCPPayload(payload)
	out := cmbridge.ActiveMCPTransportWriter()
	mcpStdoutMu.Lock()
	defer mcpStdoutMu.Unlock()
	if framing == bridge.StdioFramingContentLength {
		_, _ = fmt.Fprintf(out, "Content-Length: %d\r\nContent-Type: application/json\r\n\r\n%s", len(normalized), normalized)
	} else {
		// Wrapper invariant: line-framed MCP output is always exactly one JSON
		// payload plus exactly one newline.
		_, _ = out.Write(normalized)
		_, _ = out.Write([]byte("\n"))
	}
	cmbridge.FlushStdout()
}

// normalizeMCPPayload trims outer whitespace and guarantees a valid JSON payload.
func normalizeMCPPayload(payload []byte) []byte {
	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) > 0 && json.Valid(trimmed) {
		return trimmed
	}

	stderrf("[kaboom-bridge] ERROR: stdout invariant violation: invalid JSON payload (len=%d)\n", len(payload))
	errResp := JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      nil,
		Error: &JSONRPCError{
			Code:    -32603,
			Message: "Wrapper emitted invalid JSON payload",
		},
	}
	// Error impossible: simple struct with no circular refs or unsupported types.
	respJSON, _ := json.Marshal(errResp)
	return respJSON
}

// sendStartupError sends a JSON-RPC error response before exiting.
// This ensures the parent process (IDE) receives a proper error instead of empty response.
func sendStartupError(message string) {
	errResp := JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      "startup",
		Error: &JSONRPCError{
			Code:    -32603,
			Message: message,
		},
	}
	// Error impossible: simple struct with no circular refs or unsupported types
	respJSON, _ := json.Marshal(errResp)
	writeMCPPayload(respJSON, bridge.StdioFramingLine)
	syncStdoutBestEffort()
	time.Sleep(100 * time.Millisecond) // Allow OS to flush pipe to parent
}
