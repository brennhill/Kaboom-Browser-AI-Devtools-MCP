/**
 * @fileoverview Regression guard for CRX fallback packaging.
 * Ensures fallback zip logic includes the full extension tree so MV3 module imports cannot be omitted.
 */

import { test } from 'node:test'
import assert from 'node:assert'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const REPO_ROOT = path.resolve(__dirname, '../../..')
const BUILD_CRX = path.join(REPO_ROOT, 'scripts', 'release', 'build-crx.js')

function getFallbackZipBlock(fileText) {
  const start = fileText.indexOf("console.log('📦 Creating extension zip...')")
  assert.notStrictEqual(start, -1, 'Fallback zip section should exist in scripts/release/build-crx.js')

  const end = fileText.indexOf('if (!fs.existsSync(TEMP_ZIP))', start)
  assert.notStrictEqual(end, -1, 'Fallback zip section should include TEMP_ZIP existence check')

  return fileText.slice(start, end)
}

test('CRX fallback zip command packages entire extension directory', () => {
  const script = fs.readFileSync(BUILD_CRX, 'utf8')
  const block = getFallbackZipBlock(script)

  assert.match(
    block,
    /zip -q -r "\.\.\/\$\{TEMP_ZIP\}" \\\s*\n\s*\./m,
    'Fallback zip command must package "." from extension/ to include all files'
  )

  assert.doesNotMatch(
    block,
    /manifest\.json\s+background\.js/m,
    'Fallback zip command must not use a hardcoded allowlist'
  )
})
