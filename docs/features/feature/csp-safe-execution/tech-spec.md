---
doc_type: tech-spec
feature_id: feature-csp-safe-execution
status: implemented
last_reviewed: 2026-07-25
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# CSP-Safe Execution Tech Spec

## Pipeline
- Parser/types: `src/background/exec/csp-safe/parser.ts`, `src/background/exec/csp-safe/types.ts`
- Structured executor: `src/background/exec/csp-safe/executor.ts`
- World routing + fallback: `src/background/exec/query-execution.ts`
- Result serialization hardening: `src/inject/execute-js.ts`

## Strategy
1. MAIN world eval path.
2. ISOLATED world eval path.
3. CSP-safe structured evaluator.

## Contract
- Return JSON-safe serialized values.
- Preserve `execution_mode` to expose selected path.
- Never leak host-object stubs (e.g., DOMRect `{}`) when serializable fields exist.
