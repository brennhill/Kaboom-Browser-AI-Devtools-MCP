---
status: active
scope: process/release
ai-priority: high
tags: [release, process, quality-gates, deployment]
relates-to: [known-issues.md, docs/core/uat-v5.3-checklist.md]
last-verified: 2026-07-10
canonical: true
---

# Release Process

Kaboom MCP uses a `UNSTABLE` → `STABLE` branching model with strict quality gates. Every release goes through automated and manual verification before reaching users.

## Hardened Release Train (authoritative)

> This section is the source of truth. The manual "Build & Publish" steps further down are
> **superseded** by the CI train and kept only for historical/emergency reference.

There is exactly **one** way to publish: push an annotated `vX.Y.Z` tag whose commit is on
`STABLE`. That triggers `.github/workflows/release.yml`, which is **fail-closed** (a broken
artifact cannot be published) and **self-verifying** (a half-published release fails the run).
`publish.yml` was deleted — a second, release-less publish path is how npm and GitHub Releases
diverged in the past.

### Two consumer channels — both must work every release

| Channel | Consumer command | Source |
|---------|------------------|--------|
| npm | `npx kaboom-agentic-browser` | aggregate pkg + per-platform `@brennhill/…-<os>-<arch>` binary pkgs (via `optionalDependencies`) |
| GitHub Release | `irm …/install.ps1 \| iex`, `curl …/install.sh \| bash` | release assets downloaded + checksum-verified by the installer |

### Pipeline stages (release.yml)

1. **Gate** — tag commit is on `STABLE`; `git tag == VERSION` file; Go+JS tests; wire drift.
2. **Build + pre-publish guard** — `make build` → `make npm-binaries`, which runs
   `scripts/verify-platform-binaries.js`: every platform package's `bin/*` must exist and be
   > 1 MB. npm silently drops missing `files` entries, so this is what makes an *empty binary
   package* (the 0.8.2 incident) impossible to publish.
3. **Publish** — platform packages first, aggregate last; idempotent ("already published" is
   tolerated so a re-run is safe). Then the GitHub Release with all binaries + `checksums.txt`.
4. **`verify-published`** (matrix: ubuntu/macOS/**windows**) — installs from the *published*
   channels and runs `--version`: waits for npm propagation, execs the npm-installed Go binary,
   runs the `npx` launcher, and runs the real `install.sh`/`install.ps1` pinned to the tag.
5. **`reconcile`** — final red/green: asserts the aggregate pkg, all 5 platform pkgs, **and**
   the GitHub Release all serve `VERSION`. Runs even if step 4 failed, and prints exact repair
   steps, so the two channels can never silently diverge (the 0.8.4 incident).

> **Coverage note (no silent caps):** hosted runners execution-test `linux-x64`,
> `darwin-arm64`, and `win32-x64` end-to-end. `linux-arm64` and `darwin-x64` have no free
> hosted runners, so they are verified by the pre-publish binary guard (real file, > 1 MB,
> correct GOOS/GOARCH cross-compile) and `reconcile`'s npm-presence check, but are **not**
> exec-tested. Add self-hosted arm64/x64 runners to close this gap.

### How each past incident is now prevented

| Incident | Failure shape | Guard that makes it fail-closed |
|----------|---------------|---------------------------------|
| 0.8.2 empty npm binary (312 B) | published-but-broken | `verify-platform-binaries.js` (pre-publish) + `verify-published` npm exec (post-publish) |
| 0.8.4 missing GitHub Release (installer 404) | half-published | `reconcile` asserts the Release exists |
| 0.8.4 STRUM banner / installer regressions | stale script shipped | `verify-published` runs the real installer end-to-end |
| Local/manual publish bypassing the train | out-of-band | `publish.yml` deleted; tag-push is the only path |
| Unverifiable download (HTML/404 body) | silent corruption | installers verify SHA-256 **by default** (below) |

### Before you tag: `make preflight`

Runs locally with no registry contact. Validates semver + `optionalDependencies` pins, builds
+ stages all platforms (re-running the empty-binary guard), then `npm publish --dry-run` for
every package (which re-runs the guard via `prepublishOnly` and surfaces tarball/`files`
problems). If preflight is green, the tag is safe to push.

```bash
make preflight        # pre-tag safety (no publish)
git tag vX.Y.Z && git push origin STABLE --follow-tags
```

### Re-running / repairing a release

The train is idempotent. To retry after a transient failure, use **"Re-run failed jobs"** on
the release workflow run (same tag ref) — do not cut a new tag. npm forbids re-publishing a
version, so if a *bad* version already went public and is <72 h old, unpublish it and cut a
patch. A missing GitHub Release can be recreated with
`gh release create vX.Y.Z dist/* --generate-notes`.

### Installer integrity (strict by default)

`install.sh` and `install.ps1` now verify the SHA-256 of every downloaded binary against the
release `checksums.txt` **by default**; an unverifiable download aborts. Opt out only for
offline/mirror installs with `KABOOM_INSTALL_STRICT=0`. Both installers accept
`KABOOM_VERSION=X.Y.Z` to pin a specific release (used by `verify-published` to target the
exact tag; also handy for reproducible user installs).

## Branch Model

```
main    ─●───────────────────●────── (releases only)
          │                   ↑
          │             merge + tag
          ↓                   │
UNSTABLE ─●──●──●──●──●──●──●─● ──── (integration)
             ↑  ↑        ↑
feature/a ───●  │        │
feature/b ──────●        │
feature/c ───────────────●
```

- **`stable`** — Published releases. What's on npm and the Chrome Web Store.
- **`UNSTABLE`** — Integration branch. All features merge here first.
- **Feature branches** — Branch from `UNSTABLE`, merge back to `UNSTABLE`.

## Quality Gates

Every feature must pass all gates before merging to `UNSTABLE`. All gates must be green before `UNSTABLE` merges to `stable`.

### Gate 1: Tests Pass

```bash
make test                              # Go server tests
node --test tests/extension/*.test.js  # Extension tests
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
make build                   # Cross-platform build succeeds
```

All platforms must build: darwin-arm64, darwin-x64, linux-arm64, linux-x64, windows-x64.

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

### Gate 7: Squash & Tag

Before pushing to `UNSTABLE`, all feature work is squashed into a single commit:

```bash
# Squash all commits since branching from UNSTABLE
/squash

# Tag for pre-UAT
git tag v{version}-pre-uat-{feature}

# Push
git push origin HEAD --follow-tags
```

### Gate 8: MCP Command Completeness (MANDATORY)

**This gate cannot be skipped.** Every command exposed via MCP MUST be fully implemented.

**Rule:** If an MCP tool/command is advertised in the tool schema, it MUST:

1. Be fully functional with all documented parameters working
2. Return proper results (not stubs, placeholders, or "not implemented" errors)
3. Have corresponding tests verifying the implementation
4. Have documentation matching the actual behavior

**If a command is not fully implemented:**

1. Remove it from the MCP tool definitions (do not expose it to clients)
2. Add a TODO in the code marking it for future implementation
3. Track in `docs/core/known-issues.md` under "Planned Features"

**Verification:**

```bash
# Review all MCP tool definitions
grep -r "tools\|inputSchema" cmd/browser-agent/tools_*.go

# Ensure no stub implementations
grep -rn "TODO\|FIXME\|not implemented" cmd/browser-agent/tools_*.go

# Cross-reference with test coverage
go test -v ./cmd/browser-agent/ | grep -E "^--- (PASS|FAIL)"
```

**Why this matters:** Clients (Claude Code, IDEs, automation) rely on MCP tool schemas to understand capabilities. Advertising unimplemented commands breaks client expectations and causes confusing errors.

### Gate 9: Architecture Invariant Tests (MANDATORY)

**This gate cannot be skipped.** Critical architecture invariants must be verified before every release.

#### 9.1 MCP Stdio Silence

The server MUST NOT output anything to stdio except JSON-RPC messages. Any non-JSON-RPC output breaks LLM communication.

```bash
go test ./cmd/browser-agent -run "TestToolHandler.*Stdout" -v
go test ./cmd/browser-agent -run "TestStdioSilence" -v
```

See: `.claude/refs/mcp-stdio-invariant.md`

#### 9.2 Server Persistence

The HTTP server MUST stay alive as long as stdin remains open. This ensures browser extension connectivity throughout the MCP session.

```bash
go test ./cmd/browser-agent -run "TestServerPersistence" -v
```

**Key invariants tested:**

- Server survives 10+ seconds with open stdin (no data)
- Health endpoint responds within 100ms at all times
- Server survives stdin close (waits for SIGTERM)
- Server handles rapid health checks under load

See: `.claude/refs/mcp-stdio-invariant.md#server-persistence-invariant---critical`

#### 9.3 Behavioral Audit Tests

All MCP tools must have comprehensive behavioral tests verifying actual functionality, not just "doesn't crash".

```bash
go test ./cmd/browser-agent -run "Test.*Audit" -v
```

**Test coverage required:**

| Test File | Tools Covered | Minimum Tests |
|-----------|---------------|---------------|
| `tools_observe_audit_test.go` | observe (29 modes) | 41 tests |
| `tools_configure_audit_test.go` | configure (19 actions) | 46 tests |
| `tools_generate_audit_test.go` | generate (10 formats) | 28 tests |
| `tools_interact_audit_test.go` | interact (11 actions) | 31 tests |

## Release Checklist

When `UNSTABLE` is stable and ready for release:

### 1. Final Verification on `UNSTABLE`

```bash
# Full test suite
make test
node --test tests/extension/*.test.js

# Static analysis
go vet ./cmd/browser-agent/

# Cross-platform build
make build

# Coverage check
go test -coverprofile=coverage.out ./cmd/browser-agent/
go tool cover -func=coverage.out | grep total
```

### 2. Version Bump

**CRITICAL:** Use `/bump-version {version}` to update all locations, then **MUST run validation**:

```bash
bash scripts/validate-versions.sh
```

This validates all 17+ version locations match, including:
- All package.json files (npm, extension, server)
- Go main.go version constant
- MCP golden test file
- README badge
- **optionalDependencies in npm/kaboom-mcp/package.json** (CRITICAL - must match main version)

**If validation fails, STOP. Do not proceed with release.**

All locations updated by bump-version:

| File | Field |
|------|-------|
| `Makefile` | `VERSION :=` |
| `cmd/browser-agent/main.go` | `version` constant |
| `extension/manifest.json` | `"version"` |
| `extension/package.json` | `"version"` |
| `server/package.json` | `"version"` |
| `server/scripts/install.js` | `VERSION` constant |
| `npm/kaboom-mcp/package.json` | `"version"` + `optionalDependencies` ⚠️ |
| `npm/darwin-arm64/package.json` | `"version"` |
| `npm/darwin-x64/package.json` | `"version"` |
| `npm/linux-arm64/package.json` | `"version"` |
| `npm/linux-x64/package.json` | `"version"` |
| `npm/win32-x64/package.json` | `"version"` |
| `cmd/browser-agent/testdata/mcp-initialize.golden.json` | `"version"` |
| `README.md` | Version badge |
| `tests/extension/background.test.js` | Test assertions (2 locations) |
| `extension/background/index.test.js` | Mock manifest version |

**⚠️ CRITICAL:** `optionalDependencies` in `npm/kaboom-mcp/package.json` MUST point to the same version as the wrapper package itself. If these are mismatched, npx will install old binaries.

### 3. Merge to `stable`

```bash
git checkout stable
git merge UNSTABLE
```

### 4. Tag the Release

```bash
git tag v{version}
git push origin stable --follow-tags
```

### 5. Build & Publish

> ⚠️ **Superseded.** Publishing is fully automated by the tag-triggered release train (see
> "Hardened Release Train" at the top). Do **not** publish npm packages by hand — a manual
> publish skips `verify-published`/`reconcile` and is how the npm and GitHub Release channels
> diverged. The steps below are retained for emergency/offline reference only.

```bash
# Cross-platform binaries
make build
```

**NPM:**
```bash
cd npm && ./publish.sh
```

**PyPI:**
```bash
# Build all PyPI packages
make pypi-build

# Test PyPI first (recommended)
make pypi-test-publish

# Production PyPI
make pypi-publish
```

See `docs/pypi-distribution.md` for detailed PyPI publishing instructions.

**Chrome Web Store:**
```bash
# Upload extension/ directory via Chrome Developer Dashboard
```

### 6. Sync `UNSTABLE`

```bash
git checkout UNSTABLE
git merge stable
git push origin UNSTABLE
```

### 7. Update Marketing Site

The marketing site is a separate repo at `~/dev/kaboom-site` (Astro).
Blog posts go in `src/content/docs/blog/`. Update version numbers and
add release blog post there after tagging.

## Hotfix Process

For critical fixes that can't wait for the next release:

```bash
git checkout -b hotfix/fix-name main
# Fix, test, commit
git checkout stable && git merge hotfix/fix-name
git tag v{version}
git push origin stable --follow-tags

# Sync back
git checkout UNSTABLE && git merge hotfix/fix-name
git push origin UNSTABLE
git branch -d hotfix/fix-name
```

## Pre-UAT Tags

Every feature entering UAT gets a tagged, squashed commit:

```
v4.7.0-pre-uat-websocket-monitoring
v4.7.0-pre-uat-network-bodies
v4.7.0-pre-uat-checkpoint-diffs
```

If UAT fails, the single commit can be reverted atomically.
