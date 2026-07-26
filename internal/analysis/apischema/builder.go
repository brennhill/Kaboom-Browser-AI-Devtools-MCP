// Purpose: Turns observation accumulators into a structured APISchema snapshot.
// Why: BuildSchema, its endpoint/param/field builders, auth detection and the WS/timing
// helpers are one operation over one set of accumulators — they were split across four
// files with a single caller each, which added files without adding a seam.
// Docs: docs/features/feature/api-schema/index.md

package apischema

import (
	"math"
	"sort"
	"strings"
	"time"
)

// ============================================
// Schema Building
// ============================================

// schemaAccum holds intermediate totals during schema building.
type schemaAccum struct {
	totalObservations int
	totalErrors       int
	totalLatency      float64
	latencyCount      int
}

// BuildSchema converts all accumulators into a structured API schema.
func (s *SchemaStore) BuildSchema(filter SchemaFilter) APISchema {
	s.mu.RLock()
	defer s.mu.RUnlock()

	endpoints, acc := s.collectEndpoints(filter)

	sort.Slice(endpoints, func(i, j int) bool {
		return endpoints[i].ObservationCount > endpoints[j].ObservationCount
	})

	return APISchema{
		Endpoints:   endpoints,
		WebSockets:  s.buildWSSchemas(),
		AuthPattern: s.detectAuthPattern(),
		Coverage:    buildCoverageStats(endpoints, acc),
	}
}

// collectEndpoints builds endpoint schemas from accumulators that pass the filter.
func (s *SchemaStore) collectEndpoints(filter SchemaFilter) ([]EndpointSchema, schemaAccum) {
	var endpoints []EndpointSchema
	var acc schemaAccum

	for _, endpointAcc := range s.accumulators {
		if filter.MinObservations > 0 && endpointAcc.observationCount < filter.MinObservations {
			continue
		}
		if filter.URLFilter != "" && !strings.Contains(endpointAcc.pathPattern, filter.URLFilter) {
			continue
		}
		endpoints = append(endpoints, s.buildEndpoint(endpointAcc))
		acc.totalObservations += endpointAcc.observationCount
		acc.totalErrors += endpointAcc.errorCount
		for _, latency := range endpointAcc.latencies {
			acc.totalLatency += latency
			acc.latencyCount++
		}
	}
	return endpoints, acc
}

// buildCoverageStats computes coverage statistics from endpoints and accumulated totals.
func buildCoverageStats(endpoints []EndpointSchema, acc schemaAccum) CoverageStats {
	coverage := CoverageStats{
		TotalEndpoints: len(endpoints),
		Methods:        make(map[string]int),
	}
	for i := range endpoints {
		coverage.Methods[endpoints[i].Method]++
	}
	if acc.totalObservations > 0 {
		coverage.ErrorRate = float64(acc.totalErrors) / float64(acc.totalObservations) * 100.0
	}
	if acc.latencyCount > 0 {
		coverage.AvgResponseMs = acc.totalLatency / float64(acc.latencyCount)
	}
	return coverage
}

// ============================================
// Endpoint, path-param, query-param and field builders
// ============================================

func (s *SchemaStore) buildEndpoint(acc *endpointAccumulator) EndpointSchema {
	ep := EndpointSchema{
		Method:           acc.method,
		PathPattern:      acc.pathPattern,
		LastPath:         acc.lastPath,
		ObservationCount: acc.observationCount,
		ResponseShapes:   make(map[int]*BodyShape),
	}

	if !acc.lastSeen.IsZero() {
		ep.LastSeen = acc.lastSeen.Format(time.RFC3339)
	}

	ep.PathParams = s.buildPathParams(acc.pathPattern)
	ep.QueryParams = s.buildQueryParams(acc)

	if acc.requestCount > 0 && len(acc.requestFields) > 0 {
		ep.RequestShape = &BodyShape{
			ContentType: "application/json",
			Fields:      s.buildFields(acc.requestFields, acc.requestCount),
			Count:       acc.requestCount,
		}
	}

	for status, responseAcc := range acc.responseShapes {
		if len(responseAcc.fields) > 0 {
			ep.ResponseShapes[status] = &BodyShape{
				ContentType: responseAcc.contentType,
				Fields:      s.buildFields(responseAcc.fields, responseAcc.count),
				Count:       responseAcc.count,
			}
		}
	}

	ep.Timing = computeTimingStats(acc.latencies)

	return ep
}

func (s *SchemaStore) buildPathParams(pattern string) []PathParam {
	segments := strings.Split(pattern, "/")
	var params []PathParam
	for i, segment := range segments {
		switch segment {
		case "{uuid}":
			params = append(params, PathParam{Name: "uuid", Type: "uuid", Position: i})
		case "{id}":
			params = append(params, PathParam{Name: "id", Type: "integer", Position: i})
		case "{hash}":
			params = append(params, PathParam{Name: "hash", Type: "string", Position: i})
		}
	}
	return params
}

func (s *SchemaStore) buildQueryParams(acc *endpointAccumulator) []QueryParam {
	params := make([]QueryParam, 0, len(acc.queryParams))
	for name, paramAcc := range acc.queryParams {
		qp := QueryParam{
			Name:           name,
			ObservedValues: paramAcc.values,
		}

		if acc.observationCount > 0 {
			ratio := float64(paramAcc.count) / float64(acc.observationCount)
			qp.Required = ratio > 0.9
		}

		if paramAcc.allNumeric && len(paramAcc.values) > 0 {
			qp.Type = "integer"
		} else if paramAcc.allBoolean && len(paramAcc.values) > 0 {
			qp.Type = "boolean"
		} else {
			qp.Type = "string"
		}

		params = append(params, qp)
	}

	sort.Slice(params, func(i, j int) bool {
		return params[i].Name < params[j].Name
	})

	return params
}

func (s *SchemaStore) buildFields(fields map[string]*fieldAccumulator, totalCount int) map[string]FieldSchema {
	result := make(map[string]FieldSchema)
	for name, fieldAcc := range fields {
		fs := FieldSchema{
			Type:     majorityType(fieldAcc.typeCounts),
			Format:   fieldAcc.format,
			Observed: fieldAcc.observed,
		}

		if totalCount > 0 {
			ratio := float64(fieldAcc.observed) / float64(totalCount)
			fs.Required = ratio > 0.9
		}

		result[name] = fs
	}
	return result
}

func majorityType(typeCounts map[string]int) string {
	maxCount := 0
	maxType := "string"
	for t, c := range typeCounts {
		if c > maxCount {
			maxCount = c
			maxType = t
		}
	}
	return maxType
}

// ============================================
// Auth pattern detection across observed endpoints
// ============================================

func (s *SchemaStore) detectAuthPattern() *AuthPattern {
	hasAuthEndpoint := false
	has401 := false
	var publicPaths []string

	authKeywords := []string{"/auth", "/login", "/token"}
	publicKeywords := []string{"/health", "/public"}

	for _, acc := range s.accumulators {
		lowerPattern := strings.ToLower(acc.pathPattern)
		if containsAny(lowerPattern, authKeywords) {
			hasAuthEndpoint = true
			publicPaths = append(publicPaths, acc.pathPattern)
		}
		if containsAny(lowerPattern, publicKeywords) {
			publicPaths = append(publicPaths, acc.pathPattern)
		}
		if !has401 {
			has401 = hasStatusCode(acc.responseShapes, 401)
		}
	}

	if !hasAuthEndpoint && !has401 {
		return nil
	}
	return &AuthPattern{Type: "bearer", Header: "Authorization", AuthRate: 100.0, PublicPaths: publicPaths}
}

// containsAny returns true if s contains any of the substrings.
func containsAny(s string, substrings []string) bool {
	for _, sub := range substrings {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// hasStatusCode checks if a response shapes map contains the given status code.
func hasStatusCode(shapes map[int]*responseAccumulator, code int) bool {
	_, ok := shapes[code]
	return ok
}

// ============================================
// WebSocket schema assembly and latency percentiles
// ============================================

// buildWSSchemas converts internal WebSocket accumulators to output schemas.
func (s *SchemaStore) buildWSSchemas() []WSSchema {
	wsSchemas := make([]WSSchema, 0, len(s.wsSchemas))
	for _, ws := range s.wsSchemas {
		wsSchema := WSSchema{
			URL:           ws.url,
			TotalMessages: ws.totalMessages,
			IncomingCount: ws.incomingCount,
			OutgoingCount: ws.outgoingCount,
		}
		for messageType := range ws.messageTypes {
			wsSchema.MessageTypes = append(wsSchema.MessageTypes, messageType)
		}
		sort.Strings(wsSchema.MessageTypes)
		wsSchemas = append(wsSchemas, wsSchema)
	}
	return wsSchemas
}

func computeTimingStats(latencies []float64) TimingStats {
	if len(latencies) == 0 {
		return TimingStats{}
	}

	sorted := make([]float64, len(latencies))
	copy(sorted, latencies)
	sort.Float64s(sorted)

	sum := 0.0
	for _, latency := range sorted {
		sum += latency
	}

	n := len(sorted)
	return TimingStats{
		Avg: sum / float64(n),
		P50: percentile(sorted, 0.50),
		P95: percentile(sorted, 0.95),
		Max: sorted[n-1],
	}
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	index := p * float64(len(sorted)-1)
	lower := int(math.Floor(index))
	upper := int(math.Ceil(index))
	if lower == upper {
		return sorted[lower]
	}
	fraction := index - float64(lower)
	return sorted[lower]*(1-fraction) + sorted[upper]*fraction
}
