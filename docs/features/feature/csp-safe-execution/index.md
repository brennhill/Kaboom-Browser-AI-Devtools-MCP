---
doc_type: feature_index
feature_id: feature-csp-safe-execution
status: implemented
feature_type: feature
owners: []
last_reviewed: 2026-08-05
code_paths:
  - src/background/exec/csp-safe/types.ts
  - src/background/exec/csp-safe/parser.ts
  - src/background/exec/csp-safe/executor.ts
  - src/background/exec/query-execution.ts
  - src/inject/execute-js.ts
  - cmd/browser-agent/internal/insecureproxy/handler.go
  - cmd/browser-agent/internal/toolguard/guards.go
test_paths:
  - extension/background/exec/__tests__/query-execution-serialization.test.js
  - tests/extension/injection/execute-js.test.js
  - cmd/browser-agent/internal/insecureproxy/handler_test.go
  - cmd/browser-agent/tools_interact_gate_test.go
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

- Types: `src/background/exec/csp-safe/types.ts`
- Parser: `src/background/exec/csp-safe/parser.ts`
- Executor: `src/background/exec/csp-safe/executor.ts`
- Integration: `src/background/exec/query-execution.ts`
- Tests: `extension/background/__tests__/query-execution-serialization.test.js`

## Serialization Contract

- `execute_js` must return plain JSON-compatible values.
- Host objects with prototype getters (for example DOMRect-like values) are serialized via `toJSON()` when available, then prototype getter introspection fallback.
- This prevents empty `{}` payloads for geometry/style-like values returned from page context.

## Insecure Proxy Response Contract

- The opt-in local proxy stages each upstream response before committing its status or headers.
- Declared and streamed bodies above the fixed 50 MiB ceiling fail with HTTP 502; Kaboom never returns a successful truncated response or partial upstream body.
- CSP headers are stripped only after the response passes the size and read checks.
- The outbound request is restricted to parsed HTTP(S) URLs and always crosses
  the DNS-pinning SSRF-safe transport, including redirects. Security-analyzer
  annotations name that enforced boundary rather than suppressing it globally.
