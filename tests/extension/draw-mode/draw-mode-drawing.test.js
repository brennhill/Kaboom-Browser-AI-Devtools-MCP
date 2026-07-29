// @ts-nocheck
/**
 * @fileoverview Draw-mode pointer mechanics and annotation context capture tests.
 */

import { test, describe, mock, beforeEach } from 'node:test'
import assert from 'node:assert'
import {
  createMockElement,
  createdElements,
  documentBody,
  importDrawMode,
  setupGlobals,
} from './draw-mode-fixture.js'

// =============================================================================
// Gap 2: Drawing Mechanics — mousedown → mousemove → mouseup → text → Enter
// =============================================================================

describe('Draw Mode — Drawing Mechanics', () => {
  let dm

  beforeEach(async () => {
    setupGlobals()
    dm = await importDrawMode()
  })

  test('mousedown + mousemove + mouseup creates text input for annotation', () => {
    dm.activateDrawMode('user')

    const overlay = documentBody.children[0]
    assert.ok(overlay, 'expected overlay element')

    overlay._dispatch('mousedown', { button: 0, clientX: 100, clientY: 100 })
    overlay._dispatch('mousemove', { clientX: 250, clientY: 200 })
    overlay._dispatch('mouseup', { clientX: 250, clientY: 200 })

    const inputEls = createdElements.filter((el) => el.tagName === 'INPUT')
    assert.ok(inputEls.length > 0, 'expected text input after drawing rectangle')
  })

  test('annotation composer shows desired-outcome helper guidance', () => {
    dm.activateDrawMode('user')
    const overlay = documentBody.children[0]

    overlay._dispatch('mousedown', { button: 0, clientX: 100, clientY: 100 })
    overlay._dispatch('mouseup', { clientX: 250, clientY: 200 })

    const inputEl = createdElements.find((el) => el.tagName === 'INPUT')
    assert.ok(inputEl, 'expected text input element')
    assert.ok(
      inputEl.placeholder.includes("Don't just tell the AI what's wrong"),
      `unexpected placeholder guidance: ${inputEl.placeholder}`
    )
  })

  test('tiny rectangle (< 5px) does not create text input', () => {
    dm.activateDrawMode('user')
    const overlay = documentBody.children[0]

    const inputsBefore = createdElements.filter((el) => el.tagName === 'INPUT').length

    overlay._dispatch('mousedown', { button: 0, clientX: 100, clientY: 100 })
    overlay._dispatch('mousemove', { clientX: 102, clientY: 102 })
    overlay._dispatch('mouseup', { clientX: 102, clientY: 102 })

    const inputsAfter = createdElements.filter((el) => el.tagName === 'INPUT').length
    assert.strictEqual(inputsAfter, inputsBefore, 'no text input for tiny rectangle')
  })

  test('completing text input with Enter creates annotation', () => {
    dm.activateDrawMode('user')
    const overlay = documentBody.children[0]

    overlay._dispatch('mousedown', { button: 0, clientX: 100, clientY: 100 })
    overlay._dispatch('mousemove', { clientX: 250, clientY: 200 })
    overlay._dispatch('mouseup', { clientX: 250, clientY: 200 })

    const inputEl = createdElements.find((el) => el.tagName === 'INPUT')
    assert.ok(inputEl, 'expected text input element')

    inputEl.value = 'make this darker'

    const enterHandler = inputEl._listeners['keydown']?.[0]
    if (enterHandler) {
      enterHandler({
        key: 'Enter',
        preventDefault: mock.fn(),
        stopPropagation: mock.fn()
      })
    }

    const annotations = dm.getAnnotations()
    assert.strictEqual(annotations.length, 1, 'expected 1 annotation')
    assert.strictEqual(annotations[0].text, 'make this darker')
    assert.ok(annotations[0].id, 'expected annotation id')
    assert.ok(annotations[0].correlation_id, 'expected correlation_id')
    assert.ok(annotations[0].rect, 'expected rect')
    assert.strictEqual(annotations[0].rect.x, 100)
    assert.strictEqual(annotations[0].rect.y, 100)
    assert.strictEqual(annotations[0].rect.width, 150)
    assert.strictEqual(annotations[0].rect.height, 100)
  })

  test('annotation rect is stored in document coordinates when page is scrolled', () => {
    globalThis.window.scrollX = 40
    globalThis.window.scrollY = 120

    dm.activateDrawMode('user')
    const overlay = documentBody.children[0]

    overlay._dispatch('mousedown', { button: 0, clientX: 100, clientY: 100 })
    overlay._dispatch('mouseup', { clientX: 250, clientY: 200 })

    const inputEl = createdElements.find((el) => el.tagName === 'INPUT')
    inputEl.value = 'doc-space'
    inputEl._listeners['keydown']?.[0]?.({ key: 'Enter', preventDefault: mock.fn(), stopPropagation: mock.fn() })

    const annotations = dm.getAnnotations()
    assert.strictEqual(annotations.length, 1)
    assert.strictEqual(annotations[0].rect.x, 140)
    assert.strictEqual(annotations[0].rect.y, 220)
    assert.strictEqual(annotations[0].coord_space, 'document')
  })

  test('scrolling re-renders annotations anchored to page content', () => {
    dm.activateDrawMode('user')
    const overlay = documentBody.children[0]

    overlay._dispatch('mousedown', { button: 0, clientX: 100, clientY: 100 })
    overlay._dispatch('mouseup', { clientX: 250, clientY: 200 })

    const inputEl = createdElements.find((el) => el.tagName === 'INPUT')
    inputEl.value = 'scroll-anchor'
    inputEl._listeners['keydown']?.[0]?.({ key: 'Enter', preventDefault: mock.fn(), stopPropagation: mock.fn() })

    const canvasEl = createdElements.find((el) => el.tagName === 'CANVAS')
    assert.ok(canvasEl?._context2d, 'expected canvas 2d context')
    canvasEl._context2d.strokeRect.mock.resetCalls()

    globalThis.window.scrollX = 20
    globalThis.window.scrollY = 40

    const scrollCall = globalThis.window.addEventListener.mock.calls.find((c) => c.arguments?.[0] === 'scroll')
    assert.ok(scrollCall, 'expected scroll listener registration')
    const onScroll = scrollCall.arguments[1]
    onScroll()

    assert.ok(canvasEl._context2d.stroke.mock.calls.length > 0, 'expected annotation redraw on scroll')
    const roundedRectMoveTo = canvasEl._context2d.moveTo.mock.calls.find((c) => c.arguments[0] === 84 && c.arguments[1] === 60)
    assert.ok(roundedRectMoveTo, 'expected viewport-adjusted rounded-rect path on scroll')
  })

  test('resize after scrolling preserves annotation alignment', () => {
    dm.activateDrawMode('user')
    const overlay = documentBody.children[0]

    overlay._dispatch('mousedown', { button: 0, clientX: 100, clientY: 100 })
    overlay._dispatch('mouseup', { clientX: 250, clientY: 200 })

    const inputEl = createdElements.find((el) => el.tagName === 'INPUT')
    inputEl.value = 'resize-anchor'
    inputEl._listeners['keydown']?.[0]?.({ key: 'Enter', preventDefault: mock.fn(), stopPropagation: mock.fn() })

    const canvasEl = createdElements.find((el) => el.tagName === 'CANVAS')
    assert.ok(canvasEl?._context2d, 'expected canvas 2d context')
    canvasEl._context2d.strokeRect.mock.resetCalls()

    globalThis.window.scrollX = 15
    globalThis.window.scrollY = 30
    globalThis.window.innerWidth = 1280
    globalThis.window.innerHeight = 900

    const resizeCall = globalThis.window.addEventListener.mock.calls.find((c) => c.arguments?.[0] === 'resize')
    assert.ok(resizeCall, 'expected resize listener registration')
    const onResize = resizeCall.arguments[1]
    onResize()

    assert.ok(canvasEl._context2d.stroke.mock.calls.length > 0, 'expected redraw on resize')
    const roundedRectMoveTo = canvasEl._context2d.moveTo.mock.calls.find((c) => c.arguments[0] === 89 && c.arguments[1] === 70)
    assert.ok(roundedRectMoveTo, 'expected viewport-adjusted rounded-rect path on resize')
  })

  test('second shortcut submit path commits active annotation text before exit', async () => {
    dm.activateDrawMode('user')
    const overlay = documentBody.children[0]

    const sentMessages = []
    globalThis.chrome.runtime.sendMessage = mock.fn((msg, callback) => {
      sentMessages.push(msg)
      if (msg.type === 'kaboom_capture_screenshot' && typeof callback === 'function') {
        callback({ dataUrl: 'data:image/png;base64,mockscreenshot' })
      }
      return undefined
    })

    overlay._dispatch('mousedown', { button: 0, clientX: 100, clientY: 100 })
    overlay._dispatch('mouseup', { clientX: 250, clientY: 200 })

    const inputEl = createdElements.find((el) => el.tagName === 'INPUT')
    inputEl.value = 'submit-via-shortcut'

    dm.deactivateAndSendResults()
    await new Promise((r) => setTimeout(r, 350))

    assert.strictEqual(dm.isDrawModeActive(), false, 'draw mode should be inactive after shortcut submit')

    const completed = sentMessages.find((m) => m.type === 'draw_mode_completed')
    assert.ok(completed, 'expected DRAW_MODE_COMPLETED message')
    assert.strictEqual(completed.annotations.length, 1)
    assert.strictEqual(completed.annotations[0].text, 'submit-via-shortcut')
  })

  test('Enter after completing an annotation submits the session', async () => {
    dm.activateDrawMode('user')
    const overlay = documentBody.children[0]
    const sentMessages = []
    globalThis.chrome.runtime.sendMessage = mock.fn((msg, callback) => {
      sentMessages.push(msg)
      if (msg.type === 'kaboom_capture_screenshot' && typeof callback === 'function') {
        callback({ dataUrl: 'data:image/png;base64,mockscreenshot' })
      }
      return undefined
    })

    overlay._dispatch('mousedown', { button: 0, clientX: 100, clientY: 100 })
    overlay._dispatch('mouseup', { clientX: 250, clientY: 200 })
    const inputEl = createdElements.find((el) => el.tagName === 'INPUT')
    inputEl.value = 'submit-on-second-enter'
    inputEl._listeners['keydown'][0]({
      key: 'Enter',
      preventDefault: mock.fn(),
      stopPropagation: mock.fn()
    })

    const keydownHandler = globalThis.document.addEventListener.mock.calls.find(
      (call) => call.arguments[0] === 'keydown'
    )?.arguments[1]
    keydownHandler({
      key: 'Enter',
      preventDefault: mock.fn(),
      stopPropagation: mock.fn()
    })
    await new Promise((resolve) => setTimeout(resolve, 350))

    assert.strictEqual(dm.isDrawModeActive(), false)
    const completed = sentMessages.find((message) => message.type === 'draw_mode_completed')
    assert.ok(completed, 'second Enter should submit the session')
    assert.strictEqual(completed.annotations[0].text, 'submit-on-second-enter')
  })

  test('action bar counts annotations and undo removes the latest mark', () => {
    dm.activateDrawMode('user')
    const overlay = documentBody.children[0]
    const submitButton = createdElements.find((el) => el.id === 'kaboom-draw-submit')
    const undoButton = createdElements.find((el) => el.id === 'kaboom-draw-undo')
    const cancelButton = createdElements.find((el) => el.id === 'kaboom-draw-cancel')

    assert.ok(submitButton, 'explicit submit action should exist')
    assert.ok(undoButton, 'undo action should exist')
    assert.ok(cancelButton, 'explicit cancel action should exist')
    assert.strictEqual(submitButton.textContent, 'Submit 0 annotations')
    assert.strictEqual(submitButton.disabled, true)
    assert.strictEqual(undoButton.disabled, true)

    for (const [text, offset] of [['first change', 0], ['second change', 180]]) {
      overlay._dispatch('mousedown', { button: 0, clientX: 100 + offset, clientY: 100 })
      overlay._dispatch('mouseup', { clientX: 250 + offset, clientY: 200 })
      const inputEl = createdElements.filter((el) => el.tagName === 'INPUT').at(-1)
      inputEl.value = text
      inputEl._listeners.keydown[0]({
        key: 'Enter',
        preventDefault: mock.fn(),
        stopPropagation: mock.fn()
      })
    }

    assert.strictEqual(submitButton.textContent, 'Submit 2 annotations')
    assert.strictEqual(submitButton.disabled, false)
    undoButton._dispatch('click', { preventDefault: mock.fn(), stopPropagation: mock.fn() })
    assert.deepStrictEqual(dm.getAnnotations().map((annotation) => annotation.text), ['first change'])
    assert.strictEqual(submitButton.textContent, 'Submit 1 annotation')

    cancelButton._dispatch('click', { preventDefault: mock.fn(), stopPropagation: mock.fn() })
    assert.strictEqual(dm.isDrawModeActive(), false, 'Cancel action exits without submission')
  })

  test('explicit Submit action delivers completed annotations', async () => {
    dm.activateDrawMode('user')
    const overlay = documentBody.children[0]
    const sentMessages = []
    globalThis.chrome.runtime.sendMessage = mock.fn((message, callback) => {
      sentMessages.push(message)
      if (message.type === 'kaboom_capture_screenshot') callback?.({ dataUrl: 'data:image/png;base64,mock' })
    })

    overlay._dispatch('mousedown', { button: 0, clientX: 100, clientY: 100 })
    overlay._dispatch('mouseup', { clientX: 250, clientY: 200 })
    const inputEl = createdElements.find((el) => el.tagName === 'INPUT')
    inputEl.value = 'submit from action bar'
    inputEl._listeners.keydown[0]({
      key: 'Enter',
      preventDefault: mock.fn(),
      stopPropagation: mock.fn()
    })

    const submitButton = createdElements.find((el) => el.id === 'kaboom-draw-submit')
    submitButton._dispatch('click', { preventDefault: mock.fn(), stopPropagation: mock.fn() })
    await new Promise((resolve) => setTimeout(resolve, 350))

    const completed = sentMessages.find((message) => message.type === 'draw_mode_completed')
    assert.strictEqual(completed.annotations[0].text, 'submit from action bar')
  })

  test('Escape from the annotation editor cancels the whole session', () => {
    dm.activateDrawMode('user')
    const overlay = documentBody.children[0]
    const sentMessages = []
    globalThis.chrome.runtime.sendMessage = mock.fn((message) => {
      sentMessages.push(message)
    })

    overlay._dispatch('mousedown', { button: 0, clientX: 100, clientY: 100 })
    overlay._dispatch('mouseup', { clientX: 250, clientY: 200 })
    const inputEl = createdElements.find((el) => el.tagName === 'INPUT')
    inputEl.value = 'must not be submitted'
    inputEl._listeners['keydown'][0]({
      key: 'Escape',
      preventDefault: mock.fn(),
      stopPropagation: mock.fn()
    })

    assert.strictEqual(dm.isDrawModeActive(), false)
    assert.deepStrictEqual(dm.getAnnotations(), [])
    assert.ok(!sentMessages.some((message) => message.type === 'draw_mode_completed'))
  })

  test('second shortcut submit path with empty text keeps editor open and warns user', () => {
    dm.activateDrawMode('user')
    const overlay = documentBody.children[0]

    const sentMessages = []
    globalThis.chrome.runtime.sendMessage = mock.fn((msg, callback) => {
      sentMessages.push(msg)
      if (msg.type === 'kaboom_capture_screenshot' && typeof callback === 'function') {
        callback({ dataUrl: 'data:image/png;base64,mockscreenshot' })
      }
      return undefined
    })

    overlay._dispatch('mousedown', { button: 0, clientX: 100, clientY: 100 })
    overlay._dispatch('mouseup', { clientX: 250, clientY: 200 })

    const inputEl = createdElements.find((el) => el.tagName === 'INPUT')
    inputEl.value = ''

    dm.deactivateAndSendResults()

    assert.strictEqual(dm.isDrawModeActive(), true, 'draw mode should remain active on invalid submit')
    assert.ok(inputEl.parentElement, 'text input should remain mounted for correction')

    const errorToast = sentMessages.find((m) => m.type === 'kaboom_action_toast' && m.state === 'error')
    assert.ok(errorToast, 'expected validation error toast')
    assert.ok(!sentMessages.some((m) => m.type === 'draw_mode_completed'), 'should not send completion on invalid submit')
  })

  test('empty text on Enter discards annotation', () => {
    dm.activateDrawMode('user')
    const overlay = documentBody.children[0]

    overlay._dispatch('mousedown', { button: 0, clientX: 100, clientY: 100 })
    overlay._dispatch('mousemove', { clientX: 250, clientY: 200 })
    overlay._dispatch('mouseup', { clientX: 250, clientY: 200 })

    const inputEl = createdElements.find((el) => el.tagName === 'INPUT')
    inputEl.value = ''
    const enterHandler = inputEl._listeners['keydown']?.[0]
    if (enterHandler) {
      enterHandler({ key: 'Enter', preventDefault: mock.fn(), stopPropagation: mock.fn() })
    }

    assert.deepStrictEqual(dm.getAnnotations(), [], 'empty text should discard annotation')
  })

  test('blur with text auto-confirms annotation', () => {
    dm.activateDrawMode('user')
    const overlay = documentBody.children[0]

    overlay._dispatch('mousedown', { button: 0, clientX: 100, clientY: 100 })
    overlay._dispatch('mousemove', { clientX: 250, clientY: 200 })
    overlay._dispatch('mouseup', { clientX: 250, clientY: 200 })

    const inputEl = createdElements.find((el) => el.tagName === 'INPUT')
    inputEl.value = 'increase padding'

    const blurHandler = inputEl._listeners['blur']?.[0]
    if (blurHandler) {
      blurHandler({})
    }

    const annotations = dm.getAnnotations()
    assert.strictEqual(annotations.length, 1, 'blur with text should auto-confirm')
    assert.strictEqual(annotations[0].text, 'increase padding')
  })

  test('right-click does not start drawing', () => {
    dm.activateDrawMode('user')
    const overlay = documentBody.children[0]

    overlay._dispatch('mousedown', { button: 2, clientX: 100, clientY: 100 })
    overlay._dispatch('mousemove', { clientX: 250, clientY: 200 })
    overlay._dispatch('mouseup', { clientX: 250, clientY: 200 })

    const inputEls = createdElements.filter((el) => el.tagName === 'INPUT')
    assert.strictEqual(inputEls.length, 0, 'right-click should not create text input')
  })

  test('reverse-direction drawing normalizes rect coordinates', () => {
    dm.activateDrawMode('user')
    const overlay = documentBody.children[0]

    // Draw from bottom-right to top-left
    overlay._dispatch('mousedown', { button: 0, clientX: 300, clientY: 300 })
    overlay._dispatch('mousemove', { clientX: 100, clientY: 100 })
    overlay._dispatch('mouseup', { clientX: 100, clientY: 100 })

    const inputEl = createdElements.find((el) => el.tagName === 'INPUT')
    assert.ok(inputEl, 'expected text input for reverse-drawn rectangle')

    inputEl.value = 'test reverse'
    const enterHandler = inputEl._listeners['keydown']?.[0]
    if (enterHandler) {
      enterHandler({ key: 'Enter', preventDefault: mock.fn(), stopPropagation: mock.fn() })
    }

    const annotations = dm.getAnnotations()
    assert.strictEqual(annotations.length, 1)
    assert.strictEqual(annotations[0].rect.x, 100, 'rect.x should be min')
    assert.strictEqual(annotations[0].rect.y, 100, 'rect.y should be min')
    assert.strictEqual(annotations[0].rect.width, 200)
    assert.strictEqual(annotations[0].rect.height, 200)
  })

  test('multiple annotations accumulate in order', () => {
    dm.activateDrawMode('user')
    const overlay = documentBody.children[0]

    // Draw first annotation
    overlay._dispatch('mousedown', { button: 0, clientX: 10, clientY: 10 })
    overlay._dispatch('mouseup', { clientX: 110, clientY: 60 })
    let inputEl = createdElements.filter((el) => el.tagName === 'INPUT').pop()
    inputEl.value = 'first'
    inputEl._listeners['keydown']?.[0]?.({ key: 'Enter', preventDefault: mock.fn(), stopPropagation: mock.fn() })

    // Draw second annotation
    overlay._dispatch('mousedown', { button: 0, clientX: 200, clientY: 200 })
    overlay._dispatch('mouseup', { clientX: 350, clientY: 300 })
    inputEl = createdElements.filter((el) => el.tagName === 'INPUT').pop()
    inputEl.value = 'second'
    inputEl._listeners['keydown']?.[0]?.({ key: 'Enter', preventDefault: mock.fn(), stopPropagation: mock.fn() })

    const annotations = dm.getAnnotations()
    assert.strictEqual(annotations.length, 2)
    assert.strictEqual(annotations[0].text, 'first')
    assert.strictEqual(annotations[1].text, 'second')
  })

  test('DOM element capture populates element_summary', () => {
    dm.activateDrawMode('user')
    const overlay = documentBody.children[0]

    overlay._dispatch('mousedown', { button: 0, clientX: 100, clientY: 100 })
    overlay._dispatch('mouseup', { clientX: 250, clientY: 200 })

    const inputEl = createdElements.find((el) => el.tagName === 'INPUT')
    inputEl.value = 'test capture'
    inputEl._listeners['keydown']?.[0]?.({ key: 'Enter', preventDefault: mock.fn(), stopPropagation: mock.fn() })

    const annotations = dm.getAnnotations()
    assert.strictEqual(annotations.length, 1)
    // elementsFromPoint mock returns a button.btn-primary 'Submit'
    assert.ok(annotations[0].element_summary.includes('button'), 'element_summary should contain tag')
  })

  test('DOM element capture stores retrievable detail via correlation_id', () => {
    dm.activateDrawMode('user')
    const overlay = documentBody.children[0]

    overlay._dispatch('mousedown', { button: 0, clientX: 100, clientY: 100 })
    overlay._dispatch('mouseup', { clientX: 250, clientY: 200 })

    const inputEl = createdElements.find((el) => el.tagName === 'INPUT')
    inputEl.value = 'test detail'
    inputEl._listeners['keydown']?.[0]?.({ key: 'Enter', preventDefault: mock.fn(), stopPropagation: mock.fn() })

    const annotations = dm.getAnnotations()
    const correlationId = annotations[0].correlation_id
    assert.ok(correlationId, 'should have correlation_id')

    const detail = dm.getElementDetail(correlationId)
    assert.ok(detail, 'should retrieve detail by correlation_id')
    assert.ok(detail.selector, 'detail should have selector')
    assert.ok(detail.tag, 'detail should have tag')
    assert.ok(Array.isArray(detail.action_trail), 'detail should include action_trail')
    assert.ok(detail.ui_context, 'detail should include ui_context')
  })

  test('new annotations include bounded recent action trail', () => {
    dm.activateDrawMode('user')
    const overlay = documentBody.children[0]

    // Trigger a prior action so trail is non-empty.
    const scrollCall = globalThis.window.addEventListener.mock.calls.find((c) => c.arguments?.[0] === 'scroll')
    assert.ok(scrollCall, 'expected scroll listener registration')
    globalThis.window.scrollY = 55
    scrollCall.arguments[1]()

    overlay._dispatch('mousedown', { button: 0, clientX: 100, clientY: 100 })
    overlay._dispatch('mouseup', { clientX: 250, clientY: 200 })

    const inputEl = createdElements.find((el) => el.tagName === 'INPUT')
    inputEl.value = 'trail metadata'
    inputEl._listeners['keydown']?.[0]?.({ key: 'Enter', preventDefault: mock.fn(), stopPropagation: mock.fn() })

    const annotation = dm.getAnnotations()[0]
    assert.ok(Array.isArray(annotation.action_trail), 'expected action_trail array')
    assert.ok(annotation.action_trail.length > 0, 'expected non-empty action_trail')

    const first = annotation.action_trail[0]
    assert.ok(first.type, 'trail entry should include type')
    assert.ok(first.target_summary, 'trail entry should include target_summary')
    assert.strictEqual(typeof first.timestamp, 'number')
  })

  test('new annotations include ui context metadata and focused-element summary', () => {
    dm.activateDrawMode('user')
    const overlay = documentBody.children[0]

    const focused = createMockElement('button')
    focused.id = 'save-btn'
    focused.textContent = 'Save'
    focused.parentElement = createMockElement('div')
    globalThis.document.activeElement = focused

    overlay._dispatch('mousedown', { button: 0, clientX: 100, clientY: 100 })
    overlay._dispatch('mouseup', { clientX: 250, clientY: 200 })

    const inputEl = createdElements.find((el) => el.tagName === 'INPUT')
    inputEl.value = 'context metadata'
    inputEl._listeners['keydown']?.[0]?.({ key: 'Enter', preventDefault: mock.fn(), stopPropagation: mock.fn() })

    const annotation = dm.getAnnotations()[0]
    assert.ok(annotation.ui_context, 'expected ui_context on annotation')
    assert.ok(['light', 'dark'].includes(annotation.ui_context.theme), 'expected normalized theme')
    assert.strictEqual(annotation.ui_context.viewport.width, globalThis.window.innerWidth)
    assert.strictEqual(annotation.ui_context.viewport.height, globalThis.window.innerHeight)
    assert.strictEqual(typeof annotation.ui_context.sidebars.left_open, 'boolean')
    assert.strictEqual(typeof annotation.ui_context.sidebars.right_open, 'boolean')
    assert.ok(annotation.ui_context.focused_element, 'expected focused_element summary')
  })
})

// =============================================================================
// Gap 5: Content Script Message Routing
// =============================================================================
