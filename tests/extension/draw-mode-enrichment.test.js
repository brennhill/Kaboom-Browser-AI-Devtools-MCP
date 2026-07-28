// @ts-nocheck
/**
 * @fileoverview Draw-mode element, framework, and selector enrichment tests.
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

describe('Draw Mode — Element Detail Enrichment', () => {
  let dm

  beforeEach(async () => {
    setupGlobals()

    // Enhance mock to provide full DOM tree context for enrichment tests
    const grandparent = createMockElement('form')
    grandparent.classList._items = ['checkout-form']
    grandparent.id = 'checkout'

    const parent = createMockElement('div')
    parent.classList._items = ['actions', 'mt-4']
    parent.id = ''
    parent.parentElement = grandparent
    parent.getAttribute = (attr) => {
      if (attr === 'role') return 'group'
      return null
    }
    grandparent.getAttribute = (_attr) => null

    // Create siblings
    const prevSibling1 = createMockElement('button')
    prevSibling1.classList._items = ['btn-secondary']
    prevSibling1.textContent = 'Cancel'
    const prevSibling2 = createMockElement('span')
    prevSibling2.classList._items = ['separator']
    prevSibling2.textContent = '|'

    const target = createMockElement('button')
    target.classList._items = ['btn-primary']
    target.textContent = 'Submit'
    target.id = 'submit-btn'
    target.parentElement = parent
    target.getAttribute = (_attr) => null

    const nextSibling1 = createMockElement('a')
    nextSibling1.classList._items = ['help-link']
    nextSibling1.textContent = 'Help'
    const nextSibling2 = createMockElement('span')
    nextSibling2.classList._items = ['spacer']
    nextSibling2.textContent = ''

    // Set up parent.children with siblings
    parent.children = [prevSibling1, prevSibling2, target, nextSibling1, nextSibling2]
    for (const child of parent.children) {
      child.parentElement = parent
    }

    globalThis.document.elementsFromPoint = mock.fn((_x, _y) => [target])

    dm = await importDrawMode()
  })

  function drawAndGetDetail() {
    const beforeCount = dm.getAnnotations().length
    dm.activateDrawMode('user')
    const overlay = documentBody.children[0]

    overlay._dispatch('mousedown', { button: 0, clientX: 100, clientY: 100 })
    overlay._dispatch('mouseup', { clientX: 250, clientY: 200 })

    const inputEl = createdElements.find((el) => el.tagName === 'INPUT')
    inputEl.value = 'enrichment test'
    inputEl._listeners['keydown']?.[0]?.({ key: 'Enter', preventDefault: mock.fn(), stopPropagation: mock.fn() })

    const annotations = dm.getAnnotations()
    assert.ok(annotations.length > beforeCount)
    return dm.getElementDetail(annotations[annotations.length - 1].correlation_id)
  }

  test('element detail includes parent_context with parent and grandparent', () => {
    const detail = drawAndGetDetail()
    assert.ok(detail, 'should have detail')
    assert.ok(detail.parent_context, 'detail should have parent_context')

    const pc = detail.parent_context
    assert.ok(pc.parent, 'parent_context should have parent')
    assert.strictEqual(pc.parent.tag, 'div')
    assert.ok(Array.isArray(pc.parent.classes), 'parent classes should be array')
    assert.ok(pc.parent.classes.includes('actions'), 'parent should have actions class')

    assert.ok(pc.grandparent, 'parent_context should have grandparent')
    assert.strictEqual(pc.grandparent.tag, 'form')
    assert.strictEqual(pc.grandparent.id, 'checkout')
  })

  test('element detail includes siblings (up to 2 before and 2 after)', () => {
    const detail = drawAndGetDetail()
    assert.ok(detail, 'should have detail')
    assert.ok(Array.isArray(detail.siblings), 'detail should have siblings array')
    assert.ok(detail.siblings.length > 0, 'siblings should not be empty')
    assert.ok(detail.siblings.length <= 4, 'at most 4 siblings (2 before + 2 after)')

    // Check sibling shape
    for (const sib of detail.siblings) {
      assert.ok(sib.tag, 'sibling should have tag')
      assert.ok(Array.isArray(sib.classes), 'sibling should have classes array')
      assert.ok(['before', 'after'].includes(sib.position), 'sibling should have position before or after')
    }
  })

  test('css_framework absent when no framework detected', () => {
    const detail = drawAndGetDetail()
    assert.ok(detail, 'should have detail')
    // Mock element has btn-primary which doesn't match any framework threshold
    assert.strictEqual(detail.css_framework, undefined, 'css_framework should be absent when no framework detected')
    assert.strictEqual(detail.js_framework, undefined, 'js_framework should be absent when runtime framework not detected')
  })

  test('css_framework detects Tailwind classes', () => {
    // Override elementsFromPoint to return element with Tailwind classes
    const twEl = createMockElement('div')
    twEl.classList._items = ['flex', 'p-4', 'text-sm', 'bg-blue-500', 'rounded-lg', 'mt-2']
    twEl.textContent = 'Tailwind element'
    twEl.parentElement = createMockElement('div')
    twEl.parentElement.parentElement = null
    twEl.getAttribute = (_attr) => null
    globalThis.document.elementsFromPoint = mock.fn(() => [twEl])

    const detail = drawAndGetDetail()
    assert.ok(detail, 'should have detail')
    assert.strictEqual(detail.css_framework, 'tailwind', `expected 'tailwind', got '${detail.css_framework}'`)
  })

  test('css_framework detects Bootstrap classes', () => {
    const bsEl = createMockElement('div')
    bsEl.classList._items = ['col-md-6', 'btn-primary', 'form-control', 'container']
    bsEl.textContent = 'Bootstrap element'
    bsEl.parentElement = createMockElement('div')
    bsEl.parentElement.parentElement = null
    bsEl.getAttribute = (_attr) => null
    globalThis.document.elementsFromPoint = mock.fn(() => [bsEl])

    const detail = drawAndGetDetail()
    assert.ok(detail, 'should have detail')
    assert.strictEqual(detail.css_framework, 'bootstrap', `expected 'bootstrap', got '${detail.css_framework}'`)
  })

  test('css_framework detects CSS Modules hash pattern', () => {
    const modEl = createMockElement('div')
    modEl.classList._items = ['Button_primary__a1b2c3', 'Card_wrapper__x9y8z7']
    modEl.textContent = 'CSS Modules element'
    modEl.parentElement = createMockElement('div')
    modEl.parentElement.parentElement = null
    modEl.getAttribute = (_attr) => null
    globalThis.document.elementsFromPoint = mock.fn(() => [modEl])

    const detail = drawAndGetDetail()
    assert.ok(detail, 'should have detail')
    assert.strictEqual(detail.css_framework, 'css-modules', `expected 'css-modules', got '${detail.css_framework}'`)
  })

  test('css_framework detects styled-components/emotion pattern', () => {
    const scEl = createMockElement('div')
    scEl.classList._items = ['css-1a2b3c', 'sc-dkrFOg', 'css-4d5e6f']
    scEl.textContent = 'Styled element'
    scEl.parentElement = createMockElement('div')
    scEl.parentElement.parentElement = null
    scEl.getAttribute = (_attr) => null
    globalThis.document.elementsFromPoint = mock.fn(() => [scEl])

    const detail = drawAndGetDetail()
    assert.ok(detail, 'should have detail')
    assert.strictEqual(detail.css_framework, 'styled-components', `expected 'styled-components', got '${detail.css_framework}'`)
  })

  test('parent_context is absent when element parent is document.body', () => {
    const el = createMockElement('button')
    el.classList._items = ['main-btn']
    el.textContent = 'Click'
    el.parentElement = globalThis.document.body
    el.getAttribute = (_attr) => null
    globalThis.document.elementsFromPoint = mock.fn(() => [el])

    const detail = drawAndGetDetail()
    assert.ok(detail, 'should have detail')
    assert.strictEqual(detail.parent_context, undefined, 'parent_context should be absent when parent is body')
  })

  test('siblings is absent when element is only child', () => {
    const parent = createMockElement('div')
    parent.classList._items = ['wrapper']
    parent.getAttribute = (_attr) => null

    const el = createMockElement('button')
    el.classList._items = ['solo']
    el.textContent = 'Solo'
    el.parentElement = parent
    el.getAttribute = (_attr) => null
    parent.children = [el]

    globalThis.document.elementsFromPoint = mock.fn(() => [el])

    const detail = drawAndGetDetail()
    assert.ok(detail, 'should have detail')
    assert.strictEqual(detail.siblings, undefined, 'siblings should be absent when element is only child')
  })

  test('css_framework priority: Tailwind wins over Bootstrap when both match', () => {
    const mixedEl = createMockElement('div')
    // Has both Tailwind-specific (p-4, mt-2, text-sm) and Bootstrap (btn-primary, form-control)
    mixedEl.classList._items = ['flex', 'p-4', 'mt-2', 'text-sm', 'btn-primary', 'form-control']
    mixedEl.textContent = 'Mixed'
    mixedEl.parentElement = createMockElement('div')
    mixedEl.parentElement.parentElement = null
    mixedEl.getAttribute = (_attr) => null
    globalThis.document.elementsFromPoint = mock.fn(() => [mixedEl])

    const detail = drawAndGetDetail()
    assert.ok(detail, 'should have detail')
    assert.strictEqual(detail.css_framework, 'tailwind', 'Tailwind should win over Bootstrap when both match')
  })

  test('parent_context has null grandparent when parent has no parent element', () => {
    const parent = createMockElement('section')
    parent.classList._items = ['top-level']
    parent.parentElement = null
    parent.getAttribute = (_attr) => null

    const el = createMockElement('button')
    el.classList._items = ['action']
    el.textContent = 'Go'
    el.parentElement = parent
    el.getAttribute = (_attr) => null
    parent.children = [el]

    globalThis.document.elementsFromPoint = mock.fn(() => [el])

    const detail = drawAndGetDetail()
    assert.ok(detail, 'should have detail')
    assert.ok(detail.parent_context, 'should have parent_context')
    assert.strictEqual(detail.parent_context.parent.tag, 'section')
    assert.strictEqual(detail.parent_context.grandparent, null, 'grandparent should be null when parent has no parent')
  })

  test('siblings capped at 2 before and 2 after even with many siblings', () => {
    const parent = createMockElement('ul')
    parent.classList._items = ['list']
    parent.parentElement = null
    parent.getAttribute = (_attr) => null

    // Create 5 before + target + 5 after = 11 children
    const children = []
    for (let i = 0; i < 5; i++) {
      const li = createMockElement('li')
      li.classList._items = [`item-before-${i}`]
      li.textContent = `Before ${i}`
      li.parentElement = parent
      children.push(li)
    }
    const target = createMockElement('li')
    target.classList._items = ['selected']
    target.textContent = 'Target'
    target.parentElement = parent
    target.getAttribute = (_attr) => null
    children.push(target)
    for (let i = 0; i < 5; i++) {
      const li = createMockElement('li')
      li.classList._items = [`item-after-${i}`]
      li.textContent = `After ${i}`
      li.parentElement = parent
      children.push(li)
    }
    parent.children = children

    globalThis.document.elementsFromPoint = mock.fn(() => [target])

    const detail = drawAndGetDetail()
    assert.ok(detail, 'should have detail')
    assert.ok(Array.isArray(detail.siblings), 'should have siblings')
    assert.strictEqual(detail.siblings.length, 4, 'should cap at 4 siblings (2 before + 2 after)')

    const beforeSibs = detail.siblings.filter(s => s.position === 'before')
    const afterSibs = detail.siblings.filter(s => s.position === 'after')
    assert.strictEqual(beforeSibs.length, 2, 'exactly 2 before siblings')
    assert.strictEqual(afterSibs.length, 2, 'exactly 2 after siblings')
  })

  test('generic classes alone do not trigger Tailwind detection', () => {
    const genericEl = createMockElement('div')
    genericEl.classList._items = ['flex', 'border', 'shadow', 'rounded']
    genericEl.textContent = 'Generic'
    genericEl.parentElement = createMockElement('div')
    genericEl.parentElement.parentElement = null
    genericEl.getAttribute = (_attr) => null
    globalThis.document.elementsFromPoint = mock.fn(() => [genericEl])

    const detail = drawAndGetDetail()
    assert.ok(detail, 'should have detail')
    assert.strictEqual(detail.css_framework, undefined, 'generic CSS classes should not trigger Tailwind detection')
  })

  test('selector_candidates include semantic locator fallbacks', () => {
    const candidateEl = createMockElement('button')
    candidateEl.id = 'submit-btn'
    candidateEl.classList._items = ['btn-primary']
    candidateEl.textContent = 'Submit order'
    candidateEl.parentElement = createMockElement('div')
    candidateEl.parentElement.parentElement = null
    candidateEl.getAttribute = (attr) => {
      if (attr === 'data-testid') return 'checkout-submit'
      if (attr === 'role') return 'button'
      if (attr === 'aria-label') return 'Submit order'
      return null
    }
    globalThis.document.elementsFromPoint = mock.fn(() => [candidateEl])

    const detail = drawAndGetDetail()
    assert.ok(detail, 'should have detail')
    assert.ok(Array.isArray(detail.selector_candidates), 'selector_candidates should be an array')
    assert.ok(detail.selector_candidates.includes('css=#submit-btn'), 'should include id-based css selector')
    assert.ok(detail.selector_candidates.includes('testid=checkout-submit'), 'should include test id candidate')
    assert.ok(detail.selector_candidates.includes('role=button|Submit order'), 'should include role+name candidate')
    assert.ok(detail.selector_candidates.includes('label=Submit order'), 'should include label candidate')

    const unique = new Set(detail.selector_candidates)
    assert.strictEqual(unique.size, detail.selector_candidates.length, 'selector_candidates should be deduplicated')
  })

  test('js_framework and component are captured from runtime metadata', () => {
    const reactEl = createMockElement('button')
    reactEl.classList._items = ['btn']
    reactEl.textContent = 'Save'
    reactEl.parentElement = createMockElement('div')
    reactEl.parentElement.parentElement = null
    reactEl.getAttribute = (_attr) => null
    reactEl.__reactFiber$test = {
      type: { name: 'SaveButton' },
      _debugSource: { fileName: '/src/components/SaveButton.tsx', lineNumber: 18 },
      return: null
    }
    globalThis.document.elementsFromPoint = mock.fn(() => [reactEl])

    const detail = drawAndGetDetail()
    assert.ok(detail, 'should have detail')
    assert.strictEqual(detail.js_framework, 'react', 'should include top-level js_framework')
    assert.ok(detail.component, 'should include component detail object')
    assert.strictEqual(detail.component.framework, 'react')
    assert.strictEqual(detail.component.component, 'SaveButton')
    assert.strictEqual(detail.component.source_file, '/src/components/SaveButton.tsx')
  })
})
