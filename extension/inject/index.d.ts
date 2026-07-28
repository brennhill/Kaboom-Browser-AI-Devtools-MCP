/**
 * Purpose: Main orchestration and barrel exports for the inject context -- combines API, observers, and message handlers for page-level capture.
 * Docs: docs/features/feature/observe/index.md
 */
/**
 * @fileoverview inject/index.ts - Main orchestration and barrel exports
 * Combines API, observers, and message handlers for page-level capture.
 */
export { safeSerialize, getElementSelector, isSensitiveInput } from '../lib/page/serialize.js';
export { getContextAnnotations, setContextAnnotation, removeContextAnnotation, clearContextAnnotations } from '../lib/page/context.js';
export { getImplicitRole, isDynamicClass, computeCssPath, computeSelectors, recordEnhancedAction, getEnhancedActionBuffer, clearEnhancedActionBuffer, generatePlaywrightScript } from '../lib/page/reproduction.js';
export { recordAction, getActionBuffer, clearActionBuffer, handleClick, handleInput, handleScroll, handleKeydown, handleChange, installActionCapture, uninstallActionCapture, setActionCaptureEnabled, installNavigationCapture, uninstallNavigationCapture } from '../lib/page/actions.js';
export { parseResourceTiming, getNetworkWaterfall, trackPendingRequest, completePendingRequest, getPendingRequests, clearPendingRequests, getNetworkWaterfallForError, setNetworkWaterfallEnabled, isNetworkWaterfallEnabled, setNetworkBodyCaptureEnabled, isNetworkBodyCaptureEnabled, shouldCaptureUrl, setServerUrl, sanitizeHeaders, truncateRequestBody, truncateResponseBody, readResponseBody, readResponseBodyWithTimeout, wrapFetchWithBodies, wrapXHRWithBodies, unwrapXHR, adoptEarlyBodies } from '../lib/net/network.js';
export { getPerformanceMarks, getPerformanceMeasures, getCapturedMarks, getCapturedMeasures, installPerformanceCapture, uninstallPerformanceCapture, isPerformanceCaptureActive, getPerformanceSnapshotForError, setPerformanceMarksEnabled, isPerformanceMarksEnabled } from '../lib/analysis/performance.js';
export { postLog } from '../lib/page/bridge.js';
export { installConsoleCapture, uninstallConsoleCapture } from '../lib/page/console.js';
export { parseStackFrames, parseSourceMap, extractSnippet, extractSourceSnippets, setSourceMapCache, getSourceMapCache, getSourceMapCacheSize } from '../lib/ai-context/ai-context-parsing.js';
export { detectFramework, getReactComponentAncestry, captureStateSnapshot, generateAiSummary, enrichErrorWithAiContext, setAiContextEnabled, setAiContextStateSnapshot } from '../lib/ai-context/ai-context-enrichment.js';
export { installExceptionCapture, uninstallExceptionCapture } from '../lib/page/exceptions.js';
export { getSize, formatPayload, truncateWsMessage, createConnectionTracker } from '../lib/net/websocket-tracking.js';
export { installWebSocketCapture, setWebSocketCaptureMode, setWebSocketCaptureEnabled, getWebSocketCaptureMode, uninstallWebSocketCapture, resetForTesting } from '../lib/net/websocket.js';
export { executeDOMQuery, getPageInfo, runAxeAudit, runAxeAuditWithTimeout, formatAxeResults } from '../lib/analysis/dom-queries.js';
export { mapInitiatorType, aggregateResourceTiming, capturePerformanceSnapshot, installPerfObservers, uninstallPerfObservers, getLongTaskMetrics, getFCP, getLCP, getCLS, getINP, sendPerformanceSnapshot, isPerformanceSnapshotEnabled, setPerformanceSnapshotEnabled } from '../lib/analysis/perf-snapshot.js';
export { MAX_WATERFALL_ENTRIES, MAX_PERFORMANCE_ENTRIES, SENSITIVE_HEADERS } from '../lib/constants.js';
export { installKaboomAPI, uninstallKaboomAPI, type KaboomAPI } from './api.js';
export { install, uninstall, wrapFetch, installFetchCapture, uninstallFetchCapture, installXHRCapture, uninstallXHRCapture, installPhase1, installPhase2, getDeferralState, setDeferralEnabled, shouldDeferIntercepts, checkMemoryPressure, type DeferralState } from './observers.js';
export { installMessageListener, executeJavaScript, safeSerializeForExecute } from './message-handlers.js';
export { captureState, restoreState, highlightElement, clearHighlight, type RestoreStateResult, type RestoredCounts, type HighlightResult } from './state.js';
//# sourceMappingURL=index.d.ts.map