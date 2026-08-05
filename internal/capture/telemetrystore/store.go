// store.go — Composes and enriches independently retained browser telemetry.
// Docs: docs/features/feature/backend-log-streaming/index.md

package telemetrystore

import (
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/actionstore"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/bodystore"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/pressure"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/waterfallstore"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/wsconn"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

const maxWSEvents = 500
const wsBufferMemoryLimit = 4 * 1024 * 1024

// Dependencies are the runtime seams used during cross-stream enrichment.
type Dependencies struct {
	ActiveTestIDs func() []string
	Now           func() time.Time
	Dispatch      func(func())
	Actions       *actionstore.Store
	NetworkBodies *bodystore.Store
	WebSockets    *wsconn.Store
	Waterfall     *waterfallstore.Store
}

// Pressure is bounded-retention state for disposable browser telemetry.
type Pressure struct {
	Network          pressure.Stats `json:"network"`
	NetworkWaterfall pressure.Stats `json:"network_waterfall"`
	WebSocket        pressure.Stats `json:"websocket"`
	Actions          pressure.Stats `json:"actions"`
}

// Store composes canonical telemetry-family owners and ingestion enrichment.
type Store struct {
	mu                 sync.RWMutex
	actions            *actionstore.Store
	networkBodies      *bodystore.Store
	webSockets         *wsconn.Store
	networkWaterfall   *waterfallstore.Store
	deps               Dependencies
	navigationCallback func()
}

// New constructs the canonical telemetry composition owner.
func New(deps Dependencies) *Store {
	if deps.ActiveTestIDs == nil {
		deps.ActiveTestIDs = func() []string { return nil }
	}
	if deps.Now == nil {
		deps.Now = time.Now
	}
	if deps.Dispatch == nil {
		deps.Dispatch = util.SafeGo
	}
	if deps.Actions == nil {
		deps.Actions = actionstore.NewDefault()
	}
	if deps.NetworkBodies == nil {
		deps.NetworkBodies = bodystore.NewDefault()
	}
	if deps.WebSockets == nil {
		deps.WebSockets = wsconn.NewStore(maxWSEvents, wsBufferMemoryLimit)
	}
	if deps.Waterfall == nil {
		deps.Waterfall = waterfallstore.NewDefault()
	}
	return &Store{actions: deps.Actions, networkBodies: deps.NetworkBodies, webSockets: deps.WebSockets, networkWaterfall: deps.Waterfall, deps: deps}
}

func (s *Store) NetworkWaterfall() *waterfallstore.Store { return s.networkWaterfall }
func (s *Store) NetworkBodies() *bodystore.Store         { return s.networkBodies }
func (s *Store) Actions() *actionstore.Store             { return s.actions }
func (s *Store) WebSockets() *wsconn.Store               { return s.webSockets }

func (s *Store) SetNavigationCallback(callback func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.navigationCallback = callback
}

func (s *Store) Pressure() Pressure {
	return Pressure{Network: s.networkBodies.Stats().Pressure, NetworkWaterfall: s.networkWaterfall.Pressure(), WebSocket: s.webSockets.Stats().Pressure, Actions: s.actions.Stats().Pressure}
}

func (s *Store) ClearNetworkBuffers() types.BufferClearCounts {
	return types.BufferClearCounts{NetworkWaterfall: s.networkWaterfall.Clear(), NetworkBodies: s.networkBodies.Clear()}
}

func (s *Store) ClearAll() {
	s.webSockets.Clear()
	s.networkBodies.Clear()
	s.actions.Clear()
	s.networkWaterfall.Clear()
}

func setBinaryFormat(payload string, name *string, confidence *float64) bool {
	if *name != "" || payload == "" {
		return false
	}
	format := util.DetectBinaryFormat([]byte(payload))
	if format == nil {
		return false
	}
	*name, *confidence = format.Name, format.Confidence
	return true
}

func (s *Store) AddNetworkBodies(bodies []types.NetworkBody) {
	ids, prepared := s.deps.ActiveTestIDs(), make([]types.NetworkBody, len(bodies))
	for i := range bodies {
		prepared[i] = bodies[i]
		prepared[i].TestIDs = append([]string(nil), ids...)
		if !setBinaryFormat(prepared[i].RequestBody, &prepared[i].BinaryFormat, &prepared[i].FormatConfidence) {
			setBinaryFormat(prepared[i].ResponseBody, &prepared[i].BinaryFormat, &prepared[i].FormatConfidence)
		}
	}
	s.networkBodies.Add(prepared, s.deps.Now())
}

func (s *Store) AddWebSocketEvents(events []types.WebSocketEvent) {
	ids, prepared := s.deps.ActiveTestIDs(), make([]types.WebSocketEvent, len(events))
	for i := range events {
		prepared[i] = events[i]
		prepared[i].TestIDs = append([]string(nil), ids...)
		if prepared[i].Event == "message" {
			setBinaryFormat(prepared[i].Data, &prepared[i].BinaryFormat, &prepared[i].FormatConfidence)
		}
	}
	s.webSockets.Add(prepared, s.deps.Now())
}

func (s *Store) AddEnhancedActions(actions []types.EnhancedAction) {
	ids, prepared := s.deps.ActiveTestIDs(), make([]types.EnhancedAction, len(actions))
	copy(prepared, actions)
	for i := range prepared {
		prepared[i].TestIDs = append([]string(nil), ids...)
	}
	if !s.actions.Add(prepared, s.deps.Now()) {
		return
	}
	s.mu.RLock()
	callback := s.navigationCallback
	dispatch := s.deps.Dispatch
	s.mu.RUnlock()
	if callback != nil {
		dispatch(callback)
	}
}

// Snapshot is an immutable point-in-time counter view.
type Snapshot struct {
	NetworkTotalAdded, WebSocketTotalAdded, ActionTotalAdded                                                       int64
	NetworkCount, WebSocketCount, ActionCount, NetworkCapacity, WebSocketCapacity, ActionCapacity, ConnectionCount int
}

func (s *Store) Snapshot() Snapshot {
	n, a, w := s.networkBodies.Stats(), s.actions.Stats(), s.webSockets.Stats()
	return Snapshot{n.TotalAdded, w.TotalAdded, a.TotalAdded, n.Count, w.Count, a.Count, n.Pressure.Capacity, w.Capacity, a.Capacity, w.ConnectionCount}
}
