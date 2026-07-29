/**
 * Purpose: Own locally cached background settings and server configuration.
 * Why: Settings are loaded and mutated as one lifecycle, separate from connection health.
 */
import { DEFAULT_SERVER_URL } from '../../lib/constants.js';
let serverUrl = DEFAULT_SERVER_URL;
let debugMode = false;
let currentLogLevel = 'all';
let screenshotOnError = false;
let captureOverrides = Object.freeze({});
export function getServerUrl() {
    return serverUrl;
}
export function setServerUrl(url) {
    serverUrl = url;
}
export function isDebugMode() {
    return debugMode;
}
export function setDebugModeRaw(enabled) {
    debugMode = enabled;
}
export function getCurrentLogLevel() {
    return currentLogLevel;
}
export function setCurrentLogLevel(level) {
    currentLogLevel = level;
}
export function isScreenshotOnError() {
    return screenshotOnError;
}
export function setScreenshotOnError(enabled) {
    screenshotOnError = enabled;
}
export function isAiControlled() {
    return Object.keys(captureOverrides).length > 0;
}
export function applySettingOverrides(overrides) {
    captureOverrides = Object.freeze({ ...overrides });
    if (overrides.log_level !== undefined)
        setCurrentLogLevel(overrides.log_level);
    if (overrides.screenshot_on_error !== undefined)
        setScreenshotOnError(overrides.screenshot_on_error === 'true');
}
//# sourceMappingURL=settings-state.js.map