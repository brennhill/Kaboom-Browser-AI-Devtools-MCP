// @ts-nocheck
/**
 * @fileoverview terminal-start-pending.test.js — the panel must show a live
 * "starting" state while /terminal/start is in flight.
 *
 * Why: the daemon now retries a transient fork/exec EPERM (3 attempts, ~225ms),
 * so a spawn that used to fail instantly can legitimately take a few hundred
 * milliseconds. With no pending state the panel sat visually identical to a dead
 * one for that whole window, and a user watching a blank body could not tell
 * "working on it" from "broken" — the exact ambiguity that made the old instant
 * sandbox error feel more honest than it was.
 */

import { beforeEach, describe, mock, test } from 'node:test'
import assert from 'node:assert'

let importCounter = 0
let headChildren

function createMockElement(tag) {
  const el = {
    tag,
    id: '',
    textContent: '',
    className: '',
    style: {},
    children: [],
    appendChild: mock.fn((child) => {
      el.children.push(child)
      return child
    }),
    replaceChildren: mock.fn((...kids) => {
      el.children = kids
    }),
    setAttribute: mock.fn((k, v) => {
      el[k] = v
    })
  }
  return el
}

function resetDom() {
  headChildren = []
  const byId = {}
  globalThis.document = {
    createElement: mock.fn((tag) => createMockElement(tag)),
    getElementById: mock.fn((id) => byId[id] ?? null),
    head: {
      appendChild: mock.fn((el) => {
        headChildren.push(el)
        if (el.id) byId[el.id] = el
        return el
      })
    }
  }
}

async function loadStates() {
  return import(`../../../extension/content/ui/terminal-panel-states.js?v=${++importCounter}`)
}

/** Depth-first collect of every node in the rendered tree. */
function flatten(el, out = []) {
  out.push(el)
  for (const child of el.children ?? []) flatten(child, out)
  return out
}

describe('renderStartPending', () => {
  let renderStartPending
  let container

  beforeEach(async () => {
    mock.reset()
    resetDom()
    container = createMockElement('div')
    ;({ renderStartPending } = await loadStates())
  })

  test('replaces whatever was in the panel (a stale error must not linger)', () => {
    container.children = [createMockElement('div')]
    renderStartPending(container)

    assert.strictEqual(container.replaceChildren.mock.calls.length, 1)
    // Exactly one root was appended after clearing.
    assert.strictEqual(container.children.length, 1)
  })

  test('shows a default label so the panel never renders an unexplained spinner', () => {
    renderStartPending(container)

    const text = flatten(container)
      .map((el) => el.textContent)
      .filter(Boolean)
      .join(' ')
    assert.match(text, /starting/i)
  })

  test('accepts a caller-supplied label', () => {
    renderStartPending(container, 'Retrying terminal start…')

    const text = flatten(container)
      .map((el) => el.textContent)
      .filter(Boolean)
      .join(' ')
    assert.match(text, /Retrying terminal start/)
  })

  test('includes an animated spinner element', () => {
    renderStartPending(container)

    const spinner = flatten(container).find((el) => el.style && el.style.animation)
    assert.ok(spinner, 'expected an element carrying an animation style')
    assert.match(spinner.style.animation, /kaboom-terminal-spin/)
  })

  test('injects its keyframes exactly once, however many times it renders', () => {
    renderStartPending(container)
    renderStartPending(container)
    renderStartPending(container)

    const styles = headChildren.filter((el) => el.tag === 'style')
    assert.strictEqual(styles.length, 1, 'repeated renders must not leak <style> tags into head')
  })

  test('the injected keyframes honour prefers-reduced-motion', () => {
    renderStartPending(container)

    const style = headChildren.find((el) => el.tag === 'style')
    assert.ok(style, 'expected a style element')
    assert.match(style.textContent, /prefers-reduced-motion/)
  })
})
