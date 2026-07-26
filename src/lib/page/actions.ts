/**
 * Purpose: Captures and buffers user actions (click, input, scroll, keydown, navigation) with throttling and sensitive data redaction.
 * Docs: docs/features/feature/observe/index.md
 */

/**
 * @fileoverview User action capture and replay buffer.
 * Records click, input, scroll, keydown, and change events with throttling
 * and sensitive data redaction. Also captures navigation events (pushState,
 * replaceState, popstate) for enhanced reproduction scripts.
 */

import { MAX_ACTION_BUFFER_SIZE, SCROLL_THROTTLE_MS, ACTIONABLE_KEYS } from '../constants.js'
import { getElementSelector, isSensitiveInput } from './serialize.js'
import { recordEnhancedAction } from './reproduction.js'

// Action record type
interface ActionRecord {
  ts: string
  type: string
  target?: string
  x?: number
  y?: number
  text?: string
  inputType?: string
  value?: string
  length?: number
  scrollX?: number
  scrollY?: number
}

// Extended event target types
interface InputElement extends HTMLElement {
  type?: string
  value?: string
}

interface SelectElement extends HTMLSelectElement {
  options: HTMLOptionsCollection
  selectedIndex: number
  value: string
}

// User action replay buffer
let actionBuffer: ActionRecord[] = []
let lastScrollTime = 0
let actionCaptureEnabled = true
let clickHandler: ((event: MouseEvent) => void) | null = null
let inputHandler: ((event: Event) => void) | null = null
let scrollHandler: ((event: Event) => void) | null = null
let keydownHandler: ((event: KeyboardEvent) => void) | null = null
let changeHandler: ((event: Event) => void) | null = null

/**
 * Record a user action to the buffer
 */
export function recordAction(action: Omit<ActionRecord, 'ts'>): void {
  if (!actionCaptureEnabled) return

  actionBuffer.push({
    ts: new Date().toISOString(),
    ...action
  })

  // Keep buffer size limited
  if (actionBuffer.length > MAX_ACTION_BUFFER_SIZE) {
    actionBuffer.shift()
  }
}

/**
 * Get the current action buffer
 */
export function getActionBuffer(): ActionRecord[] {
  return [...actionBuffer]
}

/**
 * Clear the action buffer
 */
export function clearActionBuffer(): void {
  actionBuffer = []
}

/**
 * Handle click events
 */
export function handleClick(event: MouseEvent): void {
  const target = event.target as Element | null
  if (!target) return

  const action: Omit<ActionRecord, 'ts'> = {
    type: 'click',
    target: getElementSelector(target),
    x: event.clientX,
    y: event.clientY
  }

  // Include button text if available (truncated)
  const text = (target as HTMLElement).textContent || (target as HTMLElement).innerText || ''
  if (text && text.length > 0) {
    action.text = text.trim().slice(0, 50)
  }

  recordAction(action)
  recordEnhancedAction('click', target)
}

/**
 * Handle input events
 */
export function handleInput(event: Event): void {
  const target = event.target as InputElement | null
  if (!target) return

  const action: Omit<ActionRecord, 'ts'> = {
    type: 'input',
    target: getElementSelector(target),
    inputType: target.type || 'text'
  }

  // Only include value for non-sensitive fields
  if (!isSensitiveInput(target)) {
    const value = target.value || ''
    action.value = value.slice(0, 100)
    action.length = value.length
  } else {
    action.value = '[redacted]'
    action.length = (target.value || '').length
  }

  recordAction(action)
  recordEnhancedAction('input', target, { value: action.value })
}

/**
 * Handle scroll events (throttled)
 */
export function handleScroll(event: Event): void {
  const now = Date.now()
  if (now - lastScrollTime < SCROLL_THROTTLE_MS) return
  lastScrollTime = now

  const target = event.target
  recordAction({
    type: 'scroll',
    scrollX: Math.round(window.scrollX),
    scrollY: Math.round(window.scrollY),
    target: target === document ? 'document' : getElementSelector(target as Element)
  })
  recordEnhancedAction('scroll', null, { scroll_y: Math.round(window.scrollY) })
}

/**
 * Handle keydown events - only records actionable keys
 */
export function handleKeydown(event: KeyboardEvent): void {
  if (!ACTIONABLE_KEYS.has(event.key)) return
  const target = event.target as Element | null
  recordEnhancedAction('keypress', target, { key: event.key })
}

/**
 * Handle change events on select elements
 */
export function handleChange(event: Event): void {
  const target = event.target as SelectElement | null
  if (!target || !target.tagName || target.tagName.toUpperCase() !== 'SELECT') return

  const selectedOption = target.options && target.options[target.selectedIndex]
  const selectedValue = target.value || ''
  const selectedText = selectedOption ? selectedOption.text || '' : ''

  recordEnhancedAction('select', target, { selected_value: selectedValue, selected_text: selectedText })
}

/**
 * Install user action capture
 */
export function installActionCapture(): void {
  if (typeof window === 'undefined' || typeof document === 'undefined') return
  if (typeof document.addEventListener !== 'function') return

  clickHandler = handleClick
  inputHandler = handleInput
  scrollHandler = handleScroll
  keydownHandler = handleKeydown
  changeHandler = handleChange

  document.addEventListener('click', clickHandler, { capture: true, passive: true })
  document.addEventListener('input', inputHandler, { capture: true, passive: true })
  document.addEventListener('keydown', keydownHandler, { capture: true, passive: true })
  document.addEventListener('change', changeHandler, { capture: true, passive: true })
  window.addEventListener('scroll', scrollHandler, { capture: true, passive: true })
}

/**
 * Uninstall user action capture
 */
export function uninstallActionCapture(): void {
  if (clickHandler) {
    document.removeEventListener('click', clickHandler, { capture: true })
    clickHandler = null
  }
  if (inputHandler) {
    document.removeEventListener('input', inputHandler, { capture: true })
    inputHandler = null
  }
  if (keydownHandler) {
    document.removeEventListener('keydown', keydownHandler, { capture: true })
    keydownHandler = null
  }
  if (changeHandler) {
    document.removeEventListener('change', changeHandler, { capture: true })
    changeHandler = null
  }
  if (scrollHandler) {
    window.removeEventListener('scroll', scrollHandler, { capture: true })
    scrollHandler = null
  }
  clearActionBuffer()
}

/**
 * Set whether action capture is enabled
 */
export function setActionCaptureEnabled(enabled: boolean): void {
  actionCaptureEnabled = enabled
  if (!enabled) {
    clearActionBuffer()
  }
}

// =============================================================================
// NAVIGATION CAPTURE
// =============================================================================

let navigationPopstateHandler: (() => void) | null = null
let originalPushState: typeof history.pushState | null = null
let originalReplaceState: typeof history.replaceState | null = null

/**
 * Install navigation capture to record enhanced actions on navigation events
 */
export function installNavigationCapture(): void {
  if (typeof window === 'undefined') return

  // Track current URL for from_url
  let lastUrl = window.location.href

  // Popstate handler (back/forward)
  navigationPopstateHandler = function (): void {
    const toUrl = window.location.href
    recordEnhancedAction('navigate', null, { from_url: lastUrl, to_url: toUrl })
    lastUrl = toUrl
  }
  window.addEventListener('popstate', navigationPopstateHandler)

  // Patch pushState
  if (window.history && window.history.pushState) {
    originalPushState = window.history.pushState
    window.history.pushState = function (
      this: History,
      state: unknown,
      title: string,
      url?: string | URL | null
    ): void {
      const fromUrl = lastUrl
      originalPushState!.call(this, state, title, url)
      const toUrl = url || window.location.href
      recordEnhancedAction('navigate', null, { from_url: fromUrl, to_url: String(toUrl) })
      lastUrl = window.location.href
    }
  }

  // Patch replaceState
  if (window.history && window.history.replaceState) {
    originalReplaceState = window.history.replaceState
    window.history.replaceState = function (
      this: History,
      state: unknown,
      title: string,
      url?: string | URL | null
    ): void {
      const fromUrl = lastUrl
      originalReplaceState!.call(this, state, title, url)
      const toUrl = url || window.location.href
      recordEnhancedAction('navigate', null, { from_url: fromUrl, to_url: String(toUrl) })
      lastUrl = window.location.href
    }
  }
}

/**
 * Uninstall navigation capture
 */
export function uninstallNavigationCapture(): void {
  if (navigationPopstateHandler) {
    window.removeEventListener('popstate', navigationPopstateHandler)
    navigationPopstateHandler = null
  }
  if (originalPushState && window.history) {
    window.history.pushState = originalPushState
    originalPushState = null
  }
  if (originalReplaceState && window.history) {
    window.history.replaceState = originalReplaceState
    originalReplaceState = null
  }
}
