// command-element-results.test.js — Shared command element filtering contracts.

import assert from 'node:assert/strict'
import { test } from 'node:test'

import {
  collectCommandElements,
  commandPageMetadata,
  selectCommandElements
} from '../../../extension/background/commands/results/element-results.js'

test('selectCommandElements applies visibility before the requested limit', () => {
  const elements = [
    { id: 'hidden', visible: false },
    { id: 'first', visible: true },
    { id: 'second' }
  ]

  assert.deepEqual(
    selectCommandElements(elements, { visible_only: true, limit: 1 }),
    [{ id: 'first', visible: true }]
  )
})

test('selectCommandElements preserves all elements when filters are absent', () => {
  const elements = [{ id: 'first' }, { id: 'second', visible: false }]

  assert.deepEqual(selectCommandElements(elements, {}), elements)
})

test('collectCommandElements caps merged frame results and preserves the first error', () => {
  const result = collectCommandElements(
    [
      { result: { success: false, error: 'frame_failed' } },
      { result: { elements: [{ id: 1 }, { id: 2 }] } },
      { result: { elements: [{ id: 3 }] } }
    ],
    2
  )

  assert.deepEqual(result, {
    elements: [{ id: 1 }, { id: 2 }],
    firstError: 'frame_failed'
  })
})

test('commandPageMetadata normalizes optional tab fields', () => {
  assert.deepEqual(
    commandPageMetadata({ url: undefined, title: 'Page', status: undefined, favIconUrl: '', width: 800, height: 600 }),
    {
      url: '',
      title: 'Page',
      tab_status: '',
      favicon: '',
      viewport: { width: 800, height: 600 }
    }
  )
})
