// @ts-nocheck
/**
 * @fileoverview tab-groups-fixture.js — In-memory fake of the Chrome tab, tab-group
 * and optional-permission surface the driven tab group depends on.
 *
 * Models the behaviour the real module has to survive: a group vanishes when its
 * last tab leaves, adding a tab to a dissolved group throws, and
 * `chrome.permissions.request` may resolve false (user refused) or throw (called
 * outside a user gesture). No timers — every operation settles on its own promise.
 */

import { mock } from 'node:test'
import { createMockChrome } from '../shared/helpers.js'

export const TAB_GROUP_ID_NONE = -1

/**
 * @param {Object} options
 * @param {boolean} [options.withPermissionsApi] - expose chrome.permissions at all
 */
export function createTabGroupsWorld(options = {}) {
  const { withPermissionsApi = true } = options

  const world = {
    requestCalls: 0,
    tabs: new Map(),
    groups: new Map(),
    nextTabId: 1,
    nextGroupId: 100
  }

  function addTab(props = {}) {
    const id = world.nextTabId++
    const tab = {
      id,
      windowId: 1,
      index: world.tabs.size,
      url: props.url || 'https://example.com/',
      title: props.title || 'Example',
      status: 'complete',
      groupId: TAB_GROUP_ID_NONE,
      ...props
    }
    tab.id = id
    world.tabs.set(id, tab)
    return tab
  }

  function dropEmptyGroups() {
    for (const groupId of [...world.groups.keys()]) {
      const populated = [...world.tabs.values()].some((tab) => tab.groupId === groupId)
      if (!populated) world.groups.delete(groupId)
    }
  }

  function requireTab(tabId) {
    const tab = world.tabs.get(tabId)
    if (!tab) throw new Error(`No tab with id: ${tabId}`)
    return tab
  }

  const chromeTabs = {
    get: mock.fn(async (tabId) => ({ ...requireTab(tabId) })),
    query: mock.fn(async (info = {}) => {
      let found = [...world.tabs.values()]
      if (typeof info.groupId === 'number') found = found.filter((tab) => tab.groupId === info.groupId)
      if (info.active === true) found = found.filter((tab) => tab.active === true)
      return found.map((tab) => ({ ...tab }))
    }),
    create: mock.fn(async (props) => ({ ...addTab({ ...props, active: props.active === true }) })),
    update: mock.fn(async (tabId, patch) => {
      const tab = requireTab(tabId)
      Object.assign(tab, patch)
      return { ...tab }
    }),
    remove: mock.fn(async (tabId) => {
      requireTab(tabId)
      world.tabs.delete(tabId)
      dropEmptyGroups()
    }),
    reload: mock.fn(async () => {}),
    sendMessage: mock.fn(async () => ({ status: 'alive' })),
    group: mock.fn(async ({ tabIds, groupId }) => {
      const ids = Array.isArray(tabIds) ? tabIds : [tabIds]
      let target = groupId
      if (typeof target === 'number') {
        if (!world.groups.has(target)) throw new Error(`No group with id: ${target}`)
      } else {
        target = world.nextGroupId++
        world.groups.set(target, { id: target, windowId: 1, collapsed: false, color: 'grey', title: undefined })
      }
      for (const id of ids) requireTab(id).groupId = target
      dropEmptyGroups()
      return target
    }),
    ungroup: mock.fn(async (tabIds) => {
      const ids = Array.isArray(tabIds) ? tabIds : [tabIds]
      for (const id of ids) requireTab(id).groupId = TAB_GROUP_ID_NONE
      dropEmptyGroups()
    }),
    onRemoved: { addListener: mock.fn() },
    onUpdated: { addListener: mock.fn() }
  }

  const chromeTabGroups = {
    TAB_GROUP_ID_NONE,
    get: mock.fn(async (groupId) => {
      const group = world.groups.get(groupId)
      if (!group) throw new Error(`No group with id: ${groupId}`)
      return { ...group }
    }),
    query: mock.fn(async (info = {}) => {
      let found = [...world.groups.values()]
      if (typeof info.title === 'string') found = found.filter((group) => group.title === info.title)
      return found.map((group) => ({ ...group }))
    }),
    update: mock.fn(async (groupId, props) => {
      const group = world.groups.get(groupId)
      if (!group) throw new Error(`No group with id: ${groupId}`)
      Object.assign(group, props)
      return { ...group }
    })
  }

  // tabGroups is a REQUIRED manifest permission, so nothing grants it at runtime. The
  // request spy exists only so a test can prove the service worker never calls it:
  // Chrome rejects permissions.request outside a user gesture, which a worker lacks.
  const permissions = {
    request: mock.fn(async () => {
      world.requestCalls += 1
      throw new Error('This function must be called during a user gesture.')
    })
  }

  const chromeMock = createMockChrome({
    tabs: chromeTabs,
    tabGroups: chromeTabGroups,
    windows: { update: mock.fn(async () => ({ id: 1 })) }
  })
  if (withPermissionsApi) chromeMock.permissions = permissions

  world.chrome = chromeMock
  world.permissions = permissions
  world.addTab = addTab
  world.groupOf = (tabId) => world.tabs.get(tabId)?.groupId ?? null
  world.groupTitles = () => [...world.groups.values()].map((group) => group.title)
  world.groupCount = () => world.groups.size
  return world
}

/** Install `world` as the global chrome for the duration of a test. */
export function installWorld(world) {
  globalThis.chrome = world.chrome
  return world
}
