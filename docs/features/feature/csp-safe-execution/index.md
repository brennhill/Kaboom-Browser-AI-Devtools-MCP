---
doc_type: feature_index
feature_id: feature-csp-safe-execution
status: implemented
feature_type: feature
owners: []
last_reviewed: 2026-07-25
code_paths:
  - src/background/csp-safe/types.ts
  - src/background/csp-safe/parser.ts
  - src/background/csp-safe/executor.ts
  - src/background/query-execution.ts
  - src/inject/execute-js.ts
test_paths:
  - extension/background/__tests__/query-execution-serialization.test.js
  - tests/extension/execute-js.test.js
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# CSP-Safe JavaScript Execution

## TL;DR

- Status: implemented
- Tool: interact
- Mode/Action: execute_js
- Location: `docs/features/feature/csp-safe-execution`

## Problem

When a page's Content Security Policy (CSP) blocks `unsafe-eval`, `execute_js` fails because both execution paths use `new Function(code)`. This affects sites like LinkedIn, GitHub, and many enterprise apps.

## Solution: Three-Tier Fallback Chain

| Tier | World | JS Capability | Page Globals | CSP Safe |
|------|-------|--------------|--------------|----------|
| 1. new Function (MAIN) | MAIN | 100% | Yes | No |
| 2. new Function (ISOLATED) | ISOLATED | 100% | No | Yes |
| 3. Structured executor | MAIN | ~85% expressions | Yes | Yes |

Tier 2 is the big win: content scripts in ISOLATED world are exempt from page CSP, so `new Function()` works there. Tier 3 handles the rare case where MAIN world page globals are needed on CSP pages.

## Code and Tests

- Types: `src/background/csp-safe/types.ts`
- Parser: `src/background/csp-safe/parser.ts`
- Executor: `src/background/csp-safe/executor.ts`
- Integration: `src/background/query-execution.ts`
- Tests: `extension/background/__tests__/query-execution-serialization.test.js`

## Serialization Contract

- `execute_js` must return plain JSON-compatible values.
- Host objects with prototype getters (for example DOMRect-like values) are serialized via `toJSON()` when available, then prototype getter introspection fallback.
- This prevents empty `{}` payloads for geometry/style-like values returned from page context.
