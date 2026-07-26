// Purpose: Records HTTP network body observations for API schema inference.
// Why: Isolates HTTP observation ingestion from WebSocket and schema building logic.
// Docs: docs/features/feature/api-schema/index.md

package apischema

import (
	"encoding/json"
	"net/url"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

// ============================================
// HTTP Observation
// ============================================

// Observe records a network body observation for schema inference
func (s *SchemaStore) Observe(body capture.NetworkBody) {
	s.mu.Lock()
	defer s.mu.Unlock()

	parsedURL, err := url.Parse(body.URL)
	if err != nil {
		return
	}

	path := parsedURL.Path
	if path == "" {
		path = "/"
	}

	pattern := parameterizePath(path)
	key := body.Method + " " + pattern

	if _, exists := s.accumulators[key]; !exists && len(s.accumulators) >= maxSchemaEndpoints {
		return
	}

	acc := s.getOrCreateAccumulator(key, body.Method, pattern)
	s.recordBasicObservation(acc, path, body, parsedURL)
	s.recordBodyObservations(acc, body)
}

// recordBasicObservation updates counters, paths, query params, and latency.
func (s *SchemaStore) recordBasicObservation(acc *endpointAccumulator, path string, body capture.NetworkBody, parsedURL *url.URL) {
	acc.observationCount++
	acc.totalObservations++
	acc.lastSeen = timeNow()
	acc.lastPath = path

	if len(acc.actualPaths) < maxActualPaths {
		acc.actualPaths = append(acc.actualPaths, path)
	}
	s.observeQueryParams(acc, parsedURL.Query())

	if body.Duration > 0 && len(acc.latencies) < maxLatencySamples {
		acc.latencies = append(acc.latencies, float64(body.Duration))
	}
	if body.Status >= 400 {
		acc.errorCount++
	}
}

// timeNow is isolated for potential deterministic tests.
var timeNow = func() time.Time { return time.Now() }

// recordBodyObservations handles request/response body shape inference and status tracking.
func (s *SchemaStore) recordBodyObservations(acc *endpointAccumulator, body capture.NetworkBody) {
	if body.RequestBody != "" && isJSONContentType(body.ContentType) {
		s.observeRequestBody(acc, body.RequestBody)
	}

	if body.ResponseBody != "" && isJSONContentType(body.ContentType) {
		s.observeResponseBody(acc, body.Status, body.ResponseBody, body.ContentType)
		return
	}
	s.recordStatusOnly(acc, body.Status)
}

// recordStatusOnly tracks a status code without body parsing.
func (s *SchemaStore) recordStatusOnly(acc *endpointAccumulator, status int) {
	if status <= 0 {
		return
	}
	if acc.responseShapes == nil {
		acc.responseShapes = make(map[int]*responseAccumulator)
	}
	if ra, exists := acc.responseShapes[status]; exists {
		ra.count++
	} else if len(acc.responseShapes) < maxResponseShapes {
		acc.responseShapes[status] = &responseAccumulator{count: 1, fields: make(map[string]*fieldAccumulator)}
	}
}

func (s *SchemaStore) getOrCreateAccumulator(key, method, pattern string) *endpointAccumulator {
	acc, exists := s.accumulators[key]
	if !exists {
		acc = &endpointAccumulator{
			method:         method,
			pathPattern:    pattern,
			queryParams:    make(map[string]*paramAccumulator),
			requestFields:  make(map[string]*fieldAccumulator),
			responseShapes: make(map[int]*responseAccumulator),
			actualPaths:    make([]string, 0),
			latencies:      make([]float64, 0),
		}
		s.accumulators[key] = acc
	}
	return acc
}

func (s *SchemaStore) observeQueryParams(acc *endpointAccumulator, params url.Values) {
	for name, values := range params {
		pa := s.getOrCreateParam(acc, name)
		pa.count++
		for _, v := range values {
			trackParamValue(pa, v)
		}
	}
}

// getOrCreateParam returns the paramAccumulator for a query parameter, creating it if needed.
func (s *SchemaStore) getOrCreateParam(acc *endpointAccumulator, name string) *paramAccumulator {
	pa, exists := acc.queryParams[name]
	if !exists {
		pa = &paramAccumulator{values: make([]string, 0), allNumeric: true, allBoolean: true}
		acc.queryParams[name] = pa
	}
	return pa
}

// trackParamValue records a query parameter value and updates type inference flags.
func trackParamValue(pa *paramAccumulator, v string) {
	if len(pa.values) < maxQueryParamValues && !containsStr(pa.values, v) {
		pa.values = append(pa.values, v)
	}
	if !numericPattern.MatchString(v) {
		pa.allNumeric = false
	}
	if v != "true" && v != "false" {
		pa.allBoolean = false
	}
}

// containsStr returns true if slice contains the string.
func containsStr(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func (s *SchemaStore) observeRequestBody(acc *endpointAccumulator, body string) {
	var parsed map[string]any
	if json.Unmarshal([]byte(body), &parsed) != nil {
		return
	}

	acc.requestCount++
	for field, value := range parsed {
		fa, exists := acc.requestFields[field]
		if !exists {
			fa = &fieldAccumulator{
				typeCounts: make(map[string]int),
			}
			acc.requestFields[field] = fa
		}
		fa.observed++
		fieldType, format := inferTypeAndFormat(value)
		fa.typeCounts[fieldType]++
		if format != "" {
			fa.format = format
		}
	}
}

func (s *SchemaStore) observeResponseBody(acc *endpointAccumulator, status int, body, contentType string) {
	var parsed map[string]any
	if json.Unmarshal([]byte(body), &parsed) != nil {
		return
	}

	if acc.responseShapes == nil {
		acc.responseShapes = make(map[int]*responseAccumulator)
	}

	ra, exists := acc.responseShapes[status]
	if !exists {
		if len(acc.responseShapes) >= maxResponseShapes {
			return
		}
		ra = &responseAccumulator{
			contentType: contentType,
			fields:      make(map[string]*fieldAccumulator),
		}
		acc.responseShapes[status] = ra
	}
	ra.count++
	ra.contentType = contentType

	for field, value := range parsed {
		fa, exists := ra.fields[field]
		if !exists {
			fa = &fieldAccumulator{
				typeCounts: make(map[string]int),
			}
			ra.fields[field] = fa
		}
		fa.observed++
		fieldType, format := inferTypeAndFormat(value)
		fa.typeCounts[fieldType]++
		if format != "" {
			fa.format = format
		}
	}
}
