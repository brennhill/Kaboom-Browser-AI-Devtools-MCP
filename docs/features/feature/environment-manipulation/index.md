---
doc_type: feature_index
feature_id: feature-environment-manipulation
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-11-21
code_paths:
  - internal/tools/cli/parser/generate_configure.go
  - internal/tools/playbooks/resources/guides.go
  - internal/tools/configure/handler/qafixture/handler.go
  - internal/tools/configure/handler/qafixture/startup.go
  - internal/tools/configure/handler/qafixture/transport/command.go
  - internal/tools/configure/handler/qafixture/shutdown/recovery.go
  - internal/tools/commandhealth/doctor_live_checks.go
  - internal/tools/interact/handler/action_owners.go
  - internal/tools/interact/handler/interact_page.go
  - internal/composition/toolruntime/tools_configure.go
  - internal/composition/toolruntime/tools_core.go
  - internal/testgen/qafixture/wire_fixture.go
  - internal/testgen/qafixture/transaction.go
  - internal/testgen/qafixture/registry.go
  - internal/testgen/qafixture/registry_store.go
  - internal/platform/state/file/statefile.go
  - internal/platform/state/file/directory_sync_unix.go
  - internal/platform/state/file/directory_sync_windows.go
  - internal/tools/schema/configure/properties_fixture.go
  - internal/lifecycle/doctor/statediag/collector.go
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
  - src/lib/storage/fault.ts
  - src/background/init.ts
  - src/types/runtime/queries.ts
  - src/types/wire/wire-qa-fixture.ts
test_paths:
  - internal/tools/schema/invariants_test.go
  - internal/tools/mcpserver/protocol/responses_test.go
  - cmd/browser-agent/testdata/mcp-tools-list.golden.json
  - internal/lifecycle/doctor/support/projections_test.go
  - internal/tools/interact/handler/interact_browser_test.go
  - internal/tools/configure/handler/qafixture/handler_test.go
  - internal/tools/configure/handler/qafixture/startup_test.go
  - internal/tools/configure/handler/qafixture/transport/command_test.go
  - internal/tools/configure/handler/qafixture/shutdown/recovery_test.go
  - internal/testgen/qafixture/fixture_test.go
  - internal/testgen/qafixture/transaction_test.go
  - internal/testgen/qafixture/registry_test.go
  - internal/testgen/qafixture/model_test.go
  - internal/testgen/qafixture/registry_store_test.go
  - internal/platform/state/file/statefile_test.go
  - internal/tools/schema/configure/schema_test.go
  - internal/lifecycle/doctor/statediag/collector_test.go
  - tests/extension/environment-transaction/browser-state-driver.test.js
  - tests/extension/environment-transaction/chrome-state-adapter.test.js
  - tests/extension/environment-transaction/snapshot-store.test.js
  - tests/extension/state-recovery/storage-fault-fixture.js
  - tests/extension/state-recovery/lifecycle-model.test.js
  - extension/background/pending-queries-iframe.test.js
  - scripts/tests/browser/cat-35-qa-fixtures.sh
last_verified_version: 0.9.0
last_verified_date: 2026-08-02
---

# Environment Manipulation

## TL;DR

- Status: proposed
- Tool: See feature contract and `docs/core/protocol/mcp-command-option-matrix.md` for canonical tool enums.
- Mode/Action: See feature contract and `docs/core/protocol/mcp-command-option-matrix.md` for canonical `what`/`action`/`format` enums.
- Location: `docs/features/feature/environment-manipulation`
- Startup fixture reconciliation uses the canonical panic-recovering goroutine
  boundary while installing its mutation barrier synchronously, so a recovery
  panic cannot crash the daemon or let fixture work overtake reconciliation.

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
  fields. It is bounded, rejects stale generations, and delegates replacement
  and quarantine durability to the canonical `internal/platform/state/file` owner. That
  owner uses same-directory temporary writes, file sync, atomic rename, and
  directory sync where supported, with stable redacted failure stages and
  deterministic cleanup coverage. Windows' unsupported directory-sync
  semantics are isolated explicitly.
- Deterministic model-based sequences replay 100 named seeds across add,
  restore, failure, and completion transitions. A recovery obligation can only
  disappear after the registry has recorded restoration in progress; stale or
  out-of-order completion attempts remain active and failing seeds identify the
  exact step for direct replay.
- Extension snapshot persistence emits structured, redacted fault notices with
  the canonical storage-fault kind. The runtime records local lifecycle detail
  and activates the `environment_snapshot_state` Doctor incident without
  including cookies, URLs, storage values, or seed data.
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
  registration lives in a dedicated composition root. All snapshot save,
  consume, lookup, and
  reconciliation operations share one serialized mutation boundary, preventing
  concurrent writes from overwriting durable recovery obligations. After the
  extension becomes ready, daemon startup reconciles the extension store with
  the daemon's durable registry using only opaque snapshot IDs. Referenced
  snapshots remain recoverable; snapshots orphaned before registry persistence
  are pruned, and Doctor retains a redacted failure/recovery transition without
  exposing identifiers or captured browser state. Handler composition installs
  this bounded startup-recovery barrier synchronously before publishing the
  configure surface, so an early fixture mutation cannot overtake reconciliation.
- Connected QA fixture UAT restores every successful transaction through its
  canonical opaque transaction ID, including when later verification fails;
  it never substitutes ad-hoc page rewrites for the real recovery path.
- Snapshot recovery distinguishes active, consumed, and unknown opaque handles.
  Successful restores atomically replace private snapshot data with bounded,
  value-free tombstones; unknown or corrupt handles fail closed so the daemon
  retains its recovery obligation instead of reporting false success.
- Each private snapshot includes its own extension-only restore plan. Recovery
  therefore sends only the opaque snapshot handle; the daemon neither persists
  nor retransmits original cookie or storage values during restoration.
- Successful apply persists a bounded recovery obligation before returning its
  opaque transaction handle. `status` exposes only redacted lifecycle metadata,
  while `restore` is idempotent. Daemon startup waits a bounded interval for the
  extension, resumes same-generation recovery, and records pending or failed
  transitions in Doctor without raw state.
- Doctor retains one correlated active-to-recovered timeline per fixture
  transaction. Startup, extension-unavailable, rollback, and explicit restore
  transitions use stable redacted status text; malformed persisted identifiers
  are quarantined before they can reach Doctor or MCP responses.
- Connected category 35 applies synthetic state to the disposable UAT tab, then
  uses cross-origin snapshot rejection and a real invalid-domain cookie failure
  to prove redaction, exact partial-apply rollback, and explicit cleanup through
  the installed extension.
