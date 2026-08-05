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

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/actionstore"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/bodystore"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/logstore"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/perfstore"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/pressure"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/ringstore"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/waterfallstore"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/wsconn"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

const (
	maxWSEvents         = 500
	defaultWSLimit      = 50
	defaultBodyLimit    = 20
	wsBufferMemoryLimit = 4 * 1024 * 1024
)

// wsEventEntry bundles a types.WebSocketEvent with its ingestion timestamp.
type wsEventEntry struct {
	Event   types.WebSocketEvent
	AddedAt time.Time
}

// NetworkWaterfall returns the independently synchronized waterfall owner.
func (s *TelemetryStore) NetworkWaterfall() *waterfallstore.Store {
	return s.networkWaterfall
}

// NetworkBodies returns the independently synchronized HTTP body owner.
func (s *TelemetryStore) NetworkBodies() *bodystore.Store { return s.networkBodies }

// Actions returns the independently synchronized enhanced-action owner.
func (s *TelemetryStore) Actions() *actionstore.Store { return s.actions }

// BufferStore holds the WebSocket event ring and its change-coupled counters.
// Its owning TelemetryStore supplies synchronization.
type BufferStore struct {
	// WebSocket event buffer state.
	wsEvents      ringstore.Store[wsEventEntry]
	wsTotalAdded  int64
	wsMemoryTotal int64
	wsDropped     int64
}

// TelemetryPressure is the bounded retention state for disposable browser telemetry.
type TelemetryPressure struct {
	Network          pressure.Stats `json:"network"`
	NetworkWaterfall pressure.Stats `json:"network_waterfall"`
	WebSocket        pressure.Stats `json:"websocket"`
	Actions          pressure.Stats `json:"actions"`
}

// TelemetryStore owns event buffers, WebSocket connection state, navigation
// callbacks, and the synchronization that keeps those values coherent.
type TelemetryStore struct {
	mu                 sync.RWMutex
	buffers            BufferStore
	actions            *actionstore.Store
	networkBodies      *bodystore.Store
	wsConnections      wsconn.Tracker
	networkWaterfall   *waterfallstore.Store
	extension          *ExtensionRuntime
	navigationCallback func()
	dispatchCallback   func(func())
	ttl                time.Duration
}

func newTelemetryStore(extension *ExtensionRuntime) *TelemetryStore {
	return &TelemetryStore{
		buffers:          newBufferStore(),
		actions:          actionstore.NewDefault(),
		networkBodies:    bodystore.NewDefault(),
		wsConnections:    wsconn.NewTracker(),
		networkWaterfall: waterfallstore.NewDefault(),
		extension:        extension,
		dispatchCallback: util.SafeGo,
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
		wsEvents: ringstore.New[wsEventEntry](maxWSEvents),
	}
}

func (s *BufferStore) webSocketTimestamps() []time.Time {
	if s.wsEvents.Len() == 0 {
		return []time.Time{}
	}
	out := make([]time.Time, s.wsEvents.Len())
	for i := range out {
		out[i] = s.wsEvents.At(i).AddedAt
	}
	return out
}

func (s *BufferStore) webSocketEventsCopy() []types.WebSocketEvent {
	if s.wsEvents.Len() == 0 {
		return []types.WebSocketEvent{}
	}
	out := make([]types.WebSocketEvent, s.wsEvents.Len())
	for i := range out {
		out[i] = cloneWebSocketEvent(s.wsEvents.At(i).Event)
	}
	return out
}

func (s *BufferStore) clearWebSocketBuffers() {
	s.wsEvents.Clear()
	s.wsTotalAdded = 0
	s.wsMemoryTotal = 0
}

func (s *BufferStore) clearAllEventBuffers() {
	s.clearWebSocketBuffers()
}

func (s *BufferStore) appendWebSocketEvents(events []types.WebSocketEvent, now time.Time, onEvent func(types.WebSocketEvent)) {
	s.wsTotalAdded += int64(len(events))
	for i := range events {
		event := cloneWebSocketEvent(events[i])
		if onEvent != nil {
			onEvent(event)
		}
		evicted, overwritten := s.wsEvents.Push(wsEventEntry{
			Event:   event,
			AddedAt: now,
		})
		if overwritten {
			s.wsDropped++
			s.wsMemoryTotal -= wsEventMemory(&evicted.Event)
		}
		s.wsMemoryTotal += wsEventMemory(&event)
	}
	s.evictWebSocketForMemory()
}

func (s *BufferStore) evictWebSocketForMemory() {
	excess := s.wsMemoryTotal - wsBufferMemoryLimit
	if excess <= 0 {
		return
	}
	drop := 0
	for drop < s.wsEvents.Len() && excess > 0 {
		entryMem := wsEventMemory(&s.wsEvents.At(drop).Event)
		excess -= entryMem
		s.wsMemoryTotal -= entryMem
		drop++
	}
	s.wsEvents.DropOldest(drop)
	s.wsDropped += int64(drop)
}

// Pressure returns count, capacity, cumulative drops, and oldest age for each stream.
func (s *TelemetryStore) Pressure() TelemetryPressure {
	network := s.networkBodies.Stats().Pressure
	actions := s.actions.Stats().Pressure
	s.mu.RLock()
	now := time.Now()
	pressure := TelemetryPressure{
		Network:   network,
		WebSocket: pressureForRing(s.buffers.wsEvents, s.buffers.wsDropped, now, func(entry wsEventEntry) time.Time { return entry.AddedAt }),
		Actions:   actions,
	}
	s.mu.RUnlock()
	pressure.NetworkWaterfall = s.networkWaterfall.Pressure()
	return pressure
}

func pressureForRing[T any](ring ringstore.Store[T], dropped int64, now time.Time, addedAt func(T) time.Time) pressure.Stats {
	stats := pressure.Stats{Size: ring.Len(), Capacity: ring.Capacity(), Dropped: dropped}
	if ring.Len() == 0 {
		return stats
	}
	stats.OldestAge = now.Sub(addedAt(*ring.At(0)))
	if stats.OldestAge < 0 {
		stats.OldestAge = 0
	}
	return stats
}

func (s *BufferStore) webSocketTotal() int64 {
	return s.wsTotalAdded
}

func (s *BufferStore) webSocketCount() int {
	return s.wsEvents.Len()
}

const (
	// Per-entry memory estimates
	wsEventOverhead = 200 // bytes overhead per WS event
)

// ============================================
// Per-Entry Memory Calculation
// ============================================

// wsEventMemory returns the memory estimate for a single WS event.
func wsEventMemory(e *types.WebSocketEvent) int64 {
	return int64(len(e.Data)) + wsEventOverhead
}

// ============================================
// Per-Buffer Memory Accessors (caller must hold lock)
// ============================================

// calcWSMemory returns the running total of WS buffer memory (caller must hold lock).
// O(1) — maintained incrementally by add/evict/clear operations.
func (s *BufferStore) calcWSMemory() int64 {
	return s.wsMemoryTotal
}

// ClearNetworkBuffers resets network telemetry buffers and related counters.
//
// Invariants:
// - network buffers and their monotonic counters are reset together under the telemetry lock.
func (s *TelemetryStore) ClearNetworkBuffers() types.BufferClearCounts {
	waterfallCount := s.networkWaterfall.Clear()
	bodyCount := s.networkBodies.Clear()
	counts := types.BufferClearCounts{
		NetworkWaterfall: waterfallCount,
		NetworkBodies:    bodyCount,
	}
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
		WebSocketEvents: s.buffers.wsEvents.Len(),
		WebSocketStatus: s.wsConnections.Count(),
	}

	// Clear WebSocket events buffer
	s.buffers.clearWebSocketBuffers()

	// Clear WebSocket connection tracker.
	s.wsConnections.Clear()

	return counts
}

// StateResetter owns the coordinated reset of capture runtime stores.
type StateResetter struct {
	extension     *ExtensionRuntime
	telemetry     *TelemetryStore
	performance   *perfstore.Store
	extensionLogs *logstore.Extension
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
	r.performance.Clear()
	return r.extensionLogs.Clear()
}

func (s *TelemetryStore) clearAll() {
	s.mu.Lock()
	s.buffers.clearAllEventBuffers()
	s.wsConnections.Clear()
	s.mu.Unlock()
	s.networkBodies.Clear()
	s.actions.Clear()
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
	prepared := make([]types.NetworkBody, len(bodies))
	for i := range bodies {
		prepared[i] = bodies[i]
		prepared[i].TestIDs = append([]string(nil), activeTestIDs...)
		detectAndSetBinaryFormat(&prepared[i])
	}
	s.networkBodies.Add(prepared, time.Now())
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
	prepared := make([]types.WebSocketEvent, len(events))
	for i := range events {
		prepared[i] = events[i]
		prepared[i].TestIDs = append([]string(nil), activeTestIDs...)
		detectWSBinaryFormat(&prepared[i])
	}
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.buffers.appendWebSocketEvents(prepared, now, s.wsConnections.TrackEvent)
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
	for i := s.buffers.wsEvents.Len() - 1; i >= 0; i-- {
		entry := s.buffers.wsEvents.At(i)
		if s.ttl > 0 && isExpiredByTTL(entry.AddedAt, s.ttl) {
			break
		}
		if !matchesWSEventFilter(&entry.Event, filter) {
			continue
		}
		filtered = append(filtered, cloneWebSocketEvent(entry.Event))
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
	prepared := make([]types.EnhancedAction, len(actions))
	copy(prepared, actions)
	for i := range prepared {
		prepared[i].TestIDs = append([]string(nil), activeTestIDs...)
	}
	if !s.actions.Add(prepared, time.Now()) {
		return
	}
	s.mu.RLock()
	navigationCallback := s.navigationCallback
	dispatch := s.dispatchCallback
	s.mu.RUnlock()
	if navigationCallback != nil {
		dispatch(navigationCallback)
	}
}
