/**
 * Purpose: Owns the recording badge timer lifecycle for the extension action icon.
 * Why: Keep badge behavior consistent across popup, keyboard, context menu, and MCP recording entry points.
 * Docs: docs/features/feature/tab-recording/index.md
 */
let badgeTimerInterval = null;
let badgeStartTime = null;
function updateRecordingBadge() {
    if (!chrome?.action || !badgeStartTime)
        return;
    const elapsed = Math.round((Date.now() - badgeStartTime) / 1000);
    const mins = Math.floor(elapsed / 60);
    const secs = elapsed % 60;
    const text = mins > 0 ? `${mins}:${secs.toString().padStart(2, '0')}` : `${secs}s`;
    try {
        chrome.action.setBadgeText({ text });
    }
    catch {
        // Badge updates are best-effort.
    }
}
export function startRecordingBadgeTimer(startTime) {
    stopRecordingBadgeTimer();
    badgeStartTime = startTime;
    if (!chrome?.action)
        return;
    try {
        chrome.action.setBadgeBackgroundColor({ color: '#dc2626' });
    }
    catch {
        // Badge updates are best-effort.
    }
    updateRecordingBadge();
    badgeTimerInterval = setInterval(updateRecordingBadge, 1000);
}
export function stopRecordingBadgeTimer() {
    if (badgeTimerInterval) {
        clearInterval(badgeTimerInterval);
        badgeTimerInterval = null;
    }
    badgeStartTime = null;
    if (!chrome?.action)
        return;
    try {
        chrome.action.setBadgeText({ text: '' });
    }
    catch {
        // Badge updates are best-effort.
    }
}
//# sourceMappingURL=recording-badge.js.map