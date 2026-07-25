// @ts-nocheck
/**
 * @fileoverview recording-log-branding.test.js — Guards the recording stack against legacy log prefixes.
 */

import { describe, test } from 'node:test'
import assert from 'node:assert'
import { readFile } from 'node:fs/promises'

const RECORDING_SOURCES = [
  'src/background/recording/capture.ts',
  'src/background/recording/index.ts',
  'src/background/recording/listeners.ts',
  'src/offscreen/recording-worker.ts',
  'src/popup/recording.ts',
  'src/popup/recording-io.ts'
]

describe('recording log branding', () => {
  test('recording modules do not hardcode the Kaboom recording prefix', async () => {
    for (const relativePath of RECORDING_SOURCES) {
      const contents = await readFile(new URL(`../../${relativePath}`, import.meta.url), 'utf8')
      assert.doesNotMatch(
        contents,
        /\[Kaboom REC(?: offscreen)?\]/,
        `${relativePath} still hardcodes the legacy recording prefix`
      )
    }
  })
})
