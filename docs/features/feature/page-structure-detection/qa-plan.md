---
doc_type: qa-plan
feature_id: feature-page-structure-detection
status: proposed
last_reviewed: 2026-11-21
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Page Structure Detection QA Plan

## Automated Coverage
- `cmd/browser-agent/internal/toolanalyze/handlers_coverage_test.go` (`HandlePageStructure` dispatch contract)
- `internal/schema/invariants_test.go` (`page_structure` schema mode)

## Required Scenarios
1. Framework/global detection success path.
2. CSP-restricted fallback path with reduced confidence.
3. Routing mode classification for hash/history/static pages.
4. Response shape and required fields remain stable.
