/**
 * @fileoverview Icon Manager - Visual feedback for tracking and AI Pilot state
 */

export interface IconState {
  isTracking: boolean
  isPilotEnabled: boolean
}

/**
 * Update the extension icon based on tracking and pilot state
 */
export declare function updateIcon(state: IconState, tabId?: number | null): void

/**
 * Update icon for tracked tab based on current state
 */
export declare function updateTrackedTabIcon(isPilotEnabled: boolean): Promise<void>

/**
 * Update icon for all tabs based on current pilot state
 */
export declare function updateAllTabIcons(enabled: boolean): Promise<void>

/**
 * Initialize icon state on extension startup
 */
export declare function initializeIconState(pilotEnabled: boolean): Promise<void>

/**
 * Clean up when Kaboom is disabled
 */
export declare function cleanup(): void

/**
 * Update pilot icon (backward compatibility)
 */
export declare function updatePilotIcon(enabled: boolean, tabId?: number | null): void
