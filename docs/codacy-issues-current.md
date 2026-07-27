---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-02-16
---

# Codacy Issue Analysis - 2026-02-10 09:42:51

## Executive Summary

- **Total Issues**: 100
- **Security Issues**: 0 ✅ (All fixed!)
- **Code Quality Issues**: 100 (Complexity & Line Count)

### Status of Previous Fixes

✅ **G601 (SSRF False Positive)** - RESOLVED
- Files: connect_mode.go (line 78, 88)
- Fix: Added #nosec G601 comments
- Status: No longer appears in Codacy

✅ **G204 (Command Injection False Positive)** - RESOLVED  
- Files: connection_lifecycle_test.go (line 295, 480), server_persistence_test.go (line 50)
- Fix: Added #nosec G204 comments
- Status: No longer appears in Codacy

✅ **CVE-2024-34155** - RESOLVED
- File: go.mod
- Fix: Updated from `go 1.23` to `go 1.23.1`
- Status: No longer appears in Codacy

---

## Remaining Issues by Type

### Complexity Issues (Lizard_ccn-medium): 68 issues
Cyclomatic complexity warnings - functions with too many decision paths

### Line Count Issues (Lizard_nloc-medium): 24 issues  
Methods with too many lines of code (limit: 50 lines)

### File Line Count Issues (Lizard_file-nloc-medium): 8 issues
Files with too many lines of code (limit: 600 lines)

---

## Detailed Issue List


### Lizard_ccn-medium (68 issues)

**cmd/browser-agent/alerts.go** (1 issues)
- Line 291: Method handleCIWebhook has a cyclomatic complexity of 17 (limit is 8)

**cmd/browser-agent/bridge_faststart_test.go** (1 issues)
- Line 18: Method TestFastStart_InitializeRespondsImmediately has a cyclomatic complexity of 15 (limit is 8)

**cmd/browser-agent/mcp_protocol_test.go** (1 issues)
- Line 512: Method TestMCPProtocol_ToolsListStructure has a cyclomatic complexity of 10 (limit is 8)

**cmd/browser-agent/reproduction.go** (1 issues)
- Line 347: Method describeElement has a cyclomatic complexity of 13 (limit is 8)

**cmd/browser-agent/server_reliability_integration_test.go** (1 issues)
- Line 36: Method TestReliability_MCPTraffic_RealisticSession has a cyclomatic complexity of 11 (limit is 8)

**cmd/browser-agent/server_reliability_test.go** (1 issues)
- Line 234: Method TestReliability_ResourceLeaks_Goroutines has a cyclomatic complexity of 11 (limit is 8)

**cmd/browser-agent/server_routes.go** (1 issues)
- Line 45: Method handleScreenshot has a cyclomatic complexity of 18 (limit is 8)

**cmd/browser-agent/stdio_silence_test.go** (1 issues)
- Line 167: Method TestStdioSilence_MultiClientSpawn has a cyclomatic complexity of 12 (limit is 8)

**cmd/browser-agent/tools_generate.go** (1 issues)
- Line 114: Method toolGenerateCSP has a cyclomatic complexity of 12 (limit is 8)

**cmd/browser-agent/tools_observe.go** (1 issues)
- Line 144: Method  has a cyclomatic complexity of 13 (limit is 8)

**cmd/browser-agent/tools_observe_analysis.go** (1 issues)
- Line 100: Method findIgnoreCase has a cyclomatic complexity of 9 (limit is 8)

**cmd/browser-agent/tools_observe_bundling.go** (1 issues)
- Line 11: Method toolGetErrorBundles has a cyclomatic complexity of 27 (limit is 8)

**internal/ai/ai_checkpoint_compute.go** (1 issues)
- Line 185: Method computeWebSocketDiff has a cyclomatic complexity of 11 (limit is 8)

**internal/ai/ai_persistence.go** (1 issues)
- Line 468: Method LoadSessionContext has a cyclomatic complexity of 18 (limit is 8)

**internal/analysis/api_contract_analysis.go** (1 issues)
- Line 16: Method detectErrorSpike has a cyclomatic complexity of 11 (limit is 8)

**internal/analysis/api_contract_test.go** (1 issues)
- Line 1195: Method TestAPIContractAnalyze_ViolationTimestamps has a cyclomatic complexity of 10 (limit is 8)

**internal/analysis/thirdparty.go** (1 issues)
- Line 247: Method buildThirdPartyEntry has a cyclomatic complexity of 29 (limit is 8)

**internal/buffers/ring_buffer.go** (1 issues)
- Line 272: Method  has a cyclomatic complexity of 11 (limit is 8)

**internal/capture/recording.go** (1 issues)
- Line 224: Method ListRecordings has a cyclomatic complexity of 9 (limit is 8)

**internal/capture/websocket-streaming_test.go** (1 issues)
- Line 424: Method TestRecordingPersistToDisk has a cyclomatic complexity of 10 (limit is 8)

**internal/pagination/pagination.go** (1 issues)
- Line 365: Method SerializeActionEntryWithSequence has a cyclomatic complexity of 13 (limit is 8)

**internal/performance/performance_test.go** (1 issues)
- Line 1529: Method TestFormatPerformanceReportNoBaseline has a cyclomatic complexity of 16 (limit is 8)

**internal/security/security.go** (1 issues)
- Line 88: Method Scan has a cyclomatic complexity of 11 (limit is 8)

**internal/security/security_diff.go** (4 issues)
- Line 638: Method buildEphemeralSnapshot has a cyclomatic complexity of 13 (limit is 8)
- Line 550: Method compareTransport has a cyclomatic complexity of 14 (limit is 8)
- Line 389: Method compareCookies has a cyclomatic complexity of 20 (limit is 8)
- Line 120: Method TakeSnapshot has a cyclomatic complexity of 18 (limit is 8)

**internal/security/sri.go** (1 issues)
- Line 75: Method Generate has a cyclomatic complexity of 29 (limit is 8)

**internal/session/sessions_test.go** (1 issues)
- Line 65: Method TestSessionManager_CaptureSnapshot has a cyclomatic complexity of 11 (limit is 8)

**src/background/sync/batchers.ts** (1 issues)
- Line 125: Method flushWithCircuitBreaker has a cyclomatic complexity of 10 (limit is 8)

**src/background/caches/cache-limits.ts** (1 issues)
- Line 147: Method estimateBufferMemory has a cyclomatic complexity of 9 (limit is 8)

**src/background/connection-state.ts** (1 issues)
- Line 192: Method computeNextState has a cyclomatic complexity of 31 (limit is 8)

**src/background/dom-primitives.ts** (3 issues)
- Line 470: Method resolveElement has a cyclomatic complexity of 11 (limit is 8)
- Line 584: Method sendAsyncResult has a cyclomatic complexity of 21 (limit is 8)
- Line 63: Method isVisible has a cyclomatic complexity of 22 (limit is 8)

**src/background/caches/error-groups.ts** (1 issues)
- Line 101: Method processErrorGroup has a cyclomatic complexity of 10 (limit is 8)

**src/background/event-listeners.ts** (1 issues)
- Line 203: Method chrome.storage.onChanged.addListener has a cyclomatic complexity of 10 (limit is 8)

**src/background/index.ts** (1 issues)
- Line 451: Method error has a cyclomatic complexity of 17 (limit is 8)

**src/background/pending-queries.ts** (1 issues)
- Line 710: Method handleBrowserAction has a cyclomatic complexity of 22 (limit is 8)

**src/background/recording.ts** (2 issues)
- Line 402: Method stopRecording has a cyclomatic complexity of 11 (limit is 8)
- Line 518: Method (anonymous) has a cyclomatic complexity of 11 (limit is 8)

**src/background/caches/snapshots.ts** (1 issues)
- Line 261: Method findOriginalLocation has a cyclomatic complexity of 20 (limit is 8)

**src/content/favicon-replacer.ts** (1 issues)
- Line 24: Method initFaviconReplacer has a cyclomatic complexity of 9 (limit is 8)

**src/content/runtime-message-listener.ts** (2 issues)
- Line 186: Method showSubtitle has a cyclomatic complexity of 11 (limit is 8)
- Line 304: Method (anonymous) has a cyclomatic complexity of 31 (limit is 8)

**src/content/window-message-listener.ts** (1 issues)
- Line 22: Method (anonymous) has a cyclomatic complexity of 18 (limit is 8)

**src/inject/api.ts** (1 issues)
- Line 276: Method setInputValue has a cyclomatic complexity of 16 (limit is 8)

**src/inject/message-handlers.ts** (1 issues)
- Line 331: Method window.addEventListener has a cyclomatic complexity of 17 (limit is 8)

**src/inject/state.ts** (2 issues)
- Line 47: Method Date.now has a cyclomatic complexity of 12 (limit is 8)
- Line 95: Method restoreState has a cyclomatic complexity of 14 (limit is 8)

**src/lib/ai-context/ai-context.ts** (5 issues)
- Line 266: Method extractSnippet has a cyclomatic complexity of 10 (limit is 8)
- Line 235: Method parseSourceMap has a cyclomatic complexity of 9 (limit is 8)
- Line 591: Method (anonymous) has a cyclomatic complexity of 17 (limit is 8)
- Line 387: Method getReactComponentAncestry has a cyclomatic complexity of 13 (limit is 8)
- Line 437: Method captureStateSnapshot has a cyclomatic complexity of 22 (limit is 8)

**src/lib/page/bridge.ts** (1 issues)
- Line 45: Method || has a cyclomatic complexity of 13 (limit is 8)

**src/lib/analysis/dom-queries.ts** (1 issues)
- Line 190: Method || has a cyclomatic complexity of 17 (limit is 8)

**src/lib/page/exceptions.ts** (1 issues)
- Line 31: Method window.onerror has a cyclomatic complexity of 11 (limit is 8)

**src/lib/analysis/link-health.ts** (1 issues)
- Line 42: Method checkLinkHealth has a cyclomatic complexity of 18 (limit is 8)

**src/lib/net/network.ts** (1 issues)
- Line 427: Method (anonymous) has a cyclomatic complexity of 9 (limit is 8)

**src/lib/page/reproduction.ts** (3 issues)
- Line 305: Method Date.now has a cyclomatic complexity of 25 (limit is 8)
- Line 176: Method computeSelectors has a cyclomatic complexity of 29 (limit is 8)
- Line 378: Method generatePlaywrightScript has a cyclomatic complexity of 11 (limit is 8)

**src/lib/page/serialize.ts** (2 issues)
- Line 27: Method getAttribute? has a cyclomatic complexity of 18 (limit is 8)
- Line 143: Method isSensitiveInput has a cyclomatic complexity of 18 (limit is 8)

**src/lib/net/websocket.ts** (1 issues)
- Line 380: Method Array.from.sort has a cyclomatic complexity of 9 (limit is 8)

**src/popup.ts** (1 issues)
- Line 61: Method updateConnectionStatus has a cyclomatic complexity of 16 (limit is 8)

**src/popup/tab-tracking.ts** (1 issues)
- Line 24: Method (anonymous) has a cyclomatic complexity of 10 (limit is 8)


### Lizard_file-nloc-medium (8 issues)

**cmd/browser-agent/tools_interact_upload_test.go** (1 issues)
- Line 1: File cmd/browser-agent/tools_interact_upload_test.go has 549 non-comment lines of code

**internal/capture/websocket_test.go** (1 issues)
- Line 1: File internal/capture/websocket_test.go has 770 non-comment lines of code

**internal/queries/tab_targeting_test.go** (1 issues)
- Line 1: File internal/queries/tab_targeting_test.go has 521 non-comment lines of code

**src/background/dom-primitives.ts** (1 issues)
- Line 1: File src/background/dom-primitives.ts has 542 non-comment lines of code

**src/background/index.ts** (1 issues)
- Line 1: File src/background/index.ts has 504 non-comment lines of code

**src/background/pending-queries.ts** (1 issues)
- Line 1: File src/background/pending-queries.ts has 803 non-comment lines of code

**src/inject/message-handlers.ts** (1 issues)
- Line 1: File src/inject/message-handlers.ts has 580 non-comment lines of code

**src/lib/net/websocket.ts** (1 issues)
- Line 1: File src/lib/net/websocket.ts has 523 non-comment lines of code


### Lizard_nloc-medium (24 issues)

**cmd/browser-agent/alerts.go** (1 issues)
- Line 291: Method handleCIWebhook has 65 lines of code (limit is 50)

**cmd/browser-agent/main_connection.go** (1 issues)

**cmd/browser-agent/mcp_protocol_test.go** (1 issues)
- Line 512: Method TestMCPProtocol_ToolsListStructure has 58 lines of code (limit is 50)

**cmd/browser-agent/tools_interact_rich_test.go** (1 issues)
- Line 453: Method TestRichAction_DomSummaryPassthrough has 51 lines of code (limit is 50)

**cmd/kaboom-cmd/commands/configure.go** (1 issues)
- Line 8: Method ConfigureArgs has 64 lines of code (limit is 50)

**cmd/kaboom-cmd/commands/interact.go** (1 issues)
- Line 28: Method InteractArgs has 80 lines of code (limit is 50)

**internal/ai/ai_noise.go** (1 issues)
- Line 466: Method loadPersistedRules has 70 lines of code (limit is 50)

**internal/analysis/thirdparty.go** (1 issues)
- Line 247: Method buildThirdPartyEntry has 96 lines of code (limit is 50)

**internal/capture/correlation_tracking_test.go** (1 issues)
- Line 100: Method TestCorrelationIDListCommands has 51 lines of code (limit is 50)

**internal/capture/websocket-streaming_test.go** (1 issues)
- Line 1278: Method TestLogDiffCategorize has 59 lines of code (limit is 50)

**internal/pagination/cursor_test.go** (1 issues)
- Line 179: Method TestCursor_IsOlder has 73 lines of code (limit is 50)

**internal/pagination/pagination_test.go** (1 issues)
- Line 283: Method TestApplyLogCursorPagination_BeforeCursor has 53 lines of code (limit is 50)

**internal/performance/performance_test.go** (1 issues)
- Line 17: Method TestPerformanceSnapshotJSONShape has 67 lines of code (limit is 50)

**internal/security/sri.go** (1 issues)
- Line 75: Method Generate has 110 lines of code (limit is 50)

**internal/server/main_handlers.go** (1 issues)
- Line 151: Method handleScreenshot has 59 lines of code (limit is 50)

**src/background/connection-state.ts** (1 issues)
- Line 192: Method computeNextState has 91 lines of code (limit is 50)

**src/background/dom-primitives.ts** (1 issues)
- Line 584: Method sendAsyncResult has 70 lines of code (limit is 50)

**src/background/pending-queries.ts** (1 issues)
- Line 710: Method handleBrowserAction has 94 lines of code (limit is 50)

**src/background/recording.ts** (1 issues)
- Line 402: Method stopRecording has 53 lines of code (limit is 50)

**src/content/favicon-replacer.ts** (1 issues)
- Line 24: Method initFaviconReplacer has 61 lines of code (limit is 50)

**src/content/runtime-message-listener.ts** (2 issues)
- Line 304: Method (anonymous) has 68 lines of code (limit is 50)
- Line 186: Method showSubtitle has 81 lines of code (limit is 50)

**src/inject/api.ts** (1 issues)
- Line 276: Method setInputValue has 66 lines of code (limit is 50)

**src/popup.ts** (1 issues)
- Line 61: Method updateConnectionStatus has 58 lines of code (limit is 50)
