---
doc_type: feature_index
feature_id: feature-lighthouse-report
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-06-29
code_paths:
  - cmd/browser-agent/tools_analyze_dispatch.go
  - cmd/browser-agent/tools_analyze_audit.go
  - cmd/browser-agent/tools_analyze_audit_runner.go
  - internal/tools/configure/mode_specs_analyze.go
  - src/background/cdp-dispatch.ts
  - src/background/commands/analyze.ts
test_paths: []
last_verified_version: 0.8.4
last_verified_date: 2026-06-29
---

# Lighthouse Report

## TL;DR

- Status: proposed
- Tool: analyze
- Mode/Action: lighthouse_report
- Location: `docs/features/feature/lighthouse-report`

## Overview

Lighthouse Report adds `analyze({what: "lighthouse_report"})`, a mode that runs a real
Google Lighthouse audit against the tracked tab through the Chrome DevTools Protocol (CDP)
and returns a trimmed, token-efficient subset of the result: category scores, Core Web
Vitals, the highest-value improvement opportunities, and diagnostics.

This differs from the existing `analyze({what: "audit"})` mode, which produces a fast,
heuristic "Lighthouse-style" score from passively captured telemetry. The Lighthouse Report
mode runs the authoritative synthetic benchmark that developers, continuous integration (CI)
pipelines, and stakeholders reference before shipping.

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_LIGHTHOUSE_REPORT_001
- FEATURE_LIGHTHOUSE_REPORT_002
- FEATURE_LIGHTHOUSE_REPORT_003

## Related Code

- Analyze dispatch registry: `cmd/browser-agent/tools_analyze_dispatch.go`
- Existing heuristic audit (contrast): `cmd/browser-agent/tools_analyze_audit.go`
- Mode hints and parameter specs: `internal/tools/configure/mode_specs_analyze.go`
- CDP attach/detach lifecycle: `src/background/cdp-dispatch.ts`

## Code and Tests

Add concrete implementation and test links here as this feature evolves.
