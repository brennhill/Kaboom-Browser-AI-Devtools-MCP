// Purpose: Provides shared capture helper utilities for URL handling and ingest body processing.
// Why: Prevents repeated low-level helper logic across capture handlers and ingestion code paths.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import (
	"io"
	"net/http"

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
