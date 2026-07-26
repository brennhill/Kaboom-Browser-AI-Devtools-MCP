// Purpose: Defines grouped buffer-state structs used to compose Capture ring-buffer internals.
// Why: Improves maintainability by isolating related buffer fields behind focused container types.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

// NetworkWaterfallBuffer groups network waterfall ring buffer fields.
// Protected by parent Capture.mu (no separate lock).
type NetworkWaterfallBuffer struct {
	entries  []NetworkWaterfallEntry // Ring buffer of PerformanceResourceTiming data
	capacity int                     // Configurable capacity (default DefaultNetworkWaterfallCapacity=1000)
}

// ExtensionLogBuffer groups extension log ring buffer fields.
// Protected by parent Capture.mu (no separate lock).
type ExtensionLogBuffer struct {
	logs []ExtensionLog // Ring buffer of extension internal logs (max MaxExtensionLogs=500)
}
