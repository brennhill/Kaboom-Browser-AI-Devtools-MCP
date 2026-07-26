// Purpose: Provides shared capture request plumbing — URL helpers, ingest body reading, rate limiting and TTL filtering.
// Why: Prevents repeated low-level helper logic across capture handlers and ingestion code paths.
// Docs: docs/features/feature/backend-log-streaming/index.md
// Docs: docs/features/feature/rate-limiting/index.md
// Docs: docs/features/feature/ttl-retention/index.md

package capture

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// ExtractURLPath delegates to util.ExtractURLPath for cross-package callers.
func ExtractURLPath(rawURL string) string {
	return util.ExtractURLPath(rawURL)
}

// readIngestBody handles rate-limit check and body reading for ingest endpoints.
// Returns the body bytes and true on success; on failure it writes the error response
// and returns nil, false.
func (c *Capture) readIngestBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	if c.CheckRateLimit() {
		c.WriteRateLimitResponse(w)
		return nil, false
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxExtensionPostBody)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
		return nil, false
	}
	return body, true
}

// recordAndRecheck records a batch of events for rate limiting and rechecks.
// Returns true if the request may proceed; on rate limit it writes the 429 response.
func (c *Capture) recordAndRecheck(w http.ResponseWriter, count int) bool {
	c.RecordEvents(count)
	if c.CheckRateLimit() {
		c.WriteRateLimitResponse(w)
		return false
	}
	return true
}

// RecordEvents delegates to CircuitBreaker.
func (c *Capture) RecordEvents(count int) {
	c.circuit.RecordEvents(count)
}

// CheckRateLimit delegates to CircuitBreaker.
func (c *Capture) CheckRateLimit() bool {
	return c.circuit.CheckRateLimit()
}

// GetHealthStatus delegates to CircuitBreaker.
func (c *Capture) GetHealthStatus() HealthResponse {
	return c.circuit.GetHealthStatus()
}

// WriteRateLimitResponse delegates to CircuitBreaker.
func (c *Capture) WriteRateLimitResponse(w http.ResponseWriter) {
	c.circuit.WriteRateLimitResponse(w)
}

// HandleHealth returns circuit breaker state as a JSON response (used by /health).
func (c *Capture) HandleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		util.JSONResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	health := c.GetHealthStatus()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	//nolint:errcheck // HTTP response encoding errors are logged by client
	_ = json.NewEncoder(w).Encode(health)
}

// isExpiredByTTL checks if an entry is expired based on TTL.
// Returns true if the entry should be filtered out.
func isExpiredByTTL(addedAt time.Time, ttl time.Duration) bool {
	if ttl == 0 {
		return false
	}
	return time.Since(addedAt) >= ttl
}
