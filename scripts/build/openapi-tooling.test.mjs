// openapi-tooling.test.mjs — Keeps OpenAPI drift generation hermetic in clean CI.

import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import path from 'node:path'
import test from 'node:test'
import { fileURLToPath } from 'node:url'

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..')
const packageJSON = JSON.parse(readFileSync(path.join(repoRoot, 'package.json'), 'utf8'))
const packageLock = JSON.parse(readFileSync(path.join(repoRoot, 'package-lock.json'), 'utf8'))

test('OpenAPI drift generator is exact and lockfile-pinned', () => {
  assert.equal(packageJSON.devDependencies['openapi-typescript'], '7.13.0')
  assert.equal(packageLock.packages[''].devDependencies['openapi-typescript'], '7.13.0')
  assert.equal(packageLock.packages['node_modules/openapi-typescript'].version, '7.13.0')
  assert.equal(packageJSON.overrides['openapi-typescript'].typescript, '$typescript')
  assert.equal(packageJSON.overrides['js-yaml'], '4.3.1')
  assert.equal(packageLock.packages['node_modules/js-yaml'].version, '4.3.1')
})
