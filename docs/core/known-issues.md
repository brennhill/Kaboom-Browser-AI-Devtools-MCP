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

### Found by the coverage pass (2026-07-23) — all resolved

Each was first pinned by a test asserting the **broken** behaviour (none had a
test before), then that test was rewritten to assert the correct behaviour so it
failed against the unfixed code and passed after the fix. All are now resolved.

| # | Issue | Where | Severity |
|---|-------|-------|----------|
| 4 | ~~`fill_form` cannot blank a field. `{"selector":"#note","value":""}` is rejected before dispatch — the step always sends `action:"type"` and validation requires non-empty text — even though `clear:true` is sent.~~ **Resolved** — `type` with `clear:true` and empty text is now recognised as the "empty this input" intent and skips the required-text rule. Pinned by `TestFillForm_EmptyValueClearsTheField`. | `toolinteract/interact_dom.go`, `interact_dom_validation.go` | ~~Medium~~ |
| 5 | ~~`timeout_ms` is dead on both form workflows: parsed, defaulted to 15s, never read.~~ **Resolved** — `timeout_ms` now bounds the whole workflow (same total-budget rule as the navigate workflows); an exhausted budget fails naming the step it was on. Pinned by `TestFillForm_TimeoutBudgetStopsTheWorkflowAtTheOffendingStep` and `TestFillFormAndSubmit_TimeoutBudgetStopsBeforeSubmitting`. | `toolinteract/interact_workflow_forms.go` | ~~Medium~~ |
| 6 | ~~`continue_on_error:false` does not stop on extension-side failures. The `break` is inside the tool-response error branch, so a step whose tool call succeeded but whose correlated command resolved to `error` is counted as failed while the batch keeps running.~~ **Resolved** — the break now keys off the resolved step status, covering tool-side and extension-side failures alike. Pinned by `TestBatch_ContinueOnErrorFalseStopsOnExtensionSideFailure`. | `toolinteract/interact_batch.go` | ~~Medium~~ |
| 7 | ~~`extractErrorMessage` never unwraps a standard `fail()` response, so a batch step's `error` is the whole formatted banner rather than the `message` field.~~ **Resolved** — a shared `stripFailSummaryLine` helper (used by both extractors) drops the summary line before parsing. Pinned by `TestExtractErrorMessage` and `TestExtractErrorMessage_UnwrapsStandardFailResponses`. | `toolinteract/interact_batch.go` | ~~Low~~ |
| 8 | ~~`HandleStateLoad` reports `state_restore:"queued"` with an empty `restore_correlation_id` when the restore enqueue was rejected — the caller is told a restore is in flight when none is.~~ **Resolved** — a rejected enqueue now reports `state_restore:"rejected_enqueue_failed"` with a `restore_detail` reason and no correlation id. Pinned by `TestStateCov_StateLoad_BlockedRestoreEnqueueIsReportedHonestly`. | `toolinteract/interact_state_save_load.go` | ~~Medium~~ |
| 9 | ~~`scroll_position` is captured and persisted but is not part of the `hasData` check, so a snapshot containing only a scroll position reports `skipped_no_state_data` and is never restored.~~ **Resolved** — `scroll_position` now counts toward `hasData`. Pinned by `TestStateCov_StateLoad_ScrollPositionAloneCountsAsData` (and the empty-container path kept by `TestStateCov_StateLoad_EmptyContainersCountAsNoData`). | `toolinteract/interact_state_save_load.go` | ~~Low~~ |
| 10 | ~~`pruneRetryStatesLocked` evicts exactly one entry per call, so the map caps at `maxEntries` but never trims.~~ **Resolved** — it now trims oldest-first all the way to the cap in one call (deterministic tie-break by key). Pinned by `TestStateCov_PruneRetryStates_TrimsAllTheWayToTheCap`. | `toolinteract/interact_retry_contract_state.go` | ~~Low~~ |
| 11 | ~~`NormalizeTelemetryMode` validates the trimmed value and returns the untrimmed one, so `telemetry_mode:"  off  "` is accepted and stored padded.~~ **Resolved** — it now returns the trimmed value it validated. Pinned by `TestNormalizeTelemetryMode` and `TestHandleTelemetry_PaddedModeIsStoredTrimmed`. | `toolconfigure/telemetry.go` | ~~Medium~~ |
| 12 | ~~`NormalizeInteractFailureCode` ranges a map, so an error string containing two known codes resolves to an arbitrary playbook per run.~~ **Resolved** — documented rule: the code matching earliest in the error string wins, longest code breaks a tie at the same offset. Pinned by `TestNormalizeInteractFailureCode_MultiCodeStringResolvesToEarliestMatch`, `_MultiCodeStringIsDeterministicAcrossCalls`, and `_TieAtSameOffsetPrefersLongestCode`. | `playbooks/interact_failure_playbooks.go` | ~~Medium~~ |
| 13 | ~~`AppendPushPiggyback` drains the inbox before it can use the events, so if `resp.Result` fails to unmarshal the events are lost.~~ **Resolved** — the response is parsed before the drain, so an unattachable batch stays queued for the next call. Pinned by `TestAppendPushPiggyback_UnparseableResponseLeavesEventsInTheInbox`. | `toolobserve/inbox.go` | ~~Medium~~ |
| 14 | ~~`AuditInfo.ErrorRatePct` is unclamped — a tool recording 2 errors against 1 request reports `200`.~~ **Resolved** — the rate is clamped to 100 for the dashboard while raw counts stay unclamped so the discrepancy is still visible. Pinned by `TestMetrics_BuildAuditInfo_ErrorRateSaturatesAt100` and `TestMetrics_BuildAuditInfo_ErrorRateIsExactBelowTheClamp`. | `health/response_builders.go` | ~~Low~~ |
| 15 | ~~Dead branch: the pilot `default:` arm's "AI Web Pilot is disabled" warning is unreachable, since `explicitly_disabled` and `assumed_enabled` both have explicit cases.~~ **Resolved** — `enabled` has its own `pass` case and the `default:` now reports an *unrecognised* pilot state (a real fallback). Pinned by `TestRunDoctorChecks_EnabledPilotPassesWithNoFix` and `TestRunDoctorChecks_EveryPilotStateHasItsOwnDistinctWording`. | `health/doctor_live_checks.go` | ~~Low~~ |
| 16 | ~~The Kaboom workspace tab-group feature has never run: `chrome.tabGroups` is used but `"tabGroups"` is not in the manifest, so the namespace is `undefined` and the guarded call silently returns null.~~ **Resolved (with a release note)** — `"tabGroups"` added to the manifest. ⚠️ Adding a *required* permission makes Chrome disable the extension on update until the user re-approves; before the next Web Store release, decide whether to keep it required or move to `optional_permissions` with a runtime request. | `src/background/tab-state.ts` + `extension/manifest.json` | ~~Medium~~ |

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
