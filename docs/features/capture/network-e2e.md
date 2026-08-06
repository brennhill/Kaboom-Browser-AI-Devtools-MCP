---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Agent Assignment: Network Body E2E Tests

**Branch:** `feature/network-e2e`
**Worktree:** `../kaboom-network-e2e`
**Priority:** P5 (nice-to-have, parallel)

---

## Objective

Add end-to-end test coverage for network body capture, covering edge cases: large bodies, binary content, header sanitization, streaming responses.

---

## Deliverables

### 1. Test Server

**File:** `extension-tests/fixtures/test-server.mjs` (new)

Express/http server that serves test endpoints:
- `GET /large-json` — 1MB JSON response
- `GET /large-text` — 1MB text response
- `POST /echo` — Returns request body
- `GET /binary` — Returns binary data (image bytes)
- `GET /streaming` — Chunked transfer encoding
- `GET /auth-header` — Returns request headers (to verify sanitization)
- `GET /slow` — 3 second delay

### 2. E2E Test Suite

**File:** `extension-tests/network-body-e2e.test.js` (new)

Tests that require actual fetch/XHR:

1. **Large body truncation** — 1MB body truncated to configured limit
2. **Binary body handling** — Binary preserved, not corrupted
3. **Header sanitization** — Authorization header stripped from captured data
4. **POST body capture** — Request body captured correctly
5. **Content-Type detection** — JSON vs text vs binary
6. **Streaming response** — Chunked responses captured
7. **Timeout handling** — Slow responses don't block capture
8. **Error responses** — 4xx/5xx bodies captured

### 3. CI Integration

Tests should skip gracefully if test server not running (for local dev).

---

## Tests

All in `extension-tests/network-body-e2e.test.js`.

Run with test server:
```bash
node extension-tests/fixtures/test-server.mjs &
node --test extension-tests/network-body-e2e.test.js
```

---

## Verification

```bash
# Start test server in background
node extension-tests/fixtures/test-server.mjs &
TEST_SERVER_PID=$!

# Run E2E tests
node --test extension-tests/network-body-e2e.test.js

# Cleanup
kill $TEST_SERVER_PID
```

---

## Files Modified

| File | Change |
|------|--------|
| `extension-tests/fixtures/test-server.mjs` | New file — test endpoints |
| `extension-tests/network-body-e2e.test.js` | New file — E2E tests |
