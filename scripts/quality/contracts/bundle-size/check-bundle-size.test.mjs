// check-bundle-size.test.mjs — Pins the extension bundle size budget.
import test from 'node:test'
import assert from 'node:assert/strict'
import { mkdtempSync, mkdirSync, writeFileSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { checkBundleSize, BUNDLED_FILES } from './check-bundle-size.cjs'

function bundleRoot(sizes) {
  const root = mkdtempSync(join(tmpdir(), 'bundle-size-'))
  mkdirSync(join(root, 'extension', 'content'), { recursive: true })
  for (const [name, bytes] of Object.entries(sizes)) {
    writeFileSync(join(root, 'extension', name), 'x'.repeat(bytes))
  }
  return root
}

test('bundled file inventory covers every compile-ts artifact', () => {
  assert.ok(BUNDLED_FILES.includes('content.bundled.js'))
  assert.ok(BUNDLED_FILES.includes('inject.bundled.js'))
  assert.ok(BUNDLED_FILES.includes('early-patch.bundled.js'))
  assert.ok(BUNDLED_FILES.includes('offscreen.bundled.js'))
  assert.ok(BUNDLED_FILES.includes('popup.bundled.js'))
  assert.ok(BUNDLED_FILES.includes(join('content', 'draw-mode.js')))
})

test('under-budget bundles pass and report totals', () => {
  const sizes = Object.fromEntries(BUNDLED_FILES.map((f) => [f, 500]))
  sizes['content.bundled.js'] = 1000
  const root = bundleRoot(sizes)
  try {
    const result = checkBundleSize(root)
    assert.deepEqual(result.violations, [])
    assert.equal(result.totalBytes, 500 * 5 + 1000)
  } finally {
    rmSync(root, { recursive: true, force: true })
  }
})

test('oversized single file and oversized total are both flagged', () => {
  const root = bundleRoot({
    'content.bundled.js': 260_000,
    'inject.bundled.js': 260_000,
    'popup.bundled.js': 100_000
  })
  try {
    const result = checkBundleSize(root)
    assert.equal(result.violations.filter((v) => v.kind === 'file').length, 2)
    assert.equal(result.violations.filter((v) => v.kind === 'total').length, 1)
  } finally {
    rmSync(root, { recursive: true, force: true })
  }
})

test('missing bundle artifacts are reported, not skipped', () => {
  const root = bundleRoot({ 'content.bundled.js': 10 })
  try {
    const result = checkBundleSize(root)
    assert.ok(result.violations.some((v) => v.kind === 'missing'))
  } finally {
    rmSync(root, { recursive: true, force: true })
  }
})
