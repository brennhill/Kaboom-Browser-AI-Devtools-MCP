// version-sync.test.mjs — Regression coverage for atomic release-version synchronization.
import assert from 'node:assert/strict'
import { execFileSync, spawnSync } from 'node:child_process'
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { dirname, join } from 'node:path'
import { test } from 'node:test'

const script = new URL('./version-sync.mjs', import.meta.url)
const jsonTargets = [
  'package.json',
  'extension/manifest.json',
  'extension/package.json',
  'server/package.json',
  'npm/kaboom-agentic-browser/package.json',
  'npm/darwin-arm64/package.json',
  'npm/darwin-x64/package.json',
  'npm/linux-arm64/package.json',
  'npm/linux-x64/package.json',
  'npm/win32-x64/package.json',
  'packages/kaboom-ci/package.json',
  'packages/kaboom-playwright/package.json'
]

function write(root, relativePath, content) {
  const target = join(root, relativePath)
  mkdirSync(dirname(target), { recursive: true })
  writeFileSync(target, content)
}

function makeFixture({ omit = '' } = {}) {
  const root = join(tmpdir(), `kaboom-version-sync-${process.pid}-${Math.random().toString(16).slice(2)}`)
  write(root, 'VERSION', '1.2.3\n')
  for (const target of jsonTargets) {
    if (target === omit) continue
    const value = { name: target, version: '1.2.3' }
    if (target === 'npm/kaboom-agentic-browser/package.json') {
      value.optionalDependencies = {
        '@brennhill/kaboom-agentic-browser-darwin-arm64': '1.2.3'
      }
    }
    if (target === 'packages/kaboom-playwright/package.json') {
      value.dependencies = { '@anthropic/kaboom-ci': '1.2.3' }
    }
    write(root, target, `${JSON.stringify(value, null, 2)}\n`)
  }
  write(
    root,
    'package-lock.json',
    `${JSON.stringify({ name: 'kaboom', version: '1.2.3', packages: { '': { version: '1.2.3' }, 'node_modules/x': { version: '9.9.9' } } }, null, 2)}\n`
  )
  write(root, 'cmd/browser-agent/main.go', 'package main\n\nvar version = "1.2.3"\n')
  write(root, 'cmd/hooks/main.go', 'package main\n\nvar version = "1.2.3"\n')
  write(root, 'README.md', 'badge/version-1.2.3-green.svg\nCurrent version: **v1.2.3**\n')
  write(root, 'claude_skill/kaboom/SKILL.md', 'metadata:\n  version: 1.2.3\n')
  return root
}

function run(root, ...args) {
  return spawnSync(process.execPath, [script.pathname, '--root', root, ...args], {
    encoding: 'utf8'
  })
}

test('a bump updates every canonical target while preserving dependency versions', () => {
  const root = makeFixture()

  execFileSync(process.execPath, [script.pathname, '--root', root, '1.3.0'])

  assert.equal(readFileSync(join(root, 'VERSION'), 'utf8'), '1.3.0\n')
  for (const target of jsonTargets) {
    assert.equal(JSON.parse(readFileSync(join(root, target))).version, '1.3.0', target)
  }
  const lock = JSON.parse(readFileSync(join(root, 'package-lock.json')))
  assert.equal(lock.version, '1.3.0')
  assert.equal(lock.packages[''].version, '1.3.0')
  assert.equal(lock.packages['node_modules/x'].version, '9.9.9')
  assert.match(readFileSync(join(root, 'cmd/browser-agent/main.go'), 'utf8'), /"1\.3\.0"/)
  assert.match(readFileSync(join(root, 'README.md'), 'utf8'), /version-1\.3\.0-green/)
  assert.match(readFileSync(join(root, 'claude_skill/kaboom/SKILL.md'), 'utf8'), /version: 1\.3\.0/)
  assert.equal(run(root, '--check').status, 0)
})

test('sync repairs drift from VERSION without changing VERSION', () => {
  const root = makeFixture()
  const manifest = join(root, 'extension/manifest.json')
  writeFileSync(manifest, readFileSync(manifest, 'utf8').replace('1.2.3', '0.0.1'))

  assert.notEqual(run(root, '--check').status, 0)
  assert.equal(run(root, '--sync').status, 0)
  assert.equal(JSON.parse(readFileSync(manifest)).version, '1.2.3')
  assert.equal(readFileSync(join(root, 'VERSION'), 'utf8'), '1.2.3\n')
})

test('invalid input and missing targets leave every existing file untouched', () => {
  const root = makeFixture({ omit: 'server/package.json' })
  const before = readFileSync(join(root, 'VERSION'), 'utf8')

  assert.notEqual(run(root, 'not-semver').status, 0)
  assert.equal(readFileSync(join(root, 'VERSION'), 'utf8'), before)
  assert.notEqual(run(root, '1.3.0').status, 0)
  assert.equal(readFileSync(join(root, 'VERSION'), 'utf8'), before)
  assert.equal(JSON.parse(readFileSync(join(root, 'package.json'))).version, '1.2.3')
})
