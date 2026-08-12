// handler.go — Owns MCP endpoint routing and response-policy composition.

package mcpendpoint

import (
	"fmt"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/appruntime"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/mcpcall"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/mcpresponse"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/mcprouter"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/mcptelemetry"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// Config supplies immutable endpoint identity and host-owned policy seams.
type Config struct {
	Version       string
	Runtime       *appruntime.Runtime
	ErrorTotal    func() int64
	TelemetryMode func() string
	AddWarning    func(string)
	DrainWarnings func() []string
	PendingAudit  func() bool
}

// Handler owns JSON-RPC routing and response post-processing for one endpoint.
type Handler struct {
	backend          mcpcall.Backend
	version          string
	addWarning       func(string)
	passiveTelemetry *mcptelemetry.Owner
	responsePolicy   *mcpresponse.Owner
}

// New constructs a complete endpoint from explicit host policy seams and its
// immutable tool backend.
func New(config Config, backend mcpcall.Backend) *Handler {
	runtime := config.Runtime
	if runtime == nil {
		runtime = appruntime.New(config.Version)
	}
	handler := &Handler{
		backend: backend, version: config.Version,
		addWarning: config.AddWarning,
		passiveTelemetry: mcptelemetry.New(mcptelemetry.Config{
			ErrorTotal: config.ErrorTotal,
			Mode:       config.TelemetryMode,
		}),
		responsePolicy: mcpresponse.New(mcpresponse.Config{
			Runtime: runtime, AddWarning: config.AddWarning,
			DrainWarnings: config.DrainWarnings, PendingAudit: config.PendingAudit,
		}),
	}
	handler.passiveTelemetry.SetCapture(backend.Capture)
	handler.responsePolicy.SetCapture(backend.Capture)
	return handler
}

// HandleRequest validates and routes one JSON-RPC request.
func (h *Handler) HandleRequest(request mcp.JSONRPCRequest) *mcp.JSONRPCResponse {
	return mcprouter.Handle(request, mcprouter.Config{
		Version: h.version,
		Schemas: h.backend.Schemas,
		ToolCall: func(request mcp.JSONRPCRequest) mcp.JSONRPCResponse {
			return mcpcall.Handle(request, h.backend, h.responsePolicy, h.passiveTelemetry)
		},
		OnOversizedResponse: h.warnOversizedResponse,
	})
}

// warnOversizedResponse surfaces a size-backstop firing through the daemon's
// existing warning channel. The clamp only ever said so in English inside the
// response body, where nothing could act on it; a firing means the mode that
// produced the response has no adequate limit of its own.
func (h *Handler) warnOversizedResponse(method string, report mcp.ClampReport) {
	if h.addWarning == nil {
		return
	}
	h.addWarning(fmt.Sprintf(
		"%s returned %d bytes and was truncated to the %d byte limit; that mode needs its own limit or pagination",
		method, report.OriginalBytes, report.LimitBytes))
}
