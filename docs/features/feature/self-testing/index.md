---
doc_type: feature_index
feature_id: feature-self-testing
status: in-progress
feature_type: feature
owners: []
last_reviewed: 2026-09-05
code_paths:
  - .github/workflows/ci.yml
  - scripts/tests/browser/mode-content-expectations.sh
  - scripts/uat/orchestration/uat-category-script.sh
  - scripts/uat/orchestration/uat-browser-launch.sh
  - scripts/quality/affected/affected-tests.mjs
  - scripts/quality/affected/run-affected.sh
  - scripts/quality/affected/always-run.json
  - scripts/uat/runners/smoke-test.sh
  - scripts/smoke-tests/harness/framework-smoke.sh
  - scripts/smoke-tests/interact/14-browser-push.sh
  - scripts/smoke-tests/upload/15-file-upload.sh
  - scripts/smoke-tests/framework/29-framework-selector-resilience.sh
  - scripts/smoke-tests/core/30-stability-shutdown.sh
  - scripts/uat/runners/test-all-split.sh
  - scripts/uat/runners/test-original-uat.sh
  - scripts/uat/runners/test-new-uat.sh
  - scripts/uat/runners/test-js-sharded.sh
  - scripts/build/run-go-integration.sh
  - scripts/build/run-go-coverage.sh
  - scripts/build/generate-command-contract.js
  - internal/commandcontract/generated.go
  - src/types/runtime/command-contract.ts
  - scripts/uat/orchestration/uat-result-lib.sh
  - scripts/tests/framework/framework.sh
  - scripts/tests/framework/process-census.sh
  - scripts/uat/runners/test-all-tools-comprehensive.sh
  - internal/extclient/extclient.go
  - internal/capture/syncruntime/handler.go
  - cmd/browser-agent/internal/httpguard/middleware.go
  - scripts/tests/framework/uat-artifacts.sh
  - scripts/tests/framework/uat-user-state.sh
  - scripts/tests/framework/uat-user-state.test.sh
  - scripts/tests/browser/cat-33-connected-action-coverage.sh
  - scripts/tests/browser/cat-35-qa-fixtures.sh
  - scripts/tests/release/cat-34-packaged-corruption-recovery.sh
  - scripts/contracts/check-architecture-boundaries.cjs
  - scripts/quality/contracts/check-dormant-tests.sh
  - .architecture-boundaries.json
  - .go-architecture-baseline.json
  - scripts/uat/runners/test-all-tools-comprehensive.sh
  - scripts/maintenance/cleanup-test-daemons.sh
  - cmd/browser-agent/internal/testowner/testowner.go
  - cmd/browser-agent/internal/integrationtest/harness.go
  - cmd/browser-agent/server.go
  - cmd/browser-agent/internal/testpages/http.go
  - cmd/browser-agent/internal/testpages/websocket.go
  - cmd/browser-agent/internal/wsframe/frame.go
  - internal/statefault/fault.go
  - internal/statefault/store.go
  - internal/capturefixture/sync.go
  - cmd/browser-agent/internal/asynccommand/handler.go
  - internal/synctranscript/transcript.go
  - internal/synctranscript/replay.go
  - internal/synctranscript/capture.go
  - cmd/kaboom-replay-extension/main.go
  - scripts/tests/framework/uat-replay.sh
  - scripts/tests/framework/json.sh
  - scripts/tests/transcripts/record-connected-transcripts.sh
  - scripts/tests/transcripts/README.md
  - scripts/quality/workflows/check-raw-jq.mjs
  - scripts/quality/workflows/check-destructive-git.mjs
  - .github/workflows/connected-canary.yml
  - .raw-jq-baseline.json
  - internal/wirecodec/decode.go
  - scripts/contracts/wiredecode/main.go
  - .wire-decode-exemptions.json
  - scripts/contracts/goarchitecture/main.go
test_paths:
  - scripts/contracts/uatcoverage/main_test.go
  - scripts/quality/affected/affected-tests.test.mjs
  - tests/cli/uat-assertions/process-census.test.cjs
  - scripts/release/install-upgrade-regression.contract.test.mjs
  - scripts/uat/orchestration/uat-result-lib.test.mjs
  - cmd/browser-agent/internal/integrationtest/harness_test.go
  - cmd/browser-agent/internal/asynccommand/handler_test.go
  - tests/cli/contracts/smoke-layout-contract.test.cjs
  - tests/extension/contracts/tooling-contracts.test.js
  - cmd/browser-agent/integration/bridge/faststart_extended_test.go
  - cmd/browser-agent/integration/runtime/persistence_test.go
  - cmd/browser-agent/integration/runtime/reliability_lifecycle_test.go
  - cmd/browser-agent/integration/bridge/stdio_silence_test.go
  - cmd/browser-agent/internal/testpages/websocket_test.go
  - tests/cli/contracts/uat-harness-regressions.test.cjs
  - tests/cli/uat-runner/uat-runner-accounting.test.cjs
  - tests/cli/uat-assertions/assertion-falsifiability.test.cjs
  - cmd/browser-agent/internal/testowner/testowner_test.go
  - cmd/browser-agent/integration/runtime/orphan_reaping_test.go
  - tests/cli/uat-assertions/test-daemon-cleanup.test.cjs
  - tests/cli/uat-assertions/connected-fixture-determinism.test.cjs
  - internal/synctranscript/transcript_test.go
  - internal/synctranscript/replay_test.go
  - internal/synctranscript/capture_test.go
  - cmd/browser-agent/integration/replay/replay_test.go
  - scripts/tests/framework/json.test.sh
  - scripts/quality/workflows/check-raw-jq.test.mjs
  - scripts/quality/workflows/check-destructive-git.test.mjs
  - internal/wirecodec/decode_test.go
  - scripts/contracts/wiredecode/main_test.go
  - internal/perftrace/http_test.go
  - scripts/contracts/check-architecture-boundaries.test.cjs
  - tests/cli/contracts/test-layout-contract.test.cjs
  - scripts/smoke-tests/interact/14-browser-push.sh
  - scripts/smoke-tests/upload/15-file-upload.sh
  - scripts/smoke-tests/framework/29-framework-selector-resilience.sh
  - scripts/smoke-tests/core/30-stability-shutdown.sh
  - scripts/tests/contracts/cat-01-protocol.sh
  - scripts/tests/tools/cat-02-observe.sh
  - scripts/tests/tools/cat-03-generate.sh
  - scripts/tests/tools/cat-04-configure.sh
  - scripts/tests/tools/cat-05-interact.sh
  - scripts/tests/runtime/cat-06-lifecycle.sh
  - scripts/tests/runtime/cat-07-concurrency.sh
  - scripts/tests/contracts/cat-08-security.sh
  - scripts/tests/contracts/cat-09-http.sh
  - scripts/tests/contracts/cat-10-regression.sh
  - scripts/tests/capture/cat-11-data-pipeline.sh
  - scripts/tests/tools/cat-12-rich-actions.sh
  - scripts/tests/browser/cat-13-pilot-contract.sh
  - scripts/tests/browser/cat-14-extension-startup.sh
  - scripts/tests/browser/cat-15-pilot-success-path.sh
  - scripts/tests/contracts/cat-16-api-contract.sh
  - scripts/tests/runtime/cat-17-performance.sh
  - scripts/tests/capture/cat-18-recording.sh
  - scripts/tests/browser/cat-19-link-health.sh
  - scripts/tests/capture/cat-20-noise-persistence.sh
  - scripts/tests/runtime/cat-21-stress.sh
  - scripts/tests/runtime/cat-22-advanced.sh
  - scripts/tests/browser/cat-23-draw-mode.sh
  - scripts/tests/browser/cat-24-upload.sh
  - scripts/tests/browser/cat-33-connected-action-coverage.sh
  - scripts/tests/browser/cat-35-qa-fixtures.sh
  - scripts/tests/release/cat-34-packaged-corruption-recovery.sh
  - tests/cli/contracts/packaged-recovery-uat.test.cjs
  - scripts/tests/runtime/cat-26-dynamic-upgrade.sh
  - scripts/tests/workflows/cat-29-reproduction.sh
  - scripts/tests/workflows/cat-30-recording-automation.sh
  - scripts/tests/workflows/cat-31-link-crawling.sh
  - cmd/browser-agent/internal/testpages/testpages_test.go
  - cmd/browser-agent/internal/wsframe/frame_test.go
  - internal/statefault/fault_test.go
  - internal/statefault/boundary_test.go
  - internal/statefault/store_test.go
  - internal/capturefixture/sync_test.go
  - internal/extclient/extclient_test.go
  - internal/capture/syncruntime/sync_test.go
  - scripts/tests/browser/mode-content-expectations.sh
  - scripts/tests/framework/uat-fixture-state.sh
  - scripts/contracts/uatcoverage/category_resolution_test.go
  - scripts/contracts/uatcoverage/browser_launch_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Self Testing

Connected action coverage establishes the deterministic tracked fixture before
the first schema-derived command and allows the extension's bounded production
reconnect backoff to expire before declaring a handoff failure. Newly exposed
modes must provide an explicit non-destructive health payload; trace and React
profile checks exercise a paired start/stop lifecycle.

A connected category runs beside a real extension, so its contract probes must
not impersonate one. Any `/sync` post from a connected category identifies as
`kaboom-probe/<version>`: the guard admits that client class, but the sync
runtime never adopts it as the authoritative session.

The corollary is that a probe cannot verify very much. Because the daemon answers
it with a canned envelope that adopts no session, settings, or commands, every
payload gets a byte-identical reply — `{}` and a fully populated settings block
are indistinguishable. Categories 14 and 16 exist to prove settings are *applied*,
so they claim the extension identity and run on the offline port, where no browser
is attached and the claim is harmless. Category 15 no longer posts `/sync` at all;
pilot gating moved to category 13, which drives pilot state offline and asserts
the resulting `error_code`.

Posting as `kaboom-extension` erased the tracked tab. `updateSyncConnectionState`
applies `req.Settings` unconditionally, and `TrackingEnabled` / `TrackedTabID` are
plain `bool` / `int`, so a payload carrying only `pilot_enabled` deserializes the
rest as `false` / `0` and overwrites what the real extension had reported. The
real extension restores it on its next full heartbeat, which makes this a race:
measured at roughly two seconds on an idle daemon, and far longer during a run
where every category restarts the daemon. That was the source of the intermittent
`no tracked browser tab on port 7890` failures in categories scheduled after them.

The overwrite happens *before* the request is rejected. Because the UAT payloads
used `session_id` rather than the wire field `ext_session_id`, `ExtSessionID`
arrived empty and the apply phase answered 409 `stale_connection_generation` —
a valid JSON body, so every test that only asserted "a JSON response came back"
passed while exercising nothing. The payloads now use `ext_session_id`, and a
contract test fails the build if `session_id` reappears.

Every connected category restarts the daemon and the suite toggles pilot state,
so the preflight's tracked tab cannot be assumed to survive to the last
category. When a category finds the extension connected but nothing tracked, it
re-establishes its own disposable fixture tab and continues; only a
disconnected extension is a hard readiness failure, because nothing can open a
tab without it.

Fixture setup waits for the extension heartbeat, not for a tracked tab: the
fixture navigation is itself what establishes tracking, so requiring a tracked
tab first turned any transient target loss into an unrecoverable failure for
every later action needing a target. The tracked-tab gate runs after the
navigation. For the same reason, a `No tab is being tracked` response is a
recoverable target loss — the action re-establishes the fixture and retries.

The trace lifecycle is the exception to ambient targeting: a profile describes
one tab, so `analyze/performance_trace` and `analyze/react_profile` pass an
explicit `tab_id` to both `start` and `stop`. That keeps the two halves on the
same target and makes a `performance_trace_target_not_debuggable` refusal name
the tab the run meant to profile, instead of silently producing an artifact for
some other page. The refusal is not retried — Chrome refuses the same target
every time.

Actions gated on a browser permission no script can grant are classified
`permission_gated` rather than `success`. `interact/clipboard_read` passes on a
granted read or on any bounded, named permission outcome, and fails on a generic
`execution_timeout` or an unclassified body.

The shared WebSocket codec encodes extended lengths through the standard
big-endian primitives. Boundary tests cover short, 16-bit, and 64-bit headers,
while the security gate rejects unchecked narrowing conversions.

## TL;DR

- Status: in-progress
- Tool: interact, generate
- Mode/Action: execute_js, test
- Location: `docs/features/feature/self-testing`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_SELF_TESTING_001
- FEATURE_SELF_TESTING_002
- FEATURE_SELF_TESTING_003

## Code and Tests

`internal/capturefixture` is the reviewed exception to the zero-new-export
ratchet for this change. Its eight narrow helpers are test-only consumers of
canonical capture boundaries: sync-backed settings and connection state,
cached Pilot state, tracked-tab updates, and explicit disconnect lifecycle.
The package is not imported by release binaries and replaces a larger set of
unsafe mutation methods that previously compiled into `internal/capture`.

- An action whose arguments name fixture DOM establishes the fixture first.
  Nineteen actions did not: `interact/highlight` looked for `#sf-btn` on whatever
  page a preceding navigate had left up, and `observe/indexeddb` seeded its
  database against that page's origin. Whether they passed depended on execution
  order, so the category failed on a *different* pair of actions in every run.
  The gate is data-driven — any argument naming a fixture selector triggers a
  re-navigation before the action is prepared — and a contract test fails if a
  new fixture selector is not recognised by it. This costs roughly 100 seconds
  on the sweep, which buys determinism: the re-navigation also discards DOM the
  previous action mutated, so no action inherits another's leftovers.
- Chrome refusing `chrome.debugger` access to a target it treats as another
  extension's page is a security boundary working as intended — an extension
  able to debug another extension's page would be the defect. Such a refusal is
  classified and skipped, not failed; any other error from the same action still
  fails, and the refusal never enters the retry set because Chrome refuses the
  same target every time.
- A test daemon never outlives the process that started it. `t.Cleanup` stops
  each daemon on the normal path, but it does not run when the test binary is
  killed outright — a `go test` timeout, a cancelled CI job, Ctrl-C — so the
  daemon supervises its owner instead: the harness passes `KABOOM_TEST_OWNER_PID`
  and the daemon exits once that pid disappears. The variable is unset in
  production, where the watchdog is inert.
- The sweeper matches versioned process titles. Daemons rewrite their own title
  to include a compact version tag (`kaboom-test-binary-090`), so the previous
  pattern `kaboom-test-binary --daemon` — which requires a space after the base
  name — selected nothing. The cleaner ran after every category and still left
  twelve daemons alive for twenty hours, each holding a port and a state
  directory. A contract test now checks the patterns against a versioned title
  and asserts none of them can match the production daemon.
- A strict "one test daemon at a time" is deliberately *not* enforced: `go test`
  runs the four integration packages in parallel, and
  `TestBridgeStartupContention_AllClientsConverge` races concurrent clients on
  purpose. The guarantee is instead one daemon per port, zero orphans, and a
  sweeper that can actually see them.
- Every category script on disk is scheduled in a suite. A script that exists but
  appears in neither `OFFLINE_CAT_IDS` nor `CONNECTED_CAT_IDS` reads as coverage
  while never running: seven were in that state, and one of them reported 8/8
  green with every call it made failing to parse. A contract test derives the
  expected set from the filesystem, so a new category cannot be added without
  being scheduled. Category 27 is the single permitted exclusion — it blocks on
  `read -r` for human visual verification of overlays and would hang the suite.
- Tool requests must fit on one line. `send_mcp` pipes the request through
  `echo` into a line-delimited JSON-RPC reader, so a multi-line argument returns
  `Parse error: unexpected end of JSON input` and never reaches a tool. That
  envelope carries no `result.isError`, so `check_not_error` reports success and
  the test continues as though the call worked. Twenty tests across five
  categories were built this way; category 32 reported 8/8 green while every one
  of its calls failed, and was deleted rather than rewritten. Categories 22 and
  31 failed loudly instead of lying, so their requests were repaired in place and
  now fail with the real cause rather than an empty body.
- Assertion falsifiability is a contract, enforced by
  `tests/cli/uat-assertions/assertion-falsifiability.test.cjs`. Every UAT test
  that can report pass must also be able to report fail; a block that can only
  skip is a precondition gate and is exempt. The check follows calls into
  category-local and framework helpers, so delegating an assertion to
  `expect_http_status` still counts. Two further rules apply to `/sync`: tests
  must assert an HTTP status, and no test may gate on bare JSON validity. The
  daemon answers 400 (invalid JSON), 403 (bad client header) and 409 (stale
  generation) with well-formed JSON bodies, so `jq .` succeeds on every one of
  them — cat-16.3 concluded from a 403 that the server does *not* validate the
  client header, and passed in both branches.
- Client identity decides where a category can run. `kaboom-probe` requests are
  answered with a canned empty envelope that adopts no session, settings, or
  commands, so a probe cannot prove a payload was applied. Categories 14 and 16
  need that proof, so they claim the extension identity and run offline where no
  browser is attached; connected categories must never post as the extension,
  directly or through the `post_extension`/`post_logs` helpers.
- `send_mcp` captures stdout/stderr into per-call temporary files. A backgrounded
  subshell inherits `MCP_ID` unchanged (bash 3.2 has no `BASHPID` and `$$` is
  shared), so concurrent callers previously interleaved into one file and read
  back a corrupted response; concurrency tests could not have worked.
- Smoke runner lifecycle and post-run daemon availability: `scripts/uat/runners/smoke-test.sh`, `scripts/smoke-tests/harness/framework-smoke.sh`
- Smoke module contracts for push/upload/framework resilience/stability: `scripts/smoke-tests/interact/14-browser-push.sh`, `scripts/smoke-tests/upload/15-file-upload.sh`, `scripts/smoke-tests/framework/29-framework-selector-resilience.sh`, `scripts/smoke-tests/core/30-stability-shutdown.sh`
- Split UAT orchestration + integrity checks: `scripts/uat/runners/test-all-split.sh`, `scripts/uat/runners/test-original-uat.sh`, `scripts/uat/runners/test-new-uat.sh`
- Shared UAT result parsing: `scripts/uat/orchestration/uat-result-lib.sh`
- Connected action coverage classifies every live five-tool action and supports
  `KABOOM_UAT_ACTION=tool/mode` for isolated reproduction without inheriting
  state from unrelated actions. Structured extension failures retain their
  error code and lifecycle-stage message instead of truncating the surrounding
  command envelope, so order-dependent failures remain actionable.
- Stateful focused actions prepare their prerequisites before materializing
  arguments; event-recording stop therefore proves a real start/ID/stop
  lifecycle instead of using a placeholder identifier.
- Isolated history actions create real browser history: `back` navigates away
  from the deterministic fixture, while `forward` performs that navigation and
  returns with `back` before exercising the forward transition.
- Comprehensive aggregation reports pass/fail/skip counts and fails closed when
  any selected category omits or corrupts its structured result file.
- `make uat` runs both suite boundaries and emits canonical
  `artifacts/uat/uat-results.json` and `uat-results.xml` reports. Both contain
  every selected category, skip reasons, durations, prerequisite readiness,
  aggregation completeness, and user-state restoration status.
- `make check-structure` enforces runtime-context dependency direction,
  ratcheted public-surface budgets, the 800-line and 10-file physical limits,
  dormant-test detection, circular dependency reporting, and zero non-trivial
  clones across background/popup.
- Persisted-state owner tests share the stable `internal/statefault` fixture
  vocabulary for read, write, sync, rename, directory-sync, quota, corruption,
  partial-write, cancellation, and restart failures. Scenarios are deterministic,
  never retain private test sentinels, and expose only redacted classified errors.
  Key-value state consumers use one canonical wrapper which maps all ten faults
  to read/write/delete behavior without duplicating per-feature fake stores.
- Go unit/race checks and real-binary lifecycle checks run as separate,
  parallel CI jobs. Subprocess suites carry the `integration` build tag, while
  the named integration job runs every tagged transport, persistence,
  contention, reliability, and stdio contract plus the fast-start soak.
- The focused MCP transport smoke gate also enables the `integration` build
  tag. Its real bridge processes close stdin and await the explicit process-exit
  barrier; permanently skipped test-binary simulations and timing sleeps are
  not accepted as transport coverage.
- Category daemon lifecycle and result-file contract: `scripts/tests/framework/framework.sh`
- User-state guard: `scripts/tests/framework/uat-user-state.sh` snapshots the
  prior daemon executable, LaunchAgent lifecycle, version, and tracked tab
  before connected UAT, then restores them idempotently on completion or signal.
  Port cleanup is listener-only, so browser processes connected to the daemon
  are never mistaken for port owners or terminated.
- Comprehensive UAT has explicit `offline`, `connected`, and `all` suites.
  Offline contracts use an isolated daemon port; connected-browser categories
  run sequentially on the extension's configured port. The connected preflight
  and every category daemon start use the same bounded readiness barrier for
  daemon health, extension heartbeat, and a tracked browser tab, so startup
  races cannot be misreported as feature skips. Run either boundary with
  `--suite offline|connected`.
- Connected UAT opens and tracks a dedicated local test-harness tab rather than
  navigating the user's tab. Cleanup closes that tab before restoring the
  exact prior daemon and tracked-tab state, including signal-driven exits. Once
  explicit restoration completes, the later shell `EXIT` trap is a no-op so it
  cannot kill the restored daemon on the shared connected-suite port.
- Connected action coverage derives every live tool action from `tools/list`,
  runs it against the dedicated tracked fixture tab, and fails closed if a
  schema action is omitted. State-sensitive actions re-establish the fixture
  page through the readiness barrier, intent actions use isolated scopes,
  CDP actions carry the tracked tab explicitly, and visual comparisons create
  their prerequisite baseline. The connected preflight uses isolated daemon state
  so malformed user persistence cannot make the test environment
  nondeterministic; production persistence recovery is validated separately.
- The extension's literal background command registrations generate one shared
  SHA-256 command-contract identity for Go and TypeScript. Every sync reports
  that identity, and CI rejects stale generated outputs. A connected mismatch
  fails before queueing instead of allowing a command to disappear inside a
  same-version daemon/extension build skew.
- Packaged corruption recovery UAT builds and installs the public npm launcher
  plus its current-platform binary package, starts that artifact with isolated
  corrupt fixtures for every daemon-owned state family, and verifies startup,
  deterministic fallback, Doctor lifecycle history, and raw-value redaction.
- UAT category suites: `scripts/tests/cat-*.sh`
- HTTP fixtures and embedded test pages: `cmd/browser-agent/internal/testpages/http.go`
- WebSocket harness: `cmd/browser-agent/internal/testpages/websocket.go`
- RFC 6455 frame codec (shared with the terminal relay): `cmd/browser-agent/internal/wsframe/frame.go`
- Behavior tests: `cmd/browser-agent/internal/testpages/testpages_test.go`, `cmd/browser-agent/internal/wsframe/frame_test.go`

## The reachability ratchet runs without a browser

`cat-33` refuses to let the count of reachability-only modes grow, but cat-33
needs a connected extension and only 1 of 34 UAT categories runs in CI, so that
ratchet gated nothing: 11 modes shipped past a baseline of 131 without anyone
noticing.

`scripts/contracts/uatcoverage` moves the check onto every commit. It reads the
same two checked-in files the sweep reads — the shipped tool schema and
`mode-content-expectations.sh` — so it needs no browser, and it holds three things:

- The baseline equals the real count **exactly**. Slack is a mode's worth of
  free coverage: with the baseline above the count, the next mode added with no
  expectation passes both gates. Lowering it is how an improvement is locked in.
- Every expectation names a mode that still ships. A stale entry is worse than a
  missing one — it is counted as coverage for a mode that no longer exists.
- Every reachability-only mode has a human UAT case, so a mode with no automated
  content assertion is at least judged by a person.

Content expectations are shape assertions: they prove the handler emitted its
documented collection rather than an error or a bare success envelope. They do
not prove the collection holds the right things. That is what the human rig
asks (`docs/features/feature/human-uat-rig/`).

## Which tests does this change reach?

Two branches reported green gates and broke four tests on merge, all in files
their hand-written globs did not cover. The instruction not to run the whole JS
suite concurrently is what forced the scoping, so the fix is not "run more":

```bash
make affected                  # list the tests this branch's change reaches
make test-affected             # run exactly those
make affected BASE=some-ref    # against a different base
```

`scripts/quality/affected` answers "which suites import this module?" by walking
every path-shaped string literal in the tree. That is deliberately an
over-approximation: tests here import through variables (`await import(CDP)`) as
often as through literals, and a selector that missed one would reintroduce the
failure it exists to prevent. `src/**.ts` maps to `extension/**.js`, because
nothing under `tests/` mentions `src/` — without that mapping a TypeScript edit
would select nothing at all and every branch would be green.

Two escape hatches keep it honest:

- **Untraceable changes run everything.** A Makefile, a config, a golden or a
  baseline can change behaviour everywhere while importing nothing, so their
  presence forces the full suite with the file named. A selector that silently
  returns nothing is worse than no selector.
- **`always-run.json`** lists tests an import graph cannot see — the ones that
  mirror production wiring by hand. A dependency added in `init.ts` changes what
  a route delivers without changing anything the mirror imports; that is exactly
  how the `enhanced_action` route was silently killed in two files. Each entry
  states why it cannot be traced, and a contract test fails if an entry names a
  file that does not exist or carries no reason.
