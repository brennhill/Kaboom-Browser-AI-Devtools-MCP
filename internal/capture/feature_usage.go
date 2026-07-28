// feature_usage.go — Synchronizes extension feature-usage notifications.
// Purpose: Owns callback replacement and dispatch as one independent boundary.
// Why: Analytics wiring changes with feature reporting, not with Capture state.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import "sync"

// FeatureUsageObserver owns the optional consumer of extension usage reports.
type FeatureUsageObserver struct {
	mu       sync.RWMutex
	callback func(map[string]bool)
}

func newFeatureUsageObserver() *FeatureUsageObserver {
	return &FeatureUsageObserver{}
}

// SetCallback replaces the usage-report consumer.
func (o *FeatureUsageObserver) SetCallback(callback func(map[string]bool)) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.callback = callback
}

// Notify sends a report to the current consumer without holding the observer lock.
func (o *FeatureUsageObserver) Notify(features map[string]bool) {
	o.mu.RLock()
	callback := o.callback
	o.mu.RUnlock()
	if callback != nil {
		callback(features)
	}
}
