// Purpose: Package-main adapters for MCP tool schemas and the embedded OpenAPI schema.
// Why: Both public schema surfaces delegate canonical contracts without owning definitions.
// Docs: docs/features/feature/api-schema/index.md

package main

import (
	_ "embed"
	"net/http"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/schema"
)

//go:embed openapi.json
var openapiJSON []byte

// ToolsList returns all MCP tool definitions.
func (h *ToolHandler) ToolsList() []MCPTool {
	return schema.AllTools()
}

func handleOpenAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(openapiJSON); err != nil {
		stderrf("[kaboom] failed to write /openapi.json response: %v\n", err)
	}
}
