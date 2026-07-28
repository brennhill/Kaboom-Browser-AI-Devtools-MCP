// Purpose: Attaches response metadata (data_age_ms, entry count, pagination cursors) to observe results.
// Why: Every mode ends by stamping freshness onto its reply, so the plain, paginated and
// paginated-with-summary spellings of that step belong together.
// Docs: docs/features/feature/observe/index.md

package observe

import (
	"fmt"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/pagination"
)

// NOTE: data_age_ms is documented in observe tech/api docs and metadata tests;
// page_ready_for_commands and tab_status are mode-specific fields on observe(page) responses.

// ResponseMetadata provides freshness information for buffer-backed observe responses.
type ResponseMetadata struct {
	RetrievedAt string `json:"retrieved_at"`
	IsStale     bool   `json:"is_stale"`
	DataAge     string `json:"data_age"`
	// DataAgeMs is milliseconds since the newest entry in the response data.
	// Returns -1 when no data exists (sentinel for "no data").
	// Clamped to zero if the clock reports a negative age (e.g., NTP adjustment).
	DataAgeMs       int64 `json:"data_age_ms"`
	NoiseSuppressed int   `json:"noise_suppressed,omitempty"`
}

// BuildResponseMetadata constructs freshness metadata for an observe response.
func BuildResponseMetadata(cap *capture.Capture, newestEntry time.Time) ResponseMetadata {
	now := time.Now()
	meta := ResponseMetadata{
		RetrievedAt: now.Format(time.RFC3339),
		IsStale:     !cap.Extension().IsExtensionConnected(),
	}
	if !newestEntry.IsZero() {
		age := now.Sub(newestEntry)
		if age < 0 {
			age = 0
		}
		meta.DataAge = fmt.Sprintf("%.1fs", age.Seconds())
		meta.DataAgeMs = age.Milliseconds()
	} else {
		meta.DataAge = "no_data"
		meta.DataAgeMs = -1
	}
	return meta
}

// BuildPaginatedResponseMetadata merges freshness metadata with cursor pagination metadata.
func BuildPaginatedResponseMetadata(cap *capture.Capture, newestEntry time.Time, pMeta *pagination.CursorPaginationMetadata) map[string]any {
	base := BuildResponseMetadata(cap, newestEntry)
	meta := map[string]any{
		"retrieved_at": base.RetrievedAt,
		"is_stale":     base.IsStale,
		"data_age":     base.DataAge,
		"data_age_ms":  base.DataAgeMs,
		"total":        pMeta.Total,
		"has_more":     pMeta.HasMore,
	}
	if pMeta.Cursor != "" {
		meta["cursor"] = pMeta.Cursor
	}
	if pMeta.OldestTimestamp != "" {
		meta["oldest_timestamp"] = pMeta.OldestTimestamp
	}
	if pMeta.NewestTimestamp != "" {
		meta["newest_timestamp"] = pMeta.NewestTimestamp
	}
	if pMeta.CursorRestarted {
		meta["cursor_restarted"] = true
		meta["original_cursor"] = pMeta.OriginalCursor
	}
	if pMeta.Warning != "" {
		meta["warning"] = pMeta.Warning
	}
	return meta
}

// BuildPaginatedMetadataWithSummary adds a summary block to first-page paginated responses.
func BuildPaginatedMetadataWithSummary(
	cap *capture.Capture, newestEntry time.Time,
	pMeta *pagination.CursorPaginationMetadata,
	isFirstPage bool,
	summaryFn func() map[string]any,
) map[string]any {
	var meta map[string]any
	if cap != nil {
		meta = BuildPaginatedResponseMetadata(cap, newestEntry, pMeta)
	} else {
		// Nil capture — build minimal metadata for testing
		meta = map[string]any{
			"total":    pMeta.Total,
			"has_more": pMeta.HasMore,
		}
		if pMeta.Cursor != "" {
			meta["cursor"] = pMeta.Cursor
		}
	}
	if isFirstPage && summaryFn != nil {
		meta["summary"] = summaryFn()
	}
	return meta
}
