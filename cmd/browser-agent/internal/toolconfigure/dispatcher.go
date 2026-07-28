// dispatcher.go — Owns configure mode registration and canonical dispatch.
// Why: Configure routing is feature behavior, not composition-root behavior.

package toolconfigure

import (
	"encoding/json"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolrouting"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// Handler executes one configure mode.
type Handler func(mcp.JSONRPCRequest, json.RawMessage) mcp.JSONRPCResponse

// Dispatcher owns the complete configure mode registry.
type Dispatcher struct {
	registry toolrouting.Registry[*Dispatcher]
	actions  []string
}

// NewDispatcher constructs an immutable configure registry.
func NewDispatcher(handlers map[string]Handler) *Dispatcher {
	dispatcher := &Dispatcher{}
	routes := make(map[string]toolrouting.Handler[*Dispatcher], len(handlers))
	for name, handler := range handlers {
		handler := handler
		routes[name] = func(_ *Dispatcher, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
			return handler(req, args)
		}
	}
	dispatcher.actions = util.SortedMapKeys(routes)
	dispatcher.registry = toolrouting.Registry[*Dispatcher]{
		Handlers: routes,
		Resolution: toolrouting.Resolution{
			ToolName:   "configure",
			ValidModes: strings.Join(dispatcher.actions, ", "),
		},
	}
	return dispatcher
}

// Actions returns the sorted canonical configure mode names.
func (d *Dispatcher) Actions() []string {
	return append([]string(nil), d.actions...)
}

// Handle routes a configure request to its canonical mode owner.
func (d *Dispatcher) Handle(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	return toolrouting.Dispatch(d, req, args, d.registry)
}
