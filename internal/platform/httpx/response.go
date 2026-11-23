// response.go — Shared JSON HTTP response helpers for daemon endpoints.
package httpx

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

// RequireMethod checks that r.Method matches method. On mismatch, writes 405 JSON response and returns false.
func RequireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method != method {
		JSONResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return false
	}
	return true
}

// JSONResponse writes a JSON response with the given status code and data
func JSONResponse(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		fmt.Fprintf(os.Stderr, "[Kaboom] Error encoding JSON response: %v\n", err)
	}
}
