/**
 * @fileoverview Icon Manager - Visual feedback for tracking and AI Pilot state
 * Shows per-tab flame icons based on tracking and control status
 */

// Animation state
let animationInterval = null
let currentFrame = 0
const ANIMATION_FRAMES = 3
const ANIMATION_INTERVAL_MS = 400

// Badge states
const BADGE_TRACKING_ONLY = {
  text: '🔥',
  color: '#FF6B35', // Orange - tracking enabled, pilot disabled
  title: 'KaBOOM! - TRACKING THIS TAB (viewing only)'
}

const BADGE_PILOT_ACTIVE = {
  text: '⚡', // Lightning - both tracking and pilot enabled
  color: '#00D9FF', // Cyan/electric blue
  title: 'KaBOOM! - AI WEB PILOT ACTIVE (can control)'
}

const BADGE_OFF = {
  text: '',
  color: '',
  title: 'KaBOOM!'
}

/**
 * Update the extension icon based on tracking and pilot state
 * @param {Object} state - Current state
 * @param {boolean} state.isTracking - Whether a tab is being tracked
 * @param {boolean} state.isPilotEnabled - Whether AI Pilot is enabled
 * @param {number|null} [tabId] - Optional tab ID to update
 */
export function updateIcon(state, tabId = null) {
  const { isTracking, isPilotEnabled } = state

  let config
  if (!isTracking) {
    // Not tracking any tab - no badge
    config = BADGE_OFF
    stopAnimation()
  } else if (isTracking && !isPilotEnabled) {
    // Tracking but pilot disabled - static orange flame
    config = BADGE_TRACKING_ONLY
    stopAnimation()
  } else if (isTracking && isPilotEnabled) {
    // Both tracking and pilot enabled - animated electric flame
    config = BADGE_PILOT_ACTIVE
    startAnimation(tabId)
    return // Animation will handle the icon updates
  }

  // Set the icon for non-animated states
  if (tabId !== null && tabId !== undefined) {
    chrome.action.setBadgeText({ text: config.text, tabId })
    chrome.action.setBadgeBackgroundColor({ color: config.color, tabId })
    chrome.action.setTitle({ title: config.title, tabId })
  } else {
    chrome.action.setBadgeText({ text: config.text })
    chrome.action.setBadgeBackgroundColor({ color: config.color })
    chrome.action.setTitle({ title: config.title })
  }
}

/**
 * Start the flame animation (for AI Pilot active state)
 * @param {number|null} [targetTabId] - Specific tab to animate, or null for all
 */
function startAnimation(targetTabId = null) {
  if (animationInterval) return // Already running

  console.log('[KaBOOM!] Starting AI Pilot animation')

  animationInterval = setInterval(() => {
    currentFrame = (currentFrame + 1) % ANIMATION_FRAMES

    // Cycle through electric flame emojis
    const flames = ['⚡', '✨', '💫']
    // eslint-disable-next-line security/detect-object-injection -- Safe: currentFrame is bounded by animation frame modulo
    const emoji = flames[currentFrame]

    const updateTab = (tabId) => {
      chrome.action.setBadgeText({ text: emoji, tabId }).catch(() => {
        // Tab might have been closed
        console.log('[KaBOOM!] Tab closed during animation')
      })
    }

    if (targetTabId !== null && targetTabId !== undefined) {
      updateTab(targetTabId)
    } else {
      // Update globally
      chrome.action.setBadgeText({ text: emoji }).catch(() => {
        console.log('[KaBOOM!] Failed to update global badge')
      })
    }
  }, ANIMATION_INTERVAL_MS)
}

/**
 * Stop the flame animation
 */
function stopAnimation() {
  if (!animationInterval) return

  console.log('[KaBOOM!] Stopping AI Pilot animation')
  clearInterval(animationInterval)
  animationInterval = null
  currentFrame = 0
}

/**
 * Update icon for tracked tab based on current state
 * @param {boolean} isPilotEnabled - Whether AI Pilot is enabled
 */
export async function updateTrackedTabIcon(isPilotEnabled) {
  try {
    const result = await chrome.storage.local.get(['trackedTabId'])
    const trackedTabId = result.trackedTabId

    if (trackedTabId) {
      // Update the specific tracked tab
      updateIcon(
        {
          isTracking: true,
          isPilotEnabled
        },
        trackedTabId
      )
    } else {
      // No tab is being tracked
      updateIcon({
        isTracking: false,
        isPilotEnabled: false
      })
    }
  } catch (err) {
    console.error('[KaBOOM!] Failed to update tracked tab icon:', err)
  }
}

/**
 * Update icon for all tabs based on current pilot state
 * Called when pilot toggle changes
 * @param {boolean} enabled - Whether AI Web Pilot is enabled
 */
export async function updateAllTabIcons(enabled) {
  try {
    const result = await chrome.storage.local.get(['trackedTabId'])
    const isTracking = !!result.trackedTabId

    if (isTracking && result.trackedTabId) {
      // Update the tracked tab
      updateIcon(
        {
          isTracking: true,
          isPilotEnabled: enabled
        },
        result.trackedTabId
      )
    } else {
      // Clear all tabs
      const tabs = await chrome.tabs.query({})
      for (const tab of tabs) {
        if (tab.id) {
          updateIcon(
            {
              isTracking: false,
              isPilotEnabled: false
            },
            tab.id
          )
        }
      }
    }
  } catch (err) {
    console.error('[KaBOOM!] Failed to update all tab icons:', err)
  }
}

/**
 * Initialize icon state on extension startup
 * @param {boolean} pilotEnabled - Current AI Web Pilot state
 */
export async function initializeIconState(pilotEnabled) {
  // Clear any previous animation state
  stopAnimation()

  try {
    const result = await chrome.storage.local.get(['trackedTabId'])
    const isTracking = !!result.trackedTabId

    if (isTracking && result.trackedTabId) {
      updateIcon(
        {
          isTracking: true,
          isPilotEnabled: pilotEnabled
        },
        result.trackedTabId
      )
    } else {
      updateIcon({
        isTracking: false,
        isPilotEnabled: false
      })
    }
  } catch (err) {
    console.error('[KaBOOM!] Failed to initialize icon state:', err)
  }
}

/**
 * Clean up when Kaboom is disabled
 */
export function cleanup() {
  stopAnimation()
  updateIcon({
    isTracking: false,
    isPilotEnabled: false
  })
}

// For backward compatibility
export function updatePilotIcon(enabled, _tabId = null) {
  // This is called when pilot toggle changes
  updateAllTabIcons(enabled)
}
