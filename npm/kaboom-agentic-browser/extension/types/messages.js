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
export * from './telemetry.js';
export * from './websocket.js';
export * from './network.js';
export * from './performance.js';
export * from './actions.js';
export * from './ai-context.js';
export * from './accessibility.js';
export * from './dom.js';
export * from './state.js';
export * from './queries.js';
export * from './sourcemap.js';
export * from './chrome.js';
export * from './debug.js';
export * from './runtime-messages.js';
//# sourceMappingURL=messages.js.map