// @ts-nocheck
/**
 * @fileoverview browser-action-group-adoption.test.js — Proves every driving entry
 * point in browser-actions.ts routes through the one adoption helper, so a tab
 * Kaboom opened or was handed is always visible in the Kaboom group.
 *
 * Entry points covered: `new_tab`, `navigate` with `new_tab`, `switch_tab`
 * (which is also the `set_tracked` hand-over), and `close_tab`.
 */

import { beforeEach, describe, test, mock } from 'node:test'
import assert from 'node:assert'
import { createTabGroupsWorld, installWorld, TAB_GROUP_ID_NONE } from './tab-groups-fixture.js'

const groupModule = await import('../../../extension/background/tab-groups/driven-tab-group.js')
const pilotState = await import('../../../extension/background/runtime-state/pilot-state.js')
const connectionGeneration = await import('../../../extension/background/runtime-state/connection-generation.js')
const { handleBrowserAction } = await import('../../../extension/background/exec/browser-actions.js')
const { persistTrackedTab } = await import('../../../extension/background/commands/helpers.js')

const { DRIVEN_TAB_GROUP_TITLE, getDrivenTabGroupId, resetDrivenTabGroupStateForTesting } = groupModule

const noopToast = mock.fn()

beforeEach(() => {
  resetDrivenTabGroupStateForTesting()
  pilotState.resetPilotCacheForTesting(true)
  connectionGeneration.setConnectionGeneration(1)
  noopToast.mock.resetCalls()
})

describe('the tracked-tab funnel adopts, so no recovery path can forget', () => {
  test('persistTrackedTab adopts the tab it just made tracked', async () => {
    // persistTrackedTab is the funnel for the auto-track recovery paths
    // (auto_tracked_active_tab, auto_tracked_random_tab, auto_tracked_new_tab and the
    // tryAutoTrackFallback retry). Adoption lives in the funnel rather than at those
    // call sites so a future recovery path cannot silently skip the group (rule 19).
    const world = installWorld(createTabGroupsWorld())
    const tab = world.addTab()

    await persistTrackedTab({ id: tab.id, url: 'https://auto-tracked.example/', title: 'recovered' })

    assert.strictEqual(world.groupOf(tab.id), getDrivenTabGroupId())
    assert.deepStrictEqual(world.groupTitles(), [DRIVEN_TAB_GROUP_TITLE])
  })

  test('a tab is adopted exactly once when switch_tab hands it over', async () => {
    // switch_tab persists AND adopts; if both did their own adoption the tab would be
    // grouped twice, which is how a second stray group gets created.
    const world = installWorld(createTabGroupsWorld())
    const source = world.addTab()
    const target = world.addTab({ url: 'https://target.example/' })

    await handleBrowserAction(source.id, { action: 'switch_tab', tab_id: target.id }, noopToast, 'c9')

    assert.strictEqual(world.groupOf(target.id), getDrivenTabGroupId())
    assert.strictEqual(world.groupCount(), 1, 'one group, not one per adoption call')
  })
})

describe('browser actions adopt driven tabs into the Kaboom group', () => {
  test('new_tab puts the tab Kaboom opened into the group', async () => {
    const world = installWorld(createTabGroupsWorld())
    const source = world.addTab()

    const result = await handleBrowserAction(
      source.id,
      { action: 'new_tab', url: 'https://opened.example/' },
      noopToast,
      'c1'
    )

    assert.strictEqual(result.success, true)
    assert.strictEqual(world.groupOf(result.tab_id), getDrivenTabGroupId())
    assert.deepStrictEqual(world.groupTitles(), [DRIVEN_TAB_GROUP_TITLE])
  })

  test('navigate with new_tab uses the same adoption path', async () => {
    const world = installWorld(createTabGroupsWorld())
    const source = world.addTab()

    const result = await handleBrowserAction(
      source.id,
      { action: 'navigate', url: 'https://opened.example/', new_tab: true },
      noopToast,
      'c2'
    )

    assert.strictEqual(result.success, true)
    assert.strictEqual(world.groupOf(result.tab_id), getDrivenTabGroupId())
  })

  test('switch_tab adopts the tab the user handed over', async () => {
    const world = installWorld(createTabGroupsWorld())
    const source = world.addTab()
    const handedOver = world.addTab({ url: 'https://handed.example/' })

    const result = await handleBrowserAction(source.id, { action: 'switch_tab', tab_id: handedOver.id }, noopToast, 'c3')

    assert.strictEqual(result.success, true)
    assert.strictEqual(world.groupOf(handedOver.id), getDrivenTabGroupId())
    assert.notStrictEqual(getDrivenTabGroupId(), null)
  })

  test('a tab opened then handed over shares one group', async () => {
    const world = installWorld(createTabGroupsWorld())
    const source = world.addTab()
    const opened = await handleBrowserAction(
      source.id,
      { action: 'new_tab', url: 'https://opened.example/' },
      noopToast,
      'c4'
    )
    const handedOver = world.addTab()
    await handleBrowserAction(source.id, { action: 'switch_tab', tab_id: handedOver.id }, noopToast, 'c5')

    assert.strictEqual(world.groupCount(), 1)
    assert.strictEqual(world.groupOf(opened.tab_id), world.groupOf(handedOver.id))
  })

  test('close_tab on the last driven tab leaves no group behind', async () => {
    const world = installWorld(createTabGroupsWorld())
    const source = world.addTab()
    const opened = await handleBrowserAction(
      source.id,
      { action: 'new_tab', url: 'https://opened.example/' },
      noopToast,
      'c6'
    )
    assert.notStrictEqual(getDrivenTabGroupId(), null)

    const result = await handleBrowserAction(source.id, { action: 'close_tab', tab_id: opened.tab_id }, noopToast, 'c7')

    assert.strictEqual(result.success, true)
    assert.strictEqual(world.groupCount(), 0)
    assert.strictEqual(getDrivenTabGroupId(), null)
  })

  test('a refused tabGroups permission never breaks the drive', async () => {
    const world = installWorld(createTabGroupsWorld({ granted: false, onRequest: 'deny' }))
    const source = world.addTab()

    const result = await handleBrowserAction(
      source.id,
      { action: 'new_tab', url: 'https://opened.example/' },
      noopToast,
      'c8'
    )

    assert.strictEqual(result.success, true, 'the tab still opens without the permission')
    assert.strictEqual(result.url, 'https://opened.example/')
    assert.strictEqual(world.groupOf(result.tab_id), TAB_GROUP_ID_NONE)
    assert.strictEqual(world.groupCount(), 0)
  })

  test('a Chrome grouping failure never breaks the drive', async () => {
    const world = installWorld(createTabGroupsWorld())
    world.chrome.tabs.group = mock.fn(async () => {
      throw new Error('Tabs cannot be edited right now (user may be dragging a tab).')
    })
    const source = world.addTab()

    const result = await handleBrowserAction(
      source.id,
      { action: 'new_tab', url: 'https://opened.example/' },
      noopToast,
      'c9'
    )

    assert.strictEqual(result.success, true)
    assert.strictEqual(getDrivenTabGroupId(), null)
  })
})
