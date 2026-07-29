// input-rendering.js — Pointer and keyboard input, canvas rendering, and annotation text entry.
/* eslint-disable no-unused-vars, no-undef */
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
    cancelDrawMode()
    e.preventDefault()
    e.stopPropagation()
  } else if (e.key === 'Enter' && !textInput && annotations.length > 0) {
    deactivateAndSendResults()
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

function updateActionBar() {
  const count = annotations.length
  if (actionSubmitButton) {
    actionSubmitButton.textContent = `Submit ${count} ${count === 1 ? 'annotation' : 'annotations'}`
    actionSubmitButton.disabled = count === 0
    actionSubmitButton.style.cursor = count === 0 ? 'not-allowed' : 'pointer'
    actionSubmitButton.style.opacity = count === 0 ? '0.55' : '1'
  }
  if (actionUndoButton) {
    actionUndoButton.disabled = count === 0
    actionUndoButton.style.cursor = count === 0 ? 'not-allowed' : 'pointer'
    actionUndoButton.style.opacity = count === 0 ? '0.55' : '1'
  }
}

function undoLatestAnnotation() {
  const removed = annotations.pop()
  if (!removed) return
  if (removed.correlation_id) elementDetails.delete(removed.correlation_id)
  renderAnnotations()
  persistAnnotations()
  updateActionBar()
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
  inputHint.textContent = 'Enter saves annotation \u00b7 Enter again submits all \u00b7 Esc cancels session'
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
    cancelDrawMode()
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
  updateActionBar()
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
