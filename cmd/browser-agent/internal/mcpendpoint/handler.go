// handler.go — Owns MCP endpoint routing and response-policy composition.

package mcpendpoint

import (
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
	})
}
