// Purpose: Handles MCP-over-HTTP request/response IO and request debug capture.
// Why: Isolates transport-layer concerns from JSON-RPC method dispatch logic.

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/diag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// httpRequestContext collects metadata from an HTTP request for debug logging.
type httpRequestContext struct {
	startTime    time.Time
	extSessionID string
	clientID     string
	headers      map[string]string
}

// newHTTPRequestContext extracts metadata from the request headers.
func newHTTPRequestContext(r *http.Request, serverVersion string) httpRequestContext {
	ctx := httpRequestContext{
		startTime:    time.Now(),
		extSessionID: r.Header.Get("X-Kaboom-Ext-Session"),
		clientID:     r.Header.Get("X-Kaboom-Client"),
	}

	ctx.headers = make(map[string]string)
	for name, values := range r.Header {
		lower := strings.ToLower(name)
		if strings.Contains(lower, "auth") || strings.Contains(lower, "token") {
			ctx.headers[name] = "[REDACTED]"
		} else if len(values) > 0 {
			ctx.headers[name] = values[0]
		}
	}

	if extVer := r.Header.Get("X-Kaboom-Extension-Version"); extVer != "" && extVer != serverVersion {
		diag.Printf("[Kaboom] Version mismatch: server=%s extension=%s\n", serverVersion, extVer)
	}

	return ctx
}

// logDebugEntry logs an HTTP debug entry if capture is available.
func (h *MCPHandler) logDebugEntry(ctx httpRequestContext, requestBody string, status int, responseBody string, errMsg string) {
	if h.toolHandler == nil {
		return
	}
	cap := h.toolHandler.GetCapture()
	if cap == nil {
		return
	}
	entry := capture.HTTPDebugEntry{
		Timestamp:      ctx.startTime,
		Endpoint:       "/mcp",
		Method:         "POST",
		ExtSessionID:   ctx.extSessionID,
		ClientID:       ctx.clientID,
		Headers:        ctx.headers,
		RequestBody:    requestBody,
		ResponseStatus: status,
		ResponseBody:   responseBody,
		DurationMs:     time.Since(ctx.startTime).Milliseconds(),
		Error:          errMsg,
	}
	cap.LogHTTPDebugEntry(entry)
}

// truncatePreview returns s truncated to 1000 characters with "..." suffix.
func truncatePreview(s string) string {
	if len(s) > 1000 {
		return s[:1000] + "..."
	}
	return s
}

// HandleHTTP serves MCP-over-HTTP with bounded body size and debug logging.
//
// Failure semantics:
// - Non-POST or malformed JSON requests return protocol errors without invoking tool handlers.
// - Notification requests are acknowledged with HTTP 204 and no JSON-RPC body.
func (h *MCPHandler) HandleHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := newHTTPRequestContext(r, h.version)

	if r.Method != "POST" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	// Validate Content-Type: must be application/json (or empty for lenient clients)
	if ct := r.Header.Get("Content-Type"); ct != "" && !strings.Contains(ct, "application/json") {
		h.writeJSONRPCError(w, nil, -32700, "Unsupported Content-Type: "+ct)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxPostBodySize)

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		h.logDebugEntry(ctx, "", http.StatusBadRequest, "", fmt.Sprintf("Could not read body: %v", err))
		h.writeJSONRPCError(w, nil, -32700, "Read error: "+err.Error())
		return
	}

	requestPreview := truncatePreview(string(bodyBytes))

	var req mcp.JSONRPCRequest
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		h.logDebugEntry(ctx, requestPreview, http.StatusBadRequest, "", fmt.Sprintf("Parse error: %v", err))
		h.writeJSONRPCError(w, nil, -32700, "Parse error: "+err.Error())
		return
	}

	req.ClientID = ctx.clientID
	resp := h.HandleRequest(req)

	if resp == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Error impossible: simple struct with no circular refs or unsupported types
	responseJSON, _ := json.Marshal(resp)
	h.logDebugEntry(ctx, requestPreview, http.StatusOK, truncatePreview(string(responseJSON)), "")

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp) //nolint:errcheck // best-effort HTTP response write
}

// writeJSONRPCError writes a JSON-RPC error response to the HTTP response writer.
func (h *MCPHandler) writeJSONRPCError(w http.ResponseWriter, id any, code int, message string) {
	resp := mcp.JSONRPCResponse{
		JSONRPC: mcp.JSONRPCVersion,
		ID:      id,
		Error:   &mcp.JSONRPCError{Code: code, Message: message},
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp) //nolint:errcheck // best-effort HTTP error response write
}

// jsonResponse is a JSON response helper.
func jsonResponse(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		diag.Printf("[Kaboom] Error encoding JSON response: %v\n", err)
	}
}
