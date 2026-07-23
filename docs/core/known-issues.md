---
status: active
scope: issues/blockers
ai-priority: high
tags: [known-issues, v0.7.x]
last-verified: 2026-07-23
canonical: true
---

# Known Issues

## v0.7.x — Current Release

### Open Issues

| # | Issue | Severity | Details |
|---|-------|----------|---------|
| 1 | Extension timeout on first interact() | Medium | Content script may not be fully loaded when first `interact()` command is sent after navigation. **Workaround:** Retry after 2-3 seconds. |
| 2 | Tracking loss during cross-origin navigation | Medium | Extension can lose tab tracking state during AI-initiated cross-origin navigation via `interact({action: "navigate"})`. **Workaround:** Re-enable tracking via extension popup. |
| 3 | ~~Pilot test zombies~~ | ~~Low~~ | **Resolved.** Hardcoded `version: '5.2.0'` no longer present in `tests/extension/pilot-*.test.js`. |

### Found by the coverage pass (2026-07-23), documented not fixed

Each of these is pinned by a test that asserts **current** behaviour and says so,
so fixing one means updating a named test rather than discovering the change by
accident. None had a test before.

| # | Issue | Where | Severity |
|---|-------|-------|----------|
| 4 | `fill_form` cannot blank a field. `{"selector":"#note","value":""}` is rejected before dispatch — the step always sends `action:"type"` and validation requires non-empty text — even though `clear:true` is sent, which is exactly the "empty this input" intent. | `toolinteract/interact_workflow_forms.go` | Medium |
| 5 | `timeout_ms` is dead on both form workflows: parsed, defaulted to 15s, never read. The navigate workflows do use theirs. | `toolinteract/interact_workflow_forms.go` | Medium |
| 6 | `continue_on_error:false` does not stop on extension-side failures. The `break` is inside the tool-response error branch, so a step whose tool call succeeded but whose correlated command resolved to `error` is counted as failed while the batch keeps running. | `toolinteract/interact_batch.go:244` | Medium |
| 7 | `extractErrorMessage` never unwraps a standard `fail()` response, so a batch step's `error` is the whole formatted banner rather than the `message` field it was meant to carry. | `toolinteract/interact_batch.go:81` | Low |
| 8 | `HandleStateLoad` reports `state_restore:"queued"` with an empty `restore_correlation_id` when the restore enqueue was rejected — the caller is told a restore is in flight when none is. | `toolinteract/interact_state_save_load.go` | Medium |
| 9 | `scroll_position` is captured and persisted but is not part of the `hasData` check, so a snapshot containing only a scroll position reports `skipped_no_state_data` and is never restored. | `toolinteract/interact_state_save_load.go` | Low |
| 10 | `pruneRetryStatesLocked` evicts exactly one entry per call, so the map caps at `maxEntries` but never trims. Harmless today because it is called on every store. | `toolinteract/interact_retry_contract_state.go` | Low |
| 11 | `NormalizeTelemetryMode` validates the trimmed value and returns the untrimmed one, so `telemetry_mode:"  off  "` is accepted and stored padded — the stored mode then never compares equal to `"off"`. | `toolconfigure/telemetry.go:798` | Medium |
| 12 | `NormalizeInteractFailureCode` ranges a map, so an error string containing two known codes resolves to an arbitrary playbook per run. **Deliberately not pinned by a test** — any assertion would itself be flaky. | `playbooks/interact_failure_playbooks.go:104` | Medium |
| 13 | `AppendPushPiggyback` drains the inbox before it can use the events, so if `resp.Result` fails to unmarshal the events are lost rather than left for the next call. | `toolobserve/inbox.go:46` | Medium |
| 14 | `AuditInfo.ErrorRatePct` is unclamped — a tool recording 2 errors against 1 request reports `200`. | `health/metrics.go` | Low |
| 15 | Dead branch: the pilot `default:` arm's "AI Web Pilot is disabled" warning is unreachable, since `explicitly_disabled` and `assumed_enabled` both have explicit cases. The duplicate wording is a maintenance hazard. | `health/doctor_live_checks.go:96` | Low |
| 16 | The Kaboom workspace tab-group feature has never run: `chrome.tabGroups` is used but `"tabGroups"` is not in the manifest, so the namespace is `undefined` and the guarded call silently returns null. Adding a required permission on update makes Chrome disable the extension until the user re-approves, so this is a release decision. | `src/background/tab-state.ts` + `extension/manifest.json` | Medium |

### Flaky Tests (Pre-existing)

- `TestAsyncQueueReliability/Slow_polling` — times out at 30s intermittently
- `tests/extension/async-timeout.test.js` — 3 tests flaky

### Fixed in v0.7.x (was v5.8.0)

- Early-patch WebSocket capture — pages creating WS connections before inject script loads now captured
- camelCase to snake_case field mapping for network waterfall entries
- Command results routing through /sync endpoint with proper client ID filtering
- Post-navigation tracking state broadcast for favicon updates
- Empty arrays return `[]` instead of `null` in JSON responses
- Bridge timeouts return proper `extension_timeout` error code

### Fixed in v5.7.x

- Extension health check timeout (5s threshold added)
- Hardcoded version in inject.bundled.js (now reads from VERSION file via esbuild define)
- Stale compiled JS vs TS source
