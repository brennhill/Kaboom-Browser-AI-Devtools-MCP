// handler.go — Owns extension-to-daemon screenshot and draw-mode ingest state.
// Docs: docs/features/feature/annotated-screenshots/index.md

package mediaapi

import (
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/annotation"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/push"
)

const maxPostBodySize = 10 << 20

// Handler owns media ingest dependencies and per-client screenshot throttling.
type Handler struct {
	capture      *capture.Capture
	annotations  *annotation.Store
	pushRouter   *push.Router
	rateMu       sync.Mutex
	rateByClient map[string]time.Time
}

// New constructs the complete media ingest boundary.
func New(captureStore *capture.Capture, annotations *annotation.Store, pushRouter *push.Router) *Handler {
	return &Handler{
		capture:      captureStore,
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
