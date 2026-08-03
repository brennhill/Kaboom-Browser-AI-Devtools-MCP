---
doc_type: feature_index
feature_id: feature-verification-contracts
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-08-03
code_paths:
  - internal/verification/contract.go
  - internal/verification/evidence.go
  - internal/verification/store.go
  - internal/state/paths.go
  - cmd/browser-agent/internal/toolanalyze/verificationhandler/handler.go
  - cmd/browser-agent/internal/toolanalyze/analyzedispatch/dispatcher.go
  - internal/schema/analyze.go
  - internal/tools/configure/capabilities/modespecs_analyze.go
test_paths:
  - internal/verification/contract_test.go
  - internal/verification/evidence_test.go
  - internal/verification/store_test.go
  - internal/state/paths_test.go
  - cmd/browser-agent/internal/toolanalyze/verificationhandler/handler_test.go
---

# Verification Contracts

Kaboom turns acceptance criteria into a versioned, deterministic QA contract
through `analyze(what="verification")`. `operation="define"` validates the
contract; `operation="evaluate"` evaluates assertion results.

Schema version `1` requires a contract ID and one or more uniquely identified,
described assertions. Assertions may name required evidence kinds. Results use
only `PASS`, `FAIL`, `BLOCKED`, `UNVERIFIED`, or `FLAKY`. A missing assertion
result or missing required evidence is always `UNVERIFIED`; it can never be
reported as `PASS`.

Evidence submissions identify their assertion, originating Kaboom tool and
action, correlation ID, capture time, and compact content. Kaboom redacts that
content before producing a stable `sha256:` content address. The response
contains the redacted evidence catalog and binds its references to the evaluated
claims. Artifacts are atomically persisted with owner-only permissions under the
local Kaboom state directory and can be resolved by hash after daemon restart.
Catalog entries are re-hashed during evaluation; missing, modified, or stale
evidence changes the claim to `UNVERIFIED`. The default freshness window is 24
hours and may be set explicitly with `max_age_seconds`.

All evidence remains local. The artifact schema accepts provenance only from
the canonical five tools: `observe`, `generate`, `configure`, `interact`, and
`analyze`.
