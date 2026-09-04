// @ts-nocheck
/**
 * @fileoverview tab-group-permission-toggle.test.js — the popup control that grants `tabGroups`.
 *
 * Why this exists: `tabGroups` carries the Chrome warning "View and manage your tab
 * groups", so it cannot be a required manifest permission without disabling the
 * extension on update for every existing user. It must be optional and granted at
 * runtime — and `chrome.permissions.request()` only works "from inside a user
 * gesture, like a button's click handler", which an MV3 service worker never has.
 * This popup toggle is therefore the ONLY surface on which the driven tab group can
 * ever be enabled. These tests pin the two things that silently break it: losing the
 * user gesture by awaiting before the request, and reporting a grant that never
 * happened.
 */

import { beforeEach, describe, mock, test } from 'node:test'
import assert from 'node:assert'

import {
  DRIVEN_GROUP_TOGGLE_ID,
  DRIVEN_GROUP_ROW_ID,
  applyDrivenTabGroupToggle
} from '../../../extension/popup/driven-tab-group-permission.js'

function createElement(id) {
  const listeners = {}
  return {
    id,
    checked: false,
    disabled: false,
    style: {},
    listeners,
    addEventListener(type, fn) {
      listeners[type] = fn
    },
    fire(type) {
      listeners[type]?.()
    }
  }
}

function createWorld({ granted = false, onRequest = 'grant', withPermissionsApi = true } = {}) {
  const elements = {
    [DRIVEN_GROUP_TOGGLE_ID]: createElement(DRIVEN_GROUP_TOGGLE_ID),
    [DRIVEN_GROUP_ROW_ID]: createElement(DRIVEN_GROUP_ROW_ID)
  }
  let held = granted

  const permissions = {
    contains: mock.fn(async () => held),
    request: mock.fn(async () => {
      if (onRequest === 'throw') throw new Error('This function must be called during a user gesture')
      if (onRequest === 'deny') return false
      held = true
      return true
    }),
    remove: mock.fn(async () => {
      held = false
      return true
    })
  }

  globalThis.document = { getElementById: (id) => elements[id] ?? null }
  globalThis.chrome = withPermissionsApi ? { permissions } : {}

  return {
    permissions,
    toggle: elements[DRIVEN_GROUP_TOGGLE_ID],
    row: elements[DRIVEN_GROUP_ROW_ID],
    isHeld: () => held
  }
}

// A turn of the microtask queue, so an async initial read can settle.
const settle = () => new Promise((resolve) => setTimeout(resolve, 0))

describe('driven tab group permission toggle — live permission is the authority', () => {
  beforeEach(() => {
    delete globalThis.document
    delete globalThis.chrome
  })

  test('an already-granted permission shows the toggle on, read from Chrome not storage', async () => {
    const world = createWorld({ granted: true })
    applyDrivenTabGroupToggle()
    await settle()

    assert.strictEqual(world.toggle.checked, true)
    assert.strictEqual(world.permissions.contains.mock.calls.length, 1, 'state comes from the live grant')
    assert.deepStrictEqual(world.permissions.contains.mock.calls[0].arguments[0], { permissions: ['tabGroups'] })
  })

  test('an ungranted permission shows the toggle off', async () => {
    const world = createWorld({ granted: false })
    applyDrivenTabGroupToggle()
    await settle()

    assert.strictEqual(world.toggle.checked, false)
  })
})

describe('driven tab group permission toggle — the user gesture must survive', () => {
  beforeEach(() => {
    delete globalThis.document
    delete globalThis.chrome
  })

  test('checking the box requests tabGroups synchronously, before any await', async () => {
    const world = createWorld({ granted: false })
    applyDrivenTabGroupToggle()
    await settle()

    world.toggle.checked = true
    world.toggle.fire('change')

    // Asserted BEFORE awaiting anything. Chrome discards the user gesture across an
    // await, so a handler that reads permission state first would request too late
    // and every grant attempt would throw. The call must already have happened.
    assert.strictEqual(
      world.permissions.request.mock.calls.length,
      1,
      'permissions.request must be called in the same tick as the change event'
    )
    assert.deepStrictEqual(world.permissions.request.mock.calls[0].arguments[0], { permissions: ['tabGroups'] })

    await settle()
    assert.strictEqual(world.isHeld(), true)
    assert.strictEqual(world.toggle.checked, true)
  })
})

describe('driven tab group permission toggle — the control never lies about the grant', () => {
  beforeEach(() => {
    delete globalThis.document
    delete globalThis.chrome
  })

  test('a denied request leaves the toggle off', async () => {
    const world = createWorld({ granted: false, onRequest: 'deny' })
    applyDrivenTabGroupToggle()
    await settle()

    world.toggle.checked = true
    world.toggle.fire('change')
    await settle()

    assert.strictEqual(world.toggle.checked, false, 'a refused grant must not read as enabled')
    assert.strictEqual(world.isHeld(), false)
  })

  test('a request Chrome rejects leaves the toggle off instead of throwing', async () => {
    const world = createWorld({ granted: false, onRequest: 'throw' })
    applyDrivenTabGroupToggle()
    await settle()

    world.toggle.checked = true
    world.toggle.fire('change')
    await settle()

    assert.strictEqual(world.toggle.checked, false)
  })

  test('unchecking revokes the permission', async () => {
    const world = createWorld({ granted: true })
    applyDrivenTabGroupToggle()
    await settle()

    world.toggle.checked = false
    world.toggle.fire('change')
    await settle()

    assert.strictEqual(world.permissions.remove.mock.calls.length, 1)
    assert.deepStrictEqual(world.permissions.remove.mock.calls[0].arguments[0], { permissions: ['tabGroups'] })
    assert.strictEqual(world.isHeld(), false)
    assert.strictEqual(world.toggle.checked, false)
  })
})

describe('driven tab group permission toggle — unsupported browsers', () => {
  beforeEach(() => {
    delete globalThis.document
    delete globalThis.chrome
  })

  test('a browser without chrome.permissions hides the row rather than showing a dead control', async () => {
    const world = createWorld({ withPermissionsApi: false })
    applyDrivenTabGroupToggle()
    await settle()

    assert.strictEqual(world.row.style.display, 'none', 'no dead toggle is left on screen')
  })

  test('a missing toggle element is not an error', () => {
    globalThis.document = { getElementById: () => null }
    globalThis.chrome = {}
    assert.doesNotThrow(() => applyDrivenTabGroupToggle())
  })
})
