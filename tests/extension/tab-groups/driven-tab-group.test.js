// @ts-nocheck
/**
 * @fileoverview driven-tab-group.test.js — Contract tests for the tab group that
 * holds every tab Kaboom drives.
 *
 * Covers: adoption from each entry point, a refused `tabGroups` permission
 * degrading to ungrouped driving with a named reason, session end leaving no
 * orphan group, and startup reconciliation reading live `chrome.tabGroups` state
 * rather than a storage mirror.
 */

import { beforeEach, describe, test } from 'node:test'
import assert from 'node:assert'
import { createTabGroupsWorld, installWorld, TAB_GROUP_ID_NONE } from './tab-groups-fixture.js'

const groupModule = await import('../../../extension/background/tab-groups/driven-tab-group.js')
const connectionState = await import('../../../extension/background/runtime-state/connection-state.js')
const connectionGeneration = await import('../../../extension/background/runtime-state/connection-generation.js')
const logQueue = await import('../../../extension/background/runtime-state/log-queue.js')

const {
  DRIVEN_TAB_GROUP_TITLE,
  DRIVEN_TAB_GROUP_COLOR,
  adoptTabIntoDrivenGroup,
  endDrivenTabGroupSession,
  getDrivenTabGroupId,
  noteDrivenTabClosed,
  reconcileDrivenTabGroups,
  resetDrivenTabGroupStateForTesting
} = groupModule

function loggedMessages() {
  return logQueue.getExtensionLogQueueSnapshot().map((entry) => ({ message: entry.message, data: entry.data }))
}

function degradeReasons() {
  return loggedMessages()
    .filter((entry) => entry.message.includes('tab grouping unavailable'))
    .map((entry) => entry.data?.reason)
}

beforeEach(() => {
  resetDrivenTabGroupStateForTesting()
  logQueue.clearExtensionLogsForTesting()
  connectionGeneration.setConnectionGeneration(1)
})

describe('driven tab group — adoption entry points', () => {
  test('a tab Kaboom opened joins a titled, coloured group (new_tab)', async () => {
    const world = installWorld(createTabGroupsWorld())
    const tab = world.addTab({ url: 'https://opened.example/' })

    const outcome = await adoptTabIntoDrivenGroup(tab.id, 'new_tab')

    assert.strictEqual(outcome.adopted, true)
    assert.strictEqual(world.groupOf(tab.id), outcome.group_id)
    assert.deepStrictEqual(world.groupTitles(), [DRIVEN_TAB_GROUP_TITLE])
    const [group] = await world.chrome.tabGroups.query({ title: DRIVEN_TAB_GROUP_TITLE })
    assert.strictEqual(group.color, DRIVEN_TAB_GROUP_COLOR)
    assert.strictEqual(group.collapsed, false)
  })

  test('a tab the user hands over joins the same group (switch_tab)', async () => {
    const world = installWorld(createTabGroupsWorld())
    const opened = world.addTab()
    const handedOver = world.addTab()

    const first = await adoptTabIntoDrivenGroup(opened.id, 'new_tab')
    const second = await adoptTabIntoDrivenGroup(handedOver.id, 'switch_tab')

    assert.strictEqual(second.adopted, true)
    assert.strictEqual(second.group_id, first.group_id, 'one group per MCP client session')
    assert.strictEqual(world.groupCount(), 1)
  })

  test('a tracked-tab hand-over joins the same group (set_tracked)', async () => {
    const world = installWorld(createTabGroupsWorld())
    const opened = world.addTab()
    const tracked = world.addTab()

    const first = await adoptTabIntoDrivenGroup(opened.id, 'new_tab')
    const second = await adoptTabIntoDrivenGroup(tracked.id, 'tracked_tab')

    assert.strictEqual(second.group_id, first.group_id)
    assert.strictEqual(world.groupOf(tracked.id), first.group_id)
  })

  test('a nonsense tab id is refused without touching Chrome', async () => {
    const world = installWorld(createTabGroupsWorld())
    const outcome = await adoptTabIntoDrivenGroup(-1, 'new_tab')
    assert.deepStrictEqual(outcome, { adopted: false, degraded_reason: 'invalid_tab_id' })
    assert.strictEqual(world.chrome.tabs.group.mock.calls.length, 0)
  })
})

describe('driven tab group — no permission gate stands between a drive and its group', () => {
  test('grouping engages on the first drive with no user action', async () => {
    // tabGroups is a required manifest permission, so there is nothing to grant and
    // nothing to prompt for. This is the whole point: a feature that exists to show
    // which tabs the agent holds is worthless while switched off.
    const world = installWorld(createTabGroupsWorld())
    const tab = world.addTab()

    const outcome = await adoptTabIntoDrivenGroup(tab.id, 'new_tab')

    assert.strictEqual(outcome.adopted, true)
    assert.strictEqual(world.groupOf(tab.id), getDrivenTabGroupId())
  })

  test('the worker never calls permissions.request — it has no user gesture to spend', async () => {
    // Chrome requires permissions.request() to run "from inside a user gesture, like a
    // button's click handler". An MV3 service worker never has one, so a request from
    // here would reject on every drive. Guards against reintroducing that path.
    const world = installWorld(createTabGroupsWorld())

    await adoptTabIntoDrivenGroup(world.addTab().id, 'new_tab')
    await adoptTabIntoDrivenGroup(world.addTab().id, 'switch_tab')

    assert.strictEqual(world.requestCalls, 0, 'the worker must never request a permission')
  })

  test('a browser without the tab-group APIs degrades with its own reason', async () => {
    const world = installWorld(createTabGroupsWorld({ withPermissionsApi: false }))
    delete world.chrome.tabGroups
    const tab = world.addTab()

    const outcome = await adoptTabIntoDrivenGroup(tab.id, 'new_tab')

    assert.strictEqual(outcome.adopted, false)
    assert.strictEqual(outcome.degraded_reason, 'tab_groups_api_unavailable')
    assert.strictEqual(world.groupOf(tab.id), TAB_GROUP_ID_NONE, 'the tab is still drivable, just ungrouped')
    assert.deepStrictEqual(
      degradeReasons(),
      ['tab_groups_api_unavailable'],
      'the reason is logged, never swallowed'
    )
  })

})

describe('driven tab group — session end leaves no orphan', () => {
  test('ending the session ungroups every driven tab', async () => {
    const world = installWorld(createTabGroupsWorld())
    const first = world.addTab()
    const second = world.addTab()
    await adoptTabIntoDrivenGroup(first.id, 'new_tab')
    await adoptTabIntoDrivenGroup(second.id, 'switch_tab')

    await endDrivenTabGroupSession('test_session_end')

    assert.strictEqual(world.groupCount(), 0, 'no orphan group survives the session')
    assert.strictEqual(world.groupOf(first.id), TAB_GROUP_ID_NONE)
    assert.strictEqual(world.groupOf(second.id), TAB_GROUP_ID_NONE)
    assert.strictEqual(getDrivenTabGroupId(), null)
  })

  test('the daemon going away ends the group without another drive', async () => {
    const world = installWorld(createTabGroupsWorld())
    const tab = world.addTab()
    await adoptTabIntoDrivenGroup(tab.id, 'new_tab')
    connectionState.setConnectionStatus({ extensionConnected: true })

    connectionState.setConnectionStatus({ extensionConnected: false })
    await new Promise((resolve) => setImmediate(resolve))

    assert.strictEqual(world.groupCount(), 0, 'the daemon exiting must not leave a group behind')
    assert.strictEqual(getDrivenTabGroupId(), null)
  })

  test('a new MCP client session retires the previous group', async () => {
    const world = installWorld(createTabGroupsWorld())
    const oldTab = world.addTab()
    const firstGroup = await adoptTabIntoDrivenGroup(oldTab.id, 'new_tab')

    connectionGeneration.setConnectionGeneration(2)
    const newTab = world.addTab()
    const secondGroup = await adoptTabIntoDrivenGroup(newTab.id, 'new_tab')

    assert.notStrictEqual(secondGroup.group_id, firstGroup.group_id)
    assert.strictEqual(world.groupOf(oldTab.id), TAB_GROUP_ID_NONE, 'the previous session released its tabs')
    assert.strictEqual(world.groupCount(), 1)
  })

  test('closing the last driven tab forgets the group instead of driving at a dead id', async () => {
    const world = installWorld(createTabGroupsWorld())
    const tab = world.addTab()
    const first = await adoptTabIntoDrivenGroup(tab.id, 'new_tab')

    await world.chrome.tabs.remove(tab.id)
    await noteDrivenTabClosed(tab.id)
    assert.strictEqual(getDrivenTabGroupId(), null)

    const replacement = world.addTab()
    const second = await adoptTabIntoDrivenGroup(replacement.id, 'new_tab')
    assert.strictEqual(second.adopted, true)
    assert.notStrictEqual(second.group_id, first.group_id)
  })

  test('a group the user dissolved is replaced, not reused', async () => {
    const world = installWorld(createTabGroupsWorld())
    const tab = world.addTab()
    const first = await adoptTabIntoDrivenGroup(tab.id, 'new_tab')

    await world.chrome.tabs.ungroup([tab.id])
    assert.strictEqual(world.groupCount(), 0)

    const second = await adoptTabIntoDrivenGroup(tab.id, 'switch_tab')
    assert.strictEqual(second.adopted, true)
    assert.notStrictEqual(second.group_id, first.group_id)
  })
})

describe('driven tab group — startup reconciliation reads live Chrome state', () => {
  test('an orphan group from a dead worker is released on the first drive', async () => {
    const world = installWorld(createTabGroupsWorld())
    const orphanTab = world.addTab()
    const orphanGroupId = await world.chrome.tabs.group({ tabIds: [orphanTab.id] })
    await world.chrome.tabGroups.update(orphanGroupId, { title: DRIVEN_TAB_GROUP_TITLE })

    // Nothing in module state points at the orphan — it is found by querying Chrome.
    assert.strictEqual(getDrivenTabGroupId(), null)

    const tab = world.addTab()
    const outcome = await adoptTabIntoDrivenGroup(tab.id, 'new_tab')

    assert.strictEqual(world.groupOf(orphanTab.id), TAB_GROUP_ID_NONE, 'the orphan group is released')
    assert.strictEqual(world.groupCount(), 1)
    assert.strictEqual(world.groupOf(tab.id), outcome.group_id)
    assert.strictEqual(world.chrome.storage.local.get.mock.calls.length, 0, 'reconciliation reads no storage mirror')
  })

  test('reconciliation leaves the terminal workspace group alone', async () => {
    const world = installWorld(createTabGroupsWorld())
    const terminalTab = world.addTab()
    const terminalGroupId = await world.chrome.tabs.group({ tabIds: [terminalTab.id] })
    await world.chrome.tabGroups.update(terminalGroupId, { title: 'KaBOOM!' })

    const released = await reconcileDrivenTabGroups()

    assert.strictEqual(released, 0)
    assert.strictEqual(world.groupOf(terminalTab.id), terminalGroupId)
  })

  test('reconciliation never releases the group of the live session', async () => {
    const world = installWorld(createTabGroupsWorld())
    const tab = world.addTab()
    const outcome = await adoptTabIntoDrivenGroup(tab.id, 'new_tab')

    const released = await reconcileDrivenTabGroups()

    assert.strictEqual(released, 0)
    assert.strictEqual(world.groupOf(tab.id), outcome.group_id)
  })

  test('reconciliation runs once per worker, not on every drive', async () => {
    const world = installWorld(createTabGroupsWorld())
    await adoptTabIntoDrivenGroup(world.addTab().id, 'new_tab')
    const afterFirst = world.chrome.tabGroups.query.mock.calls.length
    await adoptTabIntoDrivenGroup(world.addTab().id, 'switch_tab')
    assert.strictEqual(world.chrome.tabGroups.query.mock.calls.length, afterFirst)
  })
})
