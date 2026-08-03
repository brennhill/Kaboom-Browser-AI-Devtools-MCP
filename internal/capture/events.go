// events.go — Event ingestion, bounded buffers, eviction, and event retrieval.
// Purpose: Owns network, WebSocket, and enhanced-action lifecycles end to end.
// Why: Ingestion and eviction share timestamps, memory counters, test tagging, and
// one TelemetryStore lock, so they evolve as a single consistency boundary.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/wsconn"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

type boundedRing[T any] struct {
	storage []T
	head    int
	size    int
}

func newBoundedRing[T any](capacity int) boundedRing[T] {
	if capacity < 0 {
		capacity = 0
	}
	return boundedRing[T]{storage: make([]T, capacity)}
}

func (r *boundedRing[T]) capacity() int { return len(r.storage) }
func (r *boundedRing[T]) len() int      { return r.size }

func (r *boundedRing[T]) push(value T) (evicted T, overwritten bool) {
	if len(r.storage) == 0 {
		return value, true
	}
	if r.size < len(r.storage) {
		r.storage[(r.head+r.size)%len(r.storage)] = value
		r.size++
		return evicted, false
	}
	evicted = r.storage[r.head]
	r.storage[r.head] = value
	r.head = (r.head + 1) % len(r.storage)
	return evicted, true
}

func (r *boundedRing[T]) at(index int) *T {
	return &r.storage[(r.head+index)%len(r.storage)]
}

func (r *boundedRing[T]) dropOldest(count int) {
	if count > r.size {
		count = r.size
	}
	var zero T
	for i := 0; i < count; i++ {
		r.storage[(r.head+i)%len(r.storage)] = zero
	}
	if len(r.storage) > 0 {
		r.head = (r.head + count) % len(r.storage)
	}
	r.size -= count
	if r.size == 0 {
		r.head = 0
	}
}

func (r *boundedRing[T]) clear() {
	r.dropOldest(r.size)
}

func (r *boundedRing[T]) snapshot() []T {
	out := make([]T, r.size)
	for i := range out {
		out[i] = *r.at(i)
	}
	return out
}

// wsEventEntry bundles a types.WebSocketEvent with its ingestion timestamp.
type wsEventEntry struct {
	Event   types.WebSocketEvent
	AddedAt time.Time
}

// networkBodyEntry bundles a types.NetworkBody with its ingestion timestamp.
type networkBodyEntry struct {
	Body    types.NetworkBody
	AddedAt time.Time
}

// NetworkWaterfallStore owns bounded browser resource timings and synchronization.
type NetworkWaterfallStore struct {
	mu       sync.RWMutex
	entries  boundedRing[types.NetworkWaterfallEntry]
	capacity int
	dropped  int64
}

func newNetworkWaterfallStore(capacity int) *NetworkWaterfallStore {
	return &NetworkWaterfallStore{
		entries:  newBoundedRing[types.NetworkWaterfallEntry](capacity),
		capacity: capacity,
	}
}

// NetworkWaterfall returns the independently synchronized waterfall owner.
func (s *TelemetryStore) NetworkWaterfall() *NetworkWaterfallStore {
	return s.networkWaterfall
}

// Add tags and appends resource timings at server receive time.
func (s *NetworkWaterfallStore) Add(entries []types.NetworkWaterfallEntry, pageURL string) {
	s.addAt(entries, pageURL, time.Now())
}

func (s *NetworkWaterfallStore) addAt(entries []types.NetworkWaterfallEntry, pageURL string, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range entries {
		entries[i].PageURL = pageURL
		entries[i].Timestamp = now
		_, overwritten := s.entries.push(entries[i])
		if overwritten {
			s.dropped++
		}
	}
}

// Pressure returns bounded waterfall retention metrics.
func (s *NetworkWaterfallStore) Pressure() PressureStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return pressureForRing(s.entries, s.dropped, time.Now(), func(entry types.NetworkWaterfallEntry) time.Time { return entry.Timestamp })
}

// Entries returns a detached snapshot of resource timings.
func (s *NetworkWaterfallStore) Entries() []types.NetworkWaterfallEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.entries.snapshot()
}

// Clear removes all resource timings and returns the removed count.
func (s *NetworkWaterfallStore) Clear() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := s.entries.len()
	s.entries.clear()
	return count
}

// enhancedActionEntry bundles an types.EnhancedAction with its ingestion timestamp.
type enhancedActionEntry struct {
	Action  types.EnhancedAction
	AddedAt time.Time
}

// BufferStore holds the in-memory rings used for event/body/action capture.
// Its owning TelemetryStore supplies synchronization.
type BufferStore struct {
	// WebSocket event buffer state.
	wsEvents      boundedRing[wsEventEntry]
	wsTotalAdded  int64
	wsMemoryTotal int64
	wsDropped     int64

	// Network body buffer state.
	networkBodies          boundedRing[networkBodyEntry]
	networkTotalAdded      int64
	networkErrorTotalAdded int64
	networkBodyMemoryTotal int64
	networkDropped         int64

	// Enhanced action buffer state.
	enhancedActions  boundedRing[enhancedActionEntry]
	actionTotalAdded int64
	actionDropped    int64
}

// TelemetryPressure is the bounded retention state for disposable browser telemetry.
type TelemetryPressure struct {
	Network          PressureStats `json:"network"`
	NetworkWaterfall PressureStats `json:"network_waterfall"`
	WebSocket        PressureStats `json:"websocket"`
	Actions          PressureStats `json:"actions"`
}

// TelemetryStore owns event buffers, WebSocket connection state, navigation
// callbacks, and the synchronization that keeps those values coherent.
type TelemetryStore struct {
	mu                 sync.RWMutex
	buffers            BufferStore
	wsConnections      wsconn.Tracker
	networkWaterfall   *NetworkWaterfallStore
	extension          *ExtensionRuntime
	navigationCallback func()
	ttl                time.Duration
}

func newTelemetryStore(extension *ExtensionRuntime) *TelemetryStore {
	return &TelemetryStore{
		buffers:          newBufferStore(),
		wsConnections:    wsconn.NewTracker(),
		networkWaterfall: newNetworkWaterfallStore(DefaultNetworkWaterfallCapacity),
		extension:        extension,
	}
}

// SetNavigationCallback sets the callback fired after navigation ingestion.
func (s *TelemetryStore) SetNavigationCallback(callback func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.navigationCallback = callback
}

func newBufferStore() BufferStore {
	return BufferStore{
		wsEvents:        newBoundedRing[wsEventEntry](MaxWSEvents),
		networkBodies:   newBoundedRing[networkBodyEntry](MaxNetworkBodies),
		enhancedActions: newBoundedRing[enhancedActionEntry](MaxEnhancedActions),
	}
}

func (s *BufferStore) networkTimestamps() []time.Time {
	if s.networkBodies.len() == 0 {
		return []time.Time{}
	}
	out := make([]time.Time, s.networkBodies.len())
	for i := range out {
		out[i] = s.networkBodies.at(i).AddedAt
	}
	return out
}

func (s *BufferStore) webSocketTimestamps() []time.Time {
	if s.wsEvents.len() == 0 {
		return []time.Time{}
	}
	out := make([]time.Time, s.wsEvents.len())
	for i := range out {
		out[i] = s.wsEvents.at(i).AddedAt
	}
	return out
}

func (s *BufferStore) actionTimestamps() []time.Time {
	if s.enhancedActions.len() == 0 {
		return []time.Time{}
	}
	out := make([]time.Time, s.enhancedActions.len())
	for i := range out {
		out[i] = s.enhancedActions.at(i).AddedAt
	}
	return out
}

func (s *BufferStore) networkBodiesCopy() []types.NetworkBody {
	if s.networkBodies.len() == 0 {
		return []types.NetworkBody{}
	}
	out := make([]types.NetworkBody, s.networkBodies.len())
	for i := range out {
		out[i] = s.networkBodies.at(i).Body
	}
	return out
}

func (s *BufferStore) webSocketEventsCopy() []types.WebSocketEvent {
	if s.wsEvents.len() == 0 {
		return []types.WebSocketEvent{}
	}
	out := make([]types.WebSocketEvent, s.wsEvents.len())
	for i := range out {
		out[i] = s.wsEvents.at(i).Event
	}
	return out
}

func (s *BufferStore) enhancedActionsCopy() []types.EnhancedAction {
	if s.enhancedActions.len() == 0 {
		return []types.EnhancedAction{}
	}
	out := make([]types.EnhancedAction, s.enhancedActions.len())
	for i := range out {
		out[i] = s.enhancedActions.at(i).Action
	}
	return out
}

func (s *BufferStore) clearNetworkBuffers() {
	s.networkBodies.clear()
	s.networkTotalAdded = 0
	s.networkErrorTotalAdded = 0
	s.networkBodyMemoryTotal = 0
}

func (s *BufferStore) clearWebSocketBuffers() {
	s.wsEvents.clear()
	s.wsTotalAdded = 0
	s.wsMemoryTotal = 0
}

func (s *BufferStore) clearActionBuffers() {
	s.enhancedActions.clear()
	s.actionTotalAdded = 0
}

func (s *BufferStore) clearAllEventBuffers() {
	s.clearNetworkBuffers()
	s.clearWebSocketBuffers()
	s.clearActionBuffers()
}

func (s *BufferStore) appendEnhancedActions(actions []types.EnhancedAction, now time.Time) bool {
	s.actionTotalAdded += int64(len(actions))
	hasNavigation := false
	for i := range actions {
		_, overwritten := s.enhancedActions.push(enhancedActionEntry{
			Action:  actions[i],
			AddedAt: now,
		})
		if overwritten {
			s.actionDropped++
		}
		if actions[i].Type == "navigation" {
			hasNavigation = true
		}
	}
	return hasNavigation
}

func (s *BufferStore) appendNetworkBodies(bodies []types.NetworkBody, now time.Time) {
	s.networkTotalAdded += int64(len(bodies))
	for i := range bodies {
		if bodies[i].Status >= 400 {
			s.networkErrorTotalAdded++
		}
		evicted, overwritten := s.networkBodies.push(networkBodyEntry{
			Body:    bodies[i],
			AddedAt: now,
		})
		if overwritten {
			s.networkDropped++
			s.networkBodyMemoryTotal -= nbEntryMemory(&evicted.Body)
		}
		s.networkBodyMemoryTotal += nbEntryMemory(&bodies[i])
	}
	s.evictNetworkForMemory()
}

func (s *BufferStore) appendWebSocketEvents(events []types.WebSocketEvent, now time.Time, onEvent func(types.WebSocketEvent)) {
	s.wsTotalAdded += int64(len(events))
	for i := range events {
		if onEvent != nil {
			onEvent(events[i])
		}
		evicted, overwritten := s.wsEvents.push(wsEventEntry{
			Event:   events[i],
			AddedAt: now,
		})
		if overwritten {
			s.wsDropped++
			s.wsMemoryTotal -= wsEventMemory(&evicted.Event)
		}
		s.wsMemoryTotal += wsEventMemory(&events[i])
	}
	s.evictWebSocketForMemory()
}

func (s *BufferStore) evictNetworkForMemory() {
	excess := s.networkBodyMemoryTotal - nbBufferMemoryLimit
	if excess <= 0 {
		return
	}
	drop := 0
	for drop < s.networkBodies.len() && excess > 0 {
		entryMem := nbEntryMemory(&s.networkBodies.at(drop).Body)
		excess -= entryMem
		s.networkBodyMemoryTotal -= entryMem
		drop++
	}
	s.networkBodies.dropOldest(drop)
	s.networkDropped += int64(drop)
}

func (s *BufferStore) evictWebSocketForMemory() {
	excess := s.wsMemoryTotal - wsBufferMemoryLimit
	if excess <= 0 {
		return
	}
	drop := 0
	for drop < s.wsEvents.len() && excess > 0 {
		entryMem := wsEventMemory(&s.wsEvents.at(drop).Event)
		excess -= entryMem
		s.wsMemoryTotal -= entryMem
		drop++
	}
	s.wsEvents.dropOldest(drop)
	s.wsDropped += int64(drop)
}

// Pressure returns count, capacity, cumulative drops, and oldest age for each stream.
func (s *TelemetryStore) Pressure() TelemetryPressure {
	s.mu.RLock()
	now := time.Now()
	pressure := TelemetryPressure{
		Network:   pressureForRing(s.buffers.networkBodies, s.buffers.networkDropped, now, func(entry networkBodyEntry) time.Time { return entry.AddedAt }),
		WebSocket: pressureForRing(s.buffers.wsEvents, s.buffers.wsDropped, now, func(entry wsEventEntry) time.Time { return entry.AddedAt }),
		Actions:   pressureForRing(s.buffers.enhancedActions, s.buffers.actionDropped, now, func(entry enhancedActionEntry) time.Time { return entry.AddedAt }),
	}
	s.mu.RUnlock()
	pressure.NetworkWaterfall = s.networkWaterfall.Pressure()
	return pressure
}

func pressureForRing[T any](ring boundedRing[T], dropped int64, now time.Time, addedAt func(T) time.Time) PressureStats {
	stats := PressureStats{Size: ring.len(), Capacity: ring.capacity(), Dropped: dropped}
	if ring.len() == 0 {
		return stats
	}
	stats.OldestAge = now.Sub(addedAt(*ring.at(0)))
	if stats.OldestAge < 0 {
		stats.OldestAge = 0
	}
	return stats
}

func (s *BufferStore) networkTotal() int64 {
	return s.networkTotalAdded
}

func (s *BufferStore) networkErrorTotal() int64 {
	return s.networkErrorTotalAdded
}

func (s *BufferStore) webSocketTotal() int64 {
	return s.wsTotalAdded
}

func (s *BufferStore) actionTotal() int64 {
	return s.actionTotalAdded
}

func (s *BufferStore) networkCount() int {
	return s.networkBodies.len()
}

func (s *BufferStore) webSocketCount() int {
	return s.wsEvents.len()
}

func (s *BufferStore) actionCount() int {
	return s.enhancedActions.len()
}

const (
	// Per-entry memory estimates
	wsEventOverhead     = 200 // bytes overhead per WS event
	networkBodyOverhead = 300 // bytes overhead per network body
	actionMemoryFixed   = 500 // bytes per enhanced action (fixed estimate)
)

// ============================================
// Per-Entry Memory Calculation
// ============================================

// wsEventMemory returns the memory estimate for a single WS event.
func wsEventMemory(e *types.WebSocketEvent) int64 {
	return int64(len(e.Data)) + wsEventOverhead
}

// nbEntryMemory returns the memory estimate for a single network body entry.
func nbEntryMemory(b *types.NetworkBody) int64 {
	return int64(len(b.RequestBody)+len(b.ResponseBody)) + networkBodyOverhead
}

// ============================================
// Per-Buffer Memory Accessors (caller must hold lock)
// ============================================

// calcWSMemory returns the running total of WS buffer memory (caller must hold lock).
// O(1) — maintained incrementally by add/evict/clear operations.
func (s *BufferStore) calcWSMemory() int64 {
	return s.wsMemoryTotal
}

// calcNBMemory returns the running total of network bodies buffer memory (caller must hold lock).
// O(1) — maintained incrementally by add/evict/clear operations.
func (s *BufferStore) calcNBMemory() int64 {
	return s.networkBodyMemoryTotal
}

// ClearNetworkBuffers resets network telemetry buffers and related counters.
//
// Invariants:
// - network buffers and their monotonic counters are reset together under the telemetry lock.
func (s *TelemetryStore) ClearNetworkBuffers() types.BufferClearCounts {
	waterfallCount := s.networkWaterfall.Clear()
	s.mu.Lock()
	defer s.mu.Unlock()

	counts := types.BufferClearCounts{
		NetworkWaterfall: waterfallCount,
		NetworkBodies:    s.buffers.networkBodies.len(),
	}

	// Clear network bodies buffer and reset memory tracking
	s.buffers.clearNetworkBuffers()

	return counts
}

// ClearWebSocketBuffers resets websocket events and live-connection tracking.
//
// Invariants:
// - wsEvents/wsMemoryTotal/wsTotalAdded are reset atomically.
func (s *TelemetryStore) ClearWebSocketBuffers() types.BufferClearCounts {
	s.mu.Lock()
	defer s.mu.Unlock()

	counts := types.BufferClearCounts{
		WebSocketEvents: s.buffers.wsEvents.len(),
		WebSocketStatus: s.wsConnections.Count(),
	}

	// Clear WebSocket events buffer
	s.buffers.clearWebSocketBuffers()

	// Clear WebSocket connection tracker.
	s.wsConnections.Clear()

	return counts
}

// ClearActionBuffer resets action telemetry ring and counters.
func (s *TelemetryStore) ClearActionBuffer() types.BufferClearCounts {
	s.mu.Lock()
	defer s.mu.Unlock()

	counts := types.BufferClearCounts{
		Actions: s.buffers.enhancedActions.len(),
	}

	// Clear actions buffer
	s.buffers.clearActionBuffers()

	return counts
}

// StateResetter owns the coordinated reset of capture runtime stores.
type StateResetter struct {
	extension     *ExtensionRuntime
	telemetry     *TelemetryStore
	performance   *PerformanceStore
	extensionLogs *ExtensionLogStore
}

// NewStateResetter binds reset behavior to the canonical state owners.
func NewStateResetter(capture *Capture) *StateResetter {
	return &StateResetter{
		extension:     capture.extension,
		telemetry:     capture.telemetry,
		performance:   capture.perf,
		extensionLogs: capture.extensionLogs,
	}
}

// ClearAll resets all capture-owned in-memory telemetry state — INCLUDING
// extension logs — and returns the number of extension-log entries cleared.
func (r *StateResetter) ClearAll() int {
	r.extension.ClearTestBoundaries()
	r.telemetry.clearAll()
	r.performance.clear()
	return r.extensionLogs.Clear()
}

func (s *TelemetryStore) clearAll() {
	s.mu.Lock()
	s.buffers.clearAllEventBuffers()
	s.wsConnections.Clear()
	s.mu.Unlock()
	s.networkWaterfall.Clear()
}

func detectAndSetBinaryFormat(body *types.NetworkBody) {
	if body.BinaryFormat != "" {
		return
	}
	if len(body.RequestBody) > 0 {
		if format := util.DetectBinaryFormat([]byte(body.RequestBody)); format != nil {
			body.BinaryFormat = format.Name
			body.FormatConfidence = format.Confidence
			return
		}
	}
	if len(body.ResponseBody) > 0 {
		if format := util.DetectBinaryFormat([]byte(body.ResponseBody)); format != nil {
			body.BinaryFormat = format.Name
			body.FormatConfidence = format.Confidence
		}
	}
}

func (s *TelemetryStore) AddNetworkBodies(bodies []types.NetworkBody) {
	activeTestIDs := s.extension.GetActiveTestIDs()
	for i := range bodies {
		bodies[i].TestIDs = activeTestIDs
		detectAndSetBinaryFormat(&bodies[i])
	}
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.buffers.appendNetworkBodies(bodies, now)
}

func detectWSBinaryFormat(event *types.WebSocketEvent) {
	if event.Event != "message" || event.BinaryFormat != "" || len(event.Data) == 0 {
		return
	}
	if format := util.DetectBinaryFormat([]byte(event.Data)); format != nil {
		event.BinaryFormat = format.Name
		event.FormatConfidence = format.Confidence
	}
}

func (s *TelemetryStore) AddWebSocketEvents(events []types.WebSocketEvent) {
	activeTestIDs := s.extension.GetActiveTestIDs()
	for i := range events {
		events[i].TestIDs = activeTestIDs
		detectWSBinaryFormat(&events[i])
	}
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.buffers.appendWebSocketEvents(events, now, s.wsConnections.TrackEvent)
}

func matchesWSEventFilter(event *types.WebSocketEvent, filter types.WebSocketEventFilter) bool {
	if filter.ConnectionID != "" && event.ID != filter.ConnectionID {
		return false
	}
	if filter.URLFilter != "" && !strings.Contains(event.URL, filter.URLFilter) {
		return false
	}
	if filter.Direction != "" && event.Direction != filter.Direction {
		return false
	}
	if filter.TestID != "" && !containsTestID(event.TestIDs, filter.TestID) {
		return false
	}
	return true
}

func containsTestID(testIDs []string, target string) bool {
	for _, testID := range testIDs {
		if testID == target {
			return true
		}
	}
	return false
}

func (s *TelemetryStore) GetWebSocketEvents(filter types.WebSocketEventFilter) []types.WebSocketEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	limit := filter.Limit
	if limit <= 0 {
		limit = defaultWSLimit
	}
	filtered := make([]types.WebSocketEvent, 0, limit)
	for i := s.buffers.wsEvents.len() - 1; i >= 0; i-- {
		entry := s.buffers.wsEvents.at(i)
		if s.ttl > 0 && isExpiredByTTL(entry.AddedAt, s.ttl) {
			break
		}
		if !matchesWSEventFilter(&entry.Event, filter) {
			continue
		}
		filtered = append(filtered, entry.Event)
		if len(filtered) >= limit {
			break
		}
	}
	return filtered
}

func (h *HTTPHandlers) HandleWebSocketEvents(w http.ResponseWriter, r *http.Request) {
	if !util.RequireMethod(w, r, "POST") {
		return
	}
	body, ok := h.readIngestBody(w, r)
	if !ok {
		return
	}
	var payload struct {
		Events []types.WebSocketEvent `json:"events"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		util.JSONResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return
	}
	if !h.recordAndRecheck(w, len(payload.Events)) {
		return
	}
	h.capture.telemetry.AddWebSocketEvents(payload.Events)
	w.WriteHeader(http.StatusOK)
}

func (h *HTTPHandlers) HandleWebSocketStatus(w http.ResponseWriter, _ *http.Request) {
	status := h.capture.telemetry.GetWebSocketStatus(types.WebSocketStatusFilter{})
	util.JSONResponse(w, http.StatusOK, status)
}

func (s *TelemetryStore) GetWebSocketStatus(filter types.WebSocketStatusFilter) types.WebSocketStatusResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.wsConnections.Status(filter)
}

func (s *TelemetryStore) AddEnhancedActions(actions []types.EnhancedAction) {
	activeTestIDs := s.extension.GetActiveTestIDs()
	navigationCallback := func() func() {
		s.mu.Lock()
		defer s.mu.Unlock()

		for i := range actions {
			actions[i].TestIDs = activeTestIDs
		}
		if s.buffers.appendEnhancedActions(actions, time.Now()) {
			return s.navigationCallback
		}
		return nil
	}()
	if navigationCallback != nil {
		util.SafeGo(navigationCallback)
	}
}
