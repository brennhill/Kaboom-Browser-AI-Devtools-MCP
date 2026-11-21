// handler.go — Owns extension-to-daemon screenshot and draw-mode ingest state.
// Docs: docs/features/feature/annotated-screenshots/index.md

package mediaapi

import (
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/annotation"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/app/ports"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/browsercommand"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/diag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/push"
)

const maxPostBodySize = 10 << 20

// Handler owns media ingest dependencies and per-client screenshot throttling.
type Handler struct {
	results      ports.ResultSink
	annotations  *annotation.Store
	pushRouter   *push.Router
	rateMu       sync.Mutex
	rateByClient map[string]time.Time
}

// New constructs the complete media ingest boundary.
// A nil results sink disables query/command completion (degraded composition).
func New(results ports.ResultSink, annotations *annotation.Store, pushRouter *push.Router) *Handler {
	return &Handler{
		results:      results,
		annotations:  annotations,
		pushRouter:   pushRouter,
		rateByClient: make(map[string]time.Time),
	}
}

// CleanupRateLimits removes client throttle entries older than maxAge.
func (h *Handler) CleanupRateLimits(now time.Time, maxAge time.Duration) {
	h.rateMu.Lock()
	defer h.rateMu.Unlock()
	for clientID, lastUpload := range h.rateByClient {
		if now.Sub(lastUpload) > maxAge {
			delete(h.rateByClient, clientID)
		}
	}
}

// submitUploadResult hands an uploaded extension result to the command owner.
// The upload itself already succeeded, so a rejection is reported as a
// structured diagnostic instead of failing the HTTP response.
func (h *Handler) submitUploadResult(result browsercommand.Result) {
	result.Source = browsercommand.SourceQueryResult
	if outcome := h.results.SubmitResult(result); outcome != browsercommand.OutcomeApplied {
		diag.Printf("[Kaboom] event=media_result_rejected outcome=%s query_id=%s correlation_id=%s\n",
			outcome, result.QueryID, result.CorrelationID)
	}
}
