---
feature: effect-verification
status: shipped
tool: interact
doc_type: qa-plan
feature_id: feature-effect-verification
last_reviewed: 2026-09-05
---

# QA Plan: Effect Verification

## Automated coverage

### Window boundary — `cmd/browser-agent/internal/actioneffects/effects_test.go`

Every test drives a fake clock through an injected `Wait`; nothing sleeps.

| Behaviour | Test |
| --- | --- |
| Telemetry recorded before the mark is not attributed | `TestCollectIgnoresEverythingRecordedBeforeTheAction` |
| Telemetry recorded inside the window is attributed, with status and level | `TestCollectAttributesTelemetryRecordedInsideTheWindow` |
| The window closes on the first effect, after one poll | `TestCollectClosesEarlyOnTheFirstObservedEffect` |
| An empty window spends the full budget (6 polls at 300/50) | `TestCollectSpendsTheWholeBudgetWhenNothingHappens` |
| A DOM change answers with zero polls | `TestCollectAnswersWithoutPollingWhenTheDOMAlreadyChanged` |
| A "no DOM changes" report still runs the full window | `TestCollectStillRunsTheWindowWhenTheDOMSaysNothingMoved` |
| Requests are attributed by server ingest time, with method and status | `TestCollectAttributesNetworkRequestsByServerIngestTime` |
| A URL move is reported with both endpoints | `TestCollectReportsNavigationAsAUrlChange` |
| An unchanged URL reports no navigation | `TestCollectOmitsNavigationWhenTheURLHeldStill` |
| Toast/alert classifications are named | `TestCollectNamesTransientsThePageRaised` |
| Counts stay exact while listings are capped | `TestCollectCapsTheEvidenceItReturns` |
| The attribution note names the window and disclaims causation | `TestEffectsDeclareAttributionIsTemporalNotCausal` |
| No readers wired reports unevaluated, not effect-free | `TestCollectWithNoReadersReportsNotEvaluatedRatherThanNoEffect` |

### Classification — `cmd/browser-agent/internal/actioneffects/classify_test.go`

| Behaviour | Test |
| --- | --- |
| All four outcomes, including DOM-only evidence | `TestClassifyNamesTheThreeOutcomes` |
| An unrun window is `not_evaluated` | `TestClassifyReportsAnUnevaluatedWindowRatherThanClaimingNoEffect` |
| The payload carries outcome plus its evidence | `TestPayloadCarriesTheOutcomeAndItsEvidence` |
| Empty listings are omitted; zero counts stay present | `TestPayloadOmitsEmptyEvidenceRatherThanSpendingTokensOnZeroes` |
| No-effect stops the retry and explains it | `TestApplyRetryAdviceStopsARetryOfAnActionThatChangedNothing` |
| Other outcomes write no retry fields | `TestApplyRetryAdviceLeavesOtherOutcomesToTheExistingRetryPolicy` |
| An explicit decision is not overwritten | `TestApplyRetryAdviceNeverOverridesAnExplicitDecision` |
| The DOM report is read at top level, from counts, and nested under `result` | `TestDOMChangeReadsTheSummaryEveryDOMPrimitiveAlreadyReturns` |

### Dispatcher integration — `cmd/browser-agent/internal/interactdispatch/effects_test.go`

| Behaviour | Test |
| --- | --- |
| `kaboom-knms`: a click that changed nothing is named and its retry stopped | `TestDispatchReportsAnActionThatDidNothing` |
| Telemetry inside the window flips the outcome, and no retry advice is written | `TestDispatchReportsAnActionTheTelemetryConfirms` |
| A DOM-only effect counts, with no request and no log | `TestDispatchTrustsTheDOMReportTheExtensionAlreadySends` |
| Read-only actions are charged no window and no latency | `TestDispatchOpensNoWindowForReadOnlyActions` |
| `effects: false` opts out entirely | `TestDispatchOpensNoWindowWhenTheCallerOptsOut` |
| `effect_window_ms` widens the window | `TestDispatchHonoursAWiderWindowWhenAsked` |
| An absurd window is clamped to 5000 ms | `TestDispatchClampsAnAbsurdWindowRatherThanHangingTheCaller` |
| Background mode gets no window | `TestDispatchOpensNoWindowInBackgroundMode` |
| A failed dispatch waits out zero polls and carries no block | `TestDispatchAttachesNoEffectsToAFailedAction` |
| An unwired dispatcher invents nothing | `TestDispatchWithoutEffectWiringLeavesResponsesAlone` |
| A visible DOM change costs zero polls end to end | `TestDispatchSpendsNoWindowWhenTheDOMAlreadyAnswered` |
| Effect-blind actions get no block and no retry advice | `TestDispatchOpensNoWindowForActionsItsSignalsCannotSee` |
| Every exclusion is still a mutation action, so it excludes something | `TestEveryEffectBlindActionIsStillAMutationAction` |

### Payload accessors — `internal/mcp/response_test.go`

| Behaviour | Test |
| --- | --- |
| The JSON after the summary line decodes | `TestReadResultPayloadDecodesTheJSONAfterTheSummaryLine` |
| Unreadable responses are refused, not guessed at | `TestReadResultPayloadRefusesUnreadableResponses` |
| A mutation rewrites only the JSON and keeps the summary | `TestMutateResultPayloadRewritesOnlyTheJSONAndKeepsTheSummary` |
| A no-op mutation leaves the bytes alone | `TestMutateResultPayloadLeavesTheResponseAloneWhenNothingChanged` |
| An unreadable response passes through untouched | `TestMutateResultPayloadPassesUnreadableResponsesThroughUntouched` |
| Blocks after the first (e.g. an appended screenshot) survive | `TestMutateResultPayloadKeepsBlocksAfterTheFirst` |

### CLI — `cmd/browser-agent/internal/cli/parser/commands_test.go`

`TestParseInteractArgs_EffectWindowControls` covers `--no-effects` and `--effect-window-ms`.
`TestEverySchemaPropertyHasACLIFlag` holds `effects` and `effect_window_ms` to having flags.

## Verified manually, not automatically

| Behaviour | How to check |
| --- | --- |
| A click on a dead anchor in a real page reports `dispatched_and_no_observable_effect` | Drive a live tab; the response should carry `dom_changed: false` and `retryable: false`. |
| A click behind a modal overlay reports no effect rather than success | Same, with an overlay covering the target. |
| A click that triggers a 500 reports the status in `network_requests` | Requires a fixture server returning 500. |
| End-to-end latency added to a working click | Should be one poll (≈50 ms), not the full 300 ms. |

## Not covered today

| Gap | Consequence if wrong |
| --- | --- |
| No fixture-page suite exercises the four classifications against a real browser. | The Go tests prove the classifier; nothing yet proves the extension delivers the inputs it expects on a real page. Tracked as part of the human pass/fail UAT rig (`kaboom-nbxn`). |
| Network attribution uses server ingest time, not page-relative start, and covers `fetch()` only. | A late-arriving batch can carry a request that began before the action, reporting an effect the action did not have. A request made through another transport is invisible. |
| Nothing tests the 600 ms default against a real extension's send cadence. | On a slow connection the window can close before the console or action batcher has delivered, reporting no effect for an action that had one. |
| Summary mode strips `dom_summary`, and `dom_changes` is only populated under `analyze: true`. | Under summary mode the DOM signal is silently unavailable; classification falls back to the other four signals and may report no effect for a DOM-only change. |
| `focus` and `scroll_to` are mutating actions with no `withMutationTracking` wrapper. | They always report `DOMUnknown`, so a focus that moved nothing else looks the same as a focus that did. |
| Concurrent actions from two MCP clients share one set of buffers. | Two clients acting at once will attribute each other's telemetry. There is no per-client window. |

## See also

- [Effect Verification index](./index.md)
- [Effect Verification Product Spec](./product-spec.md)
- [Effect Verification Tech Spec](./tech-spec.md)
