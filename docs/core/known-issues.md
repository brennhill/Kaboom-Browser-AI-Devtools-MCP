---
status: active
scope: issues/blockers
ai-priority: high
tags: [known-issues, v0.8.x]
last-verified: 2026-07-27
canonical: true
---

# Known Issues

Every entry below was re-verified against the working tree on the date in the
frontmatter, and carries the command that reproduces it. If you cannot reproduce
one, treat it as fixed and delete it rather than leaving it to rot.

## Test infrastructure

### 1. 30 JavaScript test files are executed by no CI job — HIGH

The only JS test job is `npm run test:ext`, which runs
`scripts/test-js-sharded.sh`. That script collects exactly:

```bash
rg --files tests/extension extension/background -g '*.test.js'
```

Two consequences, both live:

- **Whole directories are invisible to CI:** `tests/cli` (17 files),
  `tests/docs` (5), `tests/packaging` (3), `tests/site` (2).
- **The glob is `*.test.js` only**, so `.test.cjs` and `.test.mjs` are skipped
  *even inside the directories it does scan*.

```bash
# reproduce the count
find tests scripts -name '*.test.*' \
  | grep -vE '^tests/extension/.*\.test\.js$|^extension/background/.*\.test\.js$' \
  | grep -vE 'scripts/release/(install-upgrade-regression\.contract|verify-platform-binaries)\.test\.mjs' \
  | wc -l   # -> 30
```

Two of those suites are **currently red**, and were red before the July 2026
refactor series — nothing regressed them, nothing was ever watching:

| Suite | Result |
| --- | --- |
| `tests/extension/integration.test.cjs` | 8 pass, **5 fail** |
| `tests/site/gokaboom-domain-contract.test.js` | 21 pass, **2 fail** |

`integration.test.cjs` is the sharpest case: it sits in a directory CI does scan
and is skipped purely because of its file extension.

**Fix direction:** widen the glob to `*.test.{js,cjs,mjs}` and add the missing
directories, then fix or delete whatever turns red. Expect the widening to
surface more than the two failures above.

### 2. `scripts/check-dormant-tests.sh` never runs in CI — MEDIUM

The gate written to catch dormant tests is itself dormant.

```bash
rg -n 'check-dormant-tests' .github/   # -> no matches
```

It is wired into `make check-structure`, but the CI "Structure Gates" job calls
`scripts/check-file-length.sh` and `scripts/check-folder-size.cjs` **directly**
rather than going through the make target, so the third check is silently
skipped. It is also Go-only, and the dormancy problem is worse on the JS side
(issue 1).

**Fix direction:** have the CI job call `make check-structure` so newly added
gates are picked up by default, and extend the script to cover JS.

### 3. `cmd/browser-agent` tests run close to their timeout — MEDIUM

The package takes ~206s of a 600s per-package limit on a two-core CI runner, and
many of its tests spawn real daemon subprocesses against 1-second budgets. It is
therefore sensitive to *any* CPU contention elsewhere in the same
`go test ./cmd/browser-agent/... ./internal/...` invocation.

This is not theoretical: one new unit test in `internal/` that burned ~10s of CPU
under `-race` was enough to push this package past 600s and fail 21 unrelated
tests (PR #659, commit `173e8c2c`). It presented as a broad daemon/MCP breakage
with no visible connection to the change.

**If you see a wall of `cmd/browser-agent` failures at ~30s each ending in
`panic: test timed out`, suspect CPU cost you just added elsewhere, not the
daemon.** Confirm the same tests pass in isolation and on UNSTABLE before
investigating the daemon itself.

### 4. Flaky tests (pre-existing)

- `TestAsyncQueueReliability/Slow_polling` — intermittent 30s timeout
- `tests/extension/async-timeout.test.js` — 3 tests flaky
- `TestFastStart_ClientCompatibilityMatrix/claude_code` — fails only under
  full-suite load; passes in isolation. Local runs are additionally sensitive to
  anything already holding port 7891.

## Test coverage gaps

### 5. Per-package coverage numbers under-report cross-package tests — INFO

Go measures coverage per package. When a function's tests live in a *different*
package — very common here, because `cmd/browser-agent` tests exercise
`internal/tools/...` through the MCP dispatch — the function reads 0.0% while
being fully exercised.

`observe.AnalyzeErrors` reads **0.0%** in a plain `go test ./internal/...` run
and **100%** when measured correctly:

```bash
go test ./cmd/browser-agent/ -run TestToolAnalyzeErrors \
  -coverpkg=github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe \
  -coverprofile=/tmp/cov.out && go tool cover -func=/tmp/cov.out | grep AnalyzeErrors
```

**Never conclude "this is untested" from a per-package number alone.** This trap
produced a false claim in a feature doc during the July 2026 audit. Untested
*behaviour* and uncounted *coverage* are different problems with different fixes.

## Architecture

### 6. God objects remain, despite the folder counts — INFO

The July 2026 refactor series reduced *per-folder file counts* and satisfied the
ratcheting folder gate, but it did not decompose the two large types. Do not read
the folder-gate numbers as evidence that it did.

| | Current |
| --- | --- |
| `Capture` methods behind one `sync.RWMutex` | 165 |
| `cmd/browser-agent` production source files (package `main`) | 39 |
| …of which declare `*ToolHandler` methods | 20 |

Both remain structurally constrained: Go only permits methods on a type in the
package that declares it. Extracted `tool*` packages therefore expose handlers
through narrow `Deps` contracts while the remaining `*ToolHandler` shims stay
in `main`. The active refactor is also consolidating shims that form one MCP
boundary; the 10-file target is not yet met.

Note also that `src/lib` and `src/background` were **relocated into
subdirectories, not decomposed** — total file count and LOC are essentially
unchanged. The folder gate counts files per directory, so nesting satisfies it.

## Release / tooling

### 7. Repo token lacks `workflow` scope — MEDIUM

Any PR touching `.github/workflows/` cannot be merged, or have its branch
updated, via `gh`:

```
GraphQL: refusing to allow an OAuth App to create or update workflow
`.github/workflows/ci.yml` without `workflow` scope (updatePullRequestBranch)
```

Such PRs must go through the GitHub web UI.

### 8. PR #591 would revert the repository if merged — HIGH

The open dependabot PR (`@playwright/test` 1.59.1 → 1.62.0) is **115 commits
behind UNSTABLE**. Its real payload is one line in `tests/e2e/package.json`, but
merging it would touch **1,749 files and revert 26,828 lines**, including pinned
action SHAs in `ci.yml` and the entire July refactor series.

```bash
git fetch origin 'refs/pull/591/head:pr591'
git diff --stat origin/UNSTABLE..pr591 | tail -1
```

GitHub reports `mergeable=MERGEABLE`, which means *no conflicts* — not *safe*.
The branch cannot be updated via `gh` because of issue 9.

**Do not merge it.** Close it and let dependabot regenerate against UNSTABLE
(#637 already retargeted dependabot), or apply the one-line bump by hand.

### 9. Non-blocking CI checks that are permanently red — INFO

These appear on every PR and are not caused by the branch under review:

- `Cloudflare Pages: blazetorch-ai-devtools` — FAILURE (legacy brand project)
- `Cloudflare Pages: gasoline-agentic-devtools` — FAILURE (legacy brand project)
- `Codacy Static Code Analysis` — ACTION_REQUIRED

`Cloudflare Pages: gokaboom` (the live site) passes. Also note that **Security
Scan and JavaScript Checks run ESLint but are not required checks**, so
auto-merge can land lint-red PRs — run `npm run lint` before merging changes
under `extension/` or `tests/`.

## Runtime (product)

### 10. Extension timeout on first `interact()` — MEDIUM

The content script may not be fully loaded when the first `interact()` command
arrives after navigation. **Workaround:** retry after 2-3 seconds.

### 11. Tracking loss during cross-origin navigation — MEDIUM

The extension can lose tab tracking state during an AI-initiated cross-origin
navigation via `interact({action: "navigate"})`. **Workaround:** re-enable
tracking from the extension popup.

## Recently fixed

### v0.8.x

- Error clustering fingerprinted on the **raw** message, so errors differing only
  by an embedded id/uuid/url/timestamp never clustered with their own siblings —
  300 pseudo-clusters where 109 real ones existed. Now normalized. (#659)
- `analyze({what:"error_clusters"})` returned clusters and urls in Go map order,
  a different order on every call for identical input. Now sorted. (#659)
- `generate`'s CSP builder had the same map-iteration nondeterminism, in two
  places. (#657)
- Four repository gates had silently stopped running after their targets moved,
  including one reporting `PASS: bridge.go not found (skipped)` for ~4 months.
  (#654)
- ~3,400 lines of tests dormant behind `//go:build integration` re-enabled. (#651)
- ~9,500 lines of unreachable code deleted. (#657)

### v0.7.x

- Early-patch WebSocket capture for pages creating connections before inject loads
- camelCase → snake_case mapping for network waterfall entries
- Command results routed through `/sync` with client-ID filtering
- Post-navigation tracking state broadcast for favicon updates
- Empty arrays serialize as `[]` rather than `null`
- Bridge timeouts return a proper `extension_timeout` error code
- Pilot test zombies — hardcoded `version: '5.2.0'` removed from
  `tests/extension/pilot-*.test.js`

### v5.7.x

- Extension health check timeout (5s threshold added)
- Hardcoded version in `inject.bundled.js` (now read from VERSION via esbuild define)
- Stale compiled JS vs TS source
