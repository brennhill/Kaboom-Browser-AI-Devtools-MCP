/**
 * Purpose: Replaces the page favicon with the Kaboom flame icon when tab tracking is enabled and adds flickering animation when AI Pilot is active.
 * Docs: docs/features/feature/tab-tracking-ux/index.md
 */
/**
 * Original favicon href (to restore when tracking stops)
 */
let originalFaviconHref = null;
/**
 * Interval ID for flicker effect (uses setInterval, not requestAnimationFrame)
 * This ensures animation continues even when tab is hidden (visible in tab bar)
 */
let flickerInterval = null;
/**
 * Initialize favicon replacement.
 * Listens for tracking state changes and updates favicon accordingly.
 */
// #lizard forgives
export function initFaviconReplacer() {
    // Listen for tracking state updates from background
    chrome.runtime.onMessage.addListener((message, sender, _sendResponse) => {
        // Only accept messages from the extension itself (background script)
        if (sender.id !== chrome.runtime.id)
            return false;
        if (message.type === 'tracking_state_changed') {
            const newState = message.state;
            updateFavicon(newState);
        }
        // Explicitly return false so Chrome doesn't prematurely resolve
        // sendMessage promises from other listeners (e.g. DOM_QUERY, A11Y_QUERY).
        return false;
    });
    // Request initial tracking state
    chrome.runtime.sendMessage({ type: 'get_tracking_state' }, (response) => {
        if (response && response.state) {
            updateFavicon(response.state);
        }
    });
}
/**
 * Update favicon based on tracking state.
 * - Not tracked: Shows original favicon
 * - Tracked (AI Pilot off): Shows static glowing flame
 * - Tracked (AI Pilot on): Shows flickering flame
 */
function updateFavicon(state) {
    if (!state.isTracked) {
        // Restore original favicon
        restoreOriginalFavicon();
        stopFlicker();
    }
    else if (state.aiPilotEnabled) {
        // Tracked + AI Pilot on = flickering flame
        replaceFaviconWithFlame(true);
        startFlicker();
    }
    else {
        // Tracked only = static glowing flame
        replaceFaviconWithFlame(false);
        stopFlicker();
    }
}
/**
 * Save original favicon and replace with Kaboom flame.
 */
function replaceFaviconWithFlame(withGlow) {
    // Save original favicon (only once)
    if (!originalFaviconHref) {
        const existingLink = document.querySelector('link[rel*="icon"]');
        originalFaviconHref = existingLink?.href || '';
    }
    // Remove existing favicons
    const existingIcons = document.querySelectorAll('link[rel*="icon"]');
    existingIcons.forEach((icon) => icon.remove());
    // Add Kaboom flame favicon
    const link = document.createElement('link');
    link.rel = 'icon';
    link.type = 'image/svg+xml';
    link.id = 'kaboom-favicon';
    // Use glow icon if tracking, regular icon if not
    const iconPath = withGlow ? 'icons/icon-glow.svg' : 'icons/icon.svg';
    link.href = chrome.runtime.getURL(iconPath);
    document.head.appendChild(link);
}
/**
 * Restore the original page favicon.
 */
function restoreOriginalFavicon() {
    // Remove Kaboom favicon
    const kaboomIcon = document.getElementById('kaboom-favicon');
    if (kaboomIcon) {
        kaboomIcon.remove();
    }
    // Restore original
    if (originalFaviconHref) {
        const link = document.createElement('link');
        link.rel = 'icon';
        link.href = originalFaviconHref;
        document.head.appendChild(link);
    }
}
/**
 * Start flicker animation (for AI Pilot active state).
 * Realistic 8-frame flame animation:
 * - Bottom stays anchored (flames grow UPWARD, not scaled from center)
 * - Smaller flames = more orange/red (cooler, 85-92% height) + smaller darker ring
 * - Normal flame = orange-yellow gradient (100% height) + medium orange ring
 * - Larger flames = more yellow/white (hotter, 105-112% height) + larger brighter ring
 * - 150ms per frame = 1.2s full cycle (fast, visible flicker)
 * - Uses setInterval (not requestAnimationFrame) so it's visible in tab bar when tab is hidden
 */
function startFlicker() {
    if (flickerInterval !== null) {
        return; // Already flickering
    }
    // 8-frame sequence for smooth breathing effect with color temperature shift
    const flameFrames = [
        'icon-flicker-1-tiny.svg', // 85% - dark red/orange (coolest) + small dark ring
        'icon-flicker-2-small.svg', // 92% - orange + small orange ring
        'icon-flicker-3-normal.svg', // 100% - orange-yellow (base) + medium orange ring
        'icon-flicker-4-medium.svg', // 105% - yellow + medium yellow ring
        'icon-flicker-5-large.svg', // 112% - yellow/white (PEAK - hottest) + large bright ring
        'icon-flicker-6-medium.svg', // 105% - yellow + medium yellow ring (shrinking)
        'icon-flicker-7-smallmed.svg', // 96% - orange-yellow + medium ring
        'icon-flicker-8-small.svg' // 92% - orange + small orange ring (back to small)
    ];
    let currentFrameIndex = 0;
    // Use setInterval instead of requestAnimationFrame so animation continues
    // even when tab is hidden (user can see flicker in browser tab bar)
    flickerInterval = window.setInterval(() => {
        currentFrameIndex = (currentFrameIndex + 1) % flameFrames.length;
        // Update favicon
        const kaboomIcon = document.getElementById('kaboom-favicon');
        if (kaboomIcon) {
            const iconPath = `icons/${flameFrames[currentFrameIndex]}`;
            kaboomIcon.href = chrome.runtime.getURL(iconPath);
        }
    }, 150); // 150ms per frame = 1.2s full cycle (browser-limited, but visible)
}
/**
 * Stop flicker animation.
 */
function stopFlicker() {
    if (flickerInterval !== null) {
        clearInterval(flickerInterval);
        flickerInterval = null;
    }
}
//# sourceMappingURL=favicon-replacer.js.map