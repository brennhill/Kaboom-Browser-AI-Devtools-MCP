// openapi.go — Serves the immutable embedded OpenAPI document.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package httpapi

import (
	"net/http"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/diag"
)

// OpenAPI returns the read-only OpenAPI document handler.
func OpenAPI(document []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			JSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(document); err != nil {
			diag.Printf("[kaboom] failed to write /openapi.json response: %v\n", err)
		}
	}
}
