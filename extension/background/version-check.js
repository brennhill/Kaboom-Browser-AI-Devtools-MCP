/**
 * Purpose: Compares extension and server versions from /health responses and signals when an update is available.
 */
/**
 * @fileoverview Version Check - Badge display based on /health response
 */
import { isVersionNewer } from '../lib/version.js';
/**
 * Version check state
 */
let availableVersion = null;
let newVersionAvailable = false;
/**
 * Get the extension version from manifest
 */
export function getExtensionVersion() {
    const manifest = chrome.runtime.getManifest();
    return manifest.version || '0.0.0';
}
/**
 * Check if a new version is available (from last /health response)
 */
export function isNewVersionAvailable() {
    return newVersionAvailable;
}
/**
 * Get the available version from last /health response
 */
export function getAvailableVersion() {
    return availableVersion;
}
/**
 * Update version state from /health response
 * Called when extension receives /health endpoint data
 */
export function updateVersionFromHealth(healthResponse, debugLogFn) {
    const currentVersion = healthResponse.version || getExtensionVersion();
    const newAvailableVersion = healthResponse.available_version || null;
    // Update cached version
    availableVersion = newAvailableVersion;
    const extensionVersion = getExtensionVersion();
    const isNewer = newAvailableVersion && isVersionNewer(newAvailableVersion, extensionVersion);
    if (isNewer !== newVersionAvailable) {
        newVersionAvailable = isNewer ? true : false;
        updateVersionBadge();
        if (debugLogFn) {
            debugLogFn('version', 'Version check result', {
                extensionVersion,
                currentVersion,
                availableVersion: newAvailableVersion,
                updateAvailable: isNewer
            });
        }
    }
}
/**
 * Update extension badge to show version update indicator
 * If newVersionAvailable, shows a "⬆" indicator on the icon
 */
export function updateVersionBadge() {
    if (typeof chrome === 'undefined' || !chrome.action)
        return;
    if (newVersionAvailable && availableVersion) {
        chrome.action.setBadgeText({
            text: '⬆'
        });
        chrome.action.setBadgeBackgroundColor({
            color: '#0969da' // Blue for info
        });
        chrome.action.setTitle({
            title: `KaBOOM!: New version available (${availableVersion})`
        });
    }
    else {
        // Clear the version update indicator
        chrome.action.setTitle({
            title: 'KaBOOM!'
        });
    }
}
/**
 * Get update information for display in popup
 */
export function getUpdateInfo() {
    return {
        available: newVersionAvailable,
        currentVersion: getExtensionVersion(),
        availableVersion: availableVersion,
        downloadUrl: 'https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/releases/latest'
    };
}
/**
 * Reset version check state (useful for testing)
 */
export function resetVersionCheck() {
    availableVersion = null;
    newVersionAvailable = false;
}
//# sourceMappingURL=version-check.js.map