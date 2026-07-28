// response.go — Shared JSON response encoding for browser-agent HTTP APIs.

package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/diag"
)

func JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		diag.Printf("[Kaboom] Error encoding JSON response: %v\n", err)
	}
}
