---
feature: test-generation
type: quick-reference
status: reference
last_reviewed: 2026-03-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Test Generation — Quick Reference

## 30-Second Overview

```bash
# What: Generate Playwright tests from captured browser errors
# Why: Achieve TestSprite parity + WebSocket advantage
# Status: Complete, 77 tests passing, ready for validation
```

---

## Try It Now (5 minutes)

### 1. Start Demo Site

```bash
cd ~/dev/kaboom-demos
npm run demo
# Opens http://localhost:3000 with 34 intentional bugs
```

### 2. Trigger WebSocket Bug

1. Open <http://localhost:3000>
2. Click "Chat" button
3. See WebSocket connection error

### 3. Generate Test

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "generate",
    "arguments": {
      "format": "test_from_context",
      "context": "error",
      "framework": "playwright"
    }
  }
}
```

**Result:** Working Playwright test that reproduces the bug

---

## All 7 Modes

### Test Generation

```bash
# 1. Generate from error
{ "format": "test_from_context", "context": "error" }

# 2. Generate from interaction
{ "format": "test_from_context", "context": "interaction" }

# 3. Generate regression test
{ "format": "test_from_context", "context": "regression" }
```

### Self-Healing

```bash
# 4. Analyze test file
{ "format": "test_heal", "action": "analyze", "test_file": "tests/foo.spec.ts" }

# 5. Repair selectors
{ "format": "test_heal", "action": "repair", "broken_selectors": ["#old-btn"] }

# 6. Batch heal directory
{ "format": "test_heal", "action": "batch", "test_dir": "tests/" }
```

### Classification

```bash
# 7. Classify failure
{ "format": "test_classify", "action": "failure", "failure": {...} }

# 8. Batch classify
{ "format": "test_classify", "action": "batch", "failures": [...] }
```

---

## Key Files

- **[wake-up.md](wake-up.md)** — Start here
- **[validation-guide.md](validation-guide.md)** — Hands-on validation
- **[STATUS.md](STATUS.md)** — Detailed status
- **[competitive-advantage.md](competitive-advantage.md)** — Why WebSocket matters

---

## Test Coverage

```bash
# Run all tests
go test -short ./cmd/browser-agent/

# Expected: 77 tests, all passing, ~2.9 seconds
```

---

## What Makes This Special

### TestSprite can't test WebSocket apps.

Kaboom captures WebSocket frames in real-time and generates tests with frame-level assertions.

**Demo:** Chat widget bugs (connection error + message parsing)

---

## Next Step

Follow [validation-guide.md](validation-guide.md) Phase 1 (30 min) to prove it works.
