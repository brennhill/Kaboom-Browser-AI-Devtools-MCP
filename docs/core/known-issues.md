---
status: active
scope: issues/blockers
ai-priority: high
tags: [known-issues, v0.8.x]
last-verified: 2026-07-28
canonical: true
---

# Known Issues

Every entry below was re-verified against the working tree on the date in the
frontmatter, and carries the command that reproduces it. If you cannot reproduce
one, treat it as fixed and delete it rather than leaving it to rot.

## Test infrastructure

### 1. `cmd/browser-agent` tests run close to their timeout — MEDIUM

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

### 2. Flaky tests (pre-existing)

- `TestAsyncQueueReliability/Slow_polling` — intermittent 30s timeout
- `tests/extension/async-timeout.test.js` — 3 tests flaky
- `TestFastStart_ClientCompatibilityMatrix/claude_code` — fails only under
  full-suite load; passes in isolation. Local runs are additionally sensitive to
  anything already holding port 7891.

## Test coverage gaps

### 3. Per-package coverage numbers under-report cross-package tests — INFO

Go measures coverage per package. When a function's tests live in a *different*
package — very common here, because `cmd/browser-agent` tests exercise
`internal/tools/...` through the MCP dispatch — the function reads 0.0% while
being fully exercised.

This also leaves the aggregate coverage gate red: `make ci-local` passes
`go vet` and the complete race suite, then `make test-cover` reports **71.5%**
against the unchanged 89% threshold. Tracked as `kaboom-xem`; the fix must make
cross-package execution visible rather than lower or bypass the threshold.
Naively adding `-coverpkg=./...` is not sufficient: it reports **85.6%** and
adds enough instrumentation overhead to trip the documented fast-start timeout.

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

## Release / tooling

### 4. Repo token lacks `workflow` scope — MEDIUM

Any PR touching `.github/workflows/` cannot be merged, or have its branch
updated, via `gh`:

```
GraphQL: refusing to allow an OAuth App to create or update workflow
`.github/workflows/ci.yml` without `workflow` scope (updatePullRequestBranch)
```

Such PRs must go through the GitHub web UI.

### 5. PR #591 would revert the repository if merged — HIGH

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

### 6. Non-blocking CI checks that are permanently red — INFO

These appear on every PR and are not caused by the branch under review:

- `Cloudflare Pages: blazetorch-ai-devtools` — FAILURE (legacy brand project)
- `Cloudflare Pages: gasoline-agentic-devtools` — FAILURE (legacy brand project)
- `Codacy Static Code Analysis` — ACTION_REQUIRED

`Cloudflare Pages: gokaboom` (the live site) passes. Also note that **Security
Scan and JavaScript Checks run ESLint but are not required checks**, so
auto-merge can land lint-red PRs — run `npm run lint` before merging changes
under `extension/` or `tests/`.

## Runtime (product)

### 7. Extension timeout on first `interact()` — MEDIUM

The content script may not be fully loaded when the first `interact()` command
arrives after navigation. **Workaround:** retry after 2-3 seconds.

### 8. Tracking loss during cross-origin navigation — MEDIUM

The extension can lose tab tracking state during an AI-initiated cross-origin
navigation via `interact({what: "navigate"})`. **Workaround:** re-enable
tracking from the extension popup.

## Recently fixed

### v0.8.x

- Upload-handler unit tests used the production native-dialog and verification
  delays despite mocking every external result. Under concurrent JavaScript
  shard load, event-loop starvation stretched an ~8-second unit case past 100
  seconds and left the module escalation mutex occupied for the following test.
  The tests now replace only the delay/fetch time boundary and enforce a 500ms
  ceiling; production timing is unchanged. (`kaboom-00v`)
- The broad `Capture` behavior was decomposed into change-coupled
  `HTTPHandlers`, `SyncHandler`, `HealthReader`, and `StateResetter` owners.
  `Capture` now exposes only canonical owner accessors and lifecycle close;
  structural tests prohibit the deleted forwarding surfaces.
- The extension folder split was audited by responsibility and duplication.
  `background/{commands,dom,exec,recording,sync,ui}` and
  `lib/{analysis,net,page,storage,tabs}` are domain boundaries, not filename or
  size buckets. Handwritten command clones were extracted; the remaining
  `jscpd` findings are generated DOM primitives sourced from one template and
  partial set because Chrome requires self-contained injected functions.
- All authored Go, TypeScript, and JavaScript files, including tests, are now
  enforced at 800 LOC with no waiver comments. The oversized suites were split
  by change-coupled responsibility; generated and compiled outputs are excluded.
- All 207 authored Node test files now run through the canonical sharded test
  command, including `.cjs` and `.mjs` suites under CLI, docs, packaging, site,
  and release scripts. The dormant-test gate now checks Go and Node inventories,
  and CI invokes the complete `make check-structure` target.
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
