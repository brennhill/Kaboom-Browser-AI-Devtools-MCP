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

  test('background/index.js is recent (compiled from TypeScript)', () => {
    const indexPath = path.join(EXTENSION_DIR, 'background/index.js')
    assert(fs.existsSync(indexPath), 'background/index.js should exist')

    const stats = fs.statSync(indexPath)
    const ageMs = Date.now() - stats.mtimeMs
    const ageMinutes = ageMs / 1000 / 60

    // If this fails, run: make compile-ts
    assert(
      ageMinutes < 60,
      `background/index.js is ${Math.round(ageMinutes)} minutes old. Run 'make compile-ts' to recompile.`
    )
  })

  test('background/index.js has required exports', () => {
    const indexPath = path.join(EXTENSION_DIR, 'background/index.js')
    const content = fs.readFileSync(indexPath, 'utf8')

    // Check for critical exports
    assert(content.includes('export'), 'Should have ES6 exports')
    assert(
      content.includes('checkConnectionAndUpdate') || content.includes('initializeExtension'),
      'Should export initialization functions'
    )
  })

  test('background/index.js does not expose internal circuit-breaker batchers', () => {
    const indexPath = path.join(EXTENSION_DIR, 'background/index.js')
    const content = fs.readFileSync(indexPath, 'utf8')

    for (const name of [
      'logBatcherWithCB',
      'enhancedActionBatcherWithCB',
      'networkBodyBatcherWithCB',
      'perfBatcherWithCB'
    ]) {
      assert.doesNotMatch(
        content,
        new RegExp(`export const ${name}\\b`),
        `${name} is an internal factory detail, not a background entry-point API`
      )
    }
  })

  test('TypeScript source is not newer than compiled output', () => {
    const indexPath = path.join(EXTENSION_DIR, 'background/index.js')
    const srcDir = path.join(__dirname, '../../../src')

    if (!fs.existsSync(srcDir)) {
      // No TypeScript source, skip
      return
    }

    const compiledTime = fs.statSync(indexPath).mtimeMs

    // Check if any TypeScript file is newer
    const tsFiles = []
    function findTsFiles(dir) {
      const entries = fs.readdirSync(dir, { withFileTypes: true })
      for (const entry of entries) {
        const fullPath = path.join(dir, entry.name)
        if (entry.isDirectory()) {
          findTsFiles(fullPath)
        } else if (entry.name.endsWith('.ts')) {
          tsFiles.push(fullPath)
        }
      }
    }
    findTsFiles(srcDir)

    for (const tsFile of tsFiles) {
      const tsTime = fs.statSync(tsFile).mtimeMs
      if (tsTime > compiledTime) {
        assert.fail(`TypeScript source ${tsFile} is newer than compiled output. Run 'make compile-ts'`)
      }
    }
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
  test('tab state owns async configuration reads', () => {
    const tabStatePath = path.join(EXTENSION_DIR, 'background/ui/tab-state.js')
    const content = fs.readFileSync(tabStatePath, 'utf8')

    assert(content.includes('getAllConfigSettings'), 'Should export getAllConfigSettings')
    assert(content.includes('async function getAllConfigSettings'), 'Configuration reads should be Promise-based')
  })

  test('tab state owns tracked-tab reads', () => {
    const tabStatePath = path.join(EXTENSION_DIR, 'background/ui/tab-state.js')
    const content = fs.readFileSync(tabStatePath, 'utf8')

    assert(content.includes('getTrackedTabInfo'), 'Should export getTrackedTabInfo')
  })
})

describe('Module Import Chain', () => {
  test('background/index.js imports from submodules', () => {
    const indexPath = path.join(EXTENSION_DIR, 'background/index.js')
    const content = fs.readFileSync(indexPath, 'utf8')

    // Should import from modular subcomponents
    const expectedImports = [
      './sync/circuit-breaker',
      './sync/batchers',
      './sync/log-processing',
      './sync/screenshot',
      './sync/server',
      './caches/debug-log',
      './caches/cache-limits',
      './caches/error-groups',
      './caches/snapshots'
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
