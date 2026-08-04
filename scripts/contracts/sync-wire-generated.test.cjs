// sync-wire-generated.test.cjs — Verifies generated TypeScript uses the shared /sync fixture.

const { test } = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')

const root = path.resolve(__dirname, '..', '..')

test('generated sync feature keys and shared fixture remain identical', async () => {
  const fixture = JSON.parse(fs.readFileSync(path.join(__dirname, 'testdata', 'sync-roundtrip.json'), 'utf8'))
  const generated = await import(path.join(root, 'extension', 'types', 'wire', 'wire-sync.js'))

  assert.deepEqual(Object.keys(fixture.request.features_used), [...generated.SYNC_UI_FEATURES])
  assert.equal(fixture.request.connection_generation, fixture.response.connection_generation)
  assert.equal(fixture.response.commands[0].connection_generation, fixture.response.connection_generation)
  assert.equal(fixture.request.command_results[0].correlation_id, 'fixture-correlation')
  assert.equal(fixture.response.capture_overrides.constructor, Object)
})
