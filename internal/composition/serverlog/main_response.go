// Purpose: Implements shared HTTP JSON response helper(s) for server handlers.
// Why: Centralizes JSON write behavior and error logging for consistent responses.
// Docs: docs/features/feature/backend-log-streaming/index.md

package server

import (
	"net/http"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/platform/util"
)

// jsonResponse delegates to util.JSONResponse for consistent JSON HTTP responses.
func jsonResponse(w http.ResponseWriter, status int, data any) {
	util.JSONResponse(w, status, data)
}
