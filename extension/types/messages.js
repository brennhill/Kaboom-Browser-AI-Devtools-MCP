/**
 * Purpose: Provides backward-compatible message/type export surface for extension communication contracts.
 * Why: Preserves existing imports while message definitions are split into focused type modules.
 * Docs: docs/features/feature/query-service/index.md
 */
/**
 * @fileoverview Message Types for Kaboom Extension
 *
 * Comprehensive discriminated unions for all message types used in the extension.
 * This is the single source of truth for message payloads between:
 * - Background service worker
 * - Content scripts
 * - Inject scripts (page context)
 * - Popup
 *
 * NOTE: This file now re-exports types from focused modules for backward compatibility.
 * New code should import from the specific modules directly.
 */
// Re-export all types for backward compatibility
export * from './capture/telemetry.js';
export * from './capture/websocket.js';
export * from './capture/network.js';
export * from './capture/performance.js';
export * from './capture/actions.js';
export * from './capture/ai-context.js';
export * from './capture/accessibility.js';
export * from './capture/dom.js';
export * from './runtime/state.js';
export * from './runtime/queries.js';
export * from './capture/sourcemap.js';
export * from './runtime/chrome.js';
export * from './runtime/debug.js';
export * from './runtime-messages.js';
//# sourceMappingURL=messages.js.map