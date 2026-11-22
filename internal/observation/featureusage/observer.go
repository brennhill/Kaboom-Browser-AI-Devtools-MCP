// observer.go — Owns extension feature-usage callback replacement and dispatch.

package featureusage

import "sync"

// Observer owns the optional consumer of extension usage reports.
type Observer struct {
	mu       sync.RWMutex
	callback func(map[string]bool)
}

// New creates an independently synchronized feature-usage observer.
func New() *Observer {
	return &Observer{}
}

// SetCallback replaces the usage-report consumer.
func (o *Observer) SetCallback(callback func(map[string]bool)) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.callback = callback
}

// Notify sends a report to the current consumer without holding the observer lock.
func (o *Observer) Notify(features map[string]bool) {
	o.mu.RLock()
	callback := o.callback
	o.mu.RUnlock()
	if callback != nil {
		callback(features)
	}
}
