// page-telemetry.test.js — Own-property schema dispatch contracts for page telemetry validation.
// Regression: prototype names ('constructor', '__proto__', ...) must be rejected as invalid_schema,
// matching the old switch default case instead of resolving inherited properties as validators.
import { describe, test } from 'node:test'
import assert from 'node:assert/strict'

import { validatePageTelemetry } from '../../../extension/content/page-telemetry.js'

describe('page telemetry schema dispatch', () => {
  test('accepts a known message type with a valid payload', () => {
    assert.strictEqual(validatePageTelemetry('kaboom_log', { ts: '2026-08-26T00:00:00Z', level: 'info' }), null)
  })

  test('rejects unknown message types as invalid schema', () => {
    assert.strictEqual(validatePageTelemetry('not_a_kaboom_type', { ts: '1', level: 'info' }), 'invalid_schema')
  })

  test('rejects prototype-chain names as invalid schema', () => {
    for (const type of ['constructor', 'toString', 'valueOf', 'hasOwnProperty', '__proto__']) {
      assert.strictEqual(validatePageTelemetry(type, { ts: '1', level: 'info' }), 'invalid_schema')
    }
  })

  test('rejects known message types with malformed payloads', () => {
    assert.strictEqual(validatePageTelemetry('kaboom_log', { level: 'info' }), 'invalid_schema')
    assert.strictEqual(validatePageTelemetry('kaboom_network_body', { method: 'GET', url: 'x', status: '200' }), 'invalid_schema')
    assert.strictEqual(validatePageTelemetry('kaboom_log', null), 'invalid_schema')
  })
})
