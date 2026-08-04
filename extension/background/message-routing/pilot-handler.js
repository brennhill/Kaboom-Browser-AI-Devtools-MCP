/**
 * Purpose: Own AI Pilot and tracked-tab runtime messages.
 */
import { KABOOM_LOG_PREFIX } from '../../lib/brand.js';
import { StorageKey } from '../../lib/constants.js';
import { readLocalState } from '../../lib/storage/validated.js';
import { readTrackedTab } from '../../lib/tabs/tracked-tab-storage.js';
import { reportStateRecovery } from '../runtime-state/state-recovery.js';
export async function broadcastTrackingState(untrackedTabId) {
    try {
        const [aiPilotEnabled, trackedTab] = await Promise.all([readPilotPreference(), readTrackedTab()]);
        const trackedTabId = trackedTab.id;
        if (trackedTabId) {
            const trackedMessage = {
                type: 'tracking_state_changed',
                state: { isTracked: true, aiPilotEnabled }
            };
            chrome.tabs.sendMessage(trackedTabId, trackedMessage).catch(() => {
                // EXPECTED_ABSENCE: content scripts disappear during navigation; storage
                // remains authoritative and logging this would flag normal reinjection.
            });
        }
        if (untrackedTabId && untrackedTabId !== trackedTabId) {
            const untrackedMessage = {
                type: 'tracking_state_changed',
                state: { isTracked: false, aiPilotEnabled: false }
            };
            chrome.tabs.sendMessage(untrackedTabId, untrackedMessage).catch(() => {
                // EXPECTED_ABSENCE: a missing recipient is normal for an untracked or
                // closed tab; logging it would misleadingly imply tracking is unhealthy.
            });
        }
    }
    catch (error) {
        console.error(`${KABOOM_LOG_PREFIX} Failed to broadcast tracking state:`, error);
    }
}
export function createPilotMessageHandler(deps) {
    return {
        feature: 'pilot',
        handle(message, sender, sendResponse) {
            switch (message.type) {
                case 'set_ai_web_pilot_enabled': {
                    const enabled = message.enabled === true;
                    deps.setEnabled(enabled, () => {
                        void broadcastTrackingState();
                    });
                    sendResponse({ success: true });
                    return false;
                }
                case 'get_ai_web_pilot_enabled':
                    sendResponse({ enabled: deps.isEnabled() });
                    return false;
                case 'get_tracking_state':
                    readTrackedTab()
                        .then((tracked) => {
                        sendResponse({
                            state: {
                                isTracked: sender.tab?.id !== undefined && sender.tab.id === tracked.id,
                                aiPilotEnabled: deps.isEnabled(),
                                continuity: deps.getTrackingContinuity()
                            }
                        });
                    })
                        .catch(() => {
                        console.warn(`${KABOOM_LOG_PREFIX} tracking state lookup failed after validated fallback`);
                        sendResponse({
                            state: {
                                isTracked: false,
                                aiPilotEnabled: false,
                                continuity: deps.getTrackingContinuity()
                            }
                        });
                    });
                    return true;
                case 'tracking_content_ready':
                    if (sender.tab?.id !== undefined) {
                        deps.confirmTracking(sender.tab.id, message.url);
                    }
                    sendResponse({ success: true });
                    return false;
                case 'get_diagnostic_state':
                    readPilotPreference().then((storage) => {
                        sendResponse({ cache: deps.isEnabled(), storage, timestamp: new Date().toISOString() });
                    });
                    return true;
                default:
                    return undefined;
            }
        }
    };
}
function readPilotPreference() {
    return readLocalState({
        key: StorageKey.AI_WEB_PILOT_ENABLED,
        fallback: true,
        validate: (value) => typeof value === 'boolean',
        diagnostic: {
            name: 'extension_settings_state',
            detail: 'Saved AI Web Pilot preference was invalid or unreadable; enabled is active.',
            fix: 'Open extension settings and save the AI Web Pilot preference again.'
        },
        report: reportStateRecovery
    });
}
//# sourceMappingURL=pilot-handler.js.map