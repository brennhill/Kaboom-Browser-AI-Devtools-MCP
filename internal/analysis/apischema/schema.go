// Purpose: Implements API schema inference state and endpoint-shape accumulation from captured traffic.
// Why: Builds data-driven endpoint schemas that power validation and drift detection workflows.
// Docs: docs/features/feature/api-schema/index.md

// Package apischema infers API endpoint schemas from observed browser traffic.
//
// SchemaStore accumulates HTTP and WebSocket observations (path patterns with
// dynamic segments parameterized, query params, request/response field types and
// latencies) and renders them on demand as an APISchema snapshot or an OpenAPI
// stub. Accumulation is bounded by per-store caps so a long session cannot grow
// without limit.
package apischema

import (
	"sync"
	"time"
)

// ============================================
// Constants
// ============================================

const (
	maxSchemaEndpoints  = 200
	maxLatencySamples   = 100
	maxActualPaths      = 20
	maxQueryParamValues = 10
	maxResponseShapes   = 10
	maxWSSchemaConns    = 50
	maxWSMessageTypes   = 20
)

// ============================================
// Types
// ============================================

// SchemaFilter controls what endpoints are returned
type SchemaFilter struct {
	URLFilter       string
	MinObservations int
}

// APISchema is the top-level response from get_api_schema
type APISchema struct {
	Endpoints   []EndpointSchema `json:"endpoints"`
	WebSockets  []WSSchema       `json:"websockets,omitempty"`
	AuthPattern *AuthPattern     `json:"auth_pattern,omitempty"`
	Coverage    CoverageStats    `json:"coverage"`
}

// EndpointSchema describes one inferred API endpoint
type EndpointSchema struct {
	Method           string             `json:"method"`
	PathPattern      string             `json:"path_pattern"`
	LastPath         string             `json:"last_path,omitempty"`
	ObservationCount int                `json:"observation_count"`
	LastSeen         string             `json:"last_seen,omitempty"`
	PathParams       []PathParam        `json:"path_params,omitempty"`
	QueryParams      []QueryParam       `json:"query_params,omitempty"`
	RequestShape     *BodyShape         `json:"request_shape,omitempty"`
	ResponseShapes   map[int]*BodyShape `json:"response_shapes,omitempty"`
	Timing           TimingStats        `json:"timing"`
}

// PathParam describes a detected path parameter
type PathParam struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Position int    `json:"position"`
}

// QueryParam describes a detected query parameter
type QueryParam struct {
	Name           string   `json:"name"`
	Type           string   `json:"type"`
	Required       bool     `json:"required"`
	ObservedValues []string `json:"observed_values,omitempty"`
}

// BodyShape describes the inferred shape of a request or response body
type BodyShape struct {
	ContentType string                 `json:"content_type,omitempty"`
	Fields      map[string]FieldSchema `json:"fields,omitempty"`
	Count       int                    `json:"count"`
}

// FieldSchema describes a single field in a body shape
type FieldSchema struct {
	Type     string `json:"type"`
	Format   string `json:"format,omitempty"`
	Required bool   `json:"required"`
	Observed int    `json:"observed"`
}

// TimingStats holds computed latency statistics
type TimingStats struct {
	Avg float64 `json:"avg"`
	P50 float64 `json:"p50"`
	P95 float64 `json:"p95"`
	Max float64 `json:"max"`
}

// AuthPattern describes detected authentication patterns
type AuthPattern struct {
	Type        string   `json:"type"`
	Header      string   `json:"header"`
	AuthRate    float64  `json:"auth_rate_percent"`
	PublicPaths []string `json:"public_paths,omitempty"`
}

// WSSchema describes inferred WebSocket message patterns
type WSSchema struct {
	URL           string   `json:"url"`
	TotalMessages int      `json:"total_messages"`
	MessageTypes  []string `json:"message_types,omitempty"`
	IncomingCount int      `json:"incoming_count"`
	OutgoingCount int      `json:"outgoing_count"`
}

// CoverageStats holds API coverage statistics
type CoverageStats struct {
	TotalEndpoints int            `json:"total_endpoints"`
	Methods        map[string]int `json:"methods"`
	ErrorRate      float64        `json:"error_rate_percent"`
	AvgResponseMs  float64        `json:"avg_response_ms"`
}

// ============================================
// Internal accumulator types
// ============================================

// endpointAccumulator collects observations for one METHOD+path_pattern
type endpointAccumulator struct {
	method           string
	pathPattern      string
	lastPath         string
	observationCount int
	lastSeen         time.Time
	actualPaths      []string

	// Query params: name -> paramAccumulator
	queryParams map[string]*paramAccumulator

	// Request body field tracking
	requestFields map[string]*fieldAccumulator
	requestCount  int

	// Response shapes: statusCode -> responseAccumulator
	responseShapes map[int]*responseAccumulator

	// Latency samples
	latencies []float64

	// Total observations for required field calculation
	totalObservations int

	// Track status codes for error rate
	errorCount int
}

// paramAccumulator tracks query parameter values and occurrences
type paramAccumulator struct {
	count      int
	values     []string
	allNumeric bool
	allBoolean bool
}

// fieldAccumulator tracks a body field across observations
type fieldAccumulator struct {
	typeCounts map[string]int // type -> count
	format     string
	observed   int
}

// responseAccumulator tracks response shape for a specific status code
type responseAccumulator struct {
	count       int
	contentType string
	fields      map[string]*fieldAccumulator
}

// wsAccumulator tracks WebSocket message patterns for a connection URL
type wsAccumulator struct {
	url           string
	totalMessages int
	incomingCount int
	outgoingCount int
	messageTypes  map[string]bool
}

// ============================================
// SchemaStore
// ============================================

// SchemaStore manages API schema inference from observed network traffic
type SchemaStore struct {
	mu           sync.RWMutex
	accumulators map[string]*endpointAccumulator // key: "METHOD /path/pattern"
	wsSchemas    map[string]*wsAccumulator       // key: URL
}

// NewSchemaStore creates a new SchemaStore
func NewSchemaStore() *SchemaStore {
	return &SchemaStore{
		accumulators: make(map[string]*endpointAccumulator),
		wsSchemas:    make(map[string]*wsAccumulator),
	}
}

// EndpointCount returns the number of observed API endpoints.
func (s *SchemaStore) EndpointCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.accumulators)
}
