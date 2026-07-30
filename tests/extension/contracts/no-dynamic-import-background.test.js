// @ts-nocheck
/**
 * @fileoverview no-dynamic-import-background.test.js — Contract test: the MV3 service
 * worker (src/background/) must never use runtime dynamic imports. Chrome disallows
 * them in service workers; the call rejects at runtime, handlers that returned true
 * never call sendResponse, and awaiting senders hang until the port closes.
 * Regression guard for the Audit feature breakage (qa_scan_requested handler).
 *
 * Two layers:
 * 1. Source scan (src/background/**.ts + src/background.ts): flags runtime dynamic
 *    import forms. Type-only `import('mod').Type` annotations are allowed — they are
 *    erased at compile time.
 * 2. Compiled scan (extension/background.js + extension/background/**.js, excluding
 *    colocated *.test.js): authoritative — type imports are erased here, so ANY
 *    surviving dynamic import would break the service worker at runtime.
 */

import { test, describe } from 'node:test'
import assert from 'node:assert'
import { readFileSync, readdirSync, statSync, existsSync } from 'node:fs'
import { join } from 'node:path'

const ROOT = new URL('../../../', import.meta.url).pathname
const BACKGROUND_SRC_DIR = join(ROOT, 'src/background')
const BACKGROUND_SRC_ENTRY = join(ROOT, 'src/background.ts')
const BACKGROUND_OUT_DIR = join(ROOT, 'extension/background')
const BACKGROUND_OUT_ENTRY = join(ROOT, 'extension/background.js')

// Runtime dynamic-import forms (type-position `: import('m').T` / `as import('m').T`
// annotations do not match any of these).
const RUNTIME_DYNAMIC_IMPORT_PATTERNS = [
  /\bawait\s+import\s*\(/, // await import('...')
  /=\s*import\s*\(/, // const p = import('...')
  /\bvoid\s+import\s*\(/, // void import('...')
  /\breturn\s+import\s*\(/, // return import('...')
  /^\s*import\s*\(/ // bare statement import('...')
]

// In compiled JS all type annotations are erased, so any import( is a runtime import.
const COMPILED_DYNAMIC_IMPORT_PATTERN = /\bimport\s*\(/

function isCommentLine(line) {
  const trimmed = line.trimStart()
  return trimmed.startsWith('//') || trimmed.startsWith('*') || trimmed.startsWith('/*')
}

function collectFiles(dir, extension, { excludeTests = false } = {}) {
  const files = []
  for (const entry of readdirSync(dir)) {
    const fullPath = join(dir, entry)
    if (statSync(fullPath).isDirectory()) {
      files.push(...collectFiles(fullPath, extension, { excludeTests }))
    } else if (entry.endsWith(extension)) {
      if (excludeTests && entry.endsWith(`.test${extension}`)) continue
      files.push(fullPath)
    }
  }
  return files
}

function findViolations(files, patterns) {
  const violations = []
  for (const file of files) {
    const lines = readFileSync(file, 'utf8').split('\n')
    lines.forEach((line, idx) => {
      if (isCommentLine(line)) return
      if (patterns.some((p) => p.test(line))) {
        violations.push(`${file.replace(ROOT, '')}:${idx + 1}: ${line.trim()}`)
      }
    })
  }
  return violations
}

describe('service worker dynamic import contract', () => {
  test('background sources exist to scan', () => {
    assert.ok(existsSync(BACKGROUND_SRC_ENTRY), 'expected src/background.ts')
    const srcFiles = collectFiles(BACKGROUND_SRC_DIR, '.ts')
    assert.ok(srcFiles.length > 0, 'expected .ts files under src/background/')
  })

  test('no runtime dynamic import in src/background sources', () => {
    const files = [BACKGROUND_SRC_ENTRY, ...collectFiles(BACKGROUND_SRC_DIR, '.ts')]
    const violations = findViolations(files, RUNTIME_DYNAMIC_IMPORT_PATTERNS)
    assert.deepStrictEqual(
      violations,
      [],
      `Runtime dynamic imports are not allowed in the MV3 service worker:\n${violations.join('\n')}`
    )
  })

  test('no dynamic import in compiled service worker output', () => {
    assert.ok(existsSync(BACKGROUND_OUT_ENTRY), 'expected extension/background.js (run make compile-ts)')
    const files = [BACKGROUND_OUT_ENTRY, ...collectFiles(BACKGROUND_OUT_DIR, '.js', { excludeTests: true })]
    const violations = findViolations(files, [COMPILED_DYNAMIC_IMPORT_PATTERN])
    assert.deepStrictEqual(
      violations,
      [],
      `Dynamic import survived into compiled service worker output (breaks at runtime):\n${violations.join('\n')}`
    )
  })

  test('background aggregate service facade stays deleted', () => {
    assert.strictEqual(existsSync(join(ROOT, 'src/background/index.ts')), false)
  })
})
