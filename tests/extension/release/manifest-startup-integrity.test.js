/**
 * @fileoverview Startup integrity checks for MV3 extension loading.
 * Guards against service worker registration failures caused by missing manifest paths or unresolved module imports.
 */

import { test } from 'node:test'
import assert from 'node:assert'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const REPO_ROOT = path.resolve(__dirname, '../../..')
const EXT_DIR = path.join(REPO_ROOT, 'extension')
const MANIFEST_PATH = path.join(EXT_DIR, 'manifest.json')

function readManifest() {
  return JSON.parse(fs.readFileSync(MANIFEST_PATH, 'utf8'))
}

function assertFileExists(relPath, label) {
  const abs = path.join(EXT_DIR, relPath)
  assert.ok(fs.existsSync(abs), `${label} missing file: ${relPath}`)
}

function extractStaticModuleSpecifiers(js) {
  const specifiers = new Set()

  const fromRegex = /\b(?:import|export)\s+(?:[^'"]*?\s+from\s+)?['"]([^'"]+)['"]/g
  for (const match of js.matchAll(fromRegex)) {
    specifiers.add(match[1])
  }

  const dynamicRegex = /\bimport\s*\(\s*['"]([^'"]+)['"]\s*\)/g
  for (const match of js.matchAll(dynamicRegex)) {
    specifiers.add(match[1])
  }

  return [...specifiers]
}

function resolveLocalImport(baseFile, specifier) {
  if (!specifier.startsWith('./') && !specifier.startsWith('../')) return null
  return path.resolve(path.dirname(baseFile), specifier)
}

function walkModuleGraph(entryRelPath) {
  const entryAbs = path.join(EXT_DIR, entryRelPath)
  const queue = [entryAbs]
  const seen = new Set()

  while (queue.length > 0) {
    const current = queue.pop()
    if (!current || seen.has(current)) continue
    seen.add(current)

    assert.ok(fs.existsSync(current), `Module graph missing file: ${path.relative(EXT_DIR, current)}`)
    const js = fs.readFileSync(current, 'utf8')
    const imports = extractStaticModuleSpecifiers(js)

    for (const specifier of imports) {
      const resolved = resolveLocalImport(current, specifier)
      if (!resolved) continue
      queue.push(resolved)
    }
  }

  return seen
}

function collectHtmlFiles(dir, acc = []) {
  const entries = fs.readdirSync(dir, { withFileTypes: true })
  for (const entry of entries) {
    const abs = path.join(dir, entry.name)
    if (entry.isDirectory()) {
      collectHtmlFiles(abs, acc)
      continue
    }
    if (entry.isFile() && entry.name.endsWith('.html')) {
      acc.push(abs)
    }
  }
  return acc
}

test('manifest startup file references exist on disk', () => {
  const manifest = readManifest()

  assert.strictEqual(manifest.manifest_version, 3, 'manifest must remain MV3')

  assertFileExists(manifest.background.service_worker, 'background.service_worker')

  for (const script of manifest.content_scripts || []) {
    for (const jsFile of script.js || []) {
      assertFileExists(jsFile, 'content_scripts.js')
    }
  }

  if (manifest.action?.default_popup) {
    assertFileExists(manifest.action.default_popup, 'action.default_popup')
  }

  if (manifest.options_ui?.page) {
    assertFileExists(manifest.options_ui.page, 'options_ui.page')
  }

  for (const war of manifest.web_accessible_resources || []) {
    for (const resource of war.resources || []) {
      assertFileExists(resource, 'web_accessible_resources')
    }
  }
})

test('service worker module graph fully resolves from manifest entrypoint', () => {
  const manifest = readManifest()
  const graph = walkModuleGraph(manifest.background.service_worker)
  assert.ok(graph.size > 1, 'service worker graph should include imported modules')
})

test('extension HTML files avoid inline script tags (MV3 CSP safe)', () => {
  const htmlFiles = collectHtmlFiles(EXT_DIR)
  assert.ok(htmlFiles.length > 0, 'expected extension HTML files to exist')

  const inlineScriptTag = /<script\b(?![^>]*\bsrc\s*=)[^>]*>/gi
  for (const abs of htmlFiles) {
    const html = fs.readFileSync(abs, 'utf8')
    assert.doesNotMatch(
      html,
      inlineScriptTag,
      `Inline <script> is not allowed in ${path.relative(EXT_DIR, abs)} under MV3 CSP`
    )
  }
})
