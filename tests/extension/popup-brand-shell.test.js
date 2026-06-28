// @ts-nocheck
/**
 * @fileoverview popup-brand-shell.test.js — Guards popup shell branding strings.
 */

import { describe, test } from 'node:test'
import assert from 'node:assert'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const TEST_DIR = path.dirname(fileURLToPath(import.meta.url))
const REPO_ROOT = path.resolve(TEST_DIR, '../..')

describe('popup brand shell', () => {
  test('popup html uses Kaboom branding', () => {
    const popupHtml = fs.readFileSync(path.join(REPO_ROOT, 'extension/popup.html'), 'utf8')
    assert.match(popupHtml, /<title>KaBOOM! Devtools<\/title>/)
    assert.match(popupHtml, />\s*KaBOOM! Devtools\s*<\/h1>/)
  })
})
