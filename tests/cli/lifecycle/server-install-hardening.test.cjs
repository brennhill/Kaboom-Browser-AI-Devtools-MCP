const test = require('node:test')
const assert = require('node:assert')
const crypto = require('node:crypto')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')

const REPO_ROOT = path.resolve(__dirname, '../../..')
const INSTALL_SCRIPT = path.join(REPO_ROOT, 'server', 'scripts', 'install.js')
const SHIM_SCRIPT = path.join(REPO_ROOT, 'server', 'bin', 'kaboom')

// Requiring the installer must not run main() (guarded by require.main check).
const installer = require(INSTALL_SCRIPT)

function readInstallScript() {
  return fs.readFileSync(INSTALL_SCRIPT, 'utf8')
}

test('server postinstall verifies binary checksum from release checksums manifest', () => {
  const script = readInstallScript()

  assert.match(
    script,
    /releases\/download\/v\$\{VERSION\}\/checksums\.txt/,
    'install.js should fetch release checksums.txt for verification'
  )
  assert.match(
    script,
    /createHash\('sha256'\)/,
    'install.js should compute SHA-256 checksum for downloaded binary'
  )
  assert.match(
    script,
    /verifyDownloadedBinary\(/,
    'install.js should run explicit downloaded-binary verification'
  )
})

test('server postinstall validates existing daemon identity/version when port is already in use', () => {
  const script = readInstallScript()

  assert.match(
    script,
    /EXPECTED_SERVICE_NAME = 'kaboom-browser-devtools'/,
    'install.js should enforce expected health service identity'
  )
  assert.match(
    script,
    /checkServerIdentity\(port, VERSION\)/,
    'install.js should validate service identity and version before accepting in-use port'
  )
  assert.match(
    script,
    /non-matching service\/version/,
    'install.js should surface mismatch warning when port owner is not the expected daemon'
  )
})

// --- Identity-gated process cleanup (regression: blind kills by substring/port) ---

test('isKaboomServiceIdentity accepts only canonical kaboom identities', () => {
  assert.equal(installer.isKaboomServiceIdentity({ 'service-name': 'kaboom-browser-devtools' }), true)
  assert.equal(installer.isKaboomServiceIdentity({ service_name: 'kaboom-agentic-browser' }), true)
  assert.equal(installer.isKaboomServiceIdentity({ 'service-name': 'webpack-dev-server' }), false)
  assert.equal(installer.isKaboomServiceIdentity(null), false)
  assert.equal(installer.isKaboomServiceIdentity({}), false)
})

test('cleanupOldProcesses only kills ports owned by a kaboom daemon', async () => {
  if (process.platform === 'win32') return
  const calls = []
  const fakeSpawnSync = (cmd, args = []) => {
    calls.push([cmd, ...args])
    if (cmd === 'lsof') return { stdout: '4242\n' }
    return { stdout: '' }
  }

  await installer.cleanupOldProcesses({
    spawnSync: fakeSpawnSync,
    readHealth: async (port) => {
      if (port === 7890) return { 'service-name': 'kaboom-browser-devtools' }
      return { 'service-name': 'webpack-dev-server' } // unrelated dev server on 17890
    },
  })

  const killCalls = calls.filter(([cmd]) => cmd === 'kill')
  assert.deepStrictEqual(killCalls, [['kill', '-9', '4242']], 'must only kill the identified kaboom port')

  const lsofCalls = calls.filter(([cmd]) => cmd === 'lsof')
  assert.strictEqual(lsofCalls.length, 1, 'must not even enumerate pids on non-kaboom ports')
  assert.ok(lsofCalls[0].includes(':7890'))

  const pkillCalls = calls.filter(([cmd]) => cmd === 'pkill')
  assert.strictEqual(pkillCalls.length, 1, 'must use a single anchored pkill pattern')
  const pattern = pkillCalls[0][pkillCalls[0].length - 1]
  assert.match(pattern, /^kaboom-/, 'pkill pattern must anchor to canonical full binary names')
})

test('install.js no longer pkills bare process-name substrings', () => {
  const script = readInstallScript()
  assert.doesNotMatch(script, /'kaboom-agentic-browser',\s*'gasoline',\s*'browser-agent'/)
  assert.match(script, /isKaboomServiceIdentity/, 'port kills must be gated on health identity')
})

// --- Checksum manifest 404 vs checksum mismatch classification ---

test('checksum manifest fetch failure is classified as manifest-unavailable', async () => {
  await assert.rejects(
    () =>
      installer.verifyDownloadedBinary('/nonexistent-binary', 'kaboom-agentic-browser-darwin-arm64', {
        downloadText: async () => {
          throw new Error('failed to download text (404)')
        },
      }),
    (err) => err.code === 'CHECKSUM_MANIFEST_UNAVAILABLE'
  )
})

test('checksum mismatch and missing manifest entry still fail hard', async () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-checksum-'))
  const binaryPath = path.join(tmp, 'kaboom-bin')
  fs.writeFileSync(binaryPath, 'binary-bytes')

  await assert.rejects(
    () =>
      installer.verifyDownloadedBinary(binaryPath, 'kaboom-bin', {
        downloadText: async () => `${'0'.repeat(64)}  kaboom-bin\n`,
      }),
    (err) => /checksum mismatch/i.test(err.message) && err.code !== 'CHECKSUM_MANIFEST_UNAVAILABLE'
  )

  await assert.rejects(
    () =>
      installer.verifyDownloadedBinary(binaryPath, 'kaboom-bin', {
        downloadText: async () => 'abc123  some-other-binary\n',
      }),
    (err) => /missing entry/i.test(err.message) && err.code !== 'CHECKSUM_MANIFEST_UNAVAILABLE'
  )

  // Matching checksum passes.
  const goodSha = crypto.createHash('sha256').update(fs.readFileSync(binaryPath)).digest('hex')
  await installer.verifyDownloadedBinary(binaryPath, 'kaboom-bin', {
    downloadText: async () => `${goodSha}  kaboom-bin\n`,
  })

  fs.rmSync(tmp, { recursive: true, force: true })
})

test('install flow distinguishes manifest-unavailable from binary-unavailable', () => {
  const script = readInstallScript()
  assert.match(script, /CHECKSUM_MANIFEST_UNAVAILABLE/, 'main flow must branch on manifest-unavailable')
  assert.match(script, /KABOOM_REQUIRE_CHECKSUM/, 'strict mode must be controllable via env')
})

// --- server/bin/kaboom shim must launch the binary install.js actually downloads ---

test('server bin shim launches the kaboom binary name downloaded by install.js', () => {
  const shim = fs.readFileSync(SHIM_SCRIPT, 'utf8')
  assert.match(
    shim,
    /kaboom-agentic-browser-\$PLATFORM-\$ARCH/,
    'shim must reference the kaboom-agentic-browser-<plat>-<arch> binary'
  )
  assert.doesNotMatch(shim, /gasoline/, 'shim must not look for legacy gasoline binaries')
})

test('server bin shim execs the binary instead of outliving it', () => {
  // The Node launcher this replaced blocked in execFileSync, so it could not act on
  // signals aimed at the process group; when its parent died it was reparented to
  // PID 1 and never exited. Replacing the process image removes the launcher entirely.
  const shim = fs.readFileSync(SHIM_SCRIPT, 'utf8')
  assert.match(shim, /^#!\/bin\/sh\n/, 'shim must be a POSIX exec shim, not a Node launcher')
  assert.match(shim, /^exec "\$BINARY" "\$@"$/m, 'shim must exec the binary')
  const code = shim.split('\n').filter((l) => !l.trimStart().startsWith('#')).join('\n')
  assert.doesNotMatch(code, /execFileSync|spawnSync|child_process/, 'shim still uses a Node child-process API')
})
