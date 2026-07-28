// events.go — Event ingestion, bounded buffers, eviction, and event retrieval.
// Purpose: Owns network, WebSocket, and enhanced-action lifecycles end to end.
// Why: Ingestion and eviction share timestamps, memory counters, test tagging, and
// the same Capture lock, so they must evolve as one consistency boundary.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

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

// enhancedActionEntry bundles an types.EnhancedAction with its ingestion timestamp.
type enhancedActionEntry struct {
	Action  types.EnhancedAction
	AddedAt time.Time
}

// BufferStore owns the in-memory ring buffers used for event/body/action capture.
// Access is synchronized by Capture.mu (this type has no independent lock).
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

// ============================================
// Public Memory Accessors
// ============================================

// GetWebSocketBufferMemory returns approximate memory usage of WS buffer
func (c *Capture) GetWebSocketBufferMemory() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.buffers.calcWSMemory()
}

// GetNetworkBodiesBufferMemory returns approximate memory usage of network bodies buffer
func (c *Capture) GetNetworkBodiesBufferMemory() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.buffers.calcNBMemory()
}

// ClearNetworkBuffers resets network telemetry buffers and related counters.
//
// Invariants:
// - network buffers and their monotonic counters are reset together under c.mu.
func (c *Capture) ClearNetworkBuffers() types.BufferClearCounts {
	c.mu.Lock()
	defer c.mu.Unlock()

	counts := types.BufferClearCounts{
		NetworkWaterfall: c.networkWaterfall.count(),
		NetworkBodies:    len(c.buffers.networkBodies),
	}

	// Clear network waterfall buffer.
	c.networkWaterfall.clear()

	// Clear network bodies buffer and reset memory tracking
	c.buffers.clearNetworkBuffers()

	return counts
}

// ClearWebSocketBuffers resets websocket events and live-connection tracking.
//
// Invariants:
// - wsEvents/wsMemoryTotal/wsTotalAdded are reset atomically.
func (c *Capture) ClearWebSocketBuffers() types.BufferClearCounts {
	c.mu.Lock()
	defer c.mu.Unlock()

	counts := types.BufferClearCounts{
		WebSocketEvents: len(c.buffers.wsEvents),
		WebSocketStatus: c.wsConnections.Count(),
	}

	// Clear WebSocket events buffer
	c.buffers.clearWebSocketBuffers()

	// Clear WebSocket connection tracker.
	c.wsConnections.Clear()

	return counts
}

// ClearActionBuffer resets action telemetry ring and counters.
func (c *Capture) ClearActionBuffer() types.BufferClearCounts {
	c.mu.Lock()
	defer c.mu.Unlock()

	counts := types.BufferClearCounts{
		Actions: len(c.buffers.enhancedActions),
	}

	// Clear actions buffer
	c.buffers.clearActionBuffers()

	return counts
}

// ClearExtensionLogs clears the extension logs buffer.
// This is a public accessor for clearing extension logs from outside the capture package.
//
// Failure semantics:
// - Returns number of entries removed; 0 when already empty.
func (c *Capture) ClearExtensionLogs() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.extensionLogs.clear()
}

// ClearAll resets all capture-owned in-memory telemetry state — INCLUDING
// extension logs — and returns the number of extension-log entries cleared.
//
// Extension logs were previously left behind, so "All" was a lie: every caller
// wanting a genuine full reset had to remember a separate ClearExtensionLogs(),
// and any that forgot silently leaked stale logs. Folding it in makes the name
// honest. ClearExtensionLogs re-locks c.mu, so clear the underlying buffer
// directly here instead of calling it (would deadlock).
//
// Invariants:
// - Runs under one c.mu critical section to avoid partially-cleared mixed state.
func (c *Capture) ClearAll() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.buffers.clearAllEventBuffers()
	c.networkWaterfall.clear()
	c.wsConnections.Clear()
	c.extensionState.activeTestIDs = make(map[string]bool)

	// Reset performance data
	c.perf.clear()

	return c.extensionLogs.clear()
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

func (c *Capture) AddNetworkBodies(bodies []types.NetworkBody) {
	c.mu.Lock()
	defer c.mu.Unlock()

	activeTestIDs := make([]string, 0)
	for testID := range c.extensionState.activeTestIDs {
		activeTestIDs = append(activeTestIDs, testID)
	}
	c.buffers.appendNetworkBodies(bodies, activeTestIDs, time.Now())
}

func (c *Capture) GetNetworkBodyCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.buffers.networkCount()
}

func (c *Capture) AddNetworkWaterfallEntries(entries []types.NetworkWaterfallEntry, pageURL string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.networkWaterfall.appendEntries(entries, pageURL, time.Now())
}

func (c *Capture) GetNetworkWaterfallCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.networkWaterfall.count()
}

func (c *Capture) GetNetworkWaterfallEntries() []types.NetworkWaterfallEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.networkWaterfall.count() == 0 {
		return []types.NetworkWaterfallEntry{}
	}
	return c.networkWaterfall.snapshot()
}

func (b *NetworkWaterfallBuffer) appendEntries(entries []types.NetworkWaterfallEntry, pageURL string, now time.Time) {
	for i := range entries {
		entries[i].PageURL = pageURL
		entries[i].Timestamp = now
		b.entries = append(b.entries, entries[i])
	}
	if len(b.entries) <= b.capacity {
		return
	}
	kept := make([]types.NetworkWaterfallEntry, b.capacity)
	copy(kept, b.entries[len(b.entries)-b.capacity:])
	b.entries = kept
}

func (b *NetworkWaterfallBuffer) count() int {
	return len(b.entries)
}

func (b *NetworkWaterfallBuffer) snapshot() []types.NetworkWaterfallEntry {
	out := make([]types.NetworkWaterfallEntry, len(b.entries))
	copy(out, b.entries)
	return out
}

func (b *NetworkWaterfallBuffer) clear() int {
	count := len(b.entries)
	b.entries = make([]types.NetworkWaterfallEntry, 0, b.capacity)
	return count
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

func (c *Capture) AddWebSocketEvents(events []types.WebSocketEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()

	activeTestIDs := make([]string, 0)
	for testID := range c.extensionState.activeTestIDs {
		activeTestIDs = append(activeTestIDs, testID)
	}
	c.buffers.appendWebSocketEvents(events, activeTestIDs, time.Now(), c.wsConnections.TrackEvent)
}

func (c *Capture) GetWebSocketEventCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.buffers.webSocketCount()
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

func (c *Capture) GetWebSocketEvents(filter types.WebSocketEventFilter) []types.WebSocketEvent {
	c.mu.RLock()
	defer c.mu.RUnlock()

	limit := filter.Limit
	if limit <= 0 {
		limit = defaultWSLimit
	}
	filtered := make([]types.WebSocketEvent, 0, limit)
	for i := len(c.buffers.wsEvents) - 1; i >= 0; i-- {
		entry := &c.buffers.wsEvents[i]
		if c.TTL > 0 && isExpiredByTTL(entry.AddedAt, c.TTL) {
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
	c.AddWebSocketEvents(payload.Events)
	w.WriteHeader(http.StatusOK)
}

func (c *Capture) HandleWebSocketStatus(w http.ResponseWriter, _ *http.Request) {
	status := c.GetWebSocketStatus(types.WebSocketStatusFilter{})
	util.JSONResponse(w, http.StatusOK, status)
}

func (c *Capture) GetWebSocketStatus(filter types.WebSocketStatusFilter) types.WebSocketStatusResponse {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.wsConnections.Status(filter)
}

func (c *Capture) AddEnhancedActions(actions []types.EnhancedAction) {
	navigationCallback := func() func() {
		c.mu.Lock()
		defer c.mu.Unlock()

		activeTestIDs := make([]string, 0)
		for testID := range c.extensionState.activeTestIDs {
			activeTestIDs = append(activeTestIDs, testID)
		}
		for i := range actions {
			actions[i].TestIDs = activeTestIDs
		}
		if c.buffers.appendEnhancedActions(actions, time.Now()) {
			return c.navigationCallback
		}
		return nil
	}()
	if navigationCallback != nil {
		util.SafeGo(navigationCallback)
	}
}

func (c *Capture) GetEnhancedActionCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.buffers.actionCount()
}
