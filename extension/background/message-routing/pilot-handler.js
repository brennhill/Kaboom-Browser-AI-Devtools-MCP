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
            chrome.tabs
                .sendMessage(trackedTabId, {
                type: 'tracking_state_changed',
                state: { isTracked: true, aiPilotEnabled }
            })
                .catch(() => { });
        }
        if (untrackedTabId && untrackedTabId !== trackedTabId) {
            chrome.tabs
                .sendMessage(untrackedTabId, {
                type: 'tracking_state_changed',
                state: { isTracked: false, aiPilotEnabled: false }
            })
                .catch(() => { });
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
                                aiPilotEnabled: deps.isEnabled()
                            }
                        });
                    })
                        .catch(() => sendResponse({ state: { isTracked: false, aiPilotEnabled: false } }));
                    return true;
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