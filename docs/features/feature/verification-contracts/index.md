---
doc_type: feature_index
feature_id: feature-verification-contracts
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-08-03
code_paths:
  - internal/verification/contract.go
  - cmd/browser-agent/internal/toolanalyze/verificationhandler/handler.go
  - cmd/browser-agent/internal/toolanalyze/analyzedispatch/dispatcher.go
  - internal/schema/analyze.go
  - internal/tools/configure/capabilities/modespecs_analyze.go
test_paths:
  - internal/verification/contract_test.go
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

This feature defines the QA decision contract. Durable, redacted,
content-addressable evidence references are owned by the evidence feature and
are validated before evaluation.
