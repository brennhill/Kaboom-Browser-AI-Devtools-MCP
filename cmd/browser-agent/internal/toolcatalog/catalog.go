// catalog.go — Owns canonical tool modules, examples, and input schemas.
// Why: Dispatch metadata is one immutable catalog, not parallel root-level caches.

package toolcatalog

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolmodule"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// Catalog combines executable tool modules with their validation schemas.
type Catalog struct {
	modules *toolmodule.Registry
	schemas map[string]map[string]any
}

// New constructs a complete tool catalog.
func New(specs []toolmodule.Spec, tools []mcp.MCPTool) *Catalog {
	catalog := &Catalog{
		modules: toolmodule.New(specs...),
		schemas: make(map[string]map[string]any, len(tools)),
	}
	for _, tool := range tools {
		catalog.schemas[tool.Name] = tool.InputSchema
	}
	return catalog
}

// Dispatch validates and executes a named tool module.
func (c *Catalog) Dispatch(
	req mcp.JSONRPCRequest,
	name string,
	args json.RawMessage,
) (mcp.JSONRPCResponse, bool) {
	return c.modules.Dispatch(req, name, args)
}

// Schema returns the canonical input schema for a tool.
func (c *Catalog) Schema(name string) map[string]any {
	return c.schemas[name]
}

// Examples returns the module-owned examples for a tool.
func (c *Catalog) Examples(name string) []json.RawMessage {
	return c.modules.Examples(name)
}

// Register adds a module to the catalog.
func (c *Catalog) Register(name string, module toolmodule.Module) {
	c.modules.Register(name, module)
}

// Get returns a registered module.
func (c *Catalog) Get(name string) (toolmodule.Module, bool) {
	return c.modules.Get(name)
}
