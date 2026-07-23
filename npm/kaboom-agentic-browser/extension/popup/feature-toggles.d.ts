/**
 * Purpose: Manages popup feature-toggle definitions and background-dispatched toggle updates.
 * Why: Prevents state drift by routing all toggle mutations through one consistent popup workflow.
 * Docs: docs/features/feature/browser-extension-enhancement/index.md
 */
/**
 * @fileoverview Feature Toggles Module
 * Manages feature toggle configuration and initialization
 */
import type { FeatureToggleConfig } from './types.js';
/**
 * Feature toggle configuration
 */
export declare const FEATURE_TOGGLES: readonly FeatureToggleConfig[];
/**
 * Handle feature toggle change
 * CRITICAL ARCHITECTURE: Popup NEVER writes storage directly.
 * It ONLY sends a message to background, which is the single writer.
 * This prevents desynchronization bugs where UI state diverges from actual state.
 */
export declare function handleFeatureToggle(storageKey: string, messageType: string, enabled: boolean): void;
/**
 * Apply pre-loaded toggle values to DOM checkboxes and wire up change handlers.
 * Called from the orchestrator after a single batched storage read.
 */
export declare function applyFeatureToggles(result: Record<string, unknown>): void;
/**
 * Initialize all feature toggles (self-contained async version for backward compat)
 */
export declare function initFeatureToggles(): Promise<void>;
//# sourceMappingURL=feature-toggles.d.ts.map