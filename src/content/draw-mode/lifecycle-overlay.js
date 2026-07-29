// lifecycle-overlay.js — Lifecycle, public API, and overlay ownership.
/* eslint-disable no-unused-vars, no-undef */
/**
 * @fileoverview Draw Mode — Full-viewport annotation overlay.
 * Lets users draw rectangles and attach text feedback on web pages.
 * Captures DOM elements under each rectangle for LLM consumption.
 * Activated by LLM (interact draw_mode_start) or user (keyboard shortcut / popup).
 */

// ============================================================================
// STATE
// ============================================================================

let active = false
let startedBy = 'user' // 'llm' | 'user'
let sessionName = '' // Named session for multi-page review
let sessionCorrelationId = '' // Correlation ID from MCP server for result retrieval
let overlay = null
let canvas = null
let ctx = null
let textInput = null
let annotations = []
let elementDetails = new Map() // correlationId → full detail
let drawing = false
let startX = 0
let startY = 0
let currentX = 0
let currentY = 0
let rafId = null
let saveTimeout = null
let isDeactivating = false // Re-entry guard for deactivateAndSendResults
let recentActions = []

const MIN_RECT_SIZE = 5
const OVERLAY_Z_INDEX = 2147483644
const ANNOTATION_COLOR = '#ef4444'
const ANNOTATION_FILL = 'rgba(239, 68, 68, 0.15)'
const ANNOTATION_STROKE_WIDTH = 2
const COORD_SPACE_DOCUMENT = 'document'
const ACTION_TRAIL_LIMIT = 5
const ACTION_BUFFER_LIMIT = 40

// Keyboard shortcut that toggles draw mode on/off. MUST stay in sync with
// manifest.json → commands.toggle_draw_mode.suggested_key.default; the
// draw-mode-shortcut-hint test pins the two together so they cannot drift.
const TOGGLE_DRAW_MODE_SHORTCUT = 'Alt+Shift+D'

// ============================================================================
// PUBLIC API
// ============================================================================

/**
 * Activate draw mode overlay.
 * @param {string} source - 'llm' or 'user'
 * @param {string} session - Optional named session for multi-page review
 * @returns {{ status: string, annotation_count?: number }}
 */
export function activateDrawMode(source = 'user', session = '', correlationId = '') {
  if (active) {
    return { status: 'already_active', annotation_count: annotations.length }
  }
  startedBy = source
  sessionName = session
  sessionCorrelationId = correlationId
  recentActions = []
  active = true
  createOverlay()
  loadAnnotations()
  return { status: 'active', started_by: source }
}

/**
 * Deactivate draw mode and return results.
 * @returns {{ annotations: Array, elementDetails: Object }}
 */
export function deactivateDrawMode() {
  if (!active) {
    return { annotations: [], elementDetails: {} }
  }
  cancelTextInput()
  const result = {
    annotations: annotations.map((a) => ({ ...a })),
    elementDetails: Object.fromEntries(elementDetails)
  }
  active = false
  // Clear state to prevent leaks across activate/deactivate cycles
  annotations = []
  elementDetails.clear()
  recentActions = []
  sessionName = ''
  sessionCorrelationId = ''
  destroyOverlay()
  return result
}

/**
 * Get current annotations.
 * @returns {Array}
 */
export function getAnnotations() {
  return annotations.map((a) => ({ ...a }))
}

/**
 * Get full DOM/style detail for a specific annotation.
 * @param {string} correlationId
 * @returns {Object|null}
 */
export function getElementDetail(correlationId) {
  return elementDetails.get(correlationId) || null
}

/**
 * Clear all annotations.
 */
export function clearAnnotations() {
  annotations = []
  elementDetails.clear()
  if (ctx && canvas) {
    renderAnnotations()
  }
  persistAnnotations()
}

/**
 * Check if draw mode is currently active.
 * @returns {boolean}
 */
export function isDrawModeActive() {
  return active
}

// ============================================================================
// OVERLAY CREATION / DESTRUCTION
// ============================================================================

function createOverlay() {
  overlay = document.createElement('div')
  overlay.id = 'kaboom-draw-overlay'
  Object.assign(overlay.style, {
    position: 'fixed',
    top: '0',
    left: '0',
    width: '100vw',
    height: '100vh',
    zIndex: String(OVERLAY_Z_INDEX),
    cursor: 'crosshair',
    boxShadow: 'inset 0 0 30px rgba(239, 68, 68, 0.3)',
    transition: 'opacity 0.3s ease-out, box-shadow 0.3s ease-in'
  })

  // Canvas for drawing
  canvas = document.createElement('canvas')
  canvas.width = window.innerWidth
  canvas.height = window.innerHeight
  Object.assign(canvas.style, {
    position: 'absolute',
    top: '0',
    left: '0',
    width: '100%',
    height: '100%'
  })
  overlay.appendChild(canvas)
  ctx = canvas.getContext('2d')

  // Mode badge (top-right) — small indicator, no ESC hint here
  const badge = document.createElement('div')
  badge.id = 'kaboom-draw-badge'
  Object.assign(badge.style, {
    position: 'absolute',
    top: '12px',
    right: '12px',
    display: 'flex',
    alignItems: 'center',
    gap: '6px',
    padding: '6px 12px',
    background: 'rgba(0, 0, 0, 0.8)',
    color: '#ef4444',
    fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',
    fontSize: '12px',
    fontWeight: '600',
    borderRadius: '6px',
    pointerEvents: 'none',
    zIndex: String(OVERLAY_Z_INDEX + 1)
  })

  // Pulsing dot
  const dot = document.createElement('span')
  Object.assign(dot.style, {
    width: '8px',
    height: '8px',
    borderRadius: '50%',
    background: '#ef4444',
    display: 'inline-block',
    animation: 'kaboom-draw-pulse 1.5s ease-in-out infinite'
  })
  badge.appendChild(dot)
  badge.appendChild(document.createTextNode('Draw Mode'))
  overlay.appendChild(badge)

  // Persistent centered ESC hint — stays visible throughout draw mode
  const escHint = document.createElement('div')
  escHint.id = 'kaboom-draw-esc-hint'
  Object.assign(escHint.style, {
    position: 'absolute',
    bottom: '32px',
    left: '50%',
    transform: 'translateX(-50%)',
    padding: '8px 20px',
    background: 'rgba(0, 0, 0, 0.75)',
    color: '#ccc',
    fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',
    fontSize: '13px',
    fontWeight: '500',
    borderRadius: '8px',
    border: '1px solid rgba(255, 255, 255, 0.15)',
    pointerEvents: 'none',
    zIndex: String(OVERLAY_Z_INDEX + 1),
    textAlign: 'center'
  })
  // Persistent bar (stays for the whole session): Enter submits completed
  // annotations, Escape cancels, and the shortcut toggles the mode.
  escHint.textContent = `Enter submits · Esc cancels · ${TOGGLE_DRAW_MODE_SHORTCUT} toggles draw mode`
  overlay.appendChild(escHint)

  // Center instruction toast — fades out after 2.5s
  const instruction = document.createElement('div')
  instruction.id = 'kaboom-draw-instruction'
  Object.assign(instruction.style, {
    position: 'absolute',
    top: '50%',
    left: '50%',
    transform: 'translate(-50%, -50%)',
    padding: '16px 28px',
    background: 'rgba(0, 0, 0, 0.85)',
    color: '#fff',
    fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',
    fontSize: '16px',
    fontWeight: '500',
    borderRadius: '10px',
    border: '1px solid rgba(239, 68, 68, 0.4)',
    pointerEvents: 'none',
    zIndex: String(OVERLAY_Z_INDEX + 1),
    transition: 'opacity 0.5s ease-out',
    textAlign: 'center',
    lineHeight: '1.5'
  })
  instruction.innerHTML =
    'Draw a box around what you want to change' +
    '<br><span style="font-size:13px;color:#aaa">Type your instruction, press Enter to save it, then Enter again to submit.</span>' +
    '<br><span style="font-size:12px;color:#888">Esc cancels without submitting.</span>' +
    `<br><span style="font-size:12px;color:#888">Start or stop draw mode anytime with ${TOGGLE_DRAW_MODE_SHORTCUT}</span>`
  overlay.appendChild(instruction)
  setTimeout(() => {
    instruction.style.opacity = '0'
  }, 2500)
  setTimeout(() => {
    instruction.remove()
  }, 3000)

  // Inject animation keyframes
  injectStyles()

  // Event listeners
  overlay.addEventListener('mousedown', onMouseDown)
  overlay.addEventListener('mousemove', onMouseMove)
  overlay.addEventListener('mouseup', onMouseUp)
  document.addEventListener('keydown', onKeyDown)
  document.addEventListener('click', onActionClick, true)
  document.addEventListener('input', onActionInput, true)
  document.addEventListener('change', onActionChange, true)

  // Resize observer
  window.addEventListener('resize', onResize)
  window.addEventListener('scroll', onScroll, { passive: true })
  window.addEventListener('popstate', onActionNavigation)
  window.addEventListener('hashchange', onActionNavigation)

  // Warn before navigating away with unsaved annotations
  window.addEventListener('beforeunload', onBeforeUnload)

  const target = document.body || document.documentElement
  if (target) {
    target.appendChild(overlay)
  }
}

function destroyOverlay() {
  if (rafId) {
    cancelAnimationFrame(rafId)
    rafId = null
  }
  if (saveTimeout) {
    clearTimeout(saveTimeout)
    saveTimeout = null
  }
  if (overlay) {
    overlay.removeEventListener('mousedown', onMouseDown)
    overlay.removeEventListener('mousemove', onMouseMove)
    overlay.removeEventListener('mouseup', onMouseUp)
    overlay.remove()
    overlay = null
  }
  // Safety: remove any orphaned overlay left by a failed deactivation cycle
  const orphan = document.getElementById('kaboom-draw-overlay')
  if (orphan) orphan.remove()
  document.removeEventListener('keydown', onKeyDown)
  document.removeEventListener('click', onActionClick, true)
  document.removeEventListener('input', onActionInput, true)
  document.removeEventListener('change', onActionChange, true)
  window.removeEventListener('resize', onResize)
  window.removeEventListener('scroll', onScroll)
  window.removeEventListener('popstate', onActionNavigation)
  window.removeEventListener('hashchange', onActionNavigation)
  window.removeEventListener('beforeunload', onBeforeUnload)
  canvas = null
  ctx = null
  textInput = null
  drawing = false
  removeStyles()
}

function injectStyles() {
  if (document.getElementById('kaboom-draw-styles')) return
  const style = document.createElement('style')
  style.id = 'kaboom-draw-styles'
  style.textContent = `
        @keyframes kaboom-draw-pulse {
            0%, 100% { opacity: 1; }
            50% { opacity: 0.3; }
        }
    `
  document.head.appendChild(style)
}

function removeStyles() {
  const style = document.getElementById('kaboom-draw-styles')
  if (style) style.remove()
}

// ============================================================================
// EVENT HANDLERS
// ============================================================================
