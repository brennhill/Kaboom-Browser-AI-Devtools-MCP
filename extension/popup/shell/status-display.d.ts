/**
 * Purpose: Renders popup connection, health, and warning indicators from background status payloads.
 * Why: Converts raw runtime status into operator-readable diagnostics during extension/server troubleshooting.
 * Docs: docs/features/feature/browser-extension-enhancement/index.md
 */
/**
 * @fileoverview Status Display Module
 * Updates connection status display in popup
 */
import type { PopupConnectionStatus } from './types.js';
/**
 * Update the connection status display
 */
export declare function updateConnectionStatus(status: PopupConnectionStatus): void;
//# sourceMappingURL=status-display.d.ts.map