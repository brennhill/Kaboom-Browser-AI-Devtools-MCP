// handler.go — MCP-over-HTTP parsing, response encoding, and debug capture.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package mcphttp

import (
	"encoding/json"
	"fmt"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/httpapi"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/diag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

type Config struct {
	Version       string
	MaxBodySize   int64
	HandleRequest func(mcp.JSONRPCRequest) *mcp.JSONRPCResponse
	Capture       func() *capture.Capture
}

type Handler struct{ config Config }

func New(config Config) *Handler { return &Handler{config: config} }

type requestContext struct {
	startTime    time.Time
	extSessionID string
	clientID     string
	headers      map[string]string
}

func newRequestContext(request *http.Request, serverVersion string) requestContext {
	ctx := requestContext{startTime: time.Now(), extSessionID: request.Header.Get("X-Kaboom-Ext-Session"),
		clientID: request.Header.Get("X-Kaboom-Client"), headers: make(map[string]string)}
	for name, values := range request.Header {
		lower := strings.ToLower(name)
		if strings.Contains(lower, "auth") || strings.Contains(lower, "token") {
			ctx.headers[name] = "[REDACTED]"
		} else if len(values) > 0 {
			ctx.headers[name] = values[0]
		}
	}
	if extensionVersion := request.Header.Get("X-Kaboom-Extension-Version"); extensionVersion != "" && extensionVersion != serverVersion {
		diag.Printf("[Kaboom] Version mismatch: server=%s extension=%s\n", serverVersion, extensionVersion)
	}
	return ctx
}

func (handler *Handler) log(ctx requestContext, requestBody string, status int, responseBody, message string) {
	if handler.config.Capture == nil {
		return
	}
	store := handler.config.Capture()
	if store == nil {
		return
	}
	store.LogHTTPDebugEntry(types.HTTPDebugEntry{
		Timestamp: ctx.startTime, Endpoint: "/mcp", Method: http.MethodPost,
		ExtSessionID: ctx.extSessionID, ClientID: ctx.clientID, Headers: ctx.headers,
		RequestBody: requestBody, ResponseStatus: status, ResponseBody: responseBody,
		DurationMs: time.Since(ctx.startTime).Milliseconds(), Error: message,
	})
}

func preview(value string) string {
	if len(value) > 1000 {
		return value[:1000] + "..."
	}
	return value
}

func (handler *Handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	ctx := newRequestContext(request, handler.config.Version)
	if request.Method != http.MethodPost {
		httpapi.JSON(writer, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}
	if contentType := request.Header.Get("Content-Type"); contentType != "" && !strings.Contains(contentType, "application/json") {
		writeJSONRPCError(writer, nil, -32700, "Unsupported Content-Type: "+contentType)
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, handler.config.MaxBodySize)
	body, err := io.ReadAll(request.Body)
	if err != nil {
		handler.log(ctx, "", http.StatusBadRequest, "", fmt.Sprintf("Could not read body: %v", err))
		writeJSONRPCError(writer, nil, -32700, "Read error: "+err.Error())
		return
	}
	requestPreview := preview(string(body))
	var rpcRequest mcp.JSONRPCRequest
	if err := json.Unmarshal(body, &rpcRequest); err != nil {
		handler.log(ctx, requestPreview, http.StatusBadRequest, "", fmt.Sprintf("Parse error: %v", err))
		writeJSONRPCError(writer, nil, -32700, "Parse error: "+err.Error())
		return
	}
	rpcRequest.ClientID = ctx.clientID
	response := handler.config.HandleRequest(rpcRequest)
	if response == nil {
		writer.WriteHeader(http.StatusNoContent)
		return
	}
	responseJSON, _ := json.Marshal(response)
	handler.log(ctx, requestPreview, http.StatusOK, preview(string(responseJSON)), "")
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(response)
}

func writeJSONRPCError(writer http.ResponseWriter, id any, code int, message string) {
	response := mcp.JSONRPCResponse{JSONRPC: mcp.JSONRPCVersion, ID: id,
		Error: &mcp.JSONRPCError{Code: code, Message: message}}
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(response)
}
