/**
 * Purpose: Orchestrates popup initialization and binds UI modules for tracking, recording, draw mode, and pilot controls.
 * Why: Keeps popup behavior consistent by coordinating status/state hydration in one lifecycle entrypoint.
 * Docs: docs/features/feature/ai-web-pilot/index.md
 * Docs: docs/features/feature/tab-recording/index.md
 * Docs: docs/features/feature/tab-tracking-ux/index.md
 */
import { updateConnectionStatus } from './popup/status-display.js';
import { handleClearLogs, resetClearConfirm } from './popup/settings.js';
export { resetClearConfirm, handleClearLogs };
export { updateConnectionStatus };
export { FEATURE_TOGGLES, initFeatureToggles, applyFeatureToggles } from './popup/feature-toggles.js';
export { handleFeatureToggle } from './popup/feature-toggles.js';
export { initAiWebPilotToggle, handleAiWebPilotToggle, applyAiWebPilotToggle } from './popup/ai-web-pilot.js';
export { initTrackPageButton, handleTrackPageClick } from './popup/tab-tracking.js';
export { handleWebSocketModeChange } from './popup/settings.js';
export { initWebSocketModeSelector, applyWebSocketMode } from './popup/settings.js';
export { isInternalUrl } from './popup/ui-utils.js';
/**
 * Initialize the popup.
 *
 * Optimized for instant first paint:
 * 1. HTML renders with default states (idle buttons, checked toggles from markup).
 * 2. One batched storage read fetches ALL keys in parallel.
 * 3. Results are applied synchronously in a single pass (no await chains).
 * 4. Non-critical init (logo, draw mode) deferred via requestAnimationFrame.
 */
export declare function initPopup(): void;
//# sourceMappingURL=popup.d.ts.map