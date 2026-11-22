// stats_endpoint.go — HTTP handler for recording token savings from hook scripts.

package tracking

import (
	"encoding/json"
	"net/http"
)

// tokenSavingsRequest is the JSON body for POST /api/token-savings.
type tokenSavingsRequest struct {
	Category     string `json:"category"`
	TokensBefore int    `json:"tokens_before"`
	TokensAfter  int    `json:"tokens_after"`
}

// HandleRecordTokenSavings returns an HTTP handler that records token savings
// and responds with current session stats.
//
// POST /api/token-savings
//
//	{"category": "test_output", "tokens_before": 2000, "tokens_after": 50}
//
// Returns 200 with current SessionStats.
func HandleRecordTokenSavings(tracker *TokenTracker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusMethodNotAllowed)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
			return
		}

		var req tokenSavingsRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "RecordTokenSavings: invalid JSON body. Send valid JSON with category, tokens_before, tokens_after"})
			return
		}

		if req.Category == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "RecordTokenSavings: category is required. Provide a non-empty category string"})
			return
		}

		if req.TokensBefore <= 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "RecordTokenSavings: tokens_before must be positive. Provide the original token count"})
			return
		}

		tracker.Record(req.Category, req.TokensBefore, req.TokensAfter)

		stats := tracker.GetSessionStats()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(stats)
	}
}
