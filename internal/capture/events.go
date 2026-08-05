// events.go — Enriches incoming telemetry and composes independent owners.
// Purpose: Owns cross-store ingestion concerns and coordinated runtime resets.
// Why: Each telemetry family owns retention while this layer applies active test
// context and binary metadata before handing values to that canonical owner.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/actionstore"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/bodystore"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/logstore"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/perfstore"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/pressure"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/waterfallstore"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/wsconn"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

const (
	maxWSEvents         = 500
	defaultBodyLimit    = 20
	wsBufferMemoryLimit = 4 * 1024 * 1024
)

// NetworkWaterfall returns the independently synchronized waterfall owner.
func (s *TelemetryStore) NetworkWaterfall() *waterfallstore.Store {
	return s.networkWaterfall
}

// NetworkBodies returns the independently synchronized HTTP body owner.
func (s *TelemetryStore) NetworkBodies() *bodystore.Store { return s.networkBodies }

// Actions returns the independently synchronized enhanced-action owner.
func (s *TelemetryStore) Actions() *actionstore.Store { return s.actions }

// WebSockets returns the coherent event and connection-state owner.
func (s *TelemetryStore) WebSockets() *wsconn.Store { return s.webSockets }

// TelemetryPressure is the bounded retention state for disposable browser telemetry.
type TelemetryPressure struct {
	Network          pressure.Stats `json:"network"`
	NetworkWaterfall pressure.Stats `json:"network_waterfall"`
	WebSocket        pressure.Stats `json:"websocket"`
	Actions          pressure.Stats `json:"actions"`
}

// TelemetryStore composes independent telemetry owners and navigation callbacks.
type TelemetryStore struct {
	mu                 sync.RWMutex
	actions            *actionstore.Store
	networkBodies      *bodystore.Store
	webSockets         *wsconn.Store
	networkWaterfall   *waterfallstore.Store
	extension          *ExtensionRuntime
	navigationCallback func()
	dispatchCallback   func(func())
}

func newTelemetryStore(extension *ExtensionRuntime) *TelemetryStore {
	return &TelemetryStore{
		actions:          actionstore.NewDefault(),
		networkBodies:    bodystore.NewDefault(),
		webSockets:       wsconn.NewStore(maxWSEvents, wsBufferMemoryLimit),
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

// Pressure returns count, capacity, cumulative drops, and oldest age for each stream.
func (s *TelemetryStore) Pressure() TelemetryPressure {
	network := s.networkBodies.Stats().Pressure
	actions := s.actions.Stats().Pressure
	webSockets := s.webSockets.Stats().Pressure
	pressure := TelemetryPressure{
		Network:   network,
		WebSocket: webSockets,
		Actions:   actions,
	}
	pressure.NetworkWaterfall = s.networkWaterfall.Pressure()
	return pressure
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
	s.webSockets.Clear()
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
	s.webSockets.Add(prepared, time.Now())
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
	status := h.capture.telemetry.WebSockets().Status(types.WebSocketStatusFilter{})
	util.JSONResponse(w, http.StatusOK, status)
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
