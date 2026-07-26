# Full Code Review — 2026-06-10 (STABLE @ 7009d20e)

Four parallel review passes: Go server (`cmd/`, `internal/`), extension TypeScript (`src/`),
installers/packaging, and CI/test/docs infrastructure. Findings verified against the tree where
noted. Companion change shipped with this review: `scripts/uninstall.sh` + `scripts/uninstall.ps1`
(+ tests + docs) — see `docs/architecture/flow-maps/uninstall-and-cleanup.md`.

---

## CRITICAL

### CR1. CI does not run on the default branch at all
`.github/workflows/ci.yml:3-7`, `architecture-validation.yml`, `validate-versions.yml` trigger on
`branches: [main, UNSTABLE]` — but there is **no `main` branch** (remote has `STABLE`/`UNSTABLE`;
`origin/HEAD -> STABLE`). Pushes and PRs to STABLE (where dependabot PRs #566–#576 merged) run zero
gates. Only `dco.yml` covers STABLE, and it skips when the author is `brennhill`.
**Fix:** add `STABLE` to the `branches:` lists; require checks via branch protection.

### CR2. Releases publish to npm with zero tests
`.github/workflows/release.yml:86-188` and `publish.yml`: tag → build → `npm publish`, no `go test`,
no JS tests, no docs gates. Combined with CR1, code can reach npm having never passed CI.
**Fix:** gate `build-and-release` on a test job (`make ci` or `workflow_call` to CI).

### CR3. The default branch currently fails its own gates (proof CR1 matters)
- `scripts/validate-architecture.sh:20-30` requires `cmd/browser-agent/bridge.go` and
  `tools_interact.go` as "critical files" — neither exists (refactored). Script exits 1 today.
- `pypi/` does not exist, but `ci.yml:96-99`, `Makefile:434-498`, and
  `scripts/release/install-upgrade-regression.mjs:312` reference it; `docs/features/feature/enhanced-cli-config/index.md`
  lists 12 pypi paths. `make release-gate` would fail on this tree.
- All 123 feature `index.md` files have `last_reviewed` older than the 30-day window enforced by
  `scripts/docs/check-feature-bundles.js:96-140` — `npm run docs:check:strict` fails wholesale.

### CR4. `install.sh` deletes the live binaries before downloading; `--hooks-only` destroys a full install
`scripts/install.sh:271-287, 423` — `purge_legacy_install_artifacts` includes the **canonical**
`kaboom-agentic-browser` and `kaboom-hooks` names and runs before any download. If download/checksum/
smoke-test then fails, the user's working install is gone (configs/LaunchAgent point at nothing).
Worse: in `--hooks-only` mode the main binary is purged (line 423 is outside the guard) and never
reinstalled. **Fix:** purge only genuinely legacy names, and only after `download_and_verify` succeeds.

### CR5. Windows install is broken: validator requires a file that no longer exists
`scripts/install.ps1:167-185` (`Test-ExtensionStage`) hard-requires `theme-bootstrap.js`; verified
absent from `extension/` (bundled layout uses `early-patch.bundled.js`). `install.sh:195-198` accepts
either layout; the PS1 was never updated. Release-zip staging and the source-zip fallback both fail →
script dies after binary replacement, before `--install`. **Fix:** port the dual-layout check to PS1.

### CR6. MV3 service worker contains a dynamic `import()` — Audit feature broken at runtime
`src/background/message-handlers.ts:705` (verified, survives compilation to
`extension/background/message-handlers.js`). Chrome disallows dynamic import in service workers, so
`handleQaScanRequestedAsync` throws before its try/catch; the popup's awaited `qa_scan_requested`
message hangs then rejects ("message port closed"). Violates CLAUDE.md "No dynamic imports in
service worker". **Fix:** static top-level import (`terminal-widget-types.ts` is a pure helper).

### CR7. npm preinstall SIGKILLs unrelated processes by port
`npm/kaboom-agentic-browser/lib/kill-daemon.js:12-15, 98-105` — `lsof -ti :PORT | xargs kill -9`
for ports **7890–7910 and 17890** with no identity check, on every `npm install`/`uninstall`. Any
dev server on those 22 ports dies. `server/scripts/install.js:94-107` does the same for 7890/17890.
**Fix:** verify `/health` `service_name == kaboom-browser-devtools` (helper already exists in
`server/scripts/install.js:187-196`) or kill only PID-file PIDs.

---

## HIGH

### Go server
- **G-H1. Circuit breaker can never close — permanent WS-ingest lockout.**
  `internal/circuit/breaker.go:131-199` + `internal/capture/helpers.go:36-48`: the only path that
  re-evaluates the circuit runs inside `RecordEvents`, but once open, `CheckRateLimit` 429s before
  `RecordEvents` is ever called → OPEN→CLOSED is unreachable until daemon restart. Doc drift too:
  `capture-struct.go:32` claims a "memory < 30MB" close condition that isn't implemented.
- **G-H2. Log persistence rewrites the whole file on every POST at steady state + unsynchronized
  concurrent writers.** `cmd/browser-agent/server_logging_async.go:16-83`,
  `server_persistence.go:56-97`: once entries exceed `maxEntries` (1000), every `/logs` POST
  triggers a synchronous full rewrite on the request goroutine, racing the async appender (lost
  appends, duplicate entries, interleaved tmp writes). Violates "append-only I/O on hot paths".
- **G-H3. `after_cursor` + `limit` pagination returns overlapping pages / never advances.**
  `internal/pagination/pagination.go:104-127, 157`: continuation cursor is always built from the
  newest entry of the page; for after-walks it must come from the oldest. With 100 entries/limit 25:
  page 2 duplicates 24 items. Silent data loss for LLM consumers. (`buildMetadata`'s `afterCursor`
  param is declared and unused — the intended logic was lost.) Also `serverInstructions`
  (handler.go:30) advertises `after_cursor`/`before_cursor` metadata keys observe never emits.

### Extension
- **X-H1. Recording state destroyed on SW restart while offscreen keeps recording.**
  `src/background/recording.ts:51-60` deletes persisted state on module load; badge timer and popup
  state die on routine MV3 SW restarts while the offscreen `MediaRecorder` continues invisibly.
  Rehydrate from the offscreen document instead of unconditionally clearing.
- **X-H2. Telemetry: opt-out race + privacy-rule conflict.** `src/lib/telemetry-beacon.ts:14-17`
  hydrates the disable flag async while `init.ts:116` beacons immediately — opted-out users still
  emit on every SW start. Policy: CLAUDE.md Rule 7 says "no external transmission", yet extension +
  install.sh + uninstall (kept for parity) beacon to `t.gokaboom.dev` by default. Reconcile rule vs
  behavior explicitly.
- **X-H3. ESLint never lints `src/**/*.ts`.** `eslint.config.js` has no TS parser/plugin;
  `typescript-eslint` isn't installed. The "No any" rule has zero enforcement; the only `any` usage
  is `src/background/csp-safe-executor.ts` (~10, documented) plus a dead `no-explicit-any` disable.

### Installers / packaging
- **I-H1. npm `--uninstall` deletes entire shared settings files.**
  `npm/kaboom-agentic-browser/lib/uninstall.js:160-165`: when the config key empties, it
  `unlinkSync`s the file — for Zed `settings.json`, Gemini `settings.json`, OpenCode
  `opencode.json` that destroys all user settings. (The new `scripts/uninstall.sh` deliberately
  edits-in-place and never unlinks; port that behavior here.)
- **I-H2. Substring process-kill patterns match unrelated processes.** `install.sh:300-305`
  (`strum` matches "in**strum**ent"; bare `gasoline`), `kill-daemon.js:70` (`pgrep -af "kaboom"`
  matches `vim ~/dev/kaboom/...` → SIGKILL), `server/scripts/install.js:86-89` (`pkill -f
  'browser-agent'`). Anchor to full binary names.
- **I-H3. Interrupted extension promotion can delete the user's only extension copy.**
  `install.sh:62-69, 206-237`: the EXIT trap rm-rfs `BACKUP_EXT_DIR` even when the backup is the
  only copy (after `mv EXT_DIR → BACKUP` and before/while promoting).
- **I-H4. `server/` package is internally broken.** `server/scripts/install.js:18,379` downloads
  `kaboom-agentic-browser-*` but `server/bin/kaboom:20` execs `gasoline-*` — always "Binary not
  found". Its package.json name collides with `npm/kaboom-agentic-browser`. Dead code? Delete or fix.
- **I-H5. Release workflows stage binaries that npm `files` whitelists exclude; hooks binaries
  never staged.** `release.yml:143-147`/`publish.yml:80-84` copy to `bin/kaboom`, but platform
  packages whitelist `bin/kaboom-agentic-browser` + `bin/kaboom-hooks`
  (`npm/darwin-arm64/package.json:12-15`). Correct logic exists in `Makefile:200-223`
  (`npm-binaries`) but the workflows don't use it. Verify published tarballs actually contain binaries.

### CI / docs
- **D-H1. Wire-drift gate (Rule 8) effectively unenforced.** `make check-wire-drift` is in no
  workflow; `make verify-llm` runs only on PRs whose changed files match a scope regex
  (`ci.yml:48`) that **excludes `src/`** — TS-side wire edits skip the gate.
- **D-H2. Docs Cross-Reference Contract ~90% unenforced.** Nothing validates `code_paths`/
  `test_paths` resolve (file-upload, link-health, batch-sequences, mcp-persistent-server indexes all
  list deleted Go files), 92 of 126 feature dirs lack `flow-map.md`, and
  `lint-documentation.py:156-157` demotes unresolved code refs to warnings.

---

## MEDIUM (rolled up)

- **Go:** `RingBuffer.positionToIndex` wrong after `Clear()` — latent panic in unused cursor API
  (`internal/buffers/ring_buffer.go:222-241`; fix or delete). `internal/server` is a drifted copy of
  the live LogStore incl. a `0o644` world-readable log regression (`main_storage_io.go:79`) — dedupe.
  VS Code MCP config likely needs key `"servers"` not `"mcpServers"` in `mcp.json`
  (`native_install.go:117-147`; verify against VS Code docs).
- **Extension:** popup untrack leaves `TRACKED_TAB_TITLE` stale and bypasses shared helpers
  (`src/popup/tab-tracking-api.ts:31,71,115-119`; rule 18). `recording-listeners.ts:91` missing
  `.catch` → popup hang. `recording.ts:418-425` clears state before reading `name` (error path
  returns empty name). `message-handlers.ts:217-231` returns `true` without ever responding (port
  closed rejections).
- **Installers:** documented `| sh` invocation breaks on dash (`set -o pipefail`, `echo -e`,
  `==`) — docs say `| sh`, script is bash-only (`install.sh:10-11,352,764`). npm postinstall
  fabricates `~/.gemini` etc., making client auto-detection fire for absent clients
  (`lib/skills.js:70-101`). Antigravity path drift Go vs npm on Windows (`lib/config.js:160-169` vs
  `native_install.go:104-105`). `claude mcp add-json` not idempotent — every upgrade prints an
  error (`native_install.go:226`). fish PATH append fails install when `~/.config/fish` missing
  (`install.sh:728`). Windows `.new.exe` fallback path leaks permanently into MCP configs
  (`install.ps1:287-295`). Go config writes are non-atomic truncate-then-write
  (`native_install.go:279-289`; npm side already does temp+rename). Missing `checksums.txt` is
  misdiagnosed as "no binary" (`server/scripts/install.js:443-486`). Shell skills installer drifts
  from npm (hardcoded `version:1`, glob vs manifest — `install-bundled-skills.sh:124,180`).
  Unanchored checksum grep (`install.sh:462`).
- **CI:** ESLint non-blocking in the job that claims to enforce it (`ci.yml:201-206`, `|| echo`).
  Codacy step is a no-op gate (`ci.yml:244-257`). jscpd (rule 22) and `lint-tests.sh` wired to
  nothing. Circular-deps check advisory-only (`CIRCULAR_DEPS_STRICT` never set). `make ci-local`
  missing all docs gates; `make test` ≠ "all tests" (Go-short only). 10+ permanently-skipped Go
  tests (`tools_interact_audit_test.go:148,196`, `network_waterfall_test.go` stubs, etc.).
  `validate-versions.sh` orphaned; CI re-implements a weaker subset. Playwright e2e suite in no
  workflow.

## LOW (highlights)

- Go: single-token long-poll wakeup misses waiters (`dispatcher_queries.go:91-117`); connect-mode
  hard 30s timeout vs 60s async budget (`connect_mode.go:28`); WS tracker duplicate IDs in eviction
  order (`ws_connection_tracker.go:30-42`); 626/844 files don't match the mandated
  `// filename.go — Purpose.` header style (update rule or lint it).
- Extension: floating promises in catch paths (`event-listeners.ts:199`); offscreen sendMessage
  without `.catch` (×8); duplicated action-recording toggle (rules 19/21);
  pending-intent keys accessed ad-hoc (rule 18); SyncClient 1s setTimeout keep-alive anti-pattern.
- Installers: stale staging debris never swept; double daemon start on macOS; health check doesn't
  verify identity; `parseEnvVar` rejects `=` in values; CLI uninstall stops after first matching
  server name; `CLAUDECODE=` set-empty instead of unset; `--force` cleanup can target itself
  (`main_connection_force_cleanup_strategies.go:31-55`); `rm -rf "$EXT_DIR"` lacks a sanity guard.
- Docs: README says extension installs to `~/.kaboom/extension` (README.md:70) but the actual
  default is `~/KaboomAgenticDevtoolExtension`; `.gitignore:66-67` still references `pypi/gasoline-*`.

---

## Verified clean

- **stdout purity (MCP protocol):** all frames funnel through `writeMCPPayload` under a mutex;
  bridge mode dup2-isolates the transport fd; no violations found.
- **JSON snake_case:** every camelCase tag found carries a `// SPEC:` annotation (MCP/HAR/SARIF).
- **File size:** zero Go files >800 LOC (largest 681); only generated `dom-primitives.ts` (2115,
  documented exemption) on the TS side.
- **Eviction/concurrency:** capture buffer eviction is single-pass O(1); documented lock hierarchy
  held in all reviewed paths; runtime-message contract complete (all 29 message types declared);
  manifest permissions all used; wire types spot-checked in sync.

## Suggested priority

1. CR1–CR3 (turn CI on for STABLE; gate releases) — everything else regresses without this.
2. CR4, CR5, CR7 (installer data-loss/bricking; affects users immediately).
3. CR6 + X-H1 (shipped features broken at runtime).
4. G-H1/G-H3 (silent ingest outage; silent pagination data loss) — each needs a failing-first
   regression test per the bug-fix contract.
5. I-H1/I-H2 (npm uninstall data loss; process-kill collateral).
