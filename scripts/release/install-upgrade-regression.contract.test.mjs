import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const scriptPath = path.join(__dirname, 'install-upgrade-regression.mjs')
// install.sh / install.ps1 are the shipped installers and stay at scripts/ root
// (their download URLs are public); this suite lives one level down in
// scripts/release/, so reach back up for them.
const scriptsDir = path.join(__dirname, '..')
const repoRoot = path.join(__dirname, '..', '..')

test('upgrade regression script validates health service identity', () => {
  const source = fs.readFileSync(scriptPath, 'utf8')
  assert.match(source, /service-name/, 'expected service-name validation in health checks')
  assert.match(
    source,
    /kaboom-browser-devtools/i,
    'expected service identity check to enforce kaboom-browser-devtools'
  )
  assert.match(
    source,
    /KABOOM_TELEMETRY:\s*'off'/,
    'upgrade regression daemons must not emit production telemetry'
  )
})

test('shell installer uses Kaboom canonical binaries and install roots', () => {
  const source = fs.readFileSync(path.join(scriptsDir, 'install.sh'), 'utf8')
  assert.match(source, /kaboom-agentic-browser/, 'expected canonical binary name in install.sh')
  assert.match(source, /kaboom-hooks/, 'expected hooks binary name in install.sh')
  assert.match(source, /\.kaboom/, 'expected Kaboom install root in install.sh')
  assert.match(source, /KaboomAgenticDevtoolExtension/, 'expected Kaboom extension dir in install.sh')
  assert.doesNotMatch(source, /sync_binary_compat_aliases/, 'expected install.sh to stop creating legacy aliases')
})

test('powershell installer uses Kaboom canonical binaries and install roots', () => {
  const source = fs.readFileSync(path.join(scriptsDir, 'install.ps1'), 'utf8')
  assert.match(source, /kaboom-agentic-browser\.exe|kaboom\.exe/, 'expected canonical binary name in install.ps1')
  assert.match(source, /\.kaboom|KaboomAgenticDevtoolExtension/, 'expected Kaboom install roots in install.ps1')
})

test('shell installer supports --hooks-only mode', () => {
  const source = fs.readFileSync(path.join(scriptsDir, 'install.sh'), 'utf8')
  assert.match(source, /HOOKS_ONLY/, 'expected HOOKS_ONLY variable in install.sh')
  assert.match(source, /--hooks-only/, 'expected --hooks-only flag handling')
  assert.match(source, /kaboom-hooks/, 'expected kaboom-hooks binary name')
  assert.match(source, /download_and_verify/, 'expected download_and_verify helper')
})

test('shell installer downloads both binaries by default', () => {
  const source = fs.readFileSync(path.join(scriptsDir, 'install.sh'), 'utf8')
  assert.match(source, /kaboom-agentic-browser-\$PLATFORM/, 'expected main binary download')
  assert.match(source, /kaboom-hooks-\$PLATFORM/, 'expected hooks binary download')
})

test('shell installer skips extension and daemon for hooks-only', () => {
  const source = fs.readFileSync(path.join(scriptsDir, 'install.sh'), 'utf8')
  assert.match(source, /HOOKS_ONLY.*guard/, 'expected HOOKS_ONLY guard comment')
})

test('npm wrapper exposes only Kaboom commands', () => {
  const pkg = JSON.parse(
    fs.readFileSync(path.join(repoRoot, 'npm', 'kaboom-agentic-browser', 'package.json'), 'utf8')
  )
  assert.equal(pkg.bin?.['kaboom-agentic-browser'], 'bin/kaboom-agentic-browser')
  assert.equal(pkg.bin?.['kaboom-hooks'], 'bin/kaboom-hooks')
  assert.equal(pkg.bin?.['gasoline-agentic-devtools'], undefined)
  assert.equal(pkg.bin?.['gasoline-agentic-browser'], undefined)
})
