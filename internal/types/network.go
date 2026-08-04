// Purpose: Defines canonical websocket/network telemetry structures shared across capture and transport layers.
// Why: Prevents telemetry shape drift between extension producers and server-side consumers.
// Docs: docs/features/feature/normalized-event-schema/index.md

package types

import "time"

// ============================================
// WebSocket Types
// ============================================

// WebSocketEvent represents a captured WebSocket event (message, open, close, etc.)
// Wire fields: see WireWebSocketEvent in wire_websocket_event.go
type WebSocketEvent struct {
	Timestamp        string        `json:"ts,omitempty"`
	Type             string        `json:"type,omitempty"`
	Event            string        `json:"event"`
	ID               string        `json:"id"`
	URL              string        `json:"url,omitempty"`
	Direction        string        `json:"direction,omitempty"`
	Data             string        `json:"data,omitempty"`
	Size             int           `json:"size,omitempty"`
	CloseCode        int           `json:"code,omitempty"`
	CloseReason      string        `json:"reason,omitempty"`
	Sampled          *SamplingInfo `json:"sampled,omitempty"`           // server-only enrichment
	BinaryFormat     string        `json:"binary_format,omitempty"`     // server-only enrichment
	FormatConfidence float64       `json:"format_confidence,omitempty"` // server-only enrichment
	TabID            int           `json:"tab_id,omitempty"`            // Chrome tab ID that produced this event
	TestIDs          []string      `json:"test_ids,omitempty"`          // Test IDs this event belongs to
}

// SamplingInfo describes the sampling state when a message was captured
type SamplingInfo struct {
	Rate   string `json:"rate"`
	Logged string `json:"logged"`
	Window string `json:"window"`
}

// WebSocketEventFilter defines filtering criteria for WebSocket events
type WebSocketEventFilter struct {
	ConnectionID string
	URLFilter    string
	Direction    string
	Limit        int
	TestID       string // If set, filter events where TestID is in event's TestIDs array
}

// WebSocketStatusFilter defines filtering criteria for WebSocket status
type WebSocketStatusFilter struct {
	URLFilter    string
	ConnectionID string
}

// WebSocketStatusResponse is the response from get_websocket_status
type WebSocketStatusResponse struct {
	Connections []WebSocketConnection       `json:"connections"`
	Closed      []WebSocketClosedConnection `json:"closed"`
}

// WebSocketConnection represents an active WebSocket connection
type WebSocketConnection struct {
	ID          string                  `json:"id"`
	URL         string                  `json:"url"`
	State       string                  `json:"state"`
	OpenedAt    string                  `json:"opened_at,omitempty"`
	Duration    string                  `json:"duration,omitempty"`
	MessageRate WebSocketMessageRate    `json:"message_rate"`
	LastMessage WebSocketLastMessage    `json:"last_message"`
	Schema      *WebSocketSchema        `json:"schema,omitempty"`
	Sampling    WebSocketSamplingStatus `json:"sampling"`
}

// WebSocketClosedConnection represents a closed WebSocket connection
type WebSocketClosedConnection struct {
	ID            string `json:"id"`
	URL           string `json:"url"`
	State         string `json:"state"`
	OpenedAt      string `json:"opened_at,omitempty"`
	ClosedAt      string `json:"closed_at,omitempty"`
	CloseCode     int    `json:"close_code"`
	CloseReason   string `json:"close_reason"`
	TotalMessages struct {
		Incoming int `json:"incoming"`
		Outgoing int `json:"outgoing"`
	} `json:"total_messages"`
}

// WebSocketMessageRate contains rate info for a direction
type WebSocketMessageRate struct {
	Incoming WebSocketDirectionStats `json:"incoming"`
	Outgoing WebSocketDirectionStats `json:"outgoing"`
}

// WebSocketDirectionStats contains stats for a message direction
type WebSocketDirectionStats struct {
	PerSecond float64 `json:"per_second"`
	Total     int     `json:"total"`
	Bytes     int     `json:"bytes"`
}

// WebSocketLastMessage contains last message info
type WebSocketLastMessage struct {
	Incoming *WebSocketMessagePreview `json:"incoming,omitempty"`
	Outgoing *WebSocketMessagePreview `json:"outgoing,omitempty"`
}

// WebSocketMessagePreview contains a preview of the last message
type WebSocketMessagePreview struct {
	At      string `json:"at"`
	Age     string `json:"age"`
	Preview string `json:"preview"`
}

// WebSocketSchema describes detected message schema
type WebSocketSchema struct {
	DetectedKeys []string `json:"detected_keys,omitempty"`
	MessageCount int      `json:"message_count"`
	Consistent   bool     `json:"consistent"`
	Variants     []string `json:"variants,omitempty"`
}

// WebSocketSamplingStatus describes sampling state
type WebSocketSamplingStatus struct {
	Active bool   `json:"active"`
	Rate   string `json:"rate,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// ============================================
// HTTP Network Types
// ============================================

// NetworkBody represents a captured network request/response.
// Wire fields: see WireNetworkBody in wire_network.go
type NetworkBody struct {
	Timestamp         string            `json:"ts,omitempty"`
	Method            string            `json:"method"`
	URL               string            `json:"url"`
	Status            int               `json:"status"`
	RequestBody       string            `json:"request_body,omitempty"`
	ResponseBody      string            `json:"response_body,omitempty"`
	ContentType       string            `json:"content_type,omitempty"`
	Duration          int               `json:"duration,omitempty"`
	RequestTruncated  bool              `json:"request_truncated,omitempty"`
	ResponseTruncated bool              `json:"response_truncated,omitempty"`
	ResponseHeaders   map[string]string `json:"response_headers,omitempty"`  // server-only enrichment
	HasAuthHeader     bool              `json:"has_auth_header,omitempty"`   // server-only enrichment
	BinaryFormat      string            `json:"binary_format,omitempty"`     // server-only enrichment
	FormatConfidence  float64           `json:"format_confidence,omitempty"` // server-only enrichment
	TabID             int               `json:"tab_id,omitempty"`            // Chrome tab ID that produced this request
	TestIDs           []string          `json:"test_ids,omitempty"`          // Test IDs this entry belongs to
}

// NetworkBodyFilter defines filtering criteria for network bodies
type NetworkBodyFilter struct {
	URLFilter string
	Method    string
	StatusMin int
	StatusMax int
	Limit     int
	TestID    string // If set, filter entries where TestID is in entry's TestIDs array
}

// NetworkWaterfallEntry represents a single network resource timing entry
// from the browser's PerformanceResourceTiming API.
// Wire fields: see WireNetworkWaterfallEntry in wire_network.go
type NetworkWaterfallEntry struct {
	Name             string             `json:"name"`
	URL              string             `json:"url"`
	InitiatorType    string             `json:"initiator_type"`
	Duration         float64            `json:"duration"`
	StartTime        float64            `json:"start_time"`
	FetchStart       float64            `json:"fetch_start"`
	ResponseEnd      float64            `json:"response_end"`
	TransferSize     int                `json:"transfer_size"`
	DecodedBodySize  int                `json:"decoded_body_size"`
	EncodedBodySize  int                `json:"encoded_body_size"`
	PageURL          string             `json:"page_url,omitempty"`
	Timestamp        time.Time          `json:"timestamp,omitempty"` // Server-side timestamp
	QueueingMs       float64            `json:"queueing_ms,omitempty"`
	DNSMs            float64            `json:"dns_ms,omitempty"`
	TLSMs            float64            `json:"tls_ms,omitempty"`
	ConnectMs        float64            `json:"connect_ms,omitempty"`
	TTFBMs           float64            `json:"ttfb_ms,omitempty"`
	DownloadMs       float64            `json:"download_ms,omitempty"`
	Priority         string             `json:"priority,omitempty"`
	Protocol         string             `json:"protocol,omitempty"`
	CacheSource      string             `json:"cache_source,omitempty"`
	CompressionRatio float64            `json:"compression_ratio,omitempty"`
	ContentEncoding  string             `json:"content_encoding,omitempty"`
	Status           int                `json:"status,omitempty"`
	ServerTiming     []WireServerTiming `json:"server_timing,omitempty"`
	RequestID        string             `json:"request_id,omitempty"`
	Traceparent      string             `json:"traceparent,omitempty"`
	InitiatorStack   []string           `json:"initiator_stack,omitempty"`
	ReactComponent   string             `json:"react_component,omitempty"`
	RouteLoader      string             `json:"route_loader,omitempty"`
	StoreAction      string             `json:"store_action,omitempty"`
	SourceMapStatus  string             `json:"source_map_status,omitempty"`
	DuplicateGroupID string             `json:"duplicate_group_id,omitempty"`
	DuplicateCount   int                `json:"duplicate_count,omitempty"`
}

// NetworkWaterfallPayload is POSTed by the extension.
type NetworkWaterfallPayload struct {
	Entries []NetworkWaterfallEntry `json:"entries"`
	PageURL string                  `json:"page_url"`
}

// ============================================
// User Action Types
// ============================================

// EnhancedAction represents a captured user action with multi-strategy selectors.
// Wire fields: see WireEnhancedAction in wire_enhanced_action.go
type EnhancedAction struct {
	Type      string `json:"type"`
	Timestamp int64  `json:"timestamp"`
	URL       string `json:"url,omitempty"`
	// any: Selectors map contains multiple selector strategies (css, xpath, text, testId, etc.)
	// with string values, but some strategies have nested objects (e.g., aria-label with role)
	Selectors      map[string]any `json:"selectors,omitempty"`
	Value          string         `json:"value,omitempty"`
	InputType      string         `json:"input_type,omitempty"`
	Key            string         `json:"key,omitempty"`
	FromURL        string         `json:"from_url,omitempty"`
	ToURL          string         `json:"to_url,omitempty"`
	SelectedValue  string         `json:"selected_value,omitempty"`
	SelectedText   string         `json:"selected_text,omitempty"`
	ScrollY        int            `json:"scroll_y,omitempty"`
	TabID          int            `json:"tab_id,omitempty"`         // Chrome tab ID that produced this action
	TestIDs        []string       `json:"test_ids,omitempty"`       // Test IDs this action belongs to
	Source         string         `json:"source,omitempty"`         // "human" for user actions, "ai" for AI-driven actions via interact tool
	Classification string         `json:"classification,omitempty"` // Transient classification: toast, alert, snackbar, notification, tooltip, banner, flash
	DurationMs     int            `json:"duration_ms,omitempty"`    // Transient visibility duration (ms). MVP: always 0 (removal tracking not yet implemented)
	Role           string         `json:"role,omitempty"`           // ARIA role of the transient element (e.g., "alert", "status")
}

// EnhancedActionFilter defines filtering criteria for enhanced actions
type EnhancedActionFilter struct {
	LastN     int
	URLFilter string
	TestID    string // If set, filter actions where TestID is in action's TestIDs array
}
