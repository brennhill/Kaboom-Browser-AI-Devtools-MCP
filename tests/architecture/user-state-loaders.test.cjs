// user-state-loaders.test.cjs — Prevents unregistered persisted user-state readers.

const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const test = require('node:test')

const root = path.resolve(__dirname, '../..')
const manifestPath = path.join(__dirname, 'user-state-loaders.json')
const readMarkers = [/\bos\.ReadFile\s*\(/, /\.Load\s*\(/, /\breadFile\s*\(/]

function productionGoFiles(target) {
  const absolute = path.join(root, target)
  if (fs.statSync(absolute).isFile()) return [target]
  return fs
    .readdirSync(absolute, { withFileTypes: true })
    .filter((entry) => entry.isFile() && entry.name.endsWith('.go') && !entry.name.endsWith('_test.go'))
    .map((entry) => path.posix.join(target, entry.name))
}

test('every persisted user-state reader has a fallback and Doctor owner', () => {
  const manifest = JSON.parse(fs.readFileSync(manifestPath, 'utf8'))
  assert.equal(manifest.extension_contract, 'tests/extension/state-recovery/state-recovery-contract.test.js')
  assert.ok(fs.existsSync(path.join(root, manifest.extension_contract)))

  const registered = new Set()
  for (const loader of manifest.go_loaders) {
    assert.equal(typeof loader.owner, 'string')
    assert.ok(loader.owner.length > 0, `${loader.path}: missing owner`)
    assert.equal(typeof loader.fallback, 'string')
    assert.ok(loader.fallback.length > 0, `${loader.path}: missing fallback`)
    assert.equal(typeof loader.doctor, 'string')
    assert.ok(loader.doctor.length > 0, `${loader.path}: missing Doctor diagnostic`)
    assert.ok(fs.existsSync(path.join(root, loader.path)), `${loader.path}: file does not exist`)
    if (loader.scan !== false) registered.add(loader.path)
  }

  const discovered = manifest.go_scan_roots
    .flatMap(productionGoFiles)
    .filter((file) => readMarkers.some((marker) => marker.test(fs.readFileSync(path.join(root, file), 'utf8'))))

  assert.deepEqual([...new Set(discovered)].sort(), [...registered].sort())
})
