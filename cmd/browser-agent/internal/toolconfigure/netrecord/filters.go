// filters.go — Filtering and projection helpers for network recording snapshots.
// Why: Keeps request selection/serialization logic out of handler control flow.

package netrecord

import (
	"strconv"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// CollectRecordedRequests filters network bodies against a recording snapshot's filters.
func CollectRecordedRequests(bodies []types.NetworkBody, snap RecordingSnapshot) []map[string]any {
	recorded := make([]map[string]any, 0, len(bodies))
	for _, body := range bodies {
		if !MatchesRecordingFilter(body, snap.StartTime, snap.Domain, snap.Method) {
			continue
		}
		recorded = append(recorded, BuildRecordedRequestEntry(body))
	}
	return recorded
}

// BuildRecordedRequestEntry creates a map representation of a network body for recording output.
func BuildRecordedRequestEntry(body types.NetworkBody) map[string]any {
	entry := map[string]any{
		"method": body.Method,
		"url":    body.URL,
		"status": body.Status,
	}
	if body.RequestBody != "" {
		entry["request_body"] = body.RequestBody
	}
	if body.ResponseBody != "" {
		entry["response_body"] = body.ResponseBody
	}
	if body.ContentType != "" {
		entry["content_type"] = body.ContentType
	}
	if body.Duration > 0 {
		entry["duration_ms"] = body.Duration
	}
	if body.HasAuthHeader {
		entry["has_auth_header"] = true
	}
	if body.Timestamp != "" {
		entry["timestamp"] = body.Timestamp
	}
	return entry
}

// MatchesRecordingFilter checks if a network body matches recording filters.
func MatchesRecordingFilter(body types.NetworkBody, startTime time.Time, domain, method string) bool {
	// Filter by timestamp — only include entries captured after recording started.
	if body.Timestamp != "" {
		ts, err := time.Parse(time.RFC3339Nano, body.Timestamp)
		if err != nil {
			// Try millisecond epoch format (extension may send numeric timestamps).
			if msEpoch, numErr := strconv.ParseInt(body.Timestamp, 10, 64); numErr == nil {
				ts = time.UnixMilli(msEpoch)
				err = nil
			}
		}
		if err == nil && ts.Before(startTime) {
			return false
		}
		// If both RFC3339 and epoch parsing fail, include the entry (best-effort).
	}

	// Filter by domain.
	if domain != "" && !strings.Contains(strings.ToLower(body.URL), strings.ToLower(domain)) {
		return false
	}

	// Filter by HTTP method.
	if method != "" && !strings.EqualFold(body.Method, method) {
		return false
	}

	return true
}
