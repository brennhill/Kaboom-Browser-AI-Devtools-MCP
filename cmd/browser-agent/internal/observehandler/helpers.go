// helpers.go — Response builders and utilities for observe-local handlers.
// Why: Thin wrappers around internal/mcp so handlers use concise call sites.

package observehandler

import (
	"crypto/rand"
	"fmt"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// succeed builds a success JSONRPCResponse with a JSON summary + data payload.
func succeed(req mcp.JSONRPCRequest, summary string, data any) mcp.JSONRPCResponse {
	return mcp.JSONRPCResponse{JSONRPC: mcp.JSONRPCVersion, ID: req.ID, Result: mcp.JSONResponse(summary, data)}
}

// fail builds an error JSONRPCResponse with a structured error payload (isError=true).
func fail(req mcp.JSONRPCRequest, code, message, retry string, opts ...func(*mcp.StructuredError)) mcp.JSONRPCResponse {
	return mcp.JSONRPCResponse{JSONRPC: mcp.JSONRPCVersion, ID: req.ID, Result: mcp.StructuredErrorResponse(code, message, retry, opts...)}
}

// newCorrelationID generates a unique correlation ID with the given prefix.
func newCorrelationID(prefix string) string {
	return fmt.Sprintf("%s_%d_%d", prefix, time.Now().UnixNano(), randomInt63())
}

func randomInt63() int64 {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return time.Now().UnixNano()
	}
	return int64(b[0])<<56 | int64(b[1])<<48 | int64(b[2])<<40 | int64(b[3])<<32 |
		int64(b[4])<<24 | int64(b[5])<<16 | int64(b[6])<<8 | int64(b[7])
}
