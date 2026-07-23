/**
 * Purpose: Defines popup-specific UI contract types layered on top of shared extension state types.
 * Why: Keeps popup rendering and toggle wiring type-safe without leaking UI concerns into core runtime types.
 * Docs: docs/features/feature/browser-extension-enhancement/index.md
 */
/**
 * @fileoverview Popup Types
 * Type definitions for popup UI
 */
import type { ConnectionStatus, MemoryPressureState, ContextWarning } from '../types/index.js';
/**
 * Extended connection status for popup
 */
export interface PopupConnectionStatus extends ConnectionStatus {
    serverUrl?: string;
    circuitBreakerState?: 'closed' | 'open' | 'half-open';
    memoryPressure?: MemoryPressureState;
    contextWarning?: ContextWarning;
    error?: string;
}
/**
 * Feature toggle configuration type
 */
export interface FeatureToggleConfig {
    id: string;
    storageKey: string;
    messageType: string;
    default: boolean;
}
/**
 * Toggle warning configuration
 */
export interface ToggleWarningConfig {
    toggleId: string;
    warningId: string;
}
//# sourceMappingURL=types.d.ts.map