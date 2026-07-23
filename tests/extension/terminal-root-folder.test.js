// @ts-nocheck
/**
 * @fileoverview terminal-root-folder.test.js — The root-folder bar above the
 * terminal: showing the working directory, browsing for a new one, and
 * relaunching the shell there.
 *
 * Why a daemon round-trip to pick a folder: the browser cannot resolve an
 * absolute path. `<input webkitdirectory>` exposes only relative paths and
 * `showDirectoryPicker()` exposes only a folder name, so neither can produce a
 * cwd to spawn a PTY in. The daemon, which runs the shell, does the listing.
 */

import { beforeEach, describe, mock, test } from 'node:test'
import assert from 'node:assert'

let importCounter = 0
let roots = []
let fetchCalls = []
let fetchHandler = null

function walkTree(node, visit) {
  for (const child of node.children || []) {
    if (visit(child)) return child
    const found = walkTree(child, visit)
    if (found) return found
  }
  return null
}

function byId(id) {
  for (const root of roots) {
    if (root.id === id) return root
    const found = walkTree(root, (child) => child.id === id)
    if (found) return found
  }
  return null
}

function allRows(node) {
  const rows = []
  const visit = (parent) => {
    for (const child of parent.children || []) {
      rows.push(child)
      visit(child)
    }
  }
  visit(node)
  return rows
}

function createElement(tag) {
  const listeners = {}
  const el = {
    tagName: String(tag).toUpperCase(),
    id: '',
    type: '',
    value: '',
    title: '',
    placeholder: '',
    textContent: '',
    htmlFor: '',
    style: {},
    children: [],
    parentElement: null,
    appendChild: mock.fn((child) => {
      child.parentElement = el
      el.children.push(child)
      return child
    }),
    replaceChildren: mock.fn((...next) => {
      el.children = []
      for (const child of next) {
        child.parentElement = el
        el.children.push(child)
      }
    }),
    addEventListener: mock.fn((type, handler) => {
      listeners[type] = listeners[type] || []
      listeners[type].push(handler)
    }),
    dispatch: (type, event = {}) => {
      for (const handler of listeners[type] || []) {
        handler({ preventDefault() {}, stopPropagation() {}, ...event })
      }
    }
  }
  return el
}

function setupDom() {
  roots = []
  globalThis.document = {
    activeElement: null,
    createElement: (tag) => {
      const el = createElement(tag)
      roots.push(el)
      return el
    }
  }
  globalThis.chrome = {
    runtime: { id: 'test-ext' },
    storage: {
      local: {
        get: mock.fn((keys, cb) => cb({ kaboom_server_url: 'http://localhost:7890' })),
        set: mock.fn((_v, cb) => cb?.())
      },
      onChanged: { addListener: mock.fn() }
    }
  }
  fetchCalls = []
  globalThis.fetch = mock.fn(async (url) => {
    fetchCalls.push(String(url))
    if (!fetchHandler) throw new Error(`unexpected fetch: ${url}`)
    return fetchHandler(String(url))
  })
  globalThis.AbortSignal = { timeout: () => undefined }
}

function okJson(body) {
  return { ok: true, status: 200, json: async () => body }
}

async function loadBar() {
  return import(`../../extension/content/ui/terminal-root-folder.js?v=${++importCounter}`)
}

const HOME_LISTING = {
  path: '/Users/dev',
  parent: '/Users',
  entries: [
    { name: 'kaboom', path: '/Users/dev/kaboom' },
    { name: 'aether', path: '/Users/dev/aether' }
  ],
  truncated: false
}

describe('the root folder bar', () => {
  beforeEach(() => {
    mock.reset()
    setupDom()
    fetchHandler = null
  })

  test('shows the current root and offers to reload into it', async () => {
    const { createRootFolderBar } = await loadBar()

    createRootFolderBar({ initialRoot: '/Users/dev/kaboom', onApply: () => {} })

    assert.strictEqual(byId('kaboom-terminal-root-folder-input').value, '/Users/dev/kaboom',
      'the working directory is the most consequential thing about a shell; it must be visible')
    assert.ok(byId('kaboom-terminal-root-folder-browse'))
    assert.ok(byId('kaboom-terminal-root-folder-save'))
  })

  test('the apply control says reload, because the shell is replaced', async () => {
    // A PTY's cwd is fixed at spawn, so this is not a save — the running
    // session, and anything in it, is torn down.
    const { createRootFolderBar } = await loadBar()
    createRootFolderBar({ initialRoot: '', onApply: () => {} })

    assert.strictEqual(byId('kaboom-terminal-root-folder-save').textContent, 'Reload')
  })

  test('applying hands back the trimmed path', async () => {
    const applied = []
    const { createRootFolderBar } = await loadBar()
    createRootFolderBar({ initialRoot: '', onApply: (root) => applied.push(root) })

    const input = byId('kaboom-terminal-root-folder-input')
    input.value = '  /Users/dev/aether  '
    byId('kaboom-terminal-root-folder-save').dispatch('click')

    assert.deepStrictEqual(applied, ['/Users/dev/aether'])
  })

  test('Enter in the field applies, so the bar works without reaching for the mouse', async () => {
    const applied = []
    const { createRootFolderBar } = await loadBar()
    createRootFolderBar({ initialRoot: '/Users/dev/kaboom', onApply: (root) => applied.push(root) })

    byId('kaboom-terminal-root-folder-input').dispatch('keydown', { key: 'Enter' })

    assert.deepStrictEqual(applied, ['/Users/dev/kaboom'])
  })

  test('setRoot does not overwrite what the user is typing', async () => {
    const { createRootFolderBar } = await loadBar()
    const bar = createRootFolderBar({ initialRoot: '', onApply: () => {} })
    const input = byId('kaboom-terminal-root-folder-input')
    input.value = '/half/typed/pa'
    globalThis.document.activeElement = input

    bar.setRoot('/Users/dev/kaboom')

    assert.strictEqual(input.value, '/half/typed/pa')
  })
})

describe('browsing for a folder', () => {
  beforeEach(() => {
    mock.reset()
    setupDom()
    fetchHandler = () => okJson(HOME_LISTING)
  })

  test('Browse lists the sub-folders of the current root', async () => {
    const { createRootFolderBar } = await loadBar()
    createRootFolderBar({ initialRoot: '/Users/dev', onApply: () => {} })

    byId('kaboom-terminal-root-folder-browse').dispatch('click')
    await new Promise((r) => setTimeout(r, 0))

    assert.ok(fetchCalls.some((url) => url.includes('/terminal/dirs?path=%2FUsers%2Fdev')),
      'the picker asks the daemon, because the browser cannot resolve absolute paths')
    const labels = allRows(byId('kaboom-terminal-root-folder-picker')).map((row) => row.textContent)
    assert.ok(labels.some((text) => text.includes('kaboom')), `expected folders in ${JSON.stringify(labels)}`)
  })

  test('choosing a folder fills the field without reloading yet', async () => {
    const applied = []
    const { createRootFolderBar } = await loadBar()
    createRootFolderBar({ initialRoot: '/Users/dev', onApply: (root) => applied.push(root) })
    byId('kaboom-terminal-root-folder-browse').dispatch('click')
    await new Promise((r) => setTimeout(r, 0))

    byId('kaboom-terminal-root-folder-use').dispatch('click')

    assert.strictEqual(byId('kaboom-terminal-root-folder-input').value, '/Users/dev')
    assert.deepStrictEqual(applied, [],
      'picking is not committing — restarting the shell is a separate, deliberate step')
  })

  test('descending into a folder lists that folder', async () => {
    const { createRootFolderBar } = await loadBar()
    createRootFolderBar({ initialRoot: '/Users/dev', onApply: () => {} })
    byId('kaboom-terminal-root-folder-browse').dispatch('click')
    await new Promise((r) => setTimeout(r, 0))

    fetchHandler = () => okJson({ path: '/Users/dev/kaboom', parent: '/Users/dev', entries: [], truncated: false })
    const picker = byId('kaboom-terminal-root-folder-picker')
    const kaboomRow = allRows(picker).find((row) => String(row.textContent).includes('kaboom'))
    kaboomRow.dispatch('click')
    await new Promise((r) => setTimeout(r, 0))

    assert.ok(fetchCalls.some((url) => url.includes('%2FUsers%2Fdev%2Fkaboom')))
    assert.ok(allRows(byId('kaboom-terminal-root-folder-picker'))
      .some((row) => String(row.textContent).includes('Use /Users/dev/kaboom')))
  })

  test('an unreachable daemon degrades to typing instead of a dead list', async () => {
    fetchHandler = () => { throw new Error('connection refused') }
    const { createRootFolderBar } = await loadBar()
    createRootFolderBar({ initialRoot: '/Users/dev', onApply: () => {} })

    byId('kaboom-terminal-root-folder-browse').dispatch('click')
    await new Promise((r) => setTimeout(r, 0))

    const text = allRows(byId('kaboom-terminal-root-folder-picker')).map((r) => r.textContent).join(' ')
    assert.match(text, /type a path/, 'the field still works, so say so rather than showing an empty list')
  })

  test('a truncated listing says so', async () => {
    // Showing part of a directory silently reads as showing all of it.
    fetchHandler = () => okJson({ ...HOME_LISTING, truncated: true })
    const { createRootFolderBar } = await loadBar()
    createRootFolderBar({ initialRoot: '/Users/dev', onApply: () => {} })

    byId('kaboom-terminal-root-folder-browse').dispatch('click')
    await new Promise((r) => setTimeout(r, 0))

    const text = allRows(byId('kaboom-terminal-root-folder-picker')).map((r) => r.textContent).join(' ')
    assert.match(text, /not shown/)
  })

  test('the filesystem root offers no way up', async () => {
    fetchHandler = () => okJson({ path: '/', parent: '', entries: [], truncated: false })
    const { createRootFolderBar } = await loadBar()
    createRootFolderBar({ initialRoot: '/', onApply: () => {} })

    byId('kaboom-terminal-root-folder-browse').dispatch('click')
    await new Promise((r) => setTimeout(r, 0))

    assert.strictEqual(byId('kaboom-terminal-root-folder-up'), null,
      'an up control at the root goes nowhere')
  })
})
