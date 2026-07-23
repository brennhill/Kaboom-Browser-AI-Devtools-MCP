// @ts-nocheck
/**
 * @fileoverview manifest-command-limits.test.js — Guards Chrome's hard limits on
 * the manifest `commands` block.
 *
 * Regression: adding `open_terminal_panel` with a `suggested_key` made five
 * commands carry default keys. Chrome allows at most four, and it does not
 * degrade — it refuses the whole manifest with "Too many shortcuts specified for
 * 'commands': The maximum is 4", so the entire extension fails to load. Nothing
 * in the build caught it because the manifest is still valid JSON.
 */

import { describe, test } from 'node:test'
import assert from 'node:assert'
import { readFileSync } from 'node:fs'

const MAX_SUGGESTED_KEYS = 4

function loadManifest() {
  return JSON.parse(readFileSync('extension/manifest.json', 'utf8'))
}

describe('manifest commands', () => {
  test(`at most ${MAX_SUGGESTED_KEYS} commands may declare a suggested_key`, () => {
    const commands = loadManifest().commands ?? {}
    const withKeys = Object.entries(commands).filter(([, cmd]) => cmd?.suggested_key)
    assert.ok(
      withKeys.length <= MAX_SUGGESTED_KEYS,
      `${withKeys.length} commands declare suggested_key (max ${MAX_SUGGESTED_KEYS}). ` +
        `Chrome rejects the entire manifest, so the extension will not load at all. ` +
        `Ship extra commands unbound and let users assign keys at chrome://extensions/shortcuts. ` +
        `Offenders: ${withKeys.map(([name]) => name).join(', ')}`
    )
  })

  test('every command has a description so it is assignable in the shortcuts UI', () => {
    const commands = loadManifest().commands ?? {}
    for (const [name, cmd] of Object.entries(commands)) {
      assert.ok(
        typeof cmd?.description === 'string' && cmd.description.length > 0,
        `command "${name}" needs a description; without one users cannot identify it at chrome://extensions/shortcuts`
      )
    }
  })

  test('the terminal panel command is still registered', () => {
    // It may be unbound, but it must exist: it is the gesture-native path that
    // the in-page launcher button cannot reliably provide.
    const commands = loadManifest().commands ?? {}
    assert.ok(commands.open_terminal_panel, 'open_terminal_panel command is missing')
  })
})
