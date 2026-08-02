---
doc_type: feature_index
feature_id: feature-environment-manipulation
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-08-02
code_paths:
  - cmd/browser-agent/internal/cli/cli_tool_parsers_generate_configure.go
  - cmd/browser-agent/internal/playbooks/playbooks_guides.go
  - cmd/browser-agent/internal/toolconfigure/qafixture/handler.go
  - cmd/browser-agent/internal/toolinteract/action_owners.go
  - cmd/browser-agent/internal/toolinteract/interact_storage.go
  - cmd/browser-agent/tools_configure.go
  - internal/qafixture/wire_fixture.go
  - internal/qafixture/transaction.go
  - internal/qafixture/registry.go
  - internal/qafixture/registry_store.go
  - internal/schema/configure/properties_fixture.go
  - internal/tools/configure/capabilities/modespecs_configure.go
  - scripts/build/generate-wire-types.js
  - scripts/contracts/check-wire-drift.js
  - src/background/commands/helpers.ts
  - src/background/pending-queries.ts
  - src/background/environment-transaction/browser-state-driver.ts
  - src/background/environment-transaction/chrome-state-adapter.ts
  - src/background/environment-transaction/commands.ts
  - src/background/environment-transaction/runtime.ts
  - src/background/environment-transaction/snapshot-store.ts
  - src/types/runtime/queries.ts
  - src/types/wire/wire-qa-fixture.ts
test_paths:
  - cmd/browser-agent/golden_test.go
  - cmd/browser-agent/testdata/mcp-tools-list.golden.json
  - cmd/browser-agent/tools_configure_handler_test.go
  - cmd/browser-agent/internal/toolinteract/interact_storage_test.go
  - cmd/browser-agent/internal/toolconfigure/qafixture/handler_test.go
  - internal/qafixture/fixture_test.go
  - internal/qafixture/transaction_test.go
  - internal/qafixture/registry_test.go
  - internal/qafixture/registry_store_test.go
  - internal/schema/configure/schema_test.go
  - tests/extension/environment-transaction/browser-state-driver.test.js
  - tests/extension/environment-transaction/chrome-state-adapter.test.js
  - tests/extension/environment-transaction/snapshot-store.test.js
  - scripts/tests/browser/cat-35-qa-fixtures.sh
last_verified_version: 0.9.0
last_verified_date: 2026-08-02
---

# Environment Manipulation

## TL;DR

- Status: proposed
- Tool: See feature contract and `docs/core/mcp-command-option-matrix.md` for canonical tool enums.
- Mode/Action: See feature contract and `docs/core/mcp-command-option-matrix.md` for canonical `what`/`action`/`format` enums.
- Location: `docs/features/feature/environment-manipulation`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_ENVIRONMENT_MANIPULATION_001
- FEATURE_ENVIRONMENT_MANIPULATION_002
- FEATURE_ENVIRONMENT_MANIPULATION_003

## Code and Tests

- Storage and cookie mutation handlers share one canonical execution-target
  contract for tab, timeout, and JavaScript world selection.
- Characterization tests verify that every storage mutation preserves that
  contract through the queued extension command.
- `configure(what="qa_fixture", fixture_action="validate")` now validates the
  canonical version-1 QA environment document without mutating the browser or
  echoing cookies, storage, flags, or seed values. The contract bounds setup
  time and state size and rejects unknown fields and unsupported capabilities.
- `configure(what="qa_fixture", fixture_action="apply")` drives the canonical
  extension-owned snapshot before mutation and mandates rollback after any
  partial apply failure. Only opaque snapshot and correlation identifiers cross
  process boundaries; private fixture values never appear in responses.
- Snapshot, mutation, and recovery use one private
  `environment_transaction_*` daemon-extension protocol. QA-specific private
  command aliases and module paths are prohibited; the public capability is
  exposed only through `configure(what="qa_fixture")`.
- The daemon recovery registry persists only opaque transaction, snapshot,
  correlation, extension-generation, lifecycle, timestamp, and mutation-count
  fields. It is bounded, rejects stale generations, atomically replaces its
  state file, and quarantines corrupt state behind stable diagnostic codes.
- The fixture transaction coordinator generates its own correlation ID,
  captures an opaque private snapshot before the first mutation, and performs
  bounded rollback after any partial apply failure. Driver errors collapse to
  stable status codes, so raw cookies, storage, and seed values cannot enter
  MCP responses or Doctor diagnostics.
- The extension driver preflights unsupported locale, permission, network, and
  cross-origin page-state combinations before mutation. Supported navigation,
  viewport, cookies, storage, flags, and seed state are captured exactly,
  including absent keys, and restored through independent best-effort recovery
  steps. Private snapshots remain extension-owned; only an opaque snapshot ID
  crosses the daemon command boundary.
- Private snapshots use one bounded `chrome.storage.local` owner so recovery
  survives MV3 service-worker suspension. Corrupt or unavailable storage emits
  stable lifecycle diagnostics without including captured values, and runtime
  registration lives in a dedicated composition root.
- Connected category 35 applies synthetic state to the disposable UAT tab, then
  uses cross-origin snapshot rejection and a real invalid-domain cookie failure
  to prove redaction, exact partial-apply rollback, and explicit cleanup through
  the installed extension.
