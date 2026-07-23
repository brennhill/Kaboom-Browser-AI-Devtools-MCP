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
  escHint.textContent = 'Press ESC when done'
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
    'Draw a box around what you want to change<br><span style="font-size:13px;color:#aaa">Then type your instruction. Press ESC when done.</span>'
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

function onMouseDown(e) {
  if (textInput) return // Don't start new rect while typing
  if (e.button !== 0) return // Left click only
  recordRecentAction('click', e.target || overlay)
  drawing = true
  startX = e.clientX
  startY = e.clientY
  currentX = startX
  currentY = startY
}

function onMouseMove(e) {
  if (!drawing) return
  currentX = e.clientX
  currentY = e.clientY
  if (rafId) cancelAnimationFrame(rafId)
  rafId = requestAnimationFrame(renderFrame)
}

function onMouseUp(e) {
  if (!drawing) return
  drawing = false
  if (rafId) {
    cancelAnimationFrame(rafId)
    rafId = null
  }

  const rect = normalizeRect(startX, startY, e.clientX, e.clientY)

  // Ignore tiny rectangles (accidental clicks)
  if (rect.width < MIN_RECT_SIZE || rect.height < MIN_RECT_SIZE) {
    renderAnnotations()
    return
  }

  // Capture DOM elements under the rectangle
  const elementData = captureElementsUnderRect(rect)

  // Show text input
  showTextInput(rect, elementData)
}

function onKeyDown(e) {
  if (e.key === 'Escape') {
    if (textInput) {
      cancelTextInput()
      renderAnnotations()
    } else {
      // Exit draw mode entirely
      deactivateAndSendResults()
    }
    e.preventDefault()
    e.stopPropagation()
  }
}

function onResize() {
  if (!canvas) return
  canvas.width = window.innerWidth
  canvas.height = window.innerHeight
  renderAnnotations()
}

function onScroll() {
  recordRecentAction('scroll', document.activeElement, { scroll_x: Math.round(window.scrollX || 0), scroll_y: Math.round(window.scrollY || 0) })
  if (!canvas) return
  renderAnnotations()
}

function onActionClick(e) {
  recordRecentAction('click', e.target)
}

function onActionInput(e) {
  recordRecentAction('type', e.target)
}

function onActionChange(e) {
  const tag = e.target?.tagName?.toLowerCase?.() || ''
  if (tag === 'select') {
    recordRecentAction('select', e.target)
    return
  }
  recordRecentAction('change', e.target)
}

function onActionNavigation() {
  recordRecentAction('navigation', document.activeElement, { url: window.location.href })
}

function onBeforeUnload(e) {
  if (active && annotations.length > 0) {
    e.preventDefault()
    // Returning a string is required by some browsers to trigger the dialog
    e.returnValue = 'You have unsaved annotations. Are you sure you want to leave?'
    return e.returnValue
  }
}

// ============================================================================
// RENDERING
// ============================================================================

function renderFrame() {
  if (!ctx || !canvas) return
  // Clear and re-render existing annotations
  ctx.clearRect(0, 0, canvas.width, canvas.height)
  drawExistingAnnotations()

  // Draw current rubber-band rectangle
  const rect = normalizeRect(startX, startY, currentX, currentY)
  ctx.setLineDash([6, 4])
  ctx.strokeStyle = ANNOTATION_COLOR
  ctx.lineWidth = ANNOTATION_STROKE_WIDTH
  ctx.strokeRect(rect.x, rect.y, rect.width, rect.height)
  ctx.setLineDash([])
}

function renderAnnotations() {
  if (!ctx || !canvas) return
  ctx.clearRect(0, 0, canvas.width, canvas.height)
  drawExistingAnnotations()
}

function drawRoundRect(x, y, w, h, radius) {
  ctx.beginPath()
  ctx.moveTo(x + radius, y)
  ctx.lineTo(x + w - radius, y)
  ctx.quadraticCurveTo(x + w, y, x + w, y + radius)
  ctx.lineTo(x + w, y + h - radius)
  ctx.quadraticCurveTo(x + w, y + h, x + w - radius, y + h)
  ctx.lineTo(x + radius, y + h)
  ctx.quadraticCurveTo(x, y + h, x, y + h - radius)
  ctx.lineTo(x, y + radius)
  ctx.quadraticCurveTo(x, y, x + radius, y)
  ctx.closePath()
}

function drawExistingAnnotations() {
  if (!ctx) return
  for (let i = 0; i < annotations.length; i++) {
    const ann = annotations[i]
    const r = toViewportRect(ann.rect, ann.coord_space)
    if (!Number.isFinite(r.x) || !Number.isFinite(r.y) || !Number.isFinite(r.width) || !Number.isFinite(r.height)) {
      continue
    }

    // Semi-transparent fill with rounded corners
    ctx.save()
    drawRoundRect(r.x, r.y, r.width, r.height, 4)
    ctx.fillStyle = ANNOTATION_FILL
    ctx.fill()
    ctx.strokeStyle = ANNOTATION_COLOR
    ctx.lineWidth = ANNOTATION_STROKE_WIDTH
    ctx.setLineDash([])
    ctx.stroke()
    ctx.restore()

    // Number badge (top-left, offset outward)
    const badgeSize = 22
    const badgeX = r.x - 4
    const badgeY = r.y - 4
    ctx.fillStyle = ANNOTATION_COLOR
    ctx.beginPath()
    ctx.arc(badgeX, badgeY, badgeSize / 2, 0, Math.PI * 2)
    ctx.fill()
    // White ring
    ctx.strokeStyle = '#fff'
    ctx.lineWidth = 2
    ctx.stroke()
    ctx.fillStyle = '#fff'
    ctx.font = 'bold 11px -apple-system, sans-serif'
    ctx.textAlign = 'center'
    ctx.textBaseline = 'middle'
    ctx.fillText(String(i + 1), badgeX, badgeY)

    // Text label pill (below rectangle)
    if (ann.text) {
      ctx.font = '13px -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif'
      const textWidth = ctx.measureText(ann.text).width
      const padX = 10
      const padY = 6
      const pillH = 26
      const pillW = textWidth + padX * 2
      const pillX = r.x
      const pillY = r.y + r.height + 8
      const pillR = 6

      // Shadow
      ctx.save()
      ctx.shadowColor = 'rgba(0, 0, 0, 0.25)'
      ctx.shadowBlur = 8
      ctx.shadowOffsetY = 2
      drawRoundRect(pillX, pillY, pillW, pillH, pillR)
      ctx.fillStyle = 'rgba(15, 23, 42, 0.9)'
      ctx.fill()
      ctx.restore()

      // Border
      drawRoundRect(pillX, pillY, pillW, pillH, pillR)
      ctx.strokeStyle = ANNOTATION_COLOR
      ctx.lineWidth = 1.5
      ctx.stroke()

      // Text
      ctx.fillStyle = '#f1f5f9'
      ctx.textAlign = 'left'
      ctx.textBaseline = 'middle'
      ctx.fillText(ann.text, pillX + padX, pillY + pillH / 2)
    }
  }
}

// ============================================================================
// TEXT INPUT
// ============================================================================

function showTextInput(rect, elementData) {
  if (textInput) cancelTextInput()

  const input = document.createElement('input')
  input.type = 'text'
  input.placeholder = "Don't just tell the AI what's wrong, tell it what you want instead..."
  input.dataset.rectJson = JSON.stringify(rect)
  input.dataset.elementJson = JSON.stringify(elementData)

  // Clamp position so the input stays within the viewport
  const inputHeight = 36 // approximate height (padding + font + border)
  const inputGap = 8
  let inputTop = rect.y + rect.height + inputGap
  let inputLeft = rect.x
  if (inputTop + inputHeight > window.innerHeight) {
    // Place above the rectangle instead
    inputTop = Math.max(0, rect.y - inputHeight - inputGap)
  }
  if (inputLeft + 200 > window.innerWidth) {
    inputLeft = Math.max(0, window.innerWidth - 200)
  }

  Object.assign(input.style, {
    position: 'absolute',
    left: `${inputLeft}px`,
    top: `${inputTop}px`,
    minWidth: '200px',
    maxWidth: '400px',
    padding: '8px 12px',
    background: '#1a1a1a',
    color: '#e0e0e0',
    border: '2px solid ' + ANNOTATION_COLOR,
    borderRadius: '6px',
    fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',
    fontSize: '13px',
    outline: 'none',
    zIndex: String(OVERLAY_Z_INDEX + 2),
    boxShadow: '0 4px 12px rgba(0, 0, 0, 0.5)'
  })

  input.addEventListener('keydown', onTextInputKeyDown)
  input.addEventListener('blur', onTextInputBlur)

  overlay.appendChild(input)

  // Hint below input: enter submits current annotation. Re-pressing the
  // draw-mode shortcut while editing also submits and exits draw mode.
  const inputHint = document.createElement('div')
  inputHint.id = 'kaboom-draw-input-hint'
  const hintTop = parseInt(input.style.top) + 42
  Object.assign(inputHint.style, {
    position: 'absolute',
    left: input.style.left,
    top: `${hintTop}px`,
    color: '#888',
    fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',
    fontSize: '11px',
    pointerEvents: 'none',
    zIndex: String(OVERLAY_Z_INDEX + 2)
  })
  inputHint.textContent = 'Enter to submit \u00b7 Draw shortcut again submits + exits \u00b7 Esc cancels'
  overlay.appendChild(inputHint)

  textInput = input
  input.focus()
}

function onTextInputKeyDown(e) {
  e.stopPropagation()
  if (e.key === 'Enter') {
    e.preventDefault()
    confirmTextInput()
  } else if (e.key === 'Escape') {
    e.preventDefault()
    cancelTextInput()
    renderAnnotations()
  }
}

function onTextInputBlur() {
  // Auto-confirm on blur
  if (textInput) {
    confirmTextInput()
  }
}

function removeInputHint() {
  const hint = document.getElementById('kaboom-draw-input-hint')
  if (hint) hint.remove()
}

function confirmTextInput() {
  if (!textInput) return
  // Capture and null immediately to prevent re-entry from blur during remove()
  const input = textInput
  textInput = null

  const text = input.value.trim()
  const viewportRect = JSON.parse(input.dataset.rectJson)
  const rect = toDocumentRect(viewportRect)
  const elementData = JSON.parse(input.dataset.elementJson)

  // Remove input element and hint
  input.removeEventListener('keydown', onTextInputKeyDown)
  input.removeEventListener('blur', onTextInputBlur)
  input.remove()
  removeInputHint()

  // Empty text → discard annotation
  if (!text) {
    renderAnnotations()
    return
  }

  // Create annotation
  const id = `ann_${Date.now()}_${Math.random().toString(36).slice(2, 5)}`
  const correlationId = `ann_detail_${Math.random().toString(36).slice(2, 8)}`
  const actionTrail = snapshotActionTrail(ACTION_TRAIL_LIMIT)
  const uiContext = collectUIContextMetadata()

  const annotation = {
    id,
    rect,
    coord_space: COORD_SPACE_DOCUMENT,
    text,
    timestamp: Date.now(),
    page_url: window.location.href,
    element_summary: elementData.summary || '',
    correlation_id: correlationId,
    action_trail: actionTrail,
    ui_context: uiContext
  }
  annotations.push(annotation)

  // Store full detail for lazy retrieval
  elementDetails.set(correlationId, {
    ...elementData.detail,
    action_trail: actionTrail,
    ui_context: uiContext
  })

  renderAnnotations()
  persistAnnotations()
}

function cancelTextInput() {
  if (!textInput) return
  textInput.removeEventListener('keydown', onTextInputKeyDown)
  textInput.removeEventListener('blur', onTextInputBlur)
  textInput.remove()
  removeInputHint()
  textInput = null
}

// ============================================================================
// DOM ELEMENT CAPTURE
// ============================================================================

/**
 * Capture DOM elements under the drawn rectangle.
 * Temporarily hides overlay to use document.elementsFromPoint().
 * Captures all elements (capped at MAX_CAPTURED_ELEMENTS), shadow DOM, iframes, and HTML.
 */
const MAX_CAPTURED_ELEMENTS = 15

function captureElementsUnderRect(rect) {
  if (!overlay) return { summary: '', detail: {} }

  // Temporarily hide overlay
  overlay.style.pointerEvents = 'none'
  overlay.style.visibility = 'hidden'

  try {
    // Sample points: corners + center + edge midpoints for better coverage
    const points = [
      { x: rect.x + rect.width / 2, y: rect.y + rect.height / 2 }, // center
      { x: rect.x + 2, y: rect.y + 2 }, // top-left
      { x: rect.x + rect.width - 2, y: rect.y + 2 }, // top-right
      { x: rect.x + 2, y: rect.y + rect.height - 2 }, // bottom-left
      { x: rect.x + rect.width - 2, y: rect.y + rect.height - 2 }, // bottom-right
      { x: rect.x + rect.width / 2, y: rect.y + 2 }, // top-center
      { x: rect.x + rect.width / 2, y: rect.y + rect.height - 2 }, // bottom-center
      { x: rect.x + 2, y: rect.y + rect.height / 2 }, // left-center
      { x: rect.x + rect.width - 2, y: rect.y + rect.height / 2 } // right-center
    ]

    const seenElements = new Set()
    const elements = []

    for (const pt of points) {
      try {
        const els = document.elementsFromPoint(pt.x, pt.y)
        for (const el of els) {
          if (seenElements.has(el)) continue
          if (el === document.body || el === document.documentElement) continue
          seenElements.add(el)
          elements.push(el)
          if (elements.length >= MAX_CAPTURED_ELEMENTS) break
        }
      } catch {
        // elementsFromPoint may fail on some edge cases
      }
      if (elements.length >= MAX_CAPTURED_ELEMENTS) break
    }

    // Fallback: if grid sampling found nothing, try single-point probe at rect center
    if (elements.length === 0) {
      try {
        const cx = rect.x + rect.width / 2
        const cy = rect.y + rect.height / 2
        const el = document.elementFromPoint(cx, cy)
        if (el && el !== document.body && el !== document.documentElement && !seenElements.has(el)) {
          seenElements.add(el)
          elements.push(el)
        }
      } catch {
        // elementFromPoint fallback failed
      }
    }

    // Fallback: walk DOM for elements whose bounding rect overlaps the drawn rectangle
    if (elements.length === 0) {
      try {
        const candidates = document.querySelectorAll('*')
        for (const el of candidates) {
          if (el === document.body || el === document.documentElement) continue
          if (seenElements.has(el)) continue
          const br = el.getBoundingClientRect()
          if (br.width === 0 && br.height === 0) continue
          const overlaps =
            br.left < rect.x + rect.width && br.right > rect.x && br.top < rect.y + rect.height && br.bottom > rect.y
          if (overlaps) {
            seenElements.add(el)
            elements.push(el)
            if (elements.length >= MAX_CAPTURED_ELEMENTS) break
          }
        }
      } catch {
        // DOM walk fallback failed
      }
    }

    // Also probe inside same-origin iframes
    const iframeElements = captureIframeElements(rect, seenElements)

    // Pick the most relevant element for the summary
    const target = pickBestElement(elements) || elements[0]

    if (!target && elements.length === 0 && iframeElements.length === 0) {
      return { summary: '', detail: {} }
    }

    const summary = target ? buildElementSummary(target) : ''

    // Build comprehensive detail: primary element + all elements + iframes
    const primaryDetail = target ? buildElementDetail(target) : {}
    const allElementDetails = elements.slice(0, MAX_CAPTURED_ELEMENTS).map((el) => ({
      tag: el.tagName.toLowerCase(),
      selector: buildCSSSelector(el),
      text: (el.textContent || '').trim().slice(0, 100),
      classes: Array.from(el.classList).slice(0, 10)
    }))

    const detail = {
      ...primaryDetail,
      all_elements: allElementDetails,
      element_count: elements.length
    }

    if (iframeElements.length > 0) {
      detail.iframe_content = iframeElements
    }

    return { summary, detail }
  } finally {
    // Always restore overlay, even if an exception occurs
    if (overlay) {
      overlay.style.pointerEvents = ''
      overlay.style.visibility = ''
    }
  }
}

/**
 * Capture elements inside same-origin iframes that overlap the drawn rectangle.
 * Cross-origin iframes are noted but their DOM is inaccessible.
 */
function captureIframeElements(rect, seenElements) {
  const results = []
  try {
    const iframes = document.querySelectorAll('iframe')
    for (const iframe of iframes) {
      const iframeRect = iframe.getBoundingClientRect()
      // Check if iframe overlaps with drawn rectangle
      if (
        iframeRect.right < rect.x ||
        iframeRect.left > rect.x + rect.width ||
        iframeRect.bottom < rect.y ||
        iframeRect.top > rect.y + rect.height
      ) {
        continue
      }
      try {
        const iframeDoc = iframe.contentDocument
        if (!iframeDoc) {
          results.push({ src: iframe.src, access: 'cross_origin', note: 'Cannot access cross-origin iframe DOM' })
          continue
        }
        // Adjust coordinates relative to iframe
        const adjustedX = rect.x - iframeRect.left + rect.width / 2
        const adjustedY = rect.y - iframeRect.top + rect.height / 2
        const els = iframeDoc.elementsFromPoint(adjustedX, adjustedY)
        const iframeEls = []
        for (const el of els) {
          if (seenElements.has(el)) continue
          if (el === iframeDoc.body || el === iframeDoc.documentElement) continue
          seenElements.add(el)
          iframeEls.push({
            tag: el.tagName.toLowerCase(),
            selector: buildCSSSelector(el),
            text: (el.textContent || '').trim().slice(0, 100),
            outer_html: el.outerHTML.slice(0, 500)
          })
          if (iframeEls.length >= 5) break
        }
        if (iframeEls.length > 0) {
          results.push({ src: iframe.src, access: 'same_origin', elements: iframeEls })
        }
      } catch {
        results.push({ src: iframe.src, access: 'blocked', note: 'SecurityError accessing iframe' })
      }
    }
  } catch {
    // Ignore iframe enumeration errors
  }
  return results
}

/**
 * Re-capture DOM element details for all existing annotations.
 * Called right before screenshot to ensure DOM data matches the visual state.
 */
function refreshElementDetails() {
  if (!overlay) return
  for (const ann of annotations) {
    if (!ann.rect || !ann.correlation_id) continue
    try {
      const freshData = captureElementsUnderRect(toViewportRect(ann.rect, ann.coord_space))
      if (freshData.detail && Object.keys(freshData.detail).length > 0) {
        const existing = elementDetails.get(ann.correlation_id) || {}
        elementDetails.set(ann.correlation_id, {
          ...freshData.detail,
          action_trail: existing.action_trail || ann.action_trail || [],
          ui_context: existing.ui_context || ann.ui_context || collectUIContextMetadata()
        })
        ann.element_summary = freshData.summary || ann.element_summary
      }
    } catch {
      // Keep existing data if re-capture fails
    }
  }
}

/**
 * Pick the most semantically relevant element from candidates.
 * Prefers interactive elements (button, a, input) over containers (div, span).
 */
function pickBestElement(elements) {
  const interactiveTags = new Set(['BUTTON', 'A', 'INPUT', 'SELECT', 'TEXTAREA', 'LABEL'])
  for (const el of elements) {
    if (interactiveTags.has(el.tagName)) return el
  }
  // Fall back to first element with meaningful text content
  for (const el of elements) {
    const text = el.textContent?.trim()
    if (text && text.length < 200 && text.length > 0) return el
  }
  return null
}

/**
 * Build compact element summary: "tag.class1.class2 'text'"
 */
function buildElementSummary(el) {
  const tag = el.tagName.toLowerCase()
  const classes = Array.from(el.classList).slice(0, 3).join('.')
  const text = (el.textContent || '').trim().slice(0, 40)
  let summary = tag
  if (classes) summary += '.' + classes
  if (text) summary += ` '${text}'`
  return summary
}

/**
 * Build full element detail for lazy retrieval.
 */
function buildElementDetail(el) {
  const computed = window.getComputedStyle(el)
  const styleProps = [
    'background-color',
    'color',
    'font-size',
    'font-weight',
    'font-family',
    'padding',
    'margin',
    'border',
    'border-radius',
    'display',
    'position',
    'z-index',
    'width',
    'height',
    'opacity',
    'flex-direction',
    'flex-wrap',
    'align-items',
    'justify-content',
    'gap',
    'grid-template-columns',
    'grid-template-rows',
    'overflow',
    'text-align',
    'text-decoration',
    'line-height',
    'letter-spacing',
    'box-shadow',
    'transform',
    'transition',
    'cursor',
    'visibility',
    'white-space',
    'max-width',
    'min-width',
    'max-height',
    'min-height'
  ]
  const computedStyles = {}
  for (const prop of styleProps) {
    computedStyles[prop] = computed.getPropertyValue(prop)
  }

  const boundingRect = el.getBoundingClientRect()

  // Build parent selector
  let parentSelector = ''
  try {
    const parent = el.parentElement
    if (parent && parent !== document.body && parent !== document.documentElement) {
      const pTag = parent.tagName.toLowerCase()
      const pClasses = Array.from(parent.classList).slice(0, 2).join('.')
      parentSelector = pTag
      if (parent.id) parentSelector += '#' + parent.id
      else if (pClasses) parentSelector += '.' + pClasses
      parentSelector += ' > '

      const childTag = el.tagName.toLowerCase()
      const childClasses = Array.from(el.classList).slice(0, 2).join('.')
      parentSelector += childTag
      if (el.id) parentSelector += '#' + el.id
      else if (childClasses) parentSelector += '.' + childClasses
    }
  } catch {
    // Ignore selector build errors
  }

  // Capture outer HTML (truncated for large elements)
  let outerHtml = ''
  try {
    outerHtml = el.outerHTML.slice(0, 2000)
  } catch {
    // outerHTML may fail on some special elements
  }

  // Shadow DOM detection
  let shadowInfo = null
  try {
    if (el.shadowRoot) {
      // Open shadow DOM — capture inner HTML
      shadowInfo = {
        mode: 'open',
        html: el.shadowRoot.innerHTML.slice(0, 2000),
        child_count: el.shadowRoot.childElementCount
      }
    } else if (el.attachShadow) {
      // Element supports shadow DOM but may have closed shadow root
      // We can detect this heuristically: if the element has no children but renders content
      const hasVisibleContent = el.getBoundingClientRect().height > 0
      const hasLightDOMChildren = el.childElementCount > 0
      if (hasVisibleContent && !hasLightDOMChildren && el.tagName.includes('-')) {
        shadowInfo = { mode: 'closed', note: 'Element likely has closed shadow DOM (content not accessible)' }
      }
    }
  } catch {
    // Shadow DOM access may fail
  }

  // Build parent_context: structured 2-level ancestry
  let parentContext = null
  try {
    const parent = el.parentElement
    if (parent && parent !== document.body && parent !== document.documentElement) {
      const parentInfo = {
        tag: parent.tagName.toLowerCase(),
        classes: Array.from(parent.classList).slice(0, 5),
        id: parent.id || '',
        role: (parent.getAttribute && parent.getAttribute('role')) || ''
      }
      const grandparent = parent.parentElement
      let grandparentInfo = null
      if (grandparent && grandparent !== document.body && grandparent !== document.documentElement) {
        grandparentInfo = {
          tag: grandparent.tagName.toLowerCase(),
          classes: Array.from(grandparent.classList).slice(0, 5),
          id: grandparent.id || '',
          role: (grandparent.getAttribute && grandparent.getAttribute('role')) || ''
        }
      }
      parentContext = { parent: parentInfo, grandparent: grandparentInfo }
    }
  } catch {
    // Ignore parent context build errors
  }

  // Build siblings: up to 2 before and 2 after the target element
  let siblings = []
  try {
    const parent = el.parentElement
    if (parent) {
      const children = Array.from(parent.children)
      const idx = children.indexOf(el)
      if (idx >= 0) {
        const before = children.slice(Math.max(0, idx - 2), idx)
        const after = children.slice(idx + 1, idx + 3)
        for (const sib of before) {
          siblings.push({
            tag: sib.tagName.toLowerCase(),
            classes: Array.from(sib.classList).slice(0, 5),
            text: (sib.textContent || '').trim().slice(0, 60),
            position: 'before'
          })
        }
        for (const sib of after) {
          siblings.push({
            tag: sib.tagName.toLowerCase(),
            classes: Array.from(sib.classList).slice(0, 5),
            text: (sib.textContent || '').trim().slice(0, 60),
            position: 'after'
          })
        }
      }
    }
  } catch {
    // Ignore sibling capture errors
  }

  const detail = {
    selector: buildCSSSelector(el),
    tag: el.tagName.toLowerCase(),
    text_content: (el.textContent || '').trim().slice(0, 200),
    outer_html: outerHtml,
    classes: Array.from(el.classList).slice(0, 20),
    id: el.id || '',
    computed_styles: computedStyles,
    parent_selector: parentSelector,
    bounding_rect: {
      x: Math.round(boundingRect.x),
      y: Math.round(boundingRect.y),
      width: Math.round(boundingRect.width),
      height: Math.round(boundingRect.height)
    },
    a11y_flags: runA11yChecks(el, computed)
  }

  const selectorCandidates = collectSelectorCandidates(el)
  if (selectorCandidates.length > 0) {
    detail.selector_candidates = selectorCandidates
  }

  if (parentContext) {
    detail.parent_context = parentContext
  }
  if (siblings.length > 0) {
    detail.siblings = siblings
  }
  const cssFramework = detectCSSFramework(el)
  if (cssFramework) {
    detail.css_framework = cssFramework
  }

  if (shadowInfo) {
    detail.shadow_dom = shadowInfo
  }

  // CSS rule tracing — find source stylesheets and selectors
  const matchedRules = traceMatchedCSSRules(el)
  if (matchedRules.length > 0) {
    detail.matched_css_rules = matchedRules
  }

  // Framework component detection
  const componentInfo = detectComponentSource(el)
  if (componentInfo) {
    if (componentInfo.framework) {
      detail.js_framework = componentInfo.framework
    }
    detail.component = componentInfo
  }

  return detail
}

/**
 * Detect CSS framework from element class names.
 * Returns framework name string or empty string if no confident match.
 */
function detectCSSFramework(el) {
  try {
    const classes = Array.from(el.classList)
    if (classes.length === 0) return ''

    // Tailwind: utility class patterns (require at least 1 dash-pattern for confidence)
    const tailwindSpecific = /^(p-\d|m-\d|px-\d|py-\d|mx-\d|my-\d|pt-\d|pb-\d|pl-\d|pr-\d|mt-\d|mb-\d|ml-\d|mr-\d|text-(xs|sm|base|lg|xl|2xl|3xl)|font-(thin|light|normal|medium|semibold|bold)|bg-[a-z]+-\d{2,3}|w-\d|h-\d|gap-\d|space-[xy]-\d|max-w-[\w-]+|min-w-[\w-]+|max-h-[\w-]+|min-h-[\w-]+|justify-[\w-]+|items-[\w-]+|self-[\w-]+|z-\d|opacity-[\w]+|duration-[\w]+|ease-[\w-]+|translate-[\w-]+|scale-[\w-]+|rotate-[\w-]+|skew-[\w-]+|origin-[\w-]+|delay-[\w]+)$/
    const tailwindGeneric = /^(flex|grid|block|inline|hidden|rounded|border|shadow|overflow-|transition)$/
    let tailwindHits = 0
    let tailwindSpecificHits = 0
    for (const cls of classes) {
      if (tailwindSpecific.test(cls)) { tailwindHits++; tailwindSpecificHits++ }
      else if (tailwindGeneric.test(cls)) tailwindHits++
    }
    if (tailwindHits >= 3 && tailwindSpecificHits >= 1) return 'tailwind'

    // Bootstrap: component/grid patterns
    const bootstrapPatterns = /^(col-(xs|sm|md|lg|xl)-\d+|col-\d+|btn-[a-z]+|form-control|form-group|form-check|input-group|card|container|row|navbar|nav-[a-z]+|modal|badge|alert|dropdown|table|pagination)$/
    let bootstrapHits = 0
    for (const cls of classes) {
      if (bootstrapPatterns.test(cls)) bootstrapHits++
    }
    if (bootstrapHits >= 2) return 'bootstrap'

    // CSS Modules: hash-suffixed classes like Component_name__hash
    const cssModulesPattern = /^[A-Z][a-zA-Z]*_[a-zA-Z]+__[a-zA-Z0-9]{5,}$/
    let modulesHits = 0
    for (const cls of classes) {
      if (cssModulesPattern.test(cls)) modulesHits++
    }
    if (modulesHits >= 1) return 'css-modules'

    // Styled-components/Emotion: css-* or sc-* prefixed classes
    const styledPattern = /^(css-[a-z0-9]+|sc-[a-zA-Z]+)$/
    let styledHits = 0
    for (const cls of classes) {
      if (styledPattern.test(cls)) styledHits++
    }
    if (styledHits >= 2) return 'styled-components'

    return ''
  } catch {
    return ''
  }
}

/**
 * Run lightweight accessibility checks on an element.
 * Returns an array of flag strings describing potential issues.
 * @param {Element} el
 * @param {CSSStyleDeclaration} computed
 * @returns {string[]}
 */
function runA11yChecks(el, computed) {
  const flags = []
  if (!el || !el.tagName) return flags
  const tag = el.tagName.toLowerCase()
  const getAttribute = (name) => (typeof el.getAttribute === 'function' ? el.getAttribute(name) : null)

  // 1. Image without alt text
  if (tag === 'img' && !getAttribute('alt')) {
    flags.push('missing_alt_text')
  }

  // 2. Interactive element without accessible name
  const interactiveTags = ['button', 'a', 'input', 'select', 'textarea']
  if (interactiveTags.includes(tag)) {
    const hasLabel = getAttribute('aria-label') || getAttribute('aria-labelledby') || getAttribute('title')
    const hasText = (el.textContent || '').trim()
    const hasPlaceholder = getAttribute('placeholder')
    if (!hasLabel && !hasText && !hasPlaceholder) {
      flags.push('missing_accessible_name')
    }
  }

  // 3. Div/span with click handler but no role
  if ((tag === 'div' || tag === 'span') && !getAttribute('role')) {
    if (getAttribute('onclick') || getAttribute('tabindex')) {
      flags.push('interactive_without_role')
    }
  }

  // 4. Contrast ratio check (foreground vs background)
  try {
    if (computed && typeof computed.getPropertyValue === 'function') {
      const fg = parseRGBColor(computed.getPropertyValue('color'))
      const bg = parseRGBColor(computed.getPropertyValue('background-color'))
      if (fg && bg && bg.a > 0) {
        const ratio = contrastRatio(fg, bg)
        const fontSize = parseFloat(computed.getPropertyValue('font-size'))
        const isBold = parseInt(computed.getPropertyValue('font-weight'), 10) >= 700
        const isLargeText = fontSize >= 24 || (fontSize >= 18.66 && isBold)
        const minRatio = isLargeText ? 3 : 4.5
        if (ratio < minRatio) {
          flags.push(`low_contrast:${ratio.toFixed(1)}:1`)
        }
      }
    }
  } catch {
    // Ignore contrast parse errors
  }

  // 5. Focus indicator removed
  try {
    if (interactiveTags.includes(tag) && computed && typeof computed.getPropertyValue === 'function') {
      const outline = computed.getPropertyValue('outline')
      const outlineStyle = computed.getPropertyValue('outline-style')
      if (outlineStyle === 'none' || outline === '0' || outline === 'none') {
        const boxShadow = computed.getPropertyValue('box-shadow')
        if (!boxShadow || boxShadow === 'none') {
          flags.push('no_focus_indicator')
        }
      }
    }
  } catch {
    // Ignore focus indicator check errors
  }

  // 6. Missing form label
  try {
    if ((tag === 'input' || tag === 'select' || tag === 'textarea') && !getAttribute('aria-label')) {
      const id = el.id
      const hasLabelFor =
        id &&
        typeof document !== 'undefined' &&
        typeof document.querySelector === 'function' &&
        document.querySelector(`label[for="${CSS.escape(id)}"]`)
      if (!hasLabelFor) {
        const parent = typeof el.closest === 'function' ? el.closest('label') : null
        if (!parent) {
          flags.push('missing_form_label')
        }
      }
    }
  } catch {
    // Ignore form label check errors
  }

  // 7. Small touch target (< 44x44 CSS pixels per WCAG 2.5.8)
  try {
    if (interactiveTags.includes(tag) && typeof el.getBoundingClientRect === 'function') {
      const rect = el.getBoundingClientRect()
      if (rect.width > 0 && rect.height > 0 && (rect.width < 44 || rect.height < 44)) {
        flags.push(`small_touch_target:${Math.round(rect.width)}x${Math.round(rect.height)}`)
      }
    }
  } catch {
    // Ignore touch target check errors
  }

  return flags
}

/**
 * Parse an RGB/RGBA color string into {r, g, b, a}.
 * @param {string} str - e.g. "rgb(255, 0, 0)" or "rgba(0, 0, 0, 0.5)"
 * @returns {{r:number, g:number, b:number, a:number}|null}
 */
function parseRGBColor(str) {
  if (!str) return null
  const m = str.match(/rgba?\((\d+),\s*(\d+),\s*(\d+)(?:,\s*([\d.]+))?\)/)
  if (!m) return null
  return {
    r: parseInt(m[1], 10),
    g: parseInt(m[2], 10),
    b: parseInt(m[3], 10),
    a: m[4] !== undefined ? parseFloat(m[4]) : 1
  }
}

/**
 * Calculate relative luminance of an sRGB color per WCAG 2.x.
 * @param {{r:number, g:number, b:number}} c
 * @returns {number}
 */
function luminance(c) {
  const [rs, gs, bs] = [c.r / 255, c.g / 255, c.b / 255].map((v) =>
    v <= 0.04045 ? v / 12.92 : Math.pow((v + 0.055) / 1.055, 2.4)
  )
  return 0.2126 * rs + 0.7152 * gs + 0.0722 * bs
}

/**
 * Calculate WCAG contrast ratio between two colors.
 * @param {{r:number, g:number, b:number}} fg
 * @param {{r:number, g:number, b:number}} bg
 * @returns {number}
 */
function contrastRatio(fg, bg) {
  const l1 = luminance(fg)
  const l2 = luminance(bg)
  const lighter = Math.max(l1, l2)
  const darker = Math.min(l1, l2)
  return (lighter + 0.05) / (darker + 0.05)
}

/**
 * Build a CSS selector for the element.
 */
function buildCSSSelector(el) {
  const tag = el.tagName.toLowerCase()
  if (el.id) return `${tag}#${CSS.escape(el.id)}`
  const classes = Array.from(el.classList).slice(0, 3)
  if (classes.length > 0) return `${tag}.${classes.map((c) => CSS.escape(c)).join('.')}`
  return tag
}

const MAX_SELECTOR_CANDIDATES = 8

function collectSelectorCandidates(el) {
  const candidates = []
  if (!el || !el.tagName) {
    return candidates
  }

  const safeAdd = (candidate) => {
    if (!candidate || typeof candidate !== 'string') return
    const normalized = candidate.trim()
    if (!normalized || candidates.includes(normalized)) return
    if (candidates.length >= MAX_SELECTOR_CANDIDATES) return
    candidates.push(normalized)
  }
  const getAttribute = (name) => (typeof el.getAttribute === 'function' ? el.getAttribute(name) : null)
  const normalizeText = (value, max) =>
    String(value || '')
      .replace(/\s+/g, ' ')
      .replace(/\|/g, '/')
      .trim()
      .slice(0, max)
  const escapeAttr = (value) => {
    const raw = String(value || '')
    if (typeof CSS !== 'undefined' && typeof CSS.escape === 'function') return CSS.escape(raw)
    return raw.replace(/\\/g, '\\\\').replace(/"/g, '\\"')
  }
  const tag = el.tagName.toLowerCase()
  const text = normalizeText(el.textContent || '', 80)

  if (el.id) {
    safeAdd(`css=#${escapeAttr(el.id)}`)
  }

  const testID = getAttribute('data-testid') || getAttribute('data-test-id') || getAttribute('data-cy')
  if (testID) {
    safeAdd(`testid=${normalizeText(testID, 120)}`)
  }

  const ariaLabel = getAttribute('aria-label')
  if (ariaLabel) {
    safeAdd(`label=${normalizeText(ariaLabel, 120)}`)
  }

  const placeholder = getAttribute('placeholder')
  if (placeholder) {
    safeAdd(`placeholder=${normalizeText(placeholder, 120)}`)
  }

  const explicitRole = getAttribute('role')
  const implicitRole = inferImplicitRole(el)
  const role = explicitRole || implicitRole
  if (role && text) {
    safeAdd(`role=${normalizeText(role, 60)}|${text}`)
  } else if (role) {
    safeAdd(`role=${normalizeText(role, 60)}`)
  }

  if (text) {
    safeAdd(`text=${text}`)
  }

  const nameAttr = getAttribute('name')
  if (nameAttr) {
    safeAdd(`css=${tag}[name="${escapeAttr(nameAttr)}"]`)
  }

  safeAdd(`css=${buildCSSSelector(el)}`)
  return candidates
}

function inferImplicitRole(el) {
  if (!el || !el.tagName) return ''
  const tag = el.tagName.toLowerCase()
  if (tag === 'button') return 'button'
  if (tag === 'a' && typeof el.getAttribute === 'function' && el.getAttribute('href')) return 'link'
  if (tag === 'select') return 'combobox'
  if (tag === 'textarea') return 'textbox'
  if (tag === 'input') {
    const inputType = (typeof el.getAttribute === 'function' ? el.getAttribute('type') : '') || 'text'
    switch (inputType.toLowerCase()) {
      case 'button':
      case 'submit':
      case 'reset':
        return 'button'
      case 'checkbox':
        return 'checkbox'
      case 'radio':
        return 'radio'
      default:
        return 'textbox'
    }
  }
  return ''
}

/**
 * Trace CSS rules that match an element using document.styleSheets.
 * Returns matched rules with selector, properties, and source stylesheet.
 * Capped at MAX_MATCHED_RULES to avoid huge payloads.
 */
const MAX_MATCHED_RULES = 20
const MAX_RULES_EXAMINED = 5000 // Safety cap to prevent excessive work on huge stylesheets

function traceMatchedCSSRules(el) {
  const rules = []
  let totalExamined = 0
  try {
    for (const sheet of document.styleSheets) {
      if (totalExamined >= MAX_RULES_EXAMINED) break
      let cssRules
      try {
        cssRules = sheet.cssRules || sheet.rules
      } catch {
        // CORS blocks access to cross-origin stylesheets
        rules.push({
          stylesheet: sheet.href || '(inline)',
          access: 'blocked',
          note: 'Cross-origin stylesheet — rules not accessible'
        })
        continue
      }
      if (!cssRules) continue

      const sheetHref = sheet.href || '(inline)'
      for (let i = 0; i < cssRules.length; i++) {
        if (rules.length >= MAX_MATCHED_RULES) break
        if (++totalExamined > MAX_RULES_EXAMINED) break
        const rule = cssRules[i]
        if (rule.type !== CSSRule.STYLE_RULE) continue
        try {
          if (!el.matches(rule.selectorText)) continue
        } catch {
          continue // Invalid selector
        }
        // Extract only properties that differ from defaults (non-empty)
        const properties = {}
        for (let j = 0; j < rule.style.length; j++) {
          const prop = rule.style[j]
          properties[prop] = rule.style.getPropertyValue(prop)
          const priority = rule.style.getPropertyPriority(prop)
          if (priority) properties[prop] += ' !' + priority
        }
        rules.push({
          selector: rule.selectorText,
          properties,
          stylesheet: sheetHref,
          rule_index: i
        })
      }
      if (rules.length >= MAX_MATCHED_RULES) break
    }
  } catch {
    // Stylesheet enumeration may fail in rare cases
  }
  return rules
}

/**
 * Detect framework component information for an element.
 * Supports React, Vue, Angular, and common data attributes.
 */
function detectComponentSource(el) {
  const info = {}

  // React: __reactFiber$ or __reactInternalInstance$
  try {
    for (const key of Object.keys(el)) {
      if (key.startsWith('__reactFiber$') || key.startsWith('__reactInternalInstance$')) {
        const fiber = el[key]
        if (fiber) {
          info.framework = 'react'
          // Walk up to find the named component
          let node = fiber
          for (let depth = 0; depth < 10 && node; depth++) {
            if (typeof node.type === 'function' || typeof node.type === 'object') {
              const name = node.type?.displayName || node.type?.name || node.type?.render?.name
              if (name) {
                info.component = name
                // Try to get source file from _source (dev mode only)
                if (node._debugSource) {
                  info.source_file = node._debugSource.fileName
                  info.source_line = node._debugSource.lineNumber
                }
                break
              }
            }
            node = node.return
          }
        }
        break
      }
    }
  } catch {
    // React internals may throw
  }

  // Vue 2: __vue__ / Vue 3: __vue_app__ or __vueParentComponent
  if (!info.framework) {
    try {
      const vue = el.__vue__ || el.__vueParentComponent
      if (vue) {
        info.framework = 'vue'
        info.component = vue.$options?.name || vue.type?.name || vue.type?.__name || ''
        if (vue.$options?.__file) info.source_file = vue.$options.__file
        if (vue.type?.__file) info.source_file = vue.type.__file
      }
    } catch {
      // Vue internals may throw
    }
  }

  // Angular: ng-* attributes
  if (!info.framework) {
    try {
      for (const attr of el.attributes) {
        if (attr.name.startsWith('_ngcontent') || attr.name.startsWith('_nghost')) {
          info.framework = 'angular'
          // Try to get component name from ng-reflect-* or constructor
          const ngComponent = el.__ngContext__
          if (ngComponent) {
            info.component = el.constructor?.name || ''
          }
          break
        }
      }
    } catch {
      // Angular detection may fail
    }
  }

  // Common data attributes
  try {
    const testId = el.getAttribute('data-testid') || el.getAttribute('data-test-id') || el.getAttribute('data-cy')
    if (testId) info.test_id = testId
    const component = el.getAttribute('data-component') || el.getAttribute('data-source')
    if (component) info.data_component = component
  } catch {
    // Attribute access may fail
  }

  return Object.keys(info).length > 0 ? info : null
}

// ============================================================================
// PERSISTENCE (chrome.storage.session)
// ============================================================================

const MAX_PERSISTED_ANNOTATIONS = 50

// Guard: detect if chrome.storage.session is accessible in this execution context.
// In web_accessible_resource contexts the API object exists but every call throws
// "Access to storage is not allowed from this context". We disable persistence
// permanently on the first failure to avoid noisy console errors.
let storageAvailable = (typeof chrome !== 'undefined' && !!chrome.storage?.session)

function persistAnnotations() {
  if (saveTimeout) clearTimeout(saveTimeout)
  saveTimeout = setTimeout(() => {
    if (!storageAvailable) return
    try {
      const key = 'gasoline_draw_annotations'
      const toStore =
        annotations.length > MAX_PERSISTED_ANNOTATIONS ? annotations.slice(-MAX_PERSISTED_ANNOTATIONS) : annotations
      chrome.storage.session.set(
        {
          [key]: {
            annotations: toStore,
            page_url: window.location.href,
            timestamp: Date.now()
          }
        },
        () => {
          if (chrome.runtime?.lastError) {
            storageAvailable = false
          }
        }
      )
    } catch {
      storageAvailable = false
    }
  }, 500) // Debounce 500ms
}

function clearPersistedAnnotations() {
  if (!storageAvailable) return
  try {
    chrome.storage.session.remove('gasoline_draw_annotations', () => {
      if (chrome.runtime?.lastError) {
        storageAvailable = false
      }
    })
  } catch {
    storageAvailable = false
  }
}

function loadAnnotations() {
  if (!storageAvailable) return
  try {
    const key = 'gasoline_draw_annotations'
    chrome.storage.session.get([key], (result) => {
      if (chrome.runtime?.lastError) {
        storageAvailable = false
        return
      }
      const data = result?.[key]
      if (data?.annotations && data.page_url === window.location.href) {
        annotations = data.annotations.map(normalizeLoadedAnnotation)
        renderAnnotations()
      }
    })
  } catch {
    storageAvailable = false
  }
}

// ============================================================================
// DEACTIVATION + RESULT DELIVERY
// ============================================================================

/**
 * Called when user presses ESC (no active text input), from popup toggle,
 * or from KABOOM_DRAW_MODE_STOP message.
 * Captures screenshot WHILE overlay is still visible, then deactivates and sends results.
 * Protected by re-entry guard to prevent double-ESC races.
 */
export function deactivateAndSendResults() {
  if (!active || isDeactivating) return
  isDeactivating = true

  // Shortcut/popup stop while an editor is open should behave like submit, not cancel.
  if (textInput) {
    if (!submitActiveTextInputBeforeExit()) {
      isDeactivating = false
      return {
        status: 'validation_error',
        message: 'Annotation text is required before exiting draw mode.'
      }
    }
  }

  const pageUrl = window.location.href
  const currentSessionName = sessionName // capture before deactivate clears it
  const currentCorrelationId = sessionCorrelationId // capture before deactivate clears it

  /**
   * Complete the deactivation: fade out overlay, show toast, tear down,
   * send results to background, and clear persisted storage.
   */
  const finishDeactivation = (screenshotDataUrl) => {
    // Fade out the overlay before tearing it down
    if (overlay) {
      overlay.style.opacity = '0'
    }

    // Show success toast via extension messaging
    try {
      if (typeof chrome !== 'undefined' && chrome.runtime) {
        chrome.runtime.sendMessage({
          type: 'kaboom_action_toast',
          text: 'Annotations submitted',
          state: 'success',
          duration_ms: 2000
        })
      }
    } catch {
      // Extension context may be invalidated
    }

    // Delay teardown to let fade complete
    setTimeout(() => {
      isDeactivating = false
      const result = deactivateDrawMode()
      // Clear persisted annotations from storage after successful deactivation
      clearPersistedAnnotations()
      try {
        if (typeof chrome !== 'undefined' && chrome.runtime) {
          const msg = {
            type: 'draw_mode_completed',
            annotations: result.annotations,
            elementDetails: result.elementDetails,
            page_url: pageUrl,
            screenshot_data_url: screenshotDataUrl,
            correlation_id: currentCorrelationId
          }
          if (currentSessionName) {
            msg.annot_session_name = currentSessionName
          }
          chrome.runtime.sendMessage(msg)
        }
      } catch {
        // Extension context may be invalidated
      }

      // Dispatch CustomEvent so content-script peers (e.g. terminal launcher)
      // can auto-send annotation summaries without round-tripping through background.
      try {
        window.dispatchEvent(new CustomEvent('kaboom-annotations-ready', {
          detail: {
            annotations: result.annotations,
            page_url: pageUrl
          }
        }))
      } catch {
        // CustomEvent dispatch failed — non-critical
      }
    }, 300)
  }

  // Re-capture DOM data for all annotations so element details match the screenshot moment.
  // Annotations may have been drawn minutes ago; the DOM may have changed since then.
  refreshElementDetails()

  // Request screenshot capture from background BEFORE deactivating,
  // so the overlay with annotation drawings is included in the screenshot.
  if (typeof chrome !== 'undefined' && chrome.runtime) {
    let screenshotHandled = false
    // Timeout fallback: if screenshot callback never fires (extension context
    // invalidated, background unresponsive), proceed without screenshot after 1s.
    const fallbackTimer = setTimeout(() => {
      if (!screenshotHandled) {
        screenshotHandled = true
        finishDeactivation('')
      }
    }, 1000)

    try {
      chrome.runtime.sendMessage({ type: 'kaboom_capture_screenshot' }, (screenshotResponse) => {
        if (screenshotHandled) return // Timeout already fired
        screenshotHandled = true
        clearTimeout(fallbackTimer)
        finishDeactivation(screenshotResponse?.dataUrl || '')
      })
    } catch {
      // Fallback: deactivate without screenshot
      if (!screenshotHandled) {
        screenshotHandled = true
        clearTimeout(fallbackTimer)
        finishDeactivation('')
      }
    }
  } else {
    deactivateDrawMode()
    isDeactivating = false
  }
}

function submitActiveTextInputBeforeExit() {
  if (!textInput) return true
  const text = textInput.value.trim()
  if (!text) {
    try {
      if (typeof chrome !== 'undefined' && chrome.runtime) {
        chrome.runtime.sendMessage({
          type: 'kaboom_action_toast',
          text: 'Annotation text required',
          detail: 'Type feedback, then press the shortcut again to submit.',
          state: 'error',
          duration_ms: 2500
        })
      }
    } catch {
      // Extension context may be invalidated
    }
    textInput.focus()
    return false
  }

  // Reuse Enter-submit path so payload matches explicit submit behavior.
  confirmTextInput()
  return true
}

// ============================================================================
// UTILITY
// ============================================================================

function normalizeRect(x1, y1, x2, y2) {
  return {
    x: Math.min(x1, x2),
    y: Math.min(y1, y2),
    width: Math.abs(x2 - x1),
    height: Math.abs(y2 - y1)
  }
}

function scrollOffsets() {
  return {
    x: window.scrollX || window.pageXOffset || 0,
    y: window.scrollY || window.pageYOffset || 0
  }
}

function toDocumentRect(rect) {
  const scroll = scrollOffsets()
  return {
    x: rect.x + scroll.x,
    y: rect.y + scroll.y,
    width: rect.width,
    height: rect.height
  }
}

function toViewportRect(rect, coordSpace) {
  if (!rect) {
    return { x: 0, y: 0, width: 0, height: 0 }
  }
  if (coordSpace === COORD_SPACE_DOCUMENT || coordSpace === undefined || coordSpace === null || coordSpace === '') {
    const scroll = scrollOffsets()
    return {
      x: rect.x - scroll.x,
      y: rect.y - scroll.y,
      width: rect.width,
      height: rect.height
    }
  }
  return {
    x: rect.x,
    y: rect.y,
    width: rect.width,
    height: rect.height
  }
}

function normalizeLoadedAnnotation(annotation) {
  if (!annotation || !annotation.rect) return annotation
  if (annotation.coord_space === COORD_SPACE_DOCUMENT) {
    if (!Array.isArray(annotation.action_trail)) annotation.action_trail = []
    if (!annotation.ui_context) annotation.ui_context = collectUIContextMetadata()
    return annotation
  }
  return {
    ...annotation,
    rect: toDocumentRect(annotation.rect),
    coord_space: COORD_SPACE_DOCUMENT,
    action_trail: Array.isArray(annotation.action_trail) ? annotation.action_trail : [],
    ui_context: annotation.ui_context || collectUIContextMetadata()
  }
}

function recordRecentAction(type, target, extra = {}) {
  const entry = {
    type,
    target_summary: summarizeActionTarget(target),
    timestamp: Date.now(),
    ...extra
  }
  recentActions.push(entry)
  if (recentActions.length > ACTION_BUFFER_LIMIT) {
    recentActions = recentActions.slice(recentActions.length - ACTION_BUFFER_LIMIT)
  }
}

function snapshotActionTrail(limit) {
  const max = Number.isFinite(limit) && limit > 0 ? Math.floor(limit) : ACTION_TRAIL_LIMIT
  const selected = recentActions.slice(-max)
  const now = Date.now()
  return selected.map((entry, index) => ({
    type: entry.type,
    target_summary: entry.target_summary,
    timestamp: entry.timestamp,
    delta_ms: Math.max(0, now - entry.timestamp),
    order: index + 1
  }))
}

function summarizeActionTarget(target) {
  if (!target || !target.tagName) return 'unknown'
  const tag = target.tagName.toLowerCase()
  const selector = safeBuildSelector(target)
  const role = typeof target.getAttribute === 'function' ? target.getAttribute('role') || '' : ''
  const text = (target.textContent || '').trim().replace(/\s+/g, ' ').slice(0, 60)
  const parts = [selector || tag]
  if (role) parts.push(`role=${role}`)
  if (text) parts.push(`text="${text}"`)
  return parts.join(' ')
}

function collectUIContextMetadata() {
  return {
    theme: detectTheme(),
    viewport: {
      width: window.innerWidth,
      height: window.innerHeight
    },
    sidebars: {
      left_open: isSidebarOpen([
        '[data-sidebar="left"]',
        '#left-sidebar',
        '.left-sidebar',
        '.sidebar-left',
        'aside.left'
      ]),
      right_open: isSidebarOpen([
        '[data-sidebar="right"]',
        '#right-sidebar',
        '.right-sidebar',
        '.sidebar-right',
        'aside.right'
      ])
    },
    focused_element: summarizeFocusedElement()
  }
}

function detectTheme() {
  try {
    const html = document.documentElement
    const dataTheme = html?.dataset?.theme
    if (dataTheme === 'dark' || dataTheme === 'light') return dataTheme
    if (html?.classList?.contains('dark')) return 'dark'
    if (html?.classList?.contains('light')) return 'light'
    if (typeof window.matchMedia === 'function' && window.matchMedia('(prefers-color-scheme: dark)').matches) {
      return 'dark'
    }
  } catch {
    // fallback below
  }
  return 'light'
}

function isSidebarOpen(selectors) {
  for (const selector of selectors) {
    let el = null
    try {
      el = document.querySelector(selector)
    } catch {
      el = null
    }
    if (!el) continue
    const rect = el.getBoundingClientRect ? el.getBoundingClientRect() : null
    const width = rect?.width || 0
    const height = rect?.height || 0
    if (width <= 0 || height <= 0) continue
    const computed = window.getComputedStyle?.(el)
    if (computed?.display === 'none' || computed?.visibility === 'hidden') continue
    return true
  }
  return false
}

function summarizeFocusedElement() {
  const el = document.activeElement
  if (!el || el === document.body || el === document.documentElement) return null
  return {
    selector: safeBuildSelector(el),
    tag: el.tagName?.toLowerCase?.() || '',
    role: typeof el.getAttribute === 'function' ? el.getAttribute('role') || '' : '',
    text: (el.textContent || '').trim().replace(/\s+/g, ' ').slice(0, 80)
  }
}

function safeBuildSelector(el) {
  try {
    return buildCSSSelector(el)
  } catch {
    return el?.tagName?.toLowerCase?.() || 'unknown'
  }
}
