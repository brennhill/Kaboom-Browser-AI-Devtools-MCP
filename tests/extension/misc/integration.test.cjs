/**
 * Integration tests - Verify extension structure and loadability
 * These tests catch issues that unit tests miss:
 * - Manifest pointing to non-existent files
 * - Missing module exports
 * - Signature mismatches between modules
 */

const { describe, test } = require('node:test')
const assert = require('node:assert')
const fs = require('node:fs')
const path = require('node:path')

const EXTENSION_DIR = path.join(__dirname, '../../../extension')

describe('Extension Integration', () => {
  test('manifest.json exists and is valid JSON', () => {
    const manifestPath = path.join(EXTENSION_DIR, 'manifest.json')
    assert(fs.existsSync(manifestPath), 'manifest.json should exist')

    const content = fs.readFileSync(manifestPath, 'utf8')
    const manifest = JSON.parse(content) // Will throw if invalid JSON

    assert.strictEqual(manifest.manifest_version, 3)
    assert(manifest.name)
    assert(manifest.version)
  })

  test('manifest background service_worker points to existing file', () => {
    const manifestPath = path.join(EXTENSION_DIR, 'manifest.json')
    const manifest = JSON.parse(fs.readFileSync(manifestPath, 'utf8'))

    const serviceWorker = manifest.background.service_worker
    assert(serviceWorker, 'background.service_worker should be defined')

    const serviceWorkerPath = path.join(EXTENSION_DIR, serviceWorker)
    assert(fs.existsSync(serviceWorkerPath), `Service worker file should exist at: ${serviceWorkerPath}`)
  })

  test('background/init.js is compiled from TypeScript', () => {
    const indexPath = path.join(EXTENSION_DIR, 'background/init.js')
    assert(fs.existsSync(indexPath), 'background/init.js should exist')
    assert(fs.readFileSync(indexPath, 'utf8').length > 0, 'background/init.js should not be empty')
  })

  test('background/init.js has the canonical startup export', () => {
    const indexPath = path.join(EXTENSION_DIR, 'background/init.js')
    const content = fs.readFileSync(indexPath, 'utf8')

    assert(content.includes('initializeExtension'), 'Should export initializeExtension')
  })

  test('background aggregate facade is absent', () => {
    assert.strictEqual(fs.existsSync(path.join(EXTENSION_DIR, 'background/index.js')), false)
  })

  test('content.js exists', () => {
    const contentPath = path.join(EXTENSION_DIR, 'content.js')
    assert(fs.existsSync(contentPath), 'content.js should exist')
  })

  test('inject.js exists', () => {
    const injectPath = path.join(EXTENSION_DIR, 'inject.js')
    assert(fs.existsSync(injectPath), 'inject.js should exist')
  })

  test('popup.html exists', () => {
    const popupPath = path.join(EXTENSION_DIR, 'popup.html')
    assert(fs.existsSync(popupPath), 'popup.html should exist')
  })

  test('all required icons exist', () => {
    const manifest = JSON.parse(fs.readFileSync(path.join(EXTENSION_DIR, 'manifest.json'), 'utf8'))

    const iconPaths = Object.values(manifest.icons)
    for (const iconPath of iconPaths) {
      const fullPath = path.join(EXTENSION_DIR, iconPath)
      assert(fs.existsSync(fullPath), `Icon should exist: ${iconPath}`)
    }
  })
})

describe('Focused Module Signatures', () => {
  test('settings storage owns async configuration reads', () => {
    const tabStatePath = path.join(EXTENSION_DIR, 'background/ui/settings-storage.js')
    const content = fs.readFileSync(tabStatePath, 'utf8')

    assert(content.includes('getAllConfigSettings'), 'Should export getAllConfigSettings')
    assert(content.includes('async function getAllConfigSettings'), 'Configuration reads should be Promise-based')
  })

  test('tracked-tab state owns tracked-tab reads', () => {
    const tabStatePath = path.join(EXTENSION_DIR, 'background/ui/tracked-tab-state.js')
    const content = fs.readFileSync(tabStatePath, 'utf8')

    assert(content.includes('getTrackedTabInfo'), 'Should export getTrackedTabInfo')
  })
})

describe('Module Import Chain', () => {
  test('stream runtime imports its focused owners', () => {
    const indexPath = path.join(EXTENSION_DIR, 'background/orchestration/stream-runtime.js')
    const content = fs.readFileSync(indexPath, 'utf8')

    // Should import from modular subcomponents
    const expectedImports = [
      '../sync/circuit-breaker',
      '../sync/batchers',
      '../sync/log-processing',
      '../sync/screenshot',
      '../sync/server',
      '../caches/cache-limits',
      '../caches/error-groups',
      '../caches/snapshots'
    ]

    for (const importPath of expectedImports) {
      assert(
        content.includes(`from '${importPath}'`) ||
          content.includes(`from "${importPath}"`) ||
          content.includes(`from '${importPath}.js'`) ||
          content.includes(`from "${importPath}.js"`),
        `Should import from ${importPath}`
      )
    }
  })

  test('sync/server.js owns daemon transport functions', () => {
    const serverPath = path.join(EXTENSION_DIR, 'background/sync/server.js')
    const content = fs.readFileSync(serverPath, 'utf8')

    const expectedExports = ['sendLogsToServer', 'checkServerHealth', 'updateBadge']

    for (const exportName of expectedExports) {
      assert(content.includes(exportName), `Should export ${exportName}`)
    }
  })
})
