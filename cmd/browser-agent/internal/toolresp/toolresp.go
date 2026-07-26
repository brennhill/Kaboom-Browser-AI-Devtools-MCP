// toolresp.go — the tool response helpers that internal/mcp does not already own.
// Why: internal/mcp is the source of truth for Succeed/SucceedText/Fail/ParseArgs/
// LenientUnmarshal, and callers should use it directly. Four things were missing from
// it and had therefore been re-implemented in package main and in the tool sub-packages
// as each was extracted — with drift: three different correlation-ID bit layouts existed,
// two of which could emit a negative random component because they never masked the sign
// bit. This package holds those four, once.
//
// Stateless and I/O-free, so any tool package may depend on it.

package toolresp

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// SucceedRaw builds a success JSONRPCResponse with a pre-built Result payload.
func SucceedRaw(req mcp.JSONRPCRequest, result json.RawMessage) mcp.JSONRPCResponse {
	return mcp.JSONRPCResponse{JSONRPC: mcp.JSONRPCVersion, ID: req.ID, Result: result}
}

// FailJSON builds an error JSONRPCResponse with a JSON data payload (isError=true).
func FailJSON(req mcp.JSONRPCRequest, summary string, data any) mcp.JSONRPCResponse {
	return mcp.JSONRPCResponse{JSONRPC: mcp.JSONRPCVersion, ID: req.ID, Result: mcp.JSONErrorResponse(summary, data)}
}

// NewCorrelationID generates a unique correlation ID with the given prefix.
// Format: prefix_timestamp_random (e.g., "nav_1708300000000000000_4821937562").
// Only the prefix is ever parsed back out (see usageKey); the rest is opaque.
func NewCorrelationID(prefix string) string {
	return fmt.Sprintf("%s_%d_%d", prefix, time.Now().UnixNano(), RandomInt63())
}

// RandomInt63 returns a cryptographically random non-negative int64, falling back
// to the wall clock if the system entropy source fails.
func RandomInt63() int64 {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Fallback to time-based if rand fails (should never happen).
		return time.Now().UnixNano()
	}
	// #nosec G115 -- masked to the positive int63 range (top bit cleared); the uint64->int64 conversion cannot overflow.
	return int64(binary.BigEndian.Uint64(b[:]) & 0x7FFFFFFFFFFFFFFF)
}
