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
7. **Privacy** — All data stays local, no external transmission
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

## Pre-Commit Checklist

Before presenting code as complete:

- Grep for existing patterns before introducing new ones (http.Client, handler maps, error format)
- No duplicated types/constants across packages — export from source of truth
- 3+ similar functions → extract helper before continuing
- Data structs must not do I/O — keep I/O at the call site

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
17. ToolHandler naming convention is strict: `tool*` for top-level MCP mode/action entry points (e.g. `toolObserve`, `toolConfigureClear`), `handle*` for sub-action dispatch handlers on subsidiary handler types (e.g. `handleRecordStart`, `handleUpload`). Unprefixed methods are internal utilities (e.g. `drainAlerts`, `loadSummaryPref`, `buildPlaybackResult`).
18. Shared extension storage keys (`TRACKED_TAB_*`, recording state, pending intents) must be accessed through feature helpers/modules; avoid new ad-hoc read/write/remove call sites. **Authoritative source:** each shared key has exactly one authoritative writer, and any reader that *gates behavior* on a mirrored key must reconcile it against the live authoritative signal (a runtime port, an in-memory flag, an API) — a storage mirror goes stale when its writer's context dies without flushing (e.g. a Chrome-native side-panel close leaving `TERMINAL_UI_STATE='open'` and suppressing the flame forever). When you migrate a decision onto an authoritative signal, migrate **every** reader of the old mirror in the same change.
19. Multi-entry-point actions (keyboard, context menu, popup, MCP) must use one shared toggle/start-stop helper so behavior stays identical. **The invariant lives in the shared helper, not in each caller:** a function with a `forceFresh`/reset/rebuild flag must perform the required teardown (unmount/stop/clear) itself so no call site can forget it — the "terminal won't start after picking a folder" bug was one caller (`applyRootFolder`) omitting the `unmountPanel()` that another caller (`restoreTerminalPanel`) remembered.
20. Cross-context message contracts must be declared in `src/types/runtime-messages.ts` (and corresponding wire/schema files when applicable) before adding new runtime message types.
21. User-facing recording labels/toasts/badge text must come from shared helpers to keep wording and truncation consistent across entry points.
22. Duplicate code checks are required for refactors touching `src/background` or `src/popup` (`npx jscpd src/background src/popup --min-lines 8 --min-tokens 60`), and each non-trivial clone must be either extracted or documented as intentional.
23. Behavior-replacing refactors must update or delete obsolete tests in the same change (for example, replacing watermark behavior with badge behavior).
24. See `docs/core/common-patterns.md` for the canonical patterns and review checklist.
25. **Fail loud on state-mutating paths.** No operation that changes state may fail silently. `catch {}` is banned (ESLint `no-empty` with `allowEmptyCatch: false`) — every catch either handles the error or documents why swallowing is safe. A genuine failure must not be masked as a recoverable/expected state (the terminal Start bug returned HTTP 409-with-token — "session already exists" — for *any* spawn/cwd failure, so real errors reconnected silently); distinguish "already-exists/expected" from "actually failed" and surface the latter. Verify mutations landed (`make sync-version` silently no-oped on an unescaped-`@` perl until `validate-versions.sh` caught it — prefer a post-condition check over trusting a string transform).

## Finding Things

| Need                  | Location                                         |
| --------------------- | ------------------------------------------------ |
| Feature specs         | `docs/features/<name>/`                          |
| Test plans            | `docs/features/<name>/{name}-test-plan.md`       |
| Test plan template    | `docs/features/_template/template-test-plan.md`  |
| Architecture          | `.claude/refs/architecture.md`                   |
| Known issues          | `docs/core/known-issues.md`                      |
| All features          | `docs/features/feature-navigation.md`            |

<!-- gitnexus:start -->
# GitNexus — Code Intelligence

This project is indexed by GitNexus as **kaboom** (40314 symbols, 99330 relationships, 300 execution flows). Use the GitNexus MCP tools to understand code, assess impact, and navigate safely.

> If any GitNexus tool warns the index is stale, run `npx gitnexus analyze` in terminal first.

## Always Do

- **MUST run impact analysis before editing any symbol.** Before modifying a function, class, or method, run `gitnexus_impact({target: "symbolName", direction: "upstream"})` and report the blast radius (direct callers, affected processes, risk level) to the user.
- **MUST run `gitnexus_detect_changes()` before committing** to verify your changes only affect expected symbols and execution flows.
- **MUST warn the user** if impact analysis returns HIGH or CRITICAL risk before proceeding with edits.
- When exploring unfamiliar code, use `gitnexus_query({query: "concept"})` to find execution flows instead of grepping. It returns process-grouped results ranked by relevance.
- When you need full context on a specific symbol — callers, callees, which execution flows it participates in — use `gitnexus_context({name: "symbolName"})`.

## When Debugging

1. `gitnexus_query({query: "<error or symptom>"})` — find execution flows related to the issue
2. `gitnexus_context({name: "<suspect function>"})` — see all callers, callees, and process participation
3. `READ gitnexus://repo/kaboom/process/{processName}` — trace the full execution flow step by step
4. For regressions: `gitnexus_detect_changes({scope: "compare", base_ref: "main"})` — see what your branch changed

## When Refactoring

- **Renaming**: MUST use `gitnexus_rename({symbol_name: "old", new_name: "new", dry_run: true})` first. Review the preview — graph edits are safe, text_search edits need manual review. Then run with `dry_run: false`.
- **Extracting/Splitting**: MUST run `gitnexus_context({name: "target"})` to see all incoming/outgoing refs, then `gitnexus_impact({target: "target", direction: "upstream"})` to find all external callers before moving code.
- After any refactor: run `gitnexus_detect_changes({scope: "all"})` to verify only expected files changed.

## Never Do

- NEVER edit a function, class, or method without first running `gitnexus_impact` on it.
- NEVER ignore HIGH or CRITICAL risk warnings from impact analysis.
- NEVER rename symbols with find-and-replace — use `gitnexus_rename` which understands the call graph.
- NEVER commit changes without running `gitnexus_detect_changes()` to check affected scope.

## Tools Quick Reference

| Tool | When to use | Command |
|------|-------------|---------|
| `query` | Find code by concept | `gitnexus_query({query: "auth validation"})` |
| `context` | 360-degree view of one symbol | `gitnexus_context({name: "validateUser"})` |
| `impact` | Blast radius before editing | `gitnexus_impact({target: "X", direction: "upstream"})` |
| `detect_changes` | Pre-commit scope check | `gitnexus_detect_changes({scope: "staged"})` |
| `rename` | Safe multi-file rename | `gitnexus_rename({symbol_name: "old", new_name: "new", dry_run: true})` |
| `cypher` | Custom graph queries | `gitnexus_cypher({query: "MATCH ..."})` |

## Impact Risk Levels

| Depth | Meaning | Action |
|-------|---------|--------|
| d=1 | WILL BREAK — direct callers/importers | MUST update these |
| d=2 | LIKELY AFFECTED — indirect deps | Should test |
| d=3 | MAY NEED TESTING — transitive | Test if critical path |

## Resources

| Resource | Use for |
|----------|---------|
| `gitnexus://repo/kaboom/context` | Codebase overview, check index freshness |
| `gitnexus://repo/kaboom/clusters` | All functional areas |
| `gitnexus://repo/kaboom/processes` | All execution flows |
| `gitnexus://repo/kaboom/process/{name}` | Step-by-step execution trace |

## Self-Check Before Finishing

Before completing any code modification task, verify:
1. `gitnexus_impact` was run for all modified symbols
2. No HIGH/CRITICAL risk warnings were ignored
3. `gitnexus_detect_changes()` confirms changes match expected scope
4. All d=1 (WILL BREAK) dependents were updated

## Keeping the Index Fresh

After committing code changes, the GitNexus index becomes stale. Re-run analyze to update it:

```bash
npx gitnexus analyze
```

If the index previously included embeddings, preserve them by adding `--embeddings`:

```bash
npx gitnexus analyze --embeddings
```

To check whether embeddings exist, inspect `.gitnexus/meta.json` — the `stats.embeddings` field shows the count (0 means no embeddings). **Running analyze without `--embeddings` will delete any previously generated embeddings.**

> Claude Code users: A PostToolUse hook handles this automatically after `git commit` and `git merge`.

## CLI

| Task | Read this skill file |
|------|---------------------|
| Understand architecture / "How does X work?" | `.claude/skills/gitnexus/gitnexus-exploring/SKILL.md` |
| Blast radius / "What breaks if I change X?" | `.claude/skills/gitnexus/gitnexus-impact-analysis/SKILL.md` |
| Trace bugs / "Why is X failing?" | `.claude/skills/gitnexus/gitnexus-debugging/SKILL.md` |
| Rename / extract / split / refactor | `.claude/skills/gitnexus/gitnexus-refactoring/SKILL.md` |
| Tools, resources, schema reference | `.claude/skills/gitnexus/gitnexus-guide/SKILL.md` |
| Index, status, clean, wiki CLI commands | `.claude/skills/gitnexus/gitnexus-cli/SKILL.md` |

<!-- gitnexus:end -->


<!-- BEGIN BEADS INTEGRATION v:1 profile:minimal hash:6cd5cc61 -->
## Beads Issue Tracker

This project uses **bd (beads)** for issue tracking. Run `bd prime` to see full workflow context and commands.

### Quick Reference

```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --claim  # Claim work
bd close <id>         # Complete work
```

### Rules

- Use `bd` for ALL task tracking — do NOT use TodoWrite, TaskCreate, or markdown TODO lists
- Run `bd prime` for detailed command reference and session close protocol
- Use `bd remember` for persistent knowledge — do NOT use MEMORY.md files

**Architecture in one line:** issues live in a local Dolt DB; sync uses `refs/dolt/data` on your git remote; `.beads/issues.jsonl` is a passive export. See https://github.com/gastownhall/beads/blob/main/docs/SYNC_CONCEPTS.md for details and anti-patterns.

## Agent Context Profiles

The managed Beads block is task-tracking guidance, not permission to override repository, user, or orchestrator instructions.

- **Conservative (default)**: Use `bd` for task tracking. Do not run git commits, git pushes, or Dolt remote sync unless explicitly asked. At handoff, report changed files, validation, and suggested next commands.
- **Minimal**: Keep tool instruction files as pointers to `bd prime`; use the same conservative git policy unless active instructions say otherwise.
- **Team-maintainer**: Only when the repository explicitly opts in, agents may close beads, run quality gates, commit, and push as part of session close. A current "do not commit" or "do not push" instruction still wins.

## Session Completion

This protocol applies when ending a Beads implementation workflow. It is subordinate to explicit user, repository, and orchestrator instructions.

1. **File issues for remaining work** - Create beads for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **Handle git/sync by active profile**:
   ```bash
   # Conservative/minimal/default: report status and proposed commands; wait for approval.
   git status

   # Team-maintainer opt-in only, unless current instructions forbid it:
   git pull --rebase
   git push
   git status
   ```
5. **Hand off** - Summarize changes, validation, issue status, and any blocked sync/commit/push step

**Critical rules:**
- Explicit user or orchestrator instructions override this Beads block.
- Do not commit or push without clear authority from the active profile or the current user request.
- If a required sync or push is blocked, stop and report the exact command and error.
<!-- END BEADS INTEGRATION -->
