---
feature: test-generation
type: validation
status: ready_to_execute
demo_site: ~/dev/kaboom-demos
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Test Generation — Hands-On Validation Guide

## Objective

Prove that the test generation implementation actually works by:
1. Capturing real errors from the demo site
2. Generating tests from those errors
3. Verifying the generated tests reproduce the bugs
4. Healing broken selectors in real test files

## Prerequisites

```bash
# Start the demo site (ShopBroken - 34 intentional bugs)
cd ~/dev/kaboom-demos
npm run demo

# In another terminal, start Kaboom MCP server
cd ~/dev/kaboom
make dev
./dist/kaboom-mcp
```

## Validation 1: Generate Test from WebSocket Error (30 min)

**Bug:** Chat WebSocket connects to wrong endpoint + message parsing failures

**Why This Matters:** WebSocket monitoring is a **unique Kaboom feature** that TestSprite doesn't have. This validates our competitive advantage.

### Step 1: Trigger the WebSocket Errors

1. Open <http://localhost:3000> in Chrome with Kaboom extension
2. Click "Chat" in the header to open live chat widget
3. Observe multiple errors:
   - WebSocket connection error (wrong endpoint `/ws/chat` instead of `/chat`)
   - Messages showing as "undefined" (JSON.parse missing)
   - Message field mismatch (`txt` vs `text`)
   - Status stuck on "Connecting..."

#### Demo bugs hit:
- Phase 1, Bug 3: WebSocket connects to wrong endpoint
- Phase 1, Bug 4: Chat shows "Connecting..." forever
- Phase 3, Bug 1: Messages not parsed (JSON.parse missing)
- Phase 3, Bug 2: Server sends `txt`, client expects `text`

### Step 2: Verify WebSocket Errors were Captured

```bash
# Check WebSocket events (UNIQUE TO KABOOM)
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "observe",
    "arguments": {
      "mode": "websocket"
    }
  }
}
```

**Expected:** WebSocket frame data showing:
- Connection attempts to wrong endpoint
- Incoming frames with `txt` field
- Client-side parsing errors

```bash
# Check console errors
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/call",
  "params": {
    "name": "observe",
    "arguments": {
      "mode": "errors"
    }
  }
}
```

**Expected:** Error list includes:
- WebSocket connection error
- JSON.parse errors
- Field access errors (trying to read `text` from undefined)
- Each with error_id like `err_1738123456789_a1b2c3d4`

### Step 3: Generate Test from WebSocket Error

```bash
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/call",
  "params": {
    "name": "generate",
    "arguments": {
      "format": "test_from_context",
      "context": "error",
      "framework": "playwright",
      "base_url": "http://localhost:3000"
    }
  }
}
```

#### Expected Output:
- Summary: "Generated Playwright test 'error-chat-websocket.spec.ts' (X assertions)"
- Content: Valid Playwright test with:
  - `await page.goto('http://localhost:3000')`
  - Click action to open chat: `await page.locator('[data-testid="chat-button"]').click()`
  - **WebSocket monitoring** (UNIQUE TO KABOOM):
    ```typescript
    // Listen for WebSocket frames
    page.on('websocket', ws => {
      ws.on('framereceived', frame => {
        const data = JSON.parse(frame.payload);
        // Assert message structure
      });
    });
    ```
  - Error expectation for JSON.parse failure
  - Assertions for WebSocket connection state
  - Selectors using stable attributes (data-testid, aria-label, etc.)

**Key Differentiator:** The generated test includes WebSocket frame monitoring that TestSprite cannot do, because TestSprite doesn't capture WebSocket traffic in real-time.

### Step 4: Save and Run the Test

```bash
# Save generated test
cd ~/dev/kaboom-demos
mkdir -p tests
# Copy generated content to tests/generated-products-404.spec.ts

# Install Playwright if needed
npm install -D @playwright/test

# Run the test
npx playwright test tests/generated-products-404.spec.ts
```

## Success Criteria:
- ✅ Test runs without syntax errors
- ✅ Test reproduces the 404 error
- ✅ Test fails as expected (because bug exists)
- ✅ Generated selectors are valid

### Step 5: Fix the Bug and Re-run

```bash
# Fix: Change /api/product to /api/products in server/index.js
# Re-run test
npx playwright test tests/generated-products-404.spec.ts
```

**Expected:** Test now passes (bug fixed)

---

## Validation 1B: WebSocket-Specific Test Generation (15 min)

**Kaboom's Unique Advantage:** TestSprite doesn't monitor WebSocket frames.

### Generate Test from Interaction (Not Just Errors)

```bash
# Interact with chat (type messages, send, receive responses)
# Then generate test from interaction
{
  "jsonrpc": "2.0",
  "id": 4,
  "method": "tools/call",
  "params": {
    "name": "generate",
    "arguments": {
      "format": "test_from_context",
      "context": "interaction",
      "framework": "playwright",
      "base_url": "http://localhost:3000"
    }
  }
}
```

## Expected Output:
- Test includes all captured WebSocket frames
- Assertions verify message content from frames
- Assertions check typing indicator behavior
- Assertions validate message ordering

## Validation:
```bash
# Run generated test
npx playwright test tests/generated-chat-interaction.spec.ts
```

## Success Criteria:
- ✅ Test captures WebSocket connection lifecycle
- ✅ Test validates WebSocket frame content
- ✅ Test reproduces chat interaction sequence
- ✅ No manual WebSocket mocking needed (Kaboom captured it)

---

## Validation 2: Test Healing with Broken Selectors (30 min)

**Scenario:** Create a test with broken selectors, then heal them

### Step 1: Create a Test with Broken Selectors

Create `tests/broken-cart.spec.ts`:

```typescript
import { test, expect } from '@playwright/test';

test('add to cart', async ({ page }) => {
  await page.goto('http://localhost:3000');

  // These selectors are intentionally broken (using old IDs)
  await page.locator('#old-product-card-1').waitFor();
  await page.locator('#old-add-to-cart-btn').click();
  await page.locator('#old-login-email').fill('test@example.com');
  await page.locator('#old-login-password').fill('password');
  await page.locator('#old-login-submit').click();

  // Check cart count
  await expect(page.locator('#old-cart-count')).toHaveText('1');
});
```

### Step 2: Analyze the Test

```bash
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/call",
  "params": {
    "name": "generate",
    "arguments": {
      "format": "test_heal",
      "action": "analyze",
      "test_file": "tests/broken-cart.spec.ts"
    }
  }
}
```

#### Expected Output:
- Summary: "Found 6 selectors in tests/broken-cart.spec.ts"
- Selectors list: `#old-product-card-1`, `#old-add-to-cart-btn`, etc.

### Step 3: Heal the Broken Selectors

First, load the actual page so DOM is available:

```bash
# Open http://localhost:3000 in browser with Kaboom extension
# Wait for products to load (they'll 404, but DOM structure is there)
```

Then heal:

```bash
{
  "jsonrpc": "2.0",
  "id": 4,
  "method": "tools/call",
  "params": {
    "name": "generate",
    "arguments": {
      "format": "test_heal",
      "action": "repair",
      "test_file": "tests/broken-cart.spec.ts",
      "broken_selectors": [
        "#old-product-card-1",
        "#old-add-to-cart-btn",
        "#old-login-email",
        "#old-login-password",
        "#old-login-submit",
        "#old-cart-count"
      ],
      "auto_apply": false
    }
  }
}
```

## Expected Output:
- Summary: "Healed X/6 selectors (Y unhealed, Z auto-applied)"
- Healed list with:
  - `old_selector`: "#old-product-card-1"
  - `new_selector`: ".product-card" (or similar)
  - `confidence`: 0.7-0.95
  - `strategy`: "aria_match" or "text_match"

## Success Criteria:
- ✅ At least 4/6 selectors healed
- ✅ High-confidence healings (>= 0.9) use stable selectors (testId, aria)
- ✅ Suggested selectors actually exist on the page

### Step 4: Apply Fixes Manually

```bash
# Update tests/broken-cart.spec.ts with suggested selectors
# Re-run test
npx playwright test tests/broken-cart.spec.ts
```

**Expected:** Test runs further (may still fail due to other bugs, but selectors work)

---

## Validation 3: Failure Classification (20 min)

**Scenario:** Classify real test failures

### Step 1: Create Tests that Will Fail

Create `tests/failing-tests.spec.ts`:

```typescript
import { test, expect } from '@playwright/test';

test('timeout waiting for selector', async ({ page }) => {
  await page.goto('http://localhost:3000');
  await page.locator('#non-existent-button').click(); // Will timeout
});

test('network error', async ({ page }) => {
  // Trigger network error by requesting wrong endpoint
  await page.goto('http://localhost:9999/broken'); // Connection refused
});

test('assertion failure', async ({ page }) => {
  await page.goto('http://localhost:3000');
  await expect(page.locator('.product-grid')).toContainText('Unicorn'); // Will fail
});
```

### Step 2: Run Tests and Capture Failures

```bash
npx playwright test tests/failing-tests.spec.ts --reporter=json > test-results.json
```

### Step 3: Classify Each Failure

Extract error messages from test-results.json and classify:

```bash
{
  "jsonrpc": "2.0",
  "id": 5,
  "method": "tools/call",
  "params": {
    "name": "generate",
    "arguments": {
      "format": "test_classify",
      "action": "failure",
      "failure": {
        "test_name": "timeout waiting for selector",
        "error": "Timeout 30000ms exceeded. Waiting for selector '#non-existent-button'",
        "trace": "at tests/failing-tests.spec.ts:5:30",
        "duration_ms": 30000
      }
    }
  }
}
```

#### Expected Output:
- Summary: "Classified as selector_broken (90% confidence) — recommended: heal"
- Category: `selector_broken`
- Evidence: ["Selector '#non-existent-button' not found in current DOM"]
- SuggestedFix: { type: "selector_update", old: "#non-existent-button" }

### Step 4: Classify Network Error

```bash
{
  "jsonrpc": "2.0",
  "id": 6,
  "method": "tools/call",
  "params": {
    "name": "generate",
    "arguments": {
      "format": "test_classify",
      "action": "failure",
      "failure": {
        "test_name": "network error",
        "error": "net::ERR_CONNECTION_REFUSED at http://localhost:9999/broken",
        "duration_ms": 1000
      }
    }
  }
}
```

#### Expected Output:
- Category: `network_flaky`
- Confidence: ~0.85
- RecommendedAction: "mock_network"

### Step 5: Classify Assertion Failure

```bash
{
  "jsonrpc": "2.0",
  "id": 7,
  "method": "tools/call",
  "params": {
    "name": "generate",
    "arguments": {
      "format": "test_classify",
      "action": "failure",
      "failure": {
        "test_name": "assertion failure",
        "error": "Expected text to contain 'Unicorn', received: 'Wireless Headphones...'",
        "duration_ms": 500
      }
    }
  }
}
```

#### Expected Output:
- Category: `real_bug` or `test_bug`
- Confidence: ~0.7
- IsRealBug: true or false (depends on pattern)

#### Success Criteria:
- ✅ All 3 failures classified correctly
- ✅ Confidence >= 0.7 for each
- ✅ Recommended actions make sense

---

## Validation 4: Batch Classification (10 min)

```bash
{
  "jsonrpc": "2.0",
  "id": 8,
  "method": "tools/call",
  "params": {
    "name": "generate",
    "arguments": {
      "format": "test_classify",
      "action": "batch",
      "failures": [
        {
          "test_name": "test1",
          "error": "Timeout waiting for selector '#btn'"
        },
        {
          "test_name": "test2",
          "error": "net::ERR_CONNECTION_REFUSED"
        },
        {
          "test_name": "test3",
          "error": "Expected 'Hello' to be 'Goodbye'"
        }
      ]
    }
  }
}
```

### Expected Output:
- Summary: "Classified 3 failures: X real bugs, Y flaky, Z test issues, 0 uncertain"
- Total counts match
- Summary object has category breakdown

---

## Success Metrics

### Overall Success Criteria

We can claim the implementation works if:

1. **Test Generation:**
   - ✅ Generated test is valid Playwright syntax (no errors)
   - ✅ Generated test reproduces the original error
   - ✅ Generated test uses stable selectors (testId > aria > text)
   - ✅ Success rate >= 75% (3/4 bugs successfully converted to tests)

2. **Selector Healing:**
   - ✅ At least 4/6 broken selectors healed
   - ✅ High-confidence healings work when applied
   - ✅ Suggested selectors actually exist in DOM
   - ✅ No false positives (wrong element matched)

3. **Failure Classification:**
   - ✅ 3/3 failures classified correctly
   - ✅ Confidence >= 0.7 for correct classifications
   - ✅ Recommended actions are appropriate
   - ✅ Batch processing works for multiple failures

### Known Limitations to Document

After validation, document any issues found:

- Dynamic class names (CSS-in-JS) cannot be healed
- Network mocks may need manual review
- Classification ambiguity between timing_flaky and selector_broken
- DOM queries require extension connected

---

## Next Steps After Validation

1. ✅ Complete validation (all phases pass)
2. Create validation artifacts:
   - `examples/generated-test-before.spec.ts`
   - `examples/generated-test-after.spec.ts`
   - `examples/healed-selectors.md`
3. Document success rate in `VALIDATION_REPORT.md`
4. Update `LIMITATIONS.md` with any issues found
5. Consider adding integration wiring if missing:
   - Error ID assignment in `observe` tool
   - File writing for `auto_apply: true`
   - Real DOM queries (currently heuristic)

---

## Demo Script for User

### 2-minute demo showing test generation works:

```bash
# Terminal 1: Start demo
cd ~/dev/kaboom-demos && npm run demo

# Terminal 2: Start Kaboom
cd ~/dev/kaboom && ./dist/kaboom-mcp

# Browser: Open http://localhost:3000
# See 404 error in console

# Claude Code:
"Use Kaboom to observe the error, then generate a Playwright test from it"

# Result: Working test that reproduces the bug
# Time: < 2 minutes
# Proof: TestSprite parity achieved
```
