---
status: shipped
scope: feature/api-schema/implementation
ai-priority: high
tags: [implementation, architecture]
relates-to: [product-spec.md, qa-plan.md]
last-verified: 2026-03-05
doc_type: tech-spec
feature_id: feature-api-schema
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

> **[MIGRATION NOTICE]**
> Canonical location for this tech spec. Migrated from `/docs/ai-first/tech-spec-api-schema.md` on 2026-01-26.
> See also: [Product Spec](product-spec.md) and [Api Schema Review](api-schema-review.md).

# Technical Spec: API Schema Inference

## Purpose

When an AI agent is working on a project, it often needs to know what API endpoints exist, what they accept, and what they return. In traditional projects, this information lives in OpenAPI specs or documentation. In vibe-coded projects, it lives nowhere — the API is whatever the code happens to do.

Kaboom solves this by inferring the API schema from observed network traffic. Every request/response that flows through the browser is analyzed, and over time the server builds a complete picture of the API surface: endpoints, path parameters, query parameters, request/response shapes, timing characteristics, and authentication patterns.

The agent can then call `get_api_schema` to get a structured description of the API without ever reading a single line of backend code.

---

## How It Works

### Observation Phase

Every time a network body is added to the server's buffer (via the existing `AddNetworkBodies` function), the schema inference system is also notified. For each request, it:

1. **Parameterizes the path**: Replaces dynamic segments with placeholders. UUIDs become `{uuid}`, numeric IDs become `{id}`, long hex strings become `{hash}`. This groups `/api/users/123/posts` and `/api/users/456/posts` under the same pattern: `/api/users/{id}/posts`.

2. **Gets or creates an accumulator**: Each unique "METHOD + path_pattern" combination has an accumulator that collects observations over time.

3. **Records the observation**: Increments the count, updates the last-seen time, stores the actual path (for parameter detection refinement), tracks query parameters and their values, parses and stores the request/response body shapes, and records the latency.

This observation happens on every network body in a non-blocking manner (separate mutex from the main buffer).

### Schema Building Phase

When the agent calls `get_api_schema`, the server converts all accumulators into a structured schema:

1. For each accumulator, it builds an endpoint description with the method, path pattern, detected path parameters, query parameters (with type inference and required/optional detection), request body shape, response body shapes (grouped by status code), and timing statistics.

2. It detects the authentication pattern by looking for login/auth/token endpoints and 401 responses.

3. It computes API coverage statistics: total endpoints, methods breakdown, error rate, average response time.

4. It sorts endpoints by observation count (most-used first).

---

## Data Model

### Endpoint Schema

Each inferred endpoint has:
- Method and path pattern (e.g., "GET /api/users/{id}")
- The last actual path observed (e.g., "/api/users/42")
- Observation count and last-seen time
- Path parameters: name, inferred type (string/integer/uuid), position in path
- Query parameters: name, inferred type, whether required (present in >90% of requests), observed values (up to 10, for enum detection)
- Request body shape: content type and field map (field name → type + format + required flag + observation count)
- Response shapes: one per status code observed, each with count, content type, and field map
- Timing: average, P50, P95, and max latency in milliseconds

### Field Schema

A field in a request or response body is described by:
- Type: string, number, integer, boolean, array, object, or null
- Format: uuid, datetime, email, or url (detected from string values)
- Required: true if present in >90% of observations
- Observed: how many times this field was seen

When multiple observations have conflicting types for the same field, majority wins. This handles cases where a field is sometimes null.

### Auth Pattern

If the server detects auth-related endpoints (/auth, /login, /token paths) or 401 responses, it reports:
- Detected auth type (bearer by default, since Kaboom strips actual headers)
- The header name (Authorization)
- What percentage of requests are authenticated
- Which paths don't require auth (login, health, etc.)

### Type and Format Inference

String values are analyzed for format:
- UUID pattern → format "uuid"
- ISO datetime pattern → format "datetime"
- Contains @ and . → format "email"
- Starts with http:// or https:// → format "url"

Numbers are classified as "integer" if they have no fractional part, "number" otherwise.

---

## Tool Interface

### `get_api_schema`

**Parameters** (all optional):
- `url_filter`: Only include endpoints whose path contains this string
- `min_observations`: Minimum times an endpoint must be observed to be included (default: 1)
- `format`: "kaboom" (default, structured JSON) or "openapi_stub" (minimal OpenAPI 3.0 YAML)

**Returns**: The full API schema object with endpoints, WebSocket schemas, auth pattern, and coverage statistics.

The OpenAPI stub format generates valid YAML that can be pasted into API documentation tools, though it's intentionally minimal (no detailed validation rules, just structure).

---

## Query Parameter Inference

Query parameters are tracked across all requests to the same endpoint pattern:
- If a parameter appears in >90% of requests, it's marked as required
- Type is inferred from observed values: all-numeric → integer, all-boolean → boolean, otherwise string
- Up to 10 unique values are stored per parameter (useful for detecting enums)

---

## WebSocket Schema Inference

For WebSocket connections, the server tracks:
- The connection URL
- Total messages observed
- Message shapes grouped by direction (incoming/outgoing)
- If messages have a consistent "type" or "action" field, the observed values are recorded

This helps the agent understand the WebSocket protocol without documentation.

---

## Integration Point

The observation function is called from the existing `AddNetworkBodies` method. Each new network body triggers schema inference in the background (separate goroutine, separate lock). This ensures schema inference never slows down the main data path.

---

## Edge Cases

- **Max 200 endpoints tracked**: If the endpoint cap is reached, new unique endpoints are silently ignored. High-traffic endpoints already tracked continue accumulating.
- **Non-JSON bodies**: Only JSON responses contribute to shape inference. HTML, images, etc. are tracked for timing and status but not shape.
- **Max 10 response shapes per status code**: Prevents memory blowup on highly variable endpoints.
- **Max 100 latency samples per endpoint**: Older samples are not evicted, just stops collecting after 100.
- **Max 20 actual paths per endpoint**: Used for path parameter detection refinement.
- **Max 10 query param values**: For enum detection — more than 10 unique values is unlikely to be an enum.
- **Total memory cap**: 2MB for all accumulators combined.
- **Concurrent access**: Schema accumulators have their own RWMutex. Observation takes a write lock. Schema building takes a read lock. Neither blocks the main buffer mutex.

---

## Sensitive Data Handling

- Request and response bodies are used for **shape inference only** — the actual values are discarded after type detection.
- Auth tokens detected in bodies are classified as format "token" but the value is never stored in the schema.
- Email addresses are detected as format "email" but the original value is not persisted.
- Passwords are already redacted by the extension (replaced with `[redacted]`) before reaching the server.
- API keys are detected and excluded from example/observed values.

---

## Performance Constraints

- Observing a single network body: under 1ms
- Building the full schema (200 endpoints): under 50ms
- Path parameterization: under 0.1ms
- Merging field schemas from 10 observations: under 5ms
- Generating OpenAPI stub: under 100ms
- Total accumulator memory: under 2MB

---

## Test Scenarios

1. UUID in path → replaced with {uuid}
2. Numeric ID in path → replaced with {id}
3. Static path unchanged
4. Mixed UUIDs and IDs normalized correctly
5. First observation creates a new accumulator
6. Repeated observation increments count
7. Response body shape extracted: string, number, boolean, array, object, null types detected
8. Request body shape extracted with email format detection
9. Latency tracked and timing stats (avg, p50, p95, max) computed correctly
10. Query parameters tracked with value collection
11. Max 200 endpoints → 201st silently ignored
12. Required field detection: present in >90% → required=true
13. Type voting: majority type wins when observations conflict
14. UUID format detected in string values
15. Datetime format detected
16. Email format detected
17. Integer vs. float distinction (no fractional part → integer)
18. URL filter restricts output to matching endpoints
19. Min observations filter excludes low-traffic endpoints
20. OpenAPI stub output is valid YAML with paths and schemas
21. Multiple status codes on same endpoint → separate response schemas
22. Auth pattern detected when login endpoint + 401s observed
23. No auth endpoints and no 401s → auth_pattern is nil
24. WebSocket message shapes grouped by direction
25. Error rate computed correctly from status codes
26. Concurrent observations → no race conditions
27. Schema persists and loads across sessions

---

## File Location

Implementation goes in `cmd/browser-agent/ai_schema.go` with tests in `cmd/browser-agent/ai_schema_test.go`.
