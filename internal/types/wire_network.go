// Purpose: Defines canonical wire schema for network body and waterfall telemetry payloads.
// Why: Keeps network telemetry transport contracts aligned between browser capture and daemon ingestion.
// Docs: docs/features/feature/normalized-event-schema/index.md

package types

// WireNetworkBody is the canonical wire format for captured network request/response bodies.
type WireNetworkBody struct {
	Method            string `json:"method"`
	URL               string `json:"url"`
	Status            int    `json:"status"`
	RequestBody       string `json:"request_body,omitempty"`
	ResponseBody      string `json:"response_body,omitempty"`
	ContentType       string `json:"content_type,omitempty"`
	Duration          int    `json:"duration,omitempty"`
	RequestTruncated  bool   `json:"request_truncated,omitempty"`
	ResponseTruncated bool   `json:"response_truncated,omitempty"`
	TabID             int    `json:"tab_id,omitempty"`
}

// WireNetworkWaterfallEntry is the canonical wire format for a PerformanceResourceTiming entry.
type WireNetworkWaterfallEntry struct {
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

type WireServerTiming struct {
	Name        string  `json:"name"`
	DurationMs  float64 `json:"duration_ms,omitempty"`
	Description string  `json:"description,omitempty"`
}

// WireNetworkWaterfallPayload is the top-level shape POSTed to /network-waterfall.
type WireNetworkWaterfallPayload struct {
	Entries []WireNetworkWaterfallEntry `json:"entries"`
	PageURL string                      `json:"page_url"`
}
