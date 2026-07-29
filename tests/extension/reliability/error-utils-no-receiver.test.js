// @ts-nocheck
/**
 * @fileoverview error-utils-no-receiver.test.js — Tests for isNoReceiverError.
 *
 * Regression (#status-update-noise): background broadcasts (e.g. status_update)
 * reject with "Could not establish connection. Receiving end does not exist."
 * whenever no extension page is listening (the popup is closed). That is the
 * expected steady state, not an error, and must be swallowed rather than logged.
 */

import { test, describe, beforeEach } from 'node:test'
import assert from 'node:assert'

describe('isNoReceiverError', () => {
  let isNoReceiverError

  beforeEach(async () => {
    ;({ isNoReceiverError } = await import('../../../extension/lib/error-utils.js'))
  })

  test('matches the closed-popup "Receiving end does not exist" rejection', () => {
    assert.strictEqual(
      isNoReceiverError(new Error('Could not establish connection. Receiving end does not exist.')),
      true
    )
  })

  test('matches a bare "Could not establish connection" message', () => {
    assert.strictEqual(isNoReceiverError(new Error('Could not establish connection')), true)
  })

  test('matches a plain string rejection', () => {
    assert.strictEqual(isNoReceiverError('Receiving end does not exist'), true)
  })

  test('does NOT match a genuine error (so real failures still log)', () => {
    assert.strictEqual(isNoReceiverError(new Error('QUOTA_BYTES quota exceeded')), false)
    assert.strictEqual(isNoReceiverError(new Error('Extension context invalidated.')), false)
  })

  test('handles null/undefined without throwing', () => {
    assert.strictEqual(isNoReceiverError(null), false)
    assert.strictEqual(isNoReceiverError(undefined), false)
  })
})
