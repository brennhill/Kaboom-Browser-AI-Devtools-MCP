// Purpose: Unit tests for schema assembly — endpoint/param/field builders, auth detection, timing stats.
// Docs: docs/features/feature/api-schema/index.md

package apischema

import (
	"math"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

func almostEqual(got, want float64) bool {
	const eps = 0.0001
	return math.Abs(got-want) < eps
}

func findEndpointByPattern(endpoints []EndpointSchema, pattern string) *EndpointSchema {
	for i := range endpoints {
		if endpoints[i].PathPattern == pattern {
			return &endpoints[i]
		}
	}
	return nil
}

func findQueryParam(params []QueryParam, name string) *QueryParam {
	for i := range params {
		if params[i].Name == name {
			return &params[i]
		}
	}
	return nil
}

func TestBuildSchemaCoverageFiltersAndAuthDetection(t *testing.T) {
	t.Parallel()

	store := NewSchemaStore()

	store.Observe(capture.NetworkBody{
		Method:      "GET",
		URL:         "https://api.example.com/users/123?limit=10&active=true",
		Status:      200,
		Duration:    120,
		ContentType: "application/json",
		RequestBody: `{"email":"alice@example.com","count":1}`,
		ResponseBody: `{
			"id": 123,
			"name": "alice"
		}`,
	})
	store.Observe(capture.NetworkBody{
		Method:      "GET",
		URL:         "https://api.example.com/users/456?limit=20&active=false",
		Status:      500,
		Duration:    240,
		ContentType: "application/json",
		RequestBody: `{"email":"bob@example.com","count":2}`,
		ResponseBody: `{
			"id": 456,
			"name": "bob"
		}`,
	})
	store.Observe(capture.NetworkBody{
		Method:       "POST",
		URL:          "https://api.example.com/auth/login",
		Status:       401,
		Duration:     60,
		ContentType:  "application/json",
		ResponseBody: `{"error":"unauthorized"}`,
	})

	store.ObserveWebSocket(capture.WebSocketEvent{
		URL:       "wss://api.example.com/ws",
		Direction: "incoming",
		Data:      `{"type":"ping"}`,
	})
	store.ObserveWebSocket(capture.WebSocketEvent{
		URL:       "wss://api.example.com/ws",
		Direction: "outgoing",
		Data:      `{"action":"update"}`,
	})

	// MinObservations should keep only the /users/{id} endpoint.
	schema := store.BuildSchema(SchemaFilter{MinObservations: 2})

	if schema.Coverage.TotalEndpoints != 1 {
		t.Fatalf("TotalEndpoints = %d, want 1", schema.Coverage.TotalEndpoints)
	}
	if schema.Coverage.Methods["GET"] != 1 {
		t.Fatalf("Coverage.Methods = %+v, want GET=1", schema.Coverage.Methods)
	}
	if !almostEqual(schema.Coverage.ErrorRate, 50.0) {
		t.Fatalf("Coverage.ErrorRate = %.2f, want 50.00", schema.Coverage.ErrorRate)
	}
	if !almostEqual(schema.Coverage.AvgResponseMs, 180.0) {
		t.Fatalf("Coverage.AvgResponseMs = %.2f, want 180.00", schema.Coverage.AvgResponseMs)
	}

	ep := findEndpointByPattern(schema.Endpoints, "/users/{id}")
	if ep == nil {
		t.Fatalf("expected endpoint /users/{id}, got %+v", schema.Endpoints)
	}
	if ep.Method != "GET" || ep.ObservationCount != 2 {
		t.Fatalf("unexpected endpoint core fields: %+v", *ep)
	}
	if ep.RequestShape == nil || len(ep.RequestShape.Fields) != 2 {
		t.Fatalf("expected request shape fields, got %+v", ep.RequestShape)
	}
	if ep.ResponseShapes[200] == nil || ep.ResponseShapes[500] == nil {
		t.Fatalf("expected response shapes for 200 and 500, got %+v", ep.ResponseShapes)
	}

	limit := findQueryParam(ep.QueryParams, "limit")
	if limit == nil || limit.Type != "integer" || !limit.Required {
		t.Fatalf("unexpected limit query param: %+v", limit)
	}
	active := findQueryParam(ep.QueryParams, "active")
	if active == nil || active.Type != "boolean" || !active.Required {
		t.Fatalf("unexpected active query param: %+v", active)
	}

	if schema.AuthPattern == nil {
		t.Fatal("expected auth pattern to be detected")
	}
	if schema.AuthPattern.Type != "bearer" || schema.AuthPattern.Header != "Authorization" {
		t.Fatalf("unexpected auth pattern: %+v", schema.AuthPattern)
	}
	if len(schema.AuthPattern.PublicPaths) == 0 {
		t.Fatalf("expected auth-related/public paths in auth pattern: %+v", schema.AuthPattern)
	}

	if len(schema.WebSockets) != 1 {
		t.Fatalf("expected 1 websocket schema, got %+v", schema.WebSockets)
	}
	ws := schema.WebSockets[0]
	if ws.TotalMessages != 2 || ws.IncomingCount != 1 || ws.OutgoingCount != 1 {
		t.Fatalf("unexpected websocket counters: %+v", ws)
	}
	if len(ws.MessageTypes) != 2 || ws.MessageTypes[0] != "ping" || ws.MessageTypes[1] != "update" {
		t.Fatalf("unexpected websocket message types: %+v", ws.MessageTypes)
	}

	// URL filter should only keep matching path patterns.
	filtered := store.BuildSchema(SchemaFilter{URLFilter: "auth"})
	if len(filtered.Endpoints) != 1 || filtered.Endpoints[0].PathPattern != "/auth/login" {
		t.Fatalf("URL filtered endpoints = %+v, want /auth/login only", filtered.Endpoints)
	}
}

func TestBuildQueryParamsAndBuildFields(t *testing.T) {
	t.Parallel()

	store := NewSchemaStore()
	acc := &endpointAccumulator{
		observationCount: 10,
		queryParams: map[string]*paramAccumulator{
			"zeta": {
				count:      9, // 9/10 should not be required (>0.9 only)
				values:     []string{"1"},
				allNumeric: true,
				allBoolean: false,
			},
			"alpha": {
				count:      10,
				values:     []string{"true", "false"},
				allNumeric: false,
				allBoolean: true,
			},
			"empty": {
				count:      10,
				values:     nil, // no values => fallback string type
				allNumeric: true,
				allBoolean: true,
			},
		},
	}

	params := store.buildQueryParams(acc)
	if len(params) != 3 {
		t.Fatalf("buildQueryParams len = %d, want 3", len(params))
	}
	// Deterministic sorting by name.
	if params[0].Name != "alpha" || params[1].Name != "empty" || params[2].Name != "zeta" {
		t.Fatalf("unexpected sorted params order: %+v", params)
	}

	alpha := findQueryParam(params, "alpha")
	if alpha == nil || alpha.Type != "boolean" || !alpha.Required {
		t.Fatalf("alpha query param mismatch: %+v", alpha)
	}
	empty := findQueryParam(params, "empty")
	if empty == nil || empty.Type != "string" || !empty.Required {
		t.Fatalf("empty query param mismatch: %+v", empty)
	}
	zeta := findQueryParam(params, "zeta")
	if zeta == nil || zeta.Type != "integer" || zeta.Required {
		t.Fatalf("zeta query param mismatch: %+v", zeta)
	}

	fields := store.buildFields(map[string]*fieldAccumulator{
		"id": {
			typeCounts: map[string]int{"integer": 5, "string": 1},
			observed:   6, // exactly 0.9 when totalCount=10 -> not required
		},
		"email": {
			typeCounts: map[string]int{"string": 9},
			format:     "email",
			observed:   10,
		},
	}, 10)

	if fields["id"].Type != "integer" {
		t.Fatalf("id field type = %q, want integer", fields["id"].Type)
	}
	if fields["id"].Required {
		t.Fatalf("id field required should be false at ratio=0.6: %+v", fields["id"])
	}
	if fields["email"].Format != "email" || !fields["email"].Required {
		t.Fatalf("email field mismatch: %+v", fields["email"])
	}
}

func TestComputeTimingStatsAndPercentile(t *testing.T) {
	t.Parallel()

	empty := computeTimingStats(nil)
	if empty.Avg != 0 || empty.P50 != 0 || empty.P95 != 0 || empty.Max != 0 {
		t.Fatalf("computeTimingStats(nil) = %+v, want zero values", empty)
	}

	stats := computeTimingStats([]float64{300, 100, 200, 400})
	if !almostEqual(stats.Avg, 250.0) {
		t.Fatalf("Avg = %.2f, want 250.0", stats.Avg)
	}
	if !almostEqual(stats.P50, 250.0) {
		t.Fatalf("P50 = %.2f, want 250.0", stats.P50)
	}
	if !almostEqual(stats.P95, 385.0) {
		t.Fatalf("P95 = %.2f, want 385.0", stats.P95)
	}
	if !almostEqual(stats.Max, 400.0) {
		t.Fatalf("Max = %.2f, want 400.0", stats.Max)
	}

	if percentile(nil, 0.5) != 0 {
		t.Fatal("percentile(nil) should be 0")
	}
	if percentile([]float64{42}, 0.95) != 42 {
		t.Fatal("percentile(singleton) should return the element")
	}
}

func TestDetectAuthPatternNilAndOpenAPIStub(t *testing.T) {
	t.Parallel()

	noAuthStore := NewSchemaStore()
	noAuthStore.Observe(capture.NetworkBody{
		Method:      "GET",
		URL:         "https://api.example.com/users",
		Status:      200,
		ContentType: "application/json",
		ResponseBody: `{
			"id": 1
		}`,
	})
	if ap := noAuthStore.detectAuthPattern(); ap != nil {
		t.Fatalf("detectAuthPattern() = %+v, want nil when no auth signals", ap)
	}

	store := NewSchemaStore()
	store.Observe(capture.NetworkBody{
		Method:      "GET",
		URL:         "https://api.example.com/orders/123?verbose=true",
		Status:      200,
		ContentType: "application/json",
		RequestBody: `{"include":"items"}`,
		ResponseBody: `{
			"order_id": 123,
			"status": "ok"
		}`,
	})

	openAPI := store.BuildOpenAPIStub(SchemaFilter{})
	if !strings.Contains(openAPI, "openapi: \"3.0.0\"") {
		t.Fatalf("OpenAPI stub missing header: %s", openAPI)
	}
	if !strings.Contains(openAPI, "/orders/{id}:") {
		t.Fatalf("OpenAPI stub missing parameterized path: %s", openAPI)
	}
	if !strings.Contains(openAPI, "in: path") || !strings.Contains(openAPI, "in: query") {
		t.Fatalf("OpenAPI stub missing path/query parameters: %s", openAPI)
	}
	if !strings.Contains(openAPI, "requestBody:") {
		t.Fatalf("OpenAPI stub missing requestBody block: %s", openAPI)
	}
	if !strings.Contains(openAPI, "\"200\":") {
		t.Fatalf("OpenAPI stub missing 200 response: %s", openAPI)
	}
}

// ============================================
// buildPathParams Coverage Tests
// ============================================

func TestBuildPathParamsUUID(t *testing.T) {
	t.Parallel()
	store := NewSchemaStore()
	params := store.buildPathParams("/api/users/{uuid}")

	if len(params) != 1 {
		t.Fatalf("Expected 1 param, got %d", len(params))
	}
	if params[0].Name != "uuid" {
		t.Errorf("Expected name 'uuid', got: %s", params[0].Name)
	}
	if params[0].Type != "uuid" {
		t.Errorf("Expected type 'uuid', got: %s", params[0].Type)
	}
	if params[0].Position != 3 {
		t.Errorf("Expected position 3, got: %d", params[0].Position)
	}
}

func TestBuildPathParamsID(t *testing.T) {
	t.Parallel()
	store := NewSchemaStore()
	params := store.buildPathParams("/api/items/{id}")

	if len(params) != 1 {
		t.Fatalf("Expected 1 param, got %d", len(params))
	}
	if params[0].Name != "id" {
		t.Errorf("Expected name 'id', got: %s", params[0].Name)
	}
	if params[0].Type != "integer" {
		t.Errorf("Expected type 'integer', got: %s", params[0].Type)
	}
}

func TestBuildPathParamsHash(t *testing.T) {
	t.Parallel()
	store := NewSchemaStore()
	params := store.buildPathParams("/api/commits/{hash}")

	if len(params) != 1 {
		t.Fatalf("Expected 1 param, got %d", len(params))
	}
	if params[0].Name != "hash" {
		t.Errorf("Expected name 'hash', got: %s", params[0].Name)
	}
	if params[0].Type != "string" {
		t.Errorf("Expected type 'string', got: %s", params[0].Type)
	}
}

func TestBuildPathParamsMultiple(t *testing.T) {
	t.Parallel()
	store := NewSchemaStore()
	params := store.buildPathParams("/api/{uuid}/items/{id}/commits/{hash}")

	if len(params) != 3 {
		t.Fatalf("Expected 3 params, got %d", len(params))
	}

	// Check each param
	expectedNames := []string{"uuid", "id", "hash"}
	expectedTypes := []string{"uuid", "integer", "string"}
	for i, p := range params {
		if p.Name != expectedNames[i] {
			t.Errorf("Param %d: expected name '%s', got '%s'", i, expectedNames[i], p.Name)
		}
		if p.Type != expectedTypes[i] {
			t.Errorf("Param %d: expected type '%s', got '%s'", i, expectedTypes[i], p.Type)
		}
	}
}

func TestBuildPathParamsNoParams(t *testing.T) {
	t.Parallel()
	store := NewSchemaStore()
	params := store.buildPathParams("/api/users/list")

	if len(params) != 0 {
		t.Errorf("Expected 0 params for path without placeholders, got %d", len(params))
	}
}

func TestBuildPathParamsUnknownPlaceholder(t *testing.T) {
	t.Parallel()
	store := NewSchemaStore()
	params := store.buildPathParams("/api/{unknown}/items")

	// Unknown placeholders should not be recognized
	if len(params) != 0 {
		t.Errorf("Expected 0 params for unknown placeholder, got %d", len(params))
	}
}

func TestBuildPathParamsEmptyPath(t *testing.T) {
	t.Parallel()
	store := NewSchemaStore()
	params := store.buildPathParams("")

	if len(params) != 0 {
		t.Errorf("Expected nil/empty params for empty path, got %d", len(params))
	}
}

func TestBuildPathParamsRootOnly(t *testing.T) {
	t.Parallel()
	store := NewSchemaStore()
	params := store.buildPathParams("/")

	if len(params) != 0 {
		t.Errorf("Expected 0 params for root path, got %d", len(params))
	}
}
