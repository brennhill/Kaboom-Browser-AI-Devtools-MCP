import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { resolveDaemonServiceName } from './daemon-health-identity.mjs'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const scriptPath = path.join(__dirname, 'install-upgrade-regression.mjs')
const setupScriptsDir = path.join(__dirname, '..', 'setup')
const repoRoot = path.join(__dirname, '..', '..')

test('upgrade regression script validates health service identity', () => {
  const source = fs.readFileSync(scriptPath, 'utf8')
  assert.match(source, /resolveDaemonServiceName/, 'expected canonical health identity resolution')
  assert.match(source, /kaboom-browser-devtools/i, 'expected service identity check to enforce kaboom-browser-devtools')
  assert.match(source, /KABOOM_TELEMETRY:\s*'off'/, 'upgrade regression daemons must not emit production telemetry')
})

test('upgrade regression exercises a packed artifact with validated replacement and rollback evidence', () => {
  const source = fs.readFileSync(scriptPath, 'utf8')
  assert.match(source, /npm["']?,\s*\[?["']pack|npm pack/, 'must create the npm artifact under test')
  assert.match(source, /createHash\(["']sha256["']\)/, 'must checksum the candidate before replacement')
  assert.match(source, /validateCandidateVersion/, 'must validate the embedded candidate version before replacement')
  assert.match(source, /rollbackReplacement/, 'must restore the previous executable after failed readiness')
  assert.match(source, /same_version_reinstall/, 'must report the same-version reinstall scenario')
  assert.match(source, /identity_continuity/, 'must report preserved install identity and user state')
  assert.match(source, /KABOOM_UAT_EVIDENCE/, 'must support a machine-readable evidence destination')
  assert.match(source, /unclean_daemon_exit/, 'crash recovery must verify the canonical Doctor incident')
  assert.match(
    source,
    /\.failure\s*=\s*\{[\s\S]*stage:\s*currentStage/,
    'failure evidence must identify the replay stage'
  )
  assert.match(source, /replay_command/, 'failure evidence must retain exact replay instructions')
  for (const scenario of [
    'daemon_crash_recovery',
    'extension_suspension_recovery',
    'browser_restart_recovery',
    'corrupt_state_recovery',
    'uninstall_cleanup'
  ]) {
    assert.match(source, new RegExp(scenario), `must execute ${scenario}`)
  }
})

test('upgrade regression executes npm through the Windows command shell', () => {
  const source = fs.readFileSync(scriptPath, 'utf8')
  assert.match(
    source,
    /shell:\s*windowsCommandNeedsShell\(cmd\)/,
    'Windows cannot execute npm or generated .cmd wrappers directly through spawnSync'
  )
  assert.match(source, /cmd\s*===\s*['"]npm['"][\s\S]*endsWith\(['"]\.cmd['"]\)/)
})

test('scheduled and release platform matrices retain replayable lifecycle evidence', () => {
  const ci = fs.readFileSync(path.join(repoRoot, '.github', 'workflows', 'ci.yml'), 'utf8')
  const release = fs.readFileSync(path.join(repoRoot, '.github', 'workflows', 'release.yml'), 'utf8')
  for (const [name, workflow] of [
    ['ci', ci],
    ['release', release]
  ]) {
    assert.match(
      workflow,
      /ubuntu-latest[\s\S]*macos-latest[\s\S]*windows-latest/,
      `${name} must cover all supported OSes`
    )
    assert.match(workflow, /KABOOM_UAT_EVIDENCE/, `${name} must configure machine-readable lifecycle evidence`)
    assert.match(workflow, /upload-artifact/, `${name} must retain lifecycle evidence on failure`)
    assert.match(workflow, /install-upgrade-regression\.mjs/, `${name} must consume the canonical lifecycle scenarios`)
  }
  assert.match(ci, /schedule:/, 'CI must run the lifecycle matrix on a schedule')
})

test('upgrade regression always restores its disposable host', () => {
  const source = fs.readFileSync(scriptPath, 'utf8')
  assert.match(source, /process\.once\(["']SIGINT["']/, 'must restore the host when interrupted')
  assert.match(source, /process\.once\(["']SIGTERM["']/, 'must restore the host when terminated')
  assert.match(source, /rmSync\(tmpRoot,\s*\{\s*recursive:\s*true/, 'must remove the disposable host after UAT')
})

test('upgrade identity accepts the canonical operational health name', () => {
  assert.equal(resolveDaemonServiceName({ name: 'kaboom-browser-devtools' }), 'kaboom-browser-devtools')
  assert.equal(resolveDaemonServiceName({ service_name: 'legacy-snake' }), 'legacy-snake')
  assert.equal(resolveDaemonServiceName({ 'service-name': 'legacy-dash' }), 'legacy-dash')
  assert.equal(resolveDaemonServiceName(null), '')
})

test('shell installer uses Kaboom canonical binaries and install roots', () => {
  const source = fs.readFileSync(path.join(setupScriptsDir, 'install.sh'), 'utf8')
  assert.match(source, /kaboom-agentic-browser/, 'expected canonical binary name in install.sh')
  assert.match(source, /kaboom-hooks/, 'expected hooks binary name in install.sh')
  assert.match(source, /\.kaboom/, 'expected Kaboom install root in install.sh')
  assert.match(source, /KaboomAgenticDevtoolExtension/, 'expected Kaboom extension dir in install.sh')
  assert.doesNotMatch(source, /sync_binary_compat_aliases/, 'expected install.sh to stop creating legacy aliases')
})

test('powershell installer uses Kaboom canonical binaries and install roots', () => {
  const source = fs.readFileSync(path.join(setupScriptsDir, 'install.ps1'), 'utf8')
  assert.match(source, /kaboom-agentic-browser\.exe|kaboom\.exe/, 'expected canonical binary name in install.ps1')
  assert.match(source, /\.kaboom|KaboomAgenticDevtoolExtension/, 'expected Kaboom install roots in install.ps1')
})

test('shell installer supports --hooks-only mode', () => {
  const source = fs.readFileSync(path.join(setupScriptsDir, 'install.sh'), 'utf8')
  assert.match(source, /HOOKS_ONLY/, 'expected HOOKS_ONLY variable in install.sh')
  assert.match(source, /--hooks-only/, 'expected --hooks-only flag handling')
  assert.match(source, /kaboom-hooks/, 'expected kaboom-hooks binary name')
  assert.match(source, /download_and_verify/, 'expected download_and_verify helper')
})

test('shell installer downloads both binaries by default', () => {
  const source = fs.readFileSync(path.join(setupScriptsDir, 'install.sh'), 'utf8')
  assert.match(source, /kaboom-agentic-browser-\$PLATFORM/, 'expected main binary download')
  assert.match(source, /kaboom-hooks-\$PLATFORM/, 'expected hooks binary download')
})

test('shell installer skips extension and daemon for hooks-only', () => {
  const source = fs.readFileSync(path.join(setupScriptsDir, 'install.sh'), 'utf8')
  assert.match(source, /HOOKS_ONLY.*guard/, 'expected HOOKS_ONLY guard comment')
})

test('npm wrapper exposes only Kaboom commands', () => {
  const pkg = JSON.parse(fs.readFileSync(path.join(repoRoot, 'npm', 'kaboom-agentic-browser', 'package.json'), 'utf8'))
  assert.equal(pkg.bin?.['kaboom-agentic-browser'], 'bin/kaboom-agentic-browser')
  assert.equal(pkg.bin?.['kaboom-hooks'], 'bin/kaboom-hooks')
  assert.equal(pkg.bin?.['gasoline-agentic-devtools'], undefined)
  assert.equal(pkg.bin?.['gasoline-agentic-browser'], undefined)
})
