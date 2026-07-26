// Purpose: Exposes focused capture sub-store interfaces backed by lock-safe snapshot accessors.
// Why: Lets callers depend on narrow contracts instead of the full Capture god object.
// Docs: docs/features/feature/observe/index.md

package capture

// EventBufferStore exposes high-volume event buffers through read-only snapshots.
type EventBufferStore interface {
	NetworkBodies() []NetworkBody
	WebSocketEvents() []WebSocketEvent
	EnhancedActions() []EnhancedAction
}

// NetworkWaterfallStore exposes network-waterfall snapshots.
type NetworkWaterfallStore interface {
	Entries() []NetworkWaterfallEntry
	Count() int
}

// ExtensionLogStore exposes extension log snapshots.
type ExtensionLogStore interface {
	Entries() []ExtensionLog
}

// PerformanceSnapshotStore exposes performance snapshots keyed by URL.
type PerformanceSnapshotStore interface {
	Snapshots() []PerformanceSnapshot
	SnapshotByURL(url string) (PerformanceSnapshot, bool)
}

type eventBufferView struct {
	capture *Capture
}

func (v eventBufferView) NetworkBodies() []NetworkBody {
	return v.capture.GetNetworkBodies()
}

func (v eventBufferView) WebSocketEvents() []WebSocketEvent {
	return v.capture.GetAllWebSocketEvents()
}

func (v eventBufferView) EnhancedActions() []EnhancedAction {
	return v.capture.GetAllEnhancedActions()
}

type networkWaterfallView struct {
	capture *Capture
}

func (v networkWaterfallView) Entries() []NetworkWaterfallEntry {
	return v.capture.GetNetworkWaterfallEntries()
}

func (v networkWaterfallView) Count() int {
	return v.capture.GetNetworkWaterfallCount()
}

type extensionLogView struct {
	capture *Capture
}

func (v extensionLogView) Entries() []ExtensionLog {
	return v.capture.GetExtensionLogs()
}

type performanceSnapshotView struct {
	capture *Capture
}

func (v performanceSnapshotView) Snapshots() []PerformanceSnapshot {
	return v.capture.GetPerformanceSnapshots()
}

func (v performanceSnapshotView) SnapshotByURL(url string) (PerformanceSnapshot, bool) {
	return v.capture.GetPerformanceSnapshotByURL(url)
}

// EventBuffers returns a read-only sub-store view for network/websocket/action buffers.
func (c *Capture) EventBuffers() EventBufferStore {
	return eventBufferView{capture: c}
}

// NetworkWaterfallStore returns a read-only sub-store view for waterfall entries.
func (c *Capture) NetworkWaterfallStore() NetworkWaterfallStore {
	return networkWaterfallView{capture: c}
}

// ExtensionLogStore returns a read-only sub-store view for extension logs.
func (c *Capture) ExtensionLogStore() ExtensionLogStore {
	return extensionLogView{capture: c}
}

// PerformanceSnapshotStore returns a read-only sub-store view for performance snapshots.
func (c *Capture) PerformanceSnapshotStore() PerformanceSnapshotStore {
	return performanceSnapshotView{capture: c}
}
