// lifecycle.go — Canonical lifecycle_status vocabulary for async command responses.
// Why: The extension and the queue use several spellings for the same state; agents
// see exactly one, so the mapping belongs with the other response-shaping rules.

package asyncresult

import "strings"

// CanonicalLifecycleStatus maps a raw command status onto the lifecycle_status
// vocabulary exposed to agents. Unknown values pass through unchanged.
func CanonicalLifecycleStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "queued":
		return "queued"
	case "pending", "running", "still_processing":
		return "running"
	case "complete":
		return "complete"
	case "error":
		return "error"
	case "timeout", "expired":
		return "timeout"
	case "cancelled", "canceled":
		return "cancelled"
	default:
		return status
	}
}
