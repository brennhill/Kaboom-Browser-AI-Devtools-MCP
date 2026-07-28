// transients.go — Pure transient-element enrichment for async command results.
// Docs: docs/features/feature/transient-capture/index.md

package asyncresult

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"time"
)

// MaxTransientsPerResult bounds transient enrichment response size.
const MaxTransientsPerResult = 10

// AttachTransientElements adds recent transient actions to responseData.
func AttachTransientElements(responseData map[string]any, actions []types.EnhancedAction, since time.Time) {
	if responseData == nil {
		return
	}
	sinceMillis := since.UnixMilli() - 500
	transients := make([]map[string]any, 0, 4)
	for i := len(actions) - 1; i >= 0 && len(transients) < MaxTransientsPerResult; i-- {
		action := actions[i]
		if action.Timestamp < sinceMillis {
			break
		}
		if action.Type != "transient" {
			continue
		}
		transients = append(transients, map[string]any{
			"classification": action.Classification,
			"value":          action.Value,
			"role":           action.Role,
			"url":            action.URL,
			"timestamp":      action.Timestamp,
		})
	}
	if len(transients) > 0 {
		responseData["transient_elements"] = transients
	}
}
