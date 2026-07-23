/**
 * Purpose: Exposes page-context capture and action/query handlers through the inject runtime facade.
 * Why: Centralizes in-page instrumentation so browser telemetry and automation hooks share one deterministic entrypoint.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 * Docs: docs/features/feature/interact-explore/index.md
 * Docs: docs/features/feature/query-dom/index.md
 */
/**
 * @fileoverview inject.ts - Page-level capture script for browser telemetry.
 * Runs in the page context (not extension sandbox) to intercept console methods,
 * fetch/XHR requests, WebSocket connections, errors, and user actions. Posts
 * captured events to the content script via window.postMessage.
 *
 * This is a barrel/facade file that re-exports from the modular structure in
 * /inject subdirectory. The actual implementation is split into:
 * - inject/api.ts (KaboomAPI interface)
 * - inject/observers.ts (Observer setup and installation)
 * - inject/message-handlers.ts (Content script message handling)
 * - inject/state.ts (State capture/restore)
 * - inject/index.ts (Main orchestration)
 */
export { safeSerialize, getElementSelector, isSensitiveInput, getContextAnnotations, setContextAnnotation, removeContextAnnotation, clearContextAnnotations, getImplicitRole, isDynamicClass, computeCssPath, computeSelectors, recordEnhancedAction, getEnhancedActionBuffer, clearEnhancedActionBuffer, generatePlaywrightScript, recordAction, getActionBuffer, clearActionBuffer, handleClick, handleInput, handleScroll, handleKeydown, handleChange, installActionCapture, uninstallActionCapture, setActionCaptureEnabled, installNavigationCapture, uninstallNavigationCapture, parseResourceTiming, getNetworkWaterfall, trackPendingRequest, completePendingRequest, getPendingRequests, clearPendingRequests, getNetworkWaterfallForError, setNetworkWaterfallEnabled, isNetworkWaterfallEnabled, setNetworkBodyCaptureEnabled, isNetworkBodyCaptureEnabled, shouldCaptureUrl, setServerUrl, sanitizeHeaders, truncateRequestBody, truncateResponseBody, readResponseBody, readResponseBodyWithTimeout, wrapFetchWithBodies, wrapXHRWithBodies, unwrapXHR, adoptEarlyBodies, getPerformanceMarks, getPerformanceMeasures, getCapturedMarks, getCapturedMeasures, installPerformanceCapture, uninstallPerformanceCapture, isPerformanceCaptureActive, getPerformanceSnapshotForError, setPerformanceMarksEnabled, isPerformanceMarksEnabled, postLog, installConsoleCapture, uninstallConsoleCapture, parseStackFrames, parseSourceMap, extractSnippet, extractSourceSnippets, detectFramework, getReactComponentAncestry, captureStateSnapshot, generateAiSummary, enrichErrorWithAiContext, setAiContextEnabled, setAiContextStateSnapshot, setSourceMapCache, getSourceMapCache, getSourceMapCacheSize, installExceptionCapture, uninstallExceptionCapture, getSize, formatPayload, truncateWsMessage, createConnectionTracker, installWebSocketCapture, setWebSocketCaptureMode, setWebSocketCaptureEnabled, getWebSocketCaptureMode, uninstallWebSocketCapture, resetForTesting, executeDOMQuery, getPageInfo, runAxeAudit, runAxeAuditWithTimeout, formatAxeResults, mapInitiatorType, aggregateResourceTiming, capturePerformanceSnapshot, installPerfObservers, uninstallPerfObservers, getLongTaskMetrics, getFCP, getLCP, getCLS, getINP, sendPerformanceSnapshot, isPerformanceSnapshotEnabled, setPerformanceSnapshotEnabled, MAX_WATERFALL_ENTRIES, MAX_PERFORMANCE_ENTRIES, SENSITIVE_HEADERS, installKaboomAPI, uninstallKaboomAPI, install, uninstall, wrapFetch, installFetchCapture, uninstallFetchCapture, installXHRCapture, uninstallXHRCapture, installPhase1, installPhase2, getDeferralState, setDeferralEnabled, shouldDeferIntercepts, checkMemoryPressure, installMessageListener, executeJavaScript, safeSerializeForExecute, captureState, restoreState, highlightElement, clearHighlight, type KaboomAPI, type DeferralState, type RestoreStateResult, type RestoredCounts, type HighlightResult } from './inject/index.js';
//# sourceMappingURL=inject.d.ts.map