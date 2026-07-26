// Purpose: Records WebSocket event observations for API schema inference.
// Why: Isolates WebSocket observation ingestion from HTTP and schema building logic.
// Docs: docs/features/feature/api-schema/index.md

package apischema

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

// ============================================
// WebSocket Observation
// ============================================

// ObserveWebSocket records a WebSocket event for schema inference
func (s *SchemaStore) ObserveWebSocket(event capture.WebSocketEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if event.URL == "" {
		return
	}

	ws := s.getOrCreateWSAccumulator(event.URL)
	if ws == nil {
		return
	}

	ws.totalMessages++
	recordWSDirection(ws, event.Direction)
	collectWSMessageTypes(ws, event.Data)
}

// getOrCreateWSAccumulator returns the wsAccumulator for the given URL,
// creating one if needed. Returns nil when at capacity for a new URL.
func (s *SchemaStore) getOrCreateWSAccumulator(wsURL string) *wsAccumulator {
	ws, exists := s.wsSchemas[wsURL]
	if exists {
		return ws
	}
	if len(s.wsSchemas) >= maxWSSchemaConns {
		return nil
	}
	ws = &wsAccumulator{
		url:          wsURL,
		messageTypes: make(map[string]bool),
	}
	s.wsSchemas[wsURL] = ws
	return ws
}

// recordWSDirection increments the directional message counter.
func recordWSDirection(ws *wsAccumulator, direction string) {
	switch direction {
	case "incoming":
		ws.incomingCount++
	case "outgoing":
		ws.outgoingCount++
	}
}

// wsTypeKeys lists the JSON fields inspected for message-type classification.
var wsTypeKeys = []string{"type", "action"}

// collectWSMessageTypes parses JSON data and records any "type" or "action"
// string values as message types.
func collectWSMessageTypes(ws *wsAccumulator, data string) {
	if data == "" {
		return
	}
	var msg map[string]any
	if json.Unmarshal([]byte(data), &msg) != nil {
		return
	}
	for _, key := range wsTypeKeys {
		if val, ok := msg[key]; ok {
			if str, ok := val.(string); ok && len(ws.messageTypes) < maxWSMessageTypes {
				ws.messageTypes[str] = true
			}
		}
	}
}
