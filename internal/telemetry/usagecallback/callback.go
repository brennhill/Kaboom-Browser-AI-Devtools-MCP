// callback.go — Adapts extension feature usage into canonical telemetry counters.
// Docs: docs/features/feature/app-telemetry/index.md

package usagecallback

import "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/telemetry"

// New returns the callback consumed by extension feature-usage storage.
func New(tracker *telemetry.UsageTracker) func(map[string]bool) {
	return func(features map[string]bool) {
		for key, used := range features {
			if used {
				tracker.RecordToolCall("ext:"+key, 0, false)
			}
		}
	}
}
