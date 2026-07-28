---
title: "API Contract Validation"
description: "Use KaBOOM to infer API schemas from live traffic, detect breaking changes, validate response consistency, and catch contract violations during development."
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['guides', 'api', 'validation']
---

KaBOOM watches your API traffic and infers schemas automatically. No OpenAPI spec required — the `api_validation` mode learns your API's structure from real requests and flags inconsistencies, breaking changes, and unexpected responses.

## How It Works

1. **Browse your application normally** — KaBOOM captures every API request and response
2. **Analyze** — KaBOOM groups requests by endpoint pattern, normalizes dynamic segments, and infers request/response shapes
3. **Report** — See inferred schemas and any violations

No configuration needed. The validation happens against the API's own observed behavior.

---

## Quick Start

### Analyze Captured Traffic

After browsing your application for a few minutes:

```js
analyze({what: "api_validation", operation: "analyze"})
```

This processes all captured network traffic and infers schemas for each endpoint. The analyzer:
- Groups requests by URL pattern (normalizes `/users/123` and `/users/456` into `/users/:id`)
- Infers request body schemas from POST/PUT/PATCH payloads
- Infers response body schemas from successful responses
- Tracks response status codes per endpoint

### View the Report

```js
analyze({what: "api_validation", operation: "report"})
```

Returns the inferred schemas and any detected violations.

### Filter by Endpoint

```js
analyze({what: "api_validation", operation: "report", url: "/api/users"})
```

### Exclude Noisy Endpoints

```js
analyze({what: "api_validation", operation: "analyze",
        ignore_endpoints: ["/api/health", "/api/metrics", "/analytics"]})
```

### Reset

```js
analyze({what: "api_validation", operation: "clear"})
```

---

## What Gets Detected

### Response Shape Inconsistencies

The same endpoint returns different field sets:

```
GET /api/users/1  → { id, name, email, role }
GET /api/users/2  → { id, name, email }         ← missing "role"
```

This catches:
- Optional fields that should be required
- Null values where the client expects objects
- Missing fields after backend refactors
- Different response shapes between API versions

### Status Code Violations

An endpoint that normally returns 200 starts returning 500:

```
GET /api/users     → 200 (95% of requests)
GET /api/users     → 500 (5% of requests)    ← intermittent failure
```

### Type Mismatches

A field changes type between requests:

```
GET /api/orders/1  → { total: 29.99 }         ← number
GET /api/orders/2  → { total: "29.99" }        ← string
```

### New or Removed Fields

After a backend deploy, responses gain or lose fields:

```
Before:  { id, name, email }
After:   { id, name, email, avatar_url }       ← new field (usually fine)
After:   { id, name }                          ← removed field (breaking)
```

---

## Workflows

### Pre-Deploy Validation

Before deploying a backend change:

1. **Capture baseline traffic** on the current version
2. **Analyze** to establish schemas
3. **Deploy** the new version
4. **Re-analyze** against the same endpoints
5. **Compare** — any schema changes are potential breaking changes

```js
// Step 1-2: Baseline
analyze({what: "api_validation", operation: "analyze"})
analyze({what: "api_validation", operation: "report"})

// Step 3: Deploy happens here

// Step 4-5: Post-deploy
analyze({what: "api_validation", operation: "clear"})
// Browse the app again to capture new traffic
analyze({what: "api_validation", operation: "analyze"})
analyze({what: "api_validation", operation: "report"})
```

### Frontend-Backend Contract Checking

When the frontend expects a specific response shape:

```
"Analyze the API traffic and tell me if any endpoint returns inconsistent shapes."
```

The AI runs the analysis, reads the report, and cross-references with your frontend code to identify mismatches between what the API sends and what the UI expects.

### Regression Detection During Development

As you develop, leave API validation running in the background:

```
"Continuously monitor the API for any response shape changes as I work."
```

The AI periodically runs `api_validation` with `analyze` and `report` operations, flagging any new inconsistencies as you make changes.

---

## Combining with Other Tools

### Network Bodies for Detail

When the validator flags an issue, drill into the actual request/response:

```js
observe({what: "network_bodies", url: "/api/users"})
```

See the full payload including headers, status code, and body.

### Session Snapshots for Before/After

Capture the full session state before and after changes:

```js
configure({what: "diff_sessions", session_action: "capture", name: "before-api-change"})
// Make changes
configure({what: "diff_sessions", session_action: "capture", name: "after-api-change"})
configure({what: "diff_sessions", session_action: "compare",
           compare_a: "before-api-change", compare_b: "after-api-change"})
```

### Test Generation

Once you've validated the API behavior is correct, lock it in with a test:

```js
generate({what: "test", assert_network: true, assert_response_shape: true})
```

This generates a Playwright test that asserts the API response shapes match what was observed — catching future regressions automatically.

---

## Tips

**Analyze after browsing multiple flows.** The more traffic the validator sees, the more accurate the inferred schemas. Browse the main user journeys — login, list pages, detail pages, forms — before analyzing.

**Ignore health and metrics endpoints.** These change format frequently and create noise. Use `ignore_endpoints` to exclude them.

**Use with `noise_rule` for clean data.** Filter out analytics and third-party requests before analyzing so only your API traffic is included.

**Generate tests from validated contracts.** Once the API analysis shows consistent schemas, generate Playwright tests with `assert_response_shape: true` to lock in the contract.
