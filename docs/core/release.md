---
status: active
scope: process/release
ai-priority: high
tags: [release, process, quality-gates, deployment]
relates-to: [known-issues.md, docs/core/uat-v5.3-checklist.md]
last-verified: 2026-07-24
canonical: true
---

# Release Process

Kaboom MCP uses an `UNSTABLE` → `STABLE` branching model with strict quality
gates. Releases are cut by **pushing a `v*.*.*` tag onto `STABLE`**, which
triggers the automated release train in
[`.github/workflows/release.yml`](../../.github/workflows/release.yml). The train
validates, tests, builds all platform binaries, and publishes — no manual
`npm publish` step.

## Branch Model

```
STABLE   ─●───────────────────●────── (published releases; tag v* here)
          ▲                    ▲
          │  merge PR          │  merge PR + push tag
          │                    │
UNSTABLE ─●──●──●──●──●──●──●──● ────── (integration)
             ▲  ▲        ▲
feature/a ───●  │        │
feature/b ──────●        │
feature/c ───────────────●
```

- **`STABLE`** — Published releases. What's on npm and what the `curl | sh`
  installer serves. Release tags point here.
- **`UNSTABLE`** — Integration branch. Feature work merges here first.
- **Feature branches** — Branch from `UNSTABLE`, PR back to `UNSTABLE`. Never
  push directly to `STABLE`.

> The old `main`/`stable` names and the PyPI distribution channel are retired.
> There is **no PyPI package** — distribution is npm + the `curl | sh` installer
> (which pulls binaries from the GitHub Release). Chrome Web Store uploads remain
> a manual step (see [Chrome Web Store](#chrome-web-store)).

## Distribution Channels

| Channel | What ships | How |
| --- | --- | --- |
| **npm** | `kaboom-agentic-browser` aggregate package + 5 `@brennhill/kaboom-agentic-browser-<platform>` binary packages | `release.yml` on tag push |
| **`curl \| sh`** | `scripts/install.sh` served live from `STABLE`; downloads the platform binary + extension from the GitHub Release | GitHub Release assets from `release.yml` |
| **GitHub Release** | 5 platform binaries, 5 `kaboom-hooks` binaries, extension zip, `checksums.txt` | `release.yml` on tag push |
| **Chrome Web Store** | `extension/` directory | Manual upload (see below) |

## Quality Gates

Every feature must pass all gates before merging to `UNSTABLE`. All gates must be
green before `UNSTABLE` merges to `STABLE`.

### Gate 1: Tests Pass

```bash
make test                              # Go server tests
node --test tests/extension/*.test.js  # Extension tests (or: npm run test:ext)
```

No code is merged with failing tests.

### Gate 2: Test Quality

Tests must:
- Import and test actual source code (no inline logic)
- Verify behavior, not mocks
- Cover edge cases, error paths, and boundaries
- Map to specification requirements in `docs/`

### Gate 3: Specification Coverage

Every requirement in the specification has corresponding tests:
- Buffer sizes, truncation limits, timeouts
- SLO targets with validation
- Protocol compliance (JSON-RPC 2.0, MCP)
- Error conditions (invalid input, overflow, timeout)

### Gate 4: Static Analysis

```bash
go vet ./cmd/browser-agent/    # No warnings
npm run lint                   # ESLint — NOT a required CI check, so run it locally
make build                     # Cross-platform build succeeds
```

All platforms must build: darwin-arm64, darwin-x64, linux-arm64, linux-x64,
windows-x64.

> **Lint caveat:** the `Security Scan` and `JavaScript Checks` CI jobs run ESLint
> but are **not required** merge checks, so an eslint-red PR can still auto-merge.
> Always run `npm run lint` locally before merging changes under `extension/` or
> `tests/extension/`.

### Gate 5: Performance SLOs

| Metric | Target |
|--------|--------|
| `fetch()` wrapper overhead | < 0.5ms |
| WebSocket handler overhead | < 0.1ms per message |
| Page load impact | < 20ms |
| Server memory under load | < 30MB |
| MCP tool response time | < 200ms |

### Gate 6: Code Coverage

| Scope | Minimum |
|-------|---------|
| Overall (statements) | 95% |
| Per-file (statements) | 90% |

```bash
go test -coverprofile=coverage.out ./cmd/browser-agent/
go tool cover -func=coverage.out | grep total
```

Coverage must not decrease between commits.

### Gate 7: MCP Command Completeness (MANDATORY)

**This gate cannot be skipped.** Every command exposed via MCP MUST be fully
implemented.

**Rule:** If an MCP tool/command is advertised in the tool schema, it MUST:

1. Be fully functional with all documented parameters working
2. Return proper results (not stubs, placeholders, or "not implemented" errors)
3. Have corresponding tests verifying the implementation
4. Have documentation matching the actual behavior

**If a command is not fully implemented:**

1. Remove it from the MCP tool definitions (do not expose it to clients)
2. Add a TODO in the code marking it for future implementation
3. Track in `docs/core/known-issues.md` under "Planned Features"

### Gate 8: Architecture Invariant Tests (MANDATORY)

**This gate cannot be skipped.** Critical architecture invariants must be
verified before every release.

#### 8.1 MCP Stdio Silence

The server MUST NOT output anything to stdio except JSON-RPC messages. Any
non-JSON-RPC output breaks LLM communication.

```bash
go test ./cmd/browser-agent -run "TestToolHandler.*Stdout" -v
go test ./cmd/browser-agent -run "TestStdioSilence" -v
```

See: `.claude/refs/mcp-stdio-invariant.md`

#### 8.2 Server Persistence

The HTTP server MUST stay alive as long as stdin remains open.

```bash
go test ./cmd/browser-agent -run "TestServerPersistence" -v
```

See: `.claude/refs/mcp-stdio-invariant.md#server-persistence-invariant---critical`

#### 8.3 Wire Drift

Go `wire_*.go` and TS `wire-*.ts` payload contracts must stay in sync.

```bash
make check-wire-drift
```

## One-Click Release (recommended)

Once `STABLE` holds everything you want to ship, the entire release is a single
manual CI trigger — the [`Cut Release`](../../.github/workflows/cut-release.yml)
workflow (`workflow_dispatch`, run it **from the STABLE branch**):

1. Actions → **Cut Release** → *Run workflow* → set **`version`** (e.g. `0.8.7`),
   leave **`dry_run`** unchecked (or check it for a no-mutation preview).
2. It does everything: transactional version bump → `make compile-ts` →
   `make validate-versions` → wire-drift gate → `go test -short` → `npm run test:ext`
   → commit + push `STABLE` + tag `v<version>` → build 5 platforms → publish npm
   (platform + aggregate) → GitHub Release.

Publishing is delegated to the reusable
[`release-publish.yml`](../../.github/workflows/release-publish.yml) — the same
pipeline the tag-triggered `release.yml` uses, so both paths build and publish
identically. No PAT or extra secret is needed (it pushes to the unprotected
`STABLE` with the built-in `GITHUB_TOKEN` and invokes the publish pipeline
directly rather than via the tag event).

**Dry run:** check `dry_run` to bump + validate + test and upload the prepared
`git diff` as an artifact — nothing is committed, tagged, or published.

The manual checklist below is the equivalent by-hand procedure (and how to reason
about each gate); the one-click workflow performs exactly these steps.

## Release Checklist

When `UNSTABLE` is stable and ready for release:

### 1. Final Verification on `UNSTABLE`

```bash
make test
node --test tests/extension/*.test.js
go vet ./cmd/browser-agent/
make build
```

### 2. Version Bump

Version is single-sourced from `VERSION`, and one implementation owns every
mutation and check: `scripts/release/version/version-sync.mjs`.

```bash
# 1. Atomically set VERSION and every canonical target
make bump-version NEW_VERSION=0.9.1

# 2. Regenerate the extension bundle (embeds VERSION via esbuild define)
make compile-ts

# 3. Validate — this is the required "Version Consistency" CI check
make validate-versions
```

**If validation fails, STOP. Do not proceed with release.**

`make bump-version` preflights every target before writing, stages all new
contents, and rolls back committed files if a write fails. `make sync-version`
is the repair command: it leaves `VERSION` unchanged and synchronizes the same
inventory. `make validate-versions` checks that inventory without writing.

Canonical targets include:

| File | Field |
|------|------|
| `package.json` (root) | `"version"` |
| `package-lock.json` | root package versions only; dependency versions are preserved |
| `cmd/browser-agent/main.go`, `cmd/hooks/main.go` | `version` constant |
| `extension/manifest.json`, `extension/package.json` | `"version"` |
| `server/package.json` | `"version"` |
| `npm/kaboom-agentic-browser/package.json` | `"version"` **+ `optionalDependencies`** ⚠️ |
| `npm/{darwin-arm64,darwin-x64,linux-arm64,linux-x64,win32-x64}/package.json` | `"version"` |
| `packages/kaboom-ci/package.json`, `packages/kaboom-playwright/package.json` | `"version"` (+ `@anthropic/kaboom-ci` pin) |
| `README.md` | version badge + prose |
| `claude_skill/kaboom/SKILL.md` | distributed skill metadata |

> **`optionalDependencies`:** the five `@brennhill/kaboom-agentic-browser-*`
> entries in `npm/kaboom-agentic-browser/package.json` MUST equal the wrapper
> version, or npx installs old binaries. They are updated and checked by the same
> canonical transaction as the package version.

Commit the bump to a `release/<version>` branch and merge it to `STABLE` via PR.

### 3. Tag on `STABLE` (this triggers the release train)

The tag commit MUST be an ancestor of `STABLE` — `release.yml` refuses to publish
otherwise.

```bash
git checkout STABLE && git pull
git tag v0.8.6
git push origin v0.8.6
```

`release.yml` then runs automatically:

1. **check-tag-on-stable** — confirms the tag commit is on `STABLE`
2. **validate** — `validate-versions.yml`
3. **verify-tag** — tag version == `VERSION` file
4. **test** — wire-drift gate, `go test -short ./...`, `npm run test:ext`
5. **build-and-release** — build 5 platform binaries, verify the linux-x64
   binary reports the tag version, `make compile-ts`, stage binaries, **publish
   the 5 platform packages then the aggregate package** (idempotent — an
   already-published version is skipped, not an error), build the extension zip,
   generate checksums, and create the GitHub Release with all assets.

Publishing uses the `NPM_TOKEN` repository secret. If a job fails **after** the
tag exists (e.g. an expired token → npm `E404`), fix the cause and **re-run the
failed jobs** — do NOT cut a new tag. The publish step is idempotent.

### 4. Manual publish fallback

Re-run the failed jobs of the tag's [`Release`](../../.github/workflows/release.yml)
run. The publish steps are idempotent (an already-published version is treated as
success), so a re-run safely fills in whatever did not land.

There is deliberately no separate manual-publish workflow. `Publish Packages`
(`publish.yml`) used to offer one, but a second publish path could ship a version
that never passed the tag-on-STABLE gate and never ran post-publish verification —
so it was removed in favour of a single publish path.

### 5. Chrome Web Store

Upload the `extension/` directory (or the `dist/kaboom-extension-v*.zip` asset)
via the Chrome Developer Dashboard. This is not automated.

### 6. Sync `UNSTABLE`

Fast-forward `UNSTABLE` back up to the released `STABLE`:

```bash
git push origin origin/STABLE:UNSTABLE
```

### 7. Update Marketing Site

The marketing site is a separate repo at `~/dev/kaboom-site` (Astro). Add the
release blog post under `src/content/docs/blog/` and bump version references
there.

## Hotfix Process

For critical fixes that can't wait for the next integration cycle:

```bash
git checkout -b hotfix/fix-name STABLE
# Fix, test, commit
# PR hotfix/fix-name -> STABLE, merge
git checkout STABLE && git pull
git tag v{version}
git push origin v{version}          # triggers release.yml

# Sync integration branch back up
git push origin origin/STABLE:UNSTABLE
```
