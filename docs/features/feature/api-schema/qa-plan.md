---
status: proposed
scope: feature/api-schema/qa
ai-priority: medium
tags: [testing, qa]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-03-05
doc_type: qa-plan
feature_id: feature-api-schema
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: API Schema Inference

> QA plan for the API Schema Inference feature. Covers data leak analysis, LLM clarity, simplicity assessment, code-level testing, and step-by-step UAT verification.

---

## 1. Data Leak Analysis

**Goal:** Verify the feature does NOT expose data it shouldn't. API schema inference observes real network traffic and builds a structural model of the API. The schema stores field types, formats, path patterns, and query parameter values. Actual data values must NOT persist -- only structural information (types, formats, required flags). This is especially critical because the schema may be persisted across sessions and could be shared.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | Actual field values stored in schema | Verify request/response bodies are used for shape inference only -- actual values are discarded after type detection | critical |
| DL-2 | Auth tokens stored in schema | Verify tokens detected in bodies are classified as format "token" but the value is never stored | critical |
| DL-3 | Email addresses stored as examples | Verify email values detected for format inference are NOT persisted in the schema accumulator | critical |
| DL-4 | API key values in query params | Verify query parameter values stored (up to 10 for enum detection) exclude API keys and tokens | critical |
| DL-5 | Password values in schema | Verify passwords already redacted by extension (`[redacted]`) and never stored in schema | critical |
| DL-6 | Internal endpoint paths exposed | Verify path patterns (e.g., `/admin/internal/health`) are included but do not expose authentication bypass or internal-only routes inappropriately | medium |
| DL-7 | Auth pattern detection revealing token format | Verify auth detection reports generic type ("bearer") without exposing token structure or values | high |
| DL-8 | OpenAPI stub containing sensitive defaults | Verify OpenAPI YAML output does not include example values from real traffic | high |
| DL-9 | Actual path values in accumulator | Verify `last_actual_path` (e.g., `/api/users/42`) does not expose real user IDs that are sensitive | medium |
| DL-10 | Query param values for enum detection | Verify stored values (up to 10) do not contain tokens, session IDs, or PII | high |
| DL-11 | Schema persistence across sessions | Verify persisted schema files do not contain raw request/response data | critical |
| DL-12 | WebSocket message content in schema | Verify WS schema stores only message shapes and type field values, not full message content | high |

### Negative Tests (must NOT leak)
- [ ] No actual user IDs, emails, or names appear anywhere in the schema output
- [ ] No authentication tokens or API key values appear in any schema field
- [ ] No password values (even redacted placeholders) appear in field examples
- [ ] Query parameter values stored for enum detection do not include tokens or session IDs
- [ ] OpenAPI stub output contains no example values from real traffic
- [ ] WebSocket schema contains only structural type info, not message payloads
- [ ] Auth pattern detection output contains no token values or token structure details
- [ ] Persisted schema files contain only structural information (types, formats, counts)

---

## 2. LLM Clarity Assessment

**Goal:** Verify an AI agent reading the tool responses can unambiguously understand the data without misinterpretation.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | Endpoint pattern is readable | `GET /api/users/{id}` clearly shows method and parameterized path | [ ] |
| CL-2 | Path parameters are typed | Parameters show inferred type (string/integer/uuid) | [ ] |
| CL-3 | Query parameters show required/optional | `required: true/false` is clearly indicated | [ ] |
| CL-4 | Response shapes per status code | Different status codes have distinct response schemas | [ ] |
| CL-5 | Field types are standard | Types use JSON Schema vocabulary: string, number, integer, boolean, array, object, null | [ ] |
| CL-6 | Format annotations are meaningful | uuid, datetime, email, url formats clearly describe string semantics | [ ] |
| CL-7 | Timing statistics are useful | avg, P50, P95, max latency help AI identify slow endpoints | [ ] |
| CL-8 | Auth pattern is actionable | Auth detection tells AI which endpoints need auth and which don't | [ ] |
| CL-9 | Coverage statistics are clear | Total endpoints, methods breakdown, error rate, avg response time | [ ] |
| CL-10 | Observation count conveys confidence | Higher count = more confident schema; low count = preliminary | [ ] |
| CL-11 | OpenAPI stub is valid | Output can be pasted into Swagger UI or documentation tools | [ ] |
| CL-12 | Empty schema is distinguishable | No observed traffic returns empty schema with clear message | [ ] |

### Common LLM Misinterpretation Risks
- [ ] LLM might confuse `{id}` placeholder with a literal path segment -- verify parameterized paths are clearly distinguished from literal paths
- [ ] LLM might treat `observation_count: 1` as definitive schema rather than preliminary -- verify low observation count is flagged
- [ ] LLM might assume all inferred types are correct (majority voting can be wrong with few samples) -- verify confidence indication
- [ ] LLM might treat `required: true` (>90% presence) as absolute rather than statistical -- verify the threshold is documented
- [ ] LLM might not understand enum detection from stored values -- verify output clearly labels detected enums

---

## 3. Simplicity Assessment

**Goal:** Count steps and evaluate cognitive load for both human and AI users.

**Complexity Score:** Low (passive observation + single query)

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Get full API schema | 1 step: call `observe(what: "api")` | No -- already minimal |
| Filter by URL path | 1 step: add `url_filter` parameter | No |
| Filter by observation count | 1 step: add `min_observations` parameter | No |
| Get OpenAPI stub | 1 step: add `format: "openapi_stub"` parameter | No |
| Build schema from scratch | 0 steps: happens automatically as traffic flows | No -- passive observation |

### Default Behavior Verification
- [ ] Feature works with zero configuration (schema builds automatically from observed traffic)
- [ ] Default format is "kaboom" (structured JSON)
- [ ] Default `min_observations` is 1 (all endpoints included)
- [ ] No explicit "start observing" step needed -- observation begins on first network body
- [ ] Schema is available immediately after traffic is captured (no manual build step)

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | UUID in path replaced | `/api/users/550e8400-e29b-41d4-a716-446655440000/posts` | Pattern: `/api/users/{uuid}/posts` | must |
| UT-2 | Numeric ID in path replaced | `/api/users/123/posts` | Pattern: `/api/users/{id}/posts` | must |
| UT-3 | Static path unchanged | `/api/health` | Pattern: `/api/health` (no parameterization) | must |
| UT-4 | Mixed UUID and numeric IDs | `/api/orgs/456/users/a1b2c3d4-e5f6-...` | Pattern: `/api/orgs/{id}/users/{uuid}` | must |
| UT-5 | First observation creates accumulator | New endpoint observed | Accumulator with count=1, last_seen set | must |
| UT-6 | Repeated observation increments | Same endpoint observed 5 times | count=5, fields merged | must |
| UT-7 | Response body shape - string type | JSON `{"name": "Alice"}` | Field "name" type "string" | must |
| UT-8 | Response body shape - number type | JSON `{"price": 19.99}` | Field "price" type "number" | must |
| UT-9 | Response body shape - integer type | JSON `{"count": 42}` | Field "count" type "integer" (no fractional part) | must |
| UT-10 | Response body shape - boolean | JSON `{"active": true}` | Field "active" type "boolean" | must |
| UT-11 | Response body shape - array | JSON `{"items": [1,2,3]}` | Field "items" type "array" | must |
| UT-12 | Response body shape - object | JSON `{"address": {"city": "NYC"}}` | Field "address" type "object" with nested fields | must |
| UT-13 | Response body shape - null | JSON `{"deleted": null}` | Field "deleted" type "null" (or majority type from other observations) | must |
| UT-14 | UUID format detection | String value matching UUID pattern | Format "uuid" | must |
| UT-15 | Datetime format detection | ISO 8601 string | Format "datetime" | must |
| UT-16 | Email format detection | String with `@` and `.` | Format "email" | must |
| UT-17 | URL format detection | String starting with `http://` or `https://` | Format "url" | should |
| UT-18 | Required field detection | Field present in 95 of 100 observations | `required: true` | must |
| UT-19 | Optional field detection | Field present in 50 of 100 observations | `required: false` | must |
| UT-20 | Type voting with conflicts | Field is string 8 times, null 2 times | Type "string" (majority wins) | must |
| UT-21 | Query parameter tracking | `?page=1&sort=name` on 5 requests | Parameters with types and value collection | must |
| UT-22 | Query param required detection | Param present in >90% of requests | `required: true` | should |
| UT-23 | Max 200 endpoints | 201st unique endpoint | Silently ignored, existing endpoints keep accumulating | must |
| UT-24 | Latency statistics | 10 observations with varying latency | Correct avg, P50, P95, max | must |
| UT-25 | Multiple status codes | Same endpoint returning 200 and 404 | Separate response schemas per status | must |
| UT-26 | Auth pattern detection | Login endpoint + 401 responses observed | Auth type "bearer", authenticated percentage | should |
| UT-27 | No auth pattern | No login endpoints, no 401s | `auth_pattern: null` | must |
| UT-28 | URL filter | `url_filter: "/api/"` | Only matching endpoints in output | must |
| UT-29 | Min observations filter | `min_observations: 5` | Endpoints with <5 observations excluded | must |
| UT-30 | OpenAPI stub output | `format: "openapi_stub"` | Valid YAML with paths and schemas | should |
| UT-31 | Error rate computation | 10 requests: 8 success, 2 errors | Error rate 20% | should |
| UT-32 | WebSocket schema inference | WS messages with `type` field | Types recorded by direction | should |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | End-to-end schema via MCP | Extension captures traffic -> server schema inference -> `observe(what: "api")` | Full schema with endpoints, shapes, timing | must |
| IT-2 | Schema builds incrementally | Multiple page loads with API calls -> query schema at each stage | Schema grows with each new observation | must |
| IT-3 | Non-JSON bodies excluded from shape | HTML response -> schema query | Endpoint tracked for timing/status but no shape | must |
| IT-4 | Concurrent observation and query | New traffic arriving while schema is being queried | Consistent snapshot, no race conditions | must |
| IT-5 | Schema persistence | Build schema -> restart server -> query schema | Schema survives restart (if persistence enabled) | should |
| IT-6 | OpenAPI import | Generate OpenAPI stub -> import into Swagger UI | Valid OpenAPI 3.0 accepted by Swagger | should |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | Single observation processing | Wall clock time | Under 1ms | must |
| PT-2 | Full schema build (200 endpoints) | Wall clock time | Under 50ms | must |
| PT-3 | Path parameterization | Wall clock time | Under 0.1ms | must |
| PT-4 | Field schema merge (10 observations) | Wall clock time | Under 5ms | should |
| PT-5 | OpenAPI stub generation | Wall clock time | Under 100ms | should |
| PT-6 | Total accumulator memory | Memory | Under 2MB | must |
| PT-7 | Observation does not block main buffer | Latency impact on AddNetworkBodies | Zero additional latency on critical path | must |

### 4.4 Edge Case Tests

| # | Edge Case | Input/Scenario | Expected Behavior | Priority |
|---|-----------|---------------|-------------------|----------|
| EC-1 | Non-JSON response body | HTML response | Tracked for timing/status, no shape inference | must |
| EC-2 | Max 10 response shapes per status code | 11 different response shapes for status 200 | Only 10 stored, no memory blowup | must |
| EC-3 | Max 100 latency samples per endpoint | 101st request | Stops collecting after 100, stats still accurate | must |
| EC-4 | Max 20 actual paths per endpoint | 21 different actual paths | Only 20 stored | should |
| EC-5 | Max 10 query param values | 11 unique values for one param | Only 10 stored (enum detection cap) | must |
| EC-6 | Total memory cap (2MB) | Very large API surface | Accumulators bounded at 2MB total | must |
| EC-7 | Very long URL path | 500-character path | Parameterized correctly without truncation | should |
| EC-8 | Empty response body | 204 No Content | Endpoint tracked, no shape inference | must |
| EC-9 | Deeply nested JSON | 10 levels of nesting | Shape captured up to reasonable depth | should |
| EC-10 | Array of mixed types | `[1, "two", true, null]` | Array type detected, item types noted | should |
| EC-11 | Long hex string in path | `/api/commits/a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2` | Replaced with `{hash}` | must |

---

## 5. UAT Checklist (Human + AI)

> Step-by-step verification for a human working with an AI assistant. The AI executes MCP tool calls; the human observes browser behavior and confirms results.

### Prerequisites
- [ ] Kaboom server running: `./dist/kaboom --port 7890`
- [ ] Chrome extension installed and connected
- [ ] A test web page that makes multiple API calls to various endpoints (REST API with CRUD operations)
- [ ] Network body capture enabled (opt-in)
- [ ] At least 5 different API endpoints have been called with varying payloads

### Step-by-Step Verification

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-1 | `{"tool": "observe", "arguments": {"what": "api"}}` | Check against known API endpoints | Schema shows all observed endpoints with correct methods | [ ] |
| UAT-2 | Call `/api/users/1` then `/api/users/2` then `/api/users/3`, then query schema | Three calls to same pattern | Single endpoint `GET /api/users/{id}` with count=3 | [ ] |
| UAT-3 | `{"tool": "observe", "arguments": {"what": "api", "url_filter": "/users"}}` | Compare to full schema | Only user-related endpoints appear | [ ] |
| UAT-4 | `{"tool": "observe", "arguments": {"what": "api", "min_observations": 3}}` | Compare counts | Only endpoints with 3+ observations appear | [ ] |
| UAT-5 | `{"tool": "observe", "arguments": {"what": "api", "format": "openapi_stub"}}` | Paste output into Swagger Editor | Valid OpenAPI 3.0 YAML accepted | [ ] |
| UAT-6 | Make a POST request with JSON body, then query schema | Check request body shape | Endpoint shows request body fields with correct types | [ ] |
| UAT-7 | Make requests with query params `?page=1&limit=10`, then query | Check query parameter inference | Parameters shown with types and required/optional status | [ ] |
| UAT-8 | Trigger a 404 and 500 error on known endpoints, then query | Check response shapes | Separate response schemas for 200, 404, and 500 status codes | [ ] |
| UAT-9 | Check timing statistics | Compare to observed response times | avg, P50, P95, max latency values are reasonable | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | No actual user data in schema | Search schema output for known user names, emails, IDs | Not found -- only types, formats, and counts | [ ] |
| DL-UAT-2 | No auth tokens in schema | Search for known bearer token values | Not found -- auth pattern shows only type detection | [ ] |
| DL-UAT-3 | Query param values safe | Check stored query param values for tokens | No token-like values stored (API keys excluded) | [ ] |
| DL-UAT-4 | OpenAPI stub has no examples | Check YAML output for `example:` fields | No example values from real traffic | [ ] |
| DL-UAT-5 | Actual path values sanitized | Check `last_actual_path` in schema | Real user IDs in paths are present (expected -- path is structural) but no PII in query/body | [ ] |

### Regression Checks
- [ ] Existing `observe(what: "network_bodies")` still works alongside schema inference
- [ ] Network body capture performance is unaffected (observation is background, separate goroutine)
- [ ] Existing network waterfall data is unaffected
- [ ] Server memory stays within bounds with high API traffic (2MB cap)
- [ ] Schema builds correctly even with intermittent network traffic (not all at once)

---

## Sign-Off

| Area | Tester | Date | Pass/Fail |
|------|--------|------|-----------|
| Data Leak Analysis | | | |
| LLM Clarity | | | |
| Simplicity | | | |
| Code Tests | | | |
| UAT | | | |
| **Overall** | | | |
