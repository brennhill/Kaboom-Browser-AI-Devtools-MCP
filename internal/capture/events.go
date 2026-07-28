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
	entries  []types.NetworkWaterfallEntry
	capacity int
}

func newNetworkWaterfallStore(capacity int) *NetworkWaterfallStore {
	return &NetworkWaterfallStore{
		entries:  make([]types.NetworkWaterfallEntry, 0, capacity),
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
		s.entries = append(s.entries, entries[i])
	}
	if len(s.entries) <= s.capacity {
		return
	}
	kept := make([]types.NetworkWaterfallEntry, s.capacity)
	copy(kept, s.entries[len(s.entries)-s.capacity:])
	s.entries = kept
}

// Entries returns a detached snapshot of resource timings.
func (s *NetworkWaterfallStore) Entries() []types.NetworkWaterfallEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]types.NetworkWaterfallEntry, len(s.entries))
	copy(out, s.entries)
	return out
}

// Clear removes all resource timings and returns the removed count.
func (s *NetworkWaterfallStore) Clear() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := len(s.entries)
	s.entries = make([]types.NetworkWaterfallEntry, 0, s.capacity)
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
	wsEvents      []wsEventEntry
	wsTotalAdded  int64
	wsMemoryTotal int64

	// Network body buffer state.
	networkBodies          []networkBodyEntry
	networkTotalAdded      int64
	networkErrorTotalAdded int64
	networkBodyMemoryTotal int64

	// Enhanced action buffer state.
	enhancedActions  []enhancedActionEntry
	actionTotalAdded int64
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
		wsEvents:        make([]wsEventEntry, 0, MaxWSEvents),
		networkBodies:   make([]networkBodyEntry, 0, MaxNetworkBodies),
		enhancedActions: make([]enhancedActionEntry, 0, MaxEnhancedActions),
	}
}

func (s *BufferStore) networkTimestamps() []time.Time {
	if len(s.networkBodies) == 0 {
		return []time.Time{}
	}
	out := make([]time.Time, len(s.networkBodies))
	for i := range s.networkBodies {
		out[i] = s.networkBodies[i].AddedAt
	}
	return out
}

func (s *BufferStore) webSocketTimestamps() []time.Time {
	if len(s.wsEvents) == 0 {
		return []time.Time{}
	}
	out := make([]time.Time, len(s.wsEvents))
	for i := range s.wsEvents {
		out[i] = s.wsEvents[i].AddedAt
	}
	return out
}

func (s *BufferStore) actionTimestamps() []time.Time {
	if len(s.enhancedActions) == 0 {
		return []time.Time{}
	}
	out := make([]time.Time, len(s.enhancedActions))
	for i := range s.enhancedActions {
		out[i] = s.enhancedActions[i].AddedAt
	}
	return out
}

func (s *BufferStore) networkBodiesCopy() []types.NetworkBody {
	if len(s.networkBodies) == 0 {
		return []types.NetworkBody{}
	}
	out := make([]types.NetworkBody, len(s.networkBodies))
	for i := range s.networkBodies {
		out[i] = s.networkBodies[i].Body
	}
	return out
}

func (s *BufferStore) webSocketEventsCopy() []types.WebSocketEvent {
	if len(s.wsEvents) == 0 {
		return []types.WebSocketEvent{}
	}
	out := make([]types.WebSocketEvent, len(s.wsEvents))
	for i := range s.wsEvents {
		out[i] = s.wsEvents[i].Event
	}
	return out
}

func (s *BufferStore) enhancedActionsCopy() []types.EnhancedAction {
	if len(s.enhancedActions) == 0 {
		return []types.EnhancedAction{}
	}
	out := make([]types.EnhancedAction, len(s.enhancedActions))
	for i := range s.enhancedActions {
		out[i] = s.enhancedActions[i].Action
	}
	return out
}

func (s *BufferStore) clearNetworkBuffers() {
	s.networkBodies = make([]networkBodyEntry, 0)
	s.networkTotalAdded = 0
	s.networkErrorTotalAdded = 0
	s.networkBodyMemoryTotal = 0
}

func (s *BufferStore) clearWebSocketBuffers() {
	s.wsEvents = make([]wsEventEntry, 0)
	s.wsTotalAdded = 0
	s.wsMemoryTotal = 0
}

func (s *BufferStore) clearActionBuffers() {
	s.enhancedActions = make([]enhancedActionEntry, 0)
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
		s.enhancedActions = append(s.enhancedActions, enhancedActionEntry{
			Action:  actions[i],
			AddedAt: now,
		})
		if actions[i].Type == "navigation" {
			hasNavigation = true
		}
	}
	if len(s.enhancedActions) > MaxEnhancedActions {
		keep := len(s.enhancedActions) - MaxEnhancedActions
		newEntries := make([]enhancedActionEntry, MaxEnhancedActions)
		copy(newEntries, s.enhancedActions[keep:])
		s.enhancedActions = newEntries
	}
	return hasNavigation
}

func (s *BufferStore) appendNetworkBodies(bodies []types.NetworkBody, testIDs []string, now time.Time) {
	s.networkTotalAdded += int64(len(bodies))
	for i := range bodies {
		if bodies[i].Status >= 400 {
			s.networkErrorTotalAdded++
		}
		bodies[i].TestIDs = testIDs
		detectAndSetBinaryFormat(&bodies[i])
		s.networkBodies = append(s.networkBodies, networkBodyEntry{
			Body:    bodies[i],
			AddedAt: now,
		})
		s.networkBodyMemoryTotal += nbEntryMemory(&bodies[i])
	}
	s.evictNetworkByCount()
	s.evictNetworkForMemory()
}

func (s *BufferStore) appendWebSocketEvents(events []types.WebSocketEvent, testIDs []string, now time.Time, onEvent func(types.WebSocketEvent)) {
	s.wsTotalAdded += int64(len(events))
	for i := range events {
		events[i].TestIDs = testIDs
		detectWSBinaryFormat(&events[i])
		if onEvent != nil {
			onEvent(events[i])
		}
		s.wsEvents = append(s.wsEvents, wsEventEntry{
			Event:   events[i],
			AddedAt: now,
		})
		s.wsMemoryTotal += wsEventMemory(&events[i])
	}
	s.evictWebSocketByCount()
	s.evictWebSocketForMemory()
}

func (s *BufferStore) evictNetworkByCount() {
	if len(s.networkBodies) <= MaxNetworkBodies {
		return
	}
	keep := len(s.networkBodies) - MaxNetworkBodies
	for j := 0; j < keep; j++ {
		s.networkBodyMemoryTotal -= nbEntryMemory(&s.networkBodies[j].Body)
	}
	newEntries := make([]networkBodyEntry, MaxNetworkBodies)
	copy(newEntries, s.networkBodies[keep:])
	s.networkBodies = newEntries
}

func (s *BufferStore) evictNetworkForMemory() {
	excess := s.networkBodyMemoryTotal - nbBufferMemoryLimit
	if excess <= 0 {
		return
	}
	drop := 0
	for drop < len(s.networkBodies) && excess > 0 {
		entryMem := nbEntryMemory(&s.networkBodies[drop].Body)
		excess -= entryMem
		s.networkBodyMemoryTotal -= entryMem
		drop++
	}
	surviving := make([]networkBodyEntry, len(s.networkBodies)-drop)
	copy(surviving, s.networkBodies[drop:])
	s.networkBodies = surviving
}

func (s *BufferStore) evictWebSocketByCount() {
	if len(s.wsEvents) <= MaxWSEvents {
		return
	}
	drop := len(s.wsEvents) - MaxWSEvents
	for j := 0; j < drop; j++ {
		s.wsMemoryTotal -= wsEventMemory(&s.wsEvents[j].Event)
	}
	newEntries := make([]wsEventEntry, MaxWSEvents)
	copy(newEntries, s.wsEvents[drop:])
	s.wsEvents = newEntries
}

func (s *BufferStore) evictWebSocketForMemory() {
	excess := s.wsMemoryTotal - wsBufferMemoryLimit
	if excess <= 0 {
		return
	}
	drop := 0
	for drop < len(s.wsEvents) && excess > 0 {
		entryMem := wsEventMemory(&s.wsEvents[drop].Event)
		excess -= entryMem
		s.wsMemoryTotal -= entryMem
		drop++
	}
	surviving := make([]wsEventEntry, len(s.wsEvents)-drop)
	copy(surviving, s.wsEvents[drop:])
	s.wsEvents = surviving
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
	return len(s.networkBodies)
}

func (s *BufferStore) webSocketCount() int {
	return len(s.wsEvents)
}

func (s *BufferStore) actionCount() int {
	return len(s.enhancedActions)
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
// - network buffers and their monotonic counters are reset together under c.mu.
func (s *TelemetryStore) ClearNetworkBuffers() types.BufferClearCounts {
	waterfallCount := s.networkWaterfall.Clear()
	s.mu.Lock()
	defer s.mu.Unlock()

	counts := types.BufferClearCounts{
		NetworkWaterfall: waterfallCount,
		NetworkBodies:    len(s.buffers.networkBodies),
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
		WebSocketEvents: len(s.buffers.wsEvents),
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
		Actions: len(s.buffers.enhancedActions),
	}

	// Clear actions buffer
	s.buffers.clearActionBuffers()

	return counts
}

// ClearAll resets all capture-owned in-memory telemetry state — INCLUDING
// extension logs — and returns the number of extension-log entries cleared.
func (c *Capture) ClearAll() int {
	c.extension.ClearTestBoundaries()
	c.telemetry.clearAll()
	c.perf.clear()
	return c.extensionLogs.Clear()
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
	s.mu.Lock()
	defer s.mu.Unlock()
	s.buffers.appendNetworkBodies(bodies, activeTestIDs, time.Now())
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
	s.mu.Lock()
	defer s.mu.Unlock()
	s.buffers.appendWebSocketEvents(events, activeTestIDs, time.Now(), s.wsConnections.TrackEvent)
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
	for i := len(s.buffers.wsEvents) - 1; i >= 0; i-- {
		entry := &s.buffers.wsEvents[i]
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

func (c *Capture) HandleWebSocketEvents(w http.ResponseWriter, r *http.Request) {
	if !util.RequireMethod(w, r, "POST") {
		return
	}
	body, ok := c.readIngestBody(w, r)
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
	if !c.recordAndRecheck(w, len(payload.Events)) {
		return
	}
	c.telemetry.AddWebSocketEvents(payload.Events)
	w.WriteHeader(http.StatusOK)
}

func (c *Capture) HandleWebSocketStatus(w http.ResponseWriter, _ *http.Request) {
	status := c.telemetry.GetWebSocketStatus(types.WebSocketStatusFilter{})
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
