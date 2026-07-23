---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-02-16
---

# Core Documentation

This folder contains all cross-product and foundational documentation for Kaboom Agentic Browser Devtool MCP, including:
- Core product specifications
- Technical specifications that apply to the entire system
- API specifications (OpenAPI/Swagger)

For Architecture Decision Records, see [`/docs/adrs/`](../adrs/).

## In-Progress

The [`in-progress/`](in-progress/) folder holds work-in-progress specs, tracking documents, and issue trackers that are actively being developed. Once finalized, items are moved to their permanent location in `docs/features/` or `docs/core/`, or archived under `docs/archive/`.

## API Specifications

- [async-command-api.yaml](async-command-api.yaml) - OpenAPI 3.0 spec for async command execution between MCP server and browser extension (v6.0.0)

## Engineering Working Rules

- [common-patterns.md](common-patterns.md) - Required implementation/review patterns for shared state, multi-entry-point flows, runtime message contracts, duplication checks, and e2e data passing.
- [testing-guidelines.md](testing-guidelines.md) - Determinism rules for tests: poll don't sleep, warm before measuring, no network

For feature-specific documentation, see `/docs/features/feature/`.
