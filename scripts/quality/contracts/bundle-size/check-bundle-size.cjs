#!/usr/bin/env node
// check-bundle-size.cjs — Caps the shipped extension bundle footprint.
//
// verify-size caps the Go binary; nothing capped the compiled extension output
// until now. Every artifact compile-ts emits is budgeted: per-file (a single
// bundle ballooning starves the service-worker parse budget) and total (the
// footprint users download per update). Caps carry ~30% headroom over the
// current tree — growth beyond them is a reviewable event, not a surprise.
//
// Usage: node scripts/quality/contracts/check-bundle-size.cjs
// Exit 0 when within budget, 1 otherwise. Missing artifacts are failures so a
// silently-skipped bundle step cannot pass as a size win.

const fs = require('node:fs')
const path = require('node:path')

const REPO_ROOT = path.resolve(__dirname, '..', '..', '..', '..')

const BUNDLED_FILES = [
  'content.bundled.js',
  'inject.bundled.js',
  'early-patch.bundled.js',
  'offscreen.bundled.js',
  'popup.bundled.js',
  path.join('content', 'draw-mode.js')
]

const MAX_FILE_BYTES = 250_000
const MAX_TOTAL_BYTES = 600_000

function checkBundleSize(root = REPO_ROOT, limits = { maxFile: MAX_FILE_BYTES, maxTotal: MAX_TOTAL_BYTES }) {
  const extensionDir = path.join(root, 'extension')
  const violations = []
  const sizes = []
  for (const relPath of BUNDLED_FILES) {
    const full = path.join(extensionDir, relPath)
    if (!fs.existsSync(full)) {
      violations.push({ kind: 'missing', file: relPath, bytes: 0, limit: 0 })
      continue
    }
    const bytes = fs.statSync(full).size
    sizes.push({ file: relPath, bytes })
    if (bytes > limits.maxFile) {
      violations.push({ kind: 'file', file: relPath, bytes, limit: limits.maxFile })
    }
  }
  const totalBytes = sizes.reduce((sum, entry) => sum + entry.bytes, 0)
  if (totalBytes > limits.maxTotal) {
    violations.push({ kind: 'total', file: '(total)', bytes: totalBytes, limit: limits.maxTotal })
  }
  return { violations, totalBytes, sizes }
}

function main() {
  const { violations, totalBytes, sizes } = checkBundleSize()
  for (const entry of sizes) {
    const kb = (entry.bytes / 1024).toFixed(1).padStart(8)
    console.log(`  ${kb} KB  ${entry.file}`)
  }
  const totalKb = (totalBytes / 1024).toFixed(1)
  if (violations.length === 0) {
    console.log(
      `✅ Extension bundles within budget (${totalKb} KB total; limits ${MAX_FILE_BYTES / 1000}KB/file, ${MAX_TOTAL_BYTES / 1000}KB total)`
    )
    return 0
  }
  console.error(`❌ Extension bundle budget exceeded (${totalKb} KB total):\n`)
  for (const v of violations) {
    if (v.kind === 'missing') {
      console.error(`  missing  extension/${v.file} — run make compile-ts`)
    } else if (v.kind === 'file') {
      console.error(`  ${v.bytes} B  extension/${v.file} exceeds ${v.limit} B`)
    } else {
      console.error(`  ${v.bytes} B  total exceeds ${v.limit} B`)
    }
  }
  console.error('\nSplit or trim the bundle; growth beyond the cap needs a deliberate budget decision.')
  return 1
}

if (process.argv[1] && path.resolve(process.argv[1]) === __filename) {
  process.exit(main())
}

module.exports = { checkBundleSize, BUNDLED_FILES, MAX_FILE_BYTES, MAX_TOTAL_BYTES }
