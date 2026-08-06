# Kaboom MCP — Core Rules

Browser extension + MCP server for real-time browser telemetry.
**Stack:** Go (zero deps) | Chrome Extension (MV3) | MCP (JSON-RPC 2.0)

---

## 🔴 Mandatory Rules

1. **TDD** — Write tests first, then implementation
2. **No `any`** — TypeScript strict mode, no implicit any
3. **Zero Deps** — No production dependencies in Go or extension
4. **Compile TS** — Run `make compile-ts` after ANY src/ change
5. **5 Tools** — observe, generate, configure, interact, analyze
6. **Performance** — WebSocket < 0.1ms, HTTP < 0.5ms
7. **Privacy** — Captured browser/user data stays local. The only external transmission is anonymous product-usage telemetry: random install/session identifiers, version/platform, command identifiers, outcomes, timing, and aggregate counts used to measure installs and product-command usage. Never transmit URLs, prompts, page content, file contents, captured logs/network data, credentials, or personal data. Users can disable it with `KABOOM_TELEMETRY=off`.
8. **Wire Types** — `wire_*.go` and `wire-*.ts` are the source of truth for HTTP payloads. Changes to either side MUST update the counterpart. Run `make check-wire-drift`
9. **Docs Cross-Ref (Required)** — EVERY feature and EVERY refactor MUST ship with cross-referenced docs updates (feature `index.md` `code_paths`/`test_paths` + `last_reviewed`)

## Git Workflow

- Branch from `UNSTABLE`, PR to `UNSTABLE`
- Never push directly to `stable`
- Squash commits before merge

---

## Commands

```bash
make compile-ts    # REQUIRED after src/ changes
make test          # All tests
make ci-local      # Full CI locally
npm run typecheck  # TypeScript check
npm run lint       # ESLint
```

## Testing

**Primary UAT Script:** [`scripts/test-all-tools-comprehensive.sh`](scripts/test-all-tools-comprehensive.sh)

Tests: cold start, tool calls, concurrent clients, stdout purity, persistence, graceful shutdown.

```bash
./scripts/test-all-tools-comprehensive.sh  # Run full UAT
```

**UAT Rules:**

- **NEVER modify tests during UAT** — run tests as-is, report results
- If tests have issues, note them and propose changes AFTER UAT completes
- UAT validates the npm-installed version (`kaboom-mcp` from PATH)
- Extension must be connected for data flow tests to pass

## Code Standards

**JSON API fields:** ALL JSON fields use `snake_case`. No exceptions. External spec fields (MCP protocol, SARIF) are tagged with `// SPEC:<name>` comments.

**TypeScript:**

- No dynamic imports in service worker (background/)
- No circular dependencies
- Content scripts must be bundled (MV3 limitation)
- All fetch() needs try/catch + response.ok check

**Go:**

- Append-only I/O on hot paths
- Single-pass eviction (never loop-remove-recheck)
- File headers required: `// filename.go — Purpose summary.`

**File size:** Max 800 LOC. Refactor if larger.

## Documentation Cross-Reference Contract (Required)

For every feature and every refactor, update the feature `index.md` under
`docs/features/feature/<feature>/` in the same change:

- `last_reviewed`
- `code_paths` and `test_paths`

No code-only refactor is considered complete until this documentation contract is
satisfied. (Flow maps are no longer required — do not create or update them.)

## Engineering Best Practices Contract (Required)

1. Instruction precedence is strict: system > repo policy > task request > style preference.
2. If requirements are ambiguous, state assumptions explicitly before implementation.
3. Definition of done includes code + tests + docs in the same change.
4. Lint/type/test must pass, or known failures must be documented with issue links.
5. Keep modules single-purpose; avoid god objects and hidden shared state.
6. Keep public interfaces minimal and explicit; cross-feature calls go through clear boundaries.
7. Refactors must preserve behavior unless a behavior change is explicitly requested.
8. Every bug fix must include a regression test that fails before and passes after.
9. Prefer deterministic tests (mocks/fakes/controlled clocks) over sleep-based timing.
10. Enforce startup and request latency budgets with explicit timeout/retry/backoff policies.
11. Use structured logs with correlation IDs; avoid protocol-breaking stdout/stderr noise.
12. Version public contracts and keep wire schemas synchronized across Go/TS boundaries.
13. Redact secrets from logs/errors/diagnostics and never commit credentials.
14. New dependencies require explicit justification; remove unused dependencies promptly.
15. Reviews and handoffs must cover correctness, modularity, performance, testability, docs quality, and DRY adherence.
16. CI must block merges on broken docs links, missing required docs, or failing quality gates.
17. ToolHandler naming convention is strict: `tool*` for top-level MCP mode/action entry points, `handle*` for sub-action handlers/helpers.
18. Shared extension storage keys (`TRACKED_TAB_*`, recording state, pending intents) must be accessed through feature helpers/modules; avoid new ad-hoc read/write/remove call sites.
19. Multi-entry-point actions (keyboard, context menu, popup, MCP) must use one shared toggle/start-stop helper so behavior stays identical.
20. Cross-context message contracts must be declared in `src/types/runtime-messages.ts` (and corresponding wire/schema files when applicable) before adding new runtime message types.
21. User-facing recording labels/toasts/badge text must come from shared helpers to keep wording and truncation consistent across entry points.
22. Duplicate code checks are required for refactors touching `src/background` or `src/popup` (`npx jscpd src/background src/popup --min-lines 8 --min-tokens 60`), and each non-trivial clone must be either extracted or documented as intentional.
23. Behavior-replacing refactors must update or delete obsolete tests in the same change (for example, replacing watermark behavior with badge behavior).
24. See `docs/core/reliability/common-patterns.md` for the canonical patterns and review checklist.
25. **Fail loud on state-mutating paths.** No operation that changes state may fail silently. `catch {}` is banned (ESLint `no-empty` with `allowEmptyCatch: false`) — every catch either handles the error or documents why swallowing is safe. A genuine failure must not be masked as a recoverable/expected state; distinguish expected conflicts from actual failures and surface the latter.
26. **Compatibility facades are prohibited.** Migrations are atomic: move every caller to the canonical API and delete the obsolete aliases, wrappers, shims, tests, and documentation in the same change. If the migration cannot be completed end-to-end, do not begin it. Never preserve an old internal surface merely to reduce migration effort.
27. **Never silently discard failures.** Every unexpected catch/rejection must emit a redacted structured log or Doctor diagnostic. If an expected absence or cancellation intentionally produces no log, the catch must contain an adjacent `EXPECTED_ABSENCE:` comment explaining why the condition is normal and why logging it would be misleading.

## Finding Things

| Need                  | Location                                         |
| --------------------- | ------------------------------------------------ |
| Feature specs         | `docs/features/<name>/`                          |
| Test plans            | `docs/features/<name>/{name}-test-plan.md`       |
| Test plan template    | `docs/features/_template/template-test-plan.md`  |
| Architecture          | `.claude/refs/architecture.md`                   |
| Known issues          | `docs/core/quality/known-issues.md`                      |
| All features          | `docs/features/feature-navigation.md`            |


## Pre-Commit Checklist

Before presenting code as complete:

- Grep for existing patterns before introducing new ones (http.Client, handler maps, error format)
- No duplicated types/constants across packages — export from source of truth
- 3+ similar functions → extract helper before continuing
- Data structs must not do I/O — keep I/O at the call site
