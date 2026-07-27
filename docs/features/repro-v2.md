---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Agent Assignment: Reproduction Script Enhancements

**Branch:** `feature/repro-v2`
**Worktree:** `../kaboom-repro-v2`
**Priority:** P5 (nice-to-have, parallel)

---

## Objective

Enhance reproduction scripts with screenshot insertion, data fixture generation, and visual assertions. Makes generated Playwright tests more complete and self-documenting.

---

## Deliverables

### 1. Screenshot Insertion

**File:** `cmd/browser-agent/tools_generate.go`

When generating reproduction script, insert screenshot capture at key points:
```javascript
// After navigation
await page.goto('https://example.com');
await page.screenshot({ path: 'step-1-navigation.png' });

// After significant action
await page.click('#submit');
await page.screenshot({ path: 'step-2-after-submit.png' });
```

Option in MCP tool: `include_screenshots: true` (default false to keep scripts clean).

### 2. Data Fixture Generation

**File:** `cmd/browser-agent/tools_generate.go`

Extract API response data used during the session and generate fixture files:
```javascript
// fixtures/api-responses.json generated alongside script
const fixtures = require('./fixtures/api-responses.json');

// In test, mock the API
await page.route('**/api/users', route => {
  route.fulfill({ json: fixtures.users });
});
```

Option: `generate_fixtures: true`.

### 3. Visual Assertions

**File:** `cmd/browser-agent/tools_generate.go`

Add visual snapshot assertions at key checkpoints:
```javascript
await expect(page).toHaveScreenshot('checkout-page.png');
```

Option: `visual_assertions: true`.

### 4. MCP Tool Updates

**File:** `cmd/browser-agent/tools_core.go`

Update `generate` tool schema:
```json
{
  "format": "reproduction",
  "options": {
    "include_screenshots": false,
    "generate_fixtures": false,
    "visual_assertions": false
  }
}
```

---

## Tests

**File:** `cmd/browser-agent/reproduction_test.go`

1. Screenshot insertion at correct points
2. Fixture file generation from network data
3. Visual assertion insertion
4. Options default to false
5. Combined options work together

---

## Verification

```bash
go test -v ./cmd/browser-agent/ -run Reproduction
```

---

## Files Modified

| File | Change |
|------|--------|
| `cmd/browser-agent/tools_generate.go` | Screenshot, fixture, assertion generation |
| `cmd/browser-agent/tools_core.go` | Update generate tool schema |
| `cmd/browser-agent/reproduction_test.go` | Test new options |
