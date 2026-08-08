#!/usr/bin/env node
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import net from 'node:net'
import http from 'node:http'
import { createHash } from 'node:crypto'
import { fileURLToPath } from 'node:url'
import { spawn, spawnSync } from 'node:child_process'
import { resolveDaemonServiceName } from './daemon-health-identity.mjs'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const repoRoot = path.resolve(__dirname, '..', '..')
const isWindows = process.platform === 'win32'
const exeSuffix = isWindows ? '.exe' : ''
const supportedPreviousVersion = '0.8.1'
let cleanupHost = () => {}
let lifecycleEvidence = null
let currentStage = 'bootstrap'

process.once('SIGINT', () => {
  cleanupHost()
  process.exit(130)
})
process.once('SIGTERM', () => {
  cleanupHost()
  process.exit(143)
})

function info(message) {
  process.stdout.write(`[upgrade-regression] ${message}\n`)
}

function fail(message, details = '') {
  const text = [`[upgrade-regression] ERROR: ${message}`, details].filter(Boolean).join('\n')
  throw new Error(text)
}

function persistEvidence() {
  const evidencePath = process.env.KABOOM_UAT_EVIDENCE
  if (evidencePath && lifecycleEvidence) {
    fs.writeFileSync(evidencePath, `${JSON.stringify(lifecycleEvidence, null, 2)}\n`)
  }
}

function windowsCommandNeedsShell(cmd) {
  return isWindows && (cmd === 'npm' || cmd.toLowerCase().endsWith('.cmd'))
}

function run(cmd, args, options = {}) {
  const result = spawnSync(cmd, args, {
    cwd: repoRoot,
    encoding: 'utf8',
    shell: windowsCommandNeedsShell(cmd),
    ...options
  })
  if (result.status !== 0) {
    const rendered = [
      `$ ${cmd} ${args.join(' ')}`,
      result.stdout ? `stdout:\n${result.stdout}` : '',
      result.stderr ? `stderr:\n${result.stderr}` : ''
    ]
      .filter(Boolean)
      .join('\n')
    fail(`command failed (exit ${result.status})`, rendered)
  }
  return result
}

function tryRun(cmd, args, options = {}) {
  return spawnSync(cmd, args, {
    cwd: repoRoot,
    encoding: 'utf8',
    shell: windowsCommandNeedsShell(cmd),
    ...options
  })
}

function pidAlive(pid) {
  if (!pid || pid <= 0) return false
  try {
    process.kill(pid, 0)
    return true
  } catch {
    return false
  }
}

async function waitForPortHealth(port, timeoutMs = 20000) {
  const start = Date.now()
  while (Date.now() - start < timeoutMs) {
    try {
      const resp = await fetch(`http://127.0.0.1:${port}/health`)
      if (resp.ok) {
        return await resp.json()
      }
    } catch {
      // Keep polling.
    }
    await new Promise((resolve) => setTimeout(resolve, 100))
  }
  return null
}

async function expectDoctorIncident(port, code, correlation) {
  const response = await fetch(`http://127.0.0.1:${port}/doctor`)
  if (!response.ok) fail('Doctor incident query failed', `status=${response.status}`)
  const payload = JSON.stringify(await response.json())
  if (!payload.includes(code) || !payload.includes(correlation)) {
    fail('expected Doctor incident was absent', `code=${code} correlation=${correlation}`)
  }
}

async function waitForChildExit(child, timeoutMs = 10000) {
  if (!child) return true
  if (child.exitCode !== null || child.signalCode !== null) return true

  await new Promise((resolve, reject) => {
    const timer = setTimeout(() => {
      // On Windows, process handles can miss/lag exit events after external taskkill.
      // Treat "pid is already gone" as a successful exit to avoid false negatives.
      if (!pidAlive(child.pid)) {
        cleanup()
        resolve()
        return
      }
      cleanup()
      reject(new Error(`child ${child.pid} did not exit within ${timeoutMs}ms`))
    }, timeoutMs)

    const onExit = () => {
      cleanup()
      resolve()
    }
    const onError = (err) => {
      cleanup()
      reject(err)
    }
    const cleanup = () => {
      clearTimeout(timer)
      child.off('exit', onExit)
      child.off('error', onError)
    }

    child.on('exit', onExit)
    child.on('error', onError)
  })

  return true
}

function getFreePort() {
  return new Promise((resolve, reject) => {
    const server = net.createServer()
    server.listen(0, '127.0.0.1', () => {
      const address = server.address()
      if (!address || typeof address === 'string') {
        server.close(() => reject(new Error('failed to allocate free port')))
        return
      }
      const port = address.port
      server.close((err) => (err ? reject(err) : resolve(port)))
    })
    server.on('error', reject)
  })
}

function readVersionFile() {
  const versionPath = path.join(repoRoot, 'VERSION')
  return fs.readFileSync(versionPath, 'utf8').trim()
}

function sha256(file) {
  return createHash('sha256').update(fs.readFileSync(file)).digest('hex')
}

function binaryVersion(binaryPath, env) {
  const result = run(binaryPath, ['--version'], { env })
  const match = `${result.stdout || ''}${result.stderr || ''}`.match(/\d+\.\d+\.\d+/)
  return match ? match[0] : ''
}

function validateCandidateVersion(binaryPath, expectedVersion, expectedChecksum, env) {
  if (sha256(binaryPath) !== expectedChecksum) fail('candidate checksum changed before replacement')
  const actualVersion = binaryVersion(binaryPath, env)
  if (actualVersion !== expectedVersion) {
    fail('candidate embedded version mismatch', `expected=${expectedVersion} actual=${actualVersion || '<missing>'}`)
  }
}

function rollbackReplacement(activePath, backupPath) {
  fs.copyFileSync(backupPath, activePath)
  if (!isWindows) fs.chmodSync(activePath, 0o755)
}

async function exerciseFailedReadinessRollback(activePath, backupPath, candidatePath, expectedChecksum, env) {
  fs.copyFileSync(activePath, backupPath)
  fs.copyFileSync(candidatePath, activePath)
  validateCandidateVersion(activePath, readVersionFile(), expectedChecksum, env)

  const blockedPort = await getFreePort()
  const blocker = http.createServer((_request, response) => {
    response.writeHead(503)
    response.end('not ready')
  })
  await new Promise((resolve, reject) => {
    blocker.once('error', reject)
    blocker.listen(blockedPort, '127.0.0.1', resolve)
  })
  const rejected = startDaemon(activePath, blockedPort, env)
  const health = await waitForPortHealth(blockedPort, 500)
  await new Promise((resolve) => blocker.close(resolve))
  await waitForChildExit(rejected, 5000)
  if (health) fail('replacement unexpectedly became ready on a blocked service port')
  rollbackReplacement(activePath, backupPath)
}

function platformPackageDirectory() {
  const key = `${process.platform}-${process.arch}`
  const directories = {
    'darwin-arm64': 'darwin-arm64',
    'darwin-x64': 'darwin-x64',
    'linux-arm64': 'linux-arm64',
    'linux-x64': 'linux-x64',
    'win32-x64': 'win32-x64'
  }
  const directory = directories[key]
  if (!directory) fail(`unsupported packaged lifecycle platform ${key}`)
  return directory
}

function buildPackedArtifact(tmpRoot, version, browserBinary, hooksBinary, env) {
  const packageRoot = path.join(tmpRoot, 'packages')
  const packRoot = path.join(tmpRoot, 'packs')
  const installRoot = path.join(tmpRoot, 'install')
  const platformDir = platformPackageDirectory()
  const mainPackage = path.join(packageRoot, 'kaboom-agentic-browser')
  const platformPackage = path.join(packageRoot, platformDir)
  fs.cpSync(path.join(repoRoot, 'npm', 'kaboom-agentic-browser'), mainPackage, { recursive: true })
  fs.cpSync(path.join(repoRoot, 'extension'), path.join(mainPackage, 'extension'), { recursive: true })
  fs.cpSync(path.join(repoRoot, 'npm', platformDir), platformPackage, { recursive: true })
  fs.mkdirSync(path.join(platformPackage, 'bin'), { recursive: true })
  fs.mkdirSync(packRoot, { recursive: true })
  fs.copyFileSync(browserBinary, path.join(platformPackage, 'bin', `kaboom-agentic-browser${exeSuffix}`))
  fs.copyFileSync(hooksBinary, path.join(platformPackage, 'bin', `kaboom-hooks${exeSuffix}`))
  const platformTar = run('npm', ['pack', platformPackage, '--pack-destination', packRoot, '--silent'], {
    env
  }).stdout.trim()
  const mainTar = run('npm', ['pack', mainPackage, '--pack-destination', packRoot, '--silent'], { env }).stdout.trim()
  run(
    'npm',
    [
      'install',
      '--prefix',
      installRoot,
      '--ignore-scripts',
      '--omit=optional',
      path.join(packRoot, platformTar),
      path.join(packRoot, mainTar)
    ],
    { env }
  )
  const wrapperName = isWindows ? 'kaboom-agentic-browser.cmd' : 'kaboom-agentic-browser'
  const wrapper = path.join(installRoot, 'node_modules', '.bin', wrapperName)
  if (binaryVersion(wrapper, env) !== version) fail('packed npm wrapper did not expose the expected embedded version')
  const extensionManifest = JSON.parse(
    fs.readFileSync(
      path.join(installRoot, 'node_modules', 'kaboom-agentic-browser', 'extension', 'manifest.json'),
      'utf8'
    )
  )
  if (extensionManifest.version !== version) {
    fail('packed extension version mismatch', `expected=${version} actual=${extensionManifest.version || '<missing>'}`)
  }
  return { wrapper, mainTar: path.join(packRoot, mainTar), installRoot }
}

function readPidFile(pidFile) {
  if (!fs.existsSync(pidFile)) return 0
  const raw = fs.readFileSync(pidFile, 'utf8').trim()
  const pid = Number.parseInt(raw, 10)
  return Number.isFinite(pid) ? pid : 0
}

function startDaemon(binaryPath, port, env) {
  const child = spawn(binaryPath, ['--daemon', '--port', String(port)], {
    cwd: repoRoot,
    env,
    stdio: 'ignore'
  })
  return child
}

function stopDaemon(binaryPath, port, env) {
  // Best effort: daemon may already be down.
  tryRun(binaryPath, ['--stop', '--port', String(port)], { env, timeout: 15000 })
}

async function crashDaemon(child) {
  if (!child || !child.pid || !pidAlive(child.pid)) return
  if (isWindows) {
    tryRun('taskkill', ['/PID', String(child.pid), '/T', '/F'])
  } else {
    process.kill(child.pid, 'SIGKILL')
  }
  await waitForChildExit(child, 10000)
}

async function syncExtension(port, sessionID) {
  const response = await fetch(`http://127.0.0.1:${port}/sync`, {
    method: 'POST',
    headers: { 'content-type': 'application/json', 'x-kaboom-client': 'kaboom-extension/lifecycle-uat' },
    body: JSON.stringify({ ext_session_id: sessionID, in_progress: [] })
  })
  if (!response.ok) fail('extension lifecycle sync failed', `status=${response.status}`)
  return response.json()
}

function pickPython() {
  const candidates = isWindows ? ['python', 'py'] : ['python3', 'python']
  for (const candidate of candidates) {
    const check = tryRun(candidate, ['--version'])
    if (check.status === 0) {
      return candidate
    }
  }
  fail('python is required but was not found on PATH')
}

function writeShim(binDir, name, targetBinary) {
  const unixShimPath = path.join(binDir, name)
  const windowsShimPath = path.join(binDir, `${name}.cmd`)
  if (!isWindows) {
    fs.writeFileSync(unixShimPath, `#!/bin/sh\nexec "${targetBinary}" "$@"\n`, {
      mode: 0o755
    })
    return
  }
  fs.writeFileSync(windowsShimPath, `@echo off\r\n"${targetBinary}" %*\r\n`, { mode: 0o755 })
}

function ensureMcpRoundTrip(binaryPath, port, env) {
  const req =
    JSON.stringify({
      jsonrpc: '2.0',
      id: 1,
      method: 'tools/list'
    }) + '\n'
  const result = run(binaryPath, ['--port', String(port)], {
    cwd: repoRoot,
    env,
    encoding: 'utf8',
    timeout: 20000,
    input: req
  })
  if (result.status !== 0) {
    fail('wrapper MCP bridge invocation failed', `stdout:\n${result.stdout || ''}\nstderr:\n${result.stderr || ''}`)
  }
  if (!/"jsonrpc"\s*:\s*"2.0"/.test(result.stdout || '')) {
    fail('wrapper MCP bridge did not emit JSON-RPC response', result.stdout || '')
  }
}

async function expectDaemonIdentity(port, expectedVersion, timeoutMs = 20000) {
  const health = await waitForPortHealth(port, timeoutMs)
  if (!health) {
    fail(`daemon on port ${port} did not become healthy`)
  }

  const serviceName = resolveDaemonServiceName(health)
  if (serviceName.toLowerCase() !== 'kaboom-browser-devtools') {
    fail(
      `daemon service-name mismatch on port ${port}`,
      `expected=kaboom-browser-devtools actual=${serviceName || '<missing>'}`
    )
  }

  const runningVersion = typeof health.version === 'string' ? health.version.trim() : ''
  if (runningVersion !== expectedVersion) {
    fail(
      `daemon version mismatch on port ${port}`,
      `expected=${expectedVersion} actual=${runningVersion || '<missing>'}`
    )
  }
}

async function main() {
  const version = readVersionFile()
  lifecycleEvidence = {
    schema_version: 1,
    platform: `${process.platform}-${process.arch}`,
    version,
    artifact_sha256: null,
    replay_command: 'node scripts/release/install-upgrade-regression.mjs',
    scenarios: []
  }
  // The PyPI packaging tree may not exist in this repo; skip its stage when absent
  // (structure preserved so the stage resumes automatically if pypi/ returns).
  const pypiPackageDir = path.join(repoRoot, 'pypi', 'kaboom-agentic-browser')
  const hasPypiPackage = fs.existsSync(pypiPackageDir)
  const python = hasPypiPackage ? pickPython() : null
  // realpathSync resolves symlinked ancestors (e.g. macOS /var -> /private/var,
  // Windows 8.3 short paths). Without it, the daemon's upload-dir security check
  // rejects $HOME/kaboom-upload-dir because EvalSymlinks(dir) != dir, and the daemon
  // never serves /health. A real user's HOME has no symlinked ancestors, so this only
  // affects the temp-dir test environment.
  const tmpRoot = fs.realpathSync(fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-upgrade-regression-')))
  cleanupHost = () => fs.rmSync(tmpRoot, { recursive: true, force: true })
  const homeDir = path.join(tmpRoot, 'home')
  const stateDir = path.join(tmpRoot, 'state')
  const binDir = path.join(tmpRoot, 'bin')
  fs.mkdirSync(homeDir, { recursive: true })
  fs.mkdirSync(stateDir, { recursive: true })
  fs.mkdirSync(binDir, { recursive: true })
  const identityPath = path.join(homeDir, '.kaboom', 'install_id')
  const preferencesPath = path.join(homeDir, '.kaboom', 'preferences.json')
  fs.mkdirSync(path.dirname(identityPath), { recursive: true })
  fs.writeFileSync(identityPath, 'a1b2c3d4e5f6')
  fs.writeFileSync(preferencesPath, '{"capture_mode":"summary"}\n')
  const continuityBefore = {
    identity: fs.readFileSync(identityPath, 'utf8'),
    preferences: fs.readFileSync(preferencesPath, 'utf8')
  }

  const oldBinary = path.join(tmpRoot, `kaboom-old${exeSuffix}`)
  const newBinary = path.join(tmpRoot, `kaboom-new${exeSuffix}`)
  const hooksBinary = path.join(tmpRoot, `kaboom-hooks${exeSuffix}`)

  const envBase = {
    ...process.env,
    HOME: homeDir,
    USERPROFILE: homeDir,
    KABOOM_STATE_DIR: stateDir,
    KABOOM_TELEMETRY: 'off',
    KABOOM_RELEASES_URL: 'http://127.0.0.1:1/releases/latest'
  }

  const cmdPkg = process.env.KABOOM_CMD_PKG || './cmd/browser-agent'
  currentStage = 'build_artifacts'
  info('building old/new binaries')
  run('go', ['build', '-ldflags', `-X main.version=${supportedPreviousVersion}`, '-o', oldBinary, cmdPkg], {
    env: envBase
  })
  run('go', ['build', '-ldflags', `-X main.version=${version}`, '-o', newBinary, cmdPkg], {
    env: envBase
  })
  run('go', ['build', '-ldflags', `-X main.version=${version}`, '-o', hooksBinary, './cmd/hooks'], { env: envBase })

  info('packing and installing the npm artifact under test')
  const packed = buildPackedArtifact(tmpRoot, version, newBinary, hooksBinary, envBase)
  const candidateChecksum = sha256(newBinary)
  validateCandidateVersion(newBinary, version, candidateChecksum, envBase)
  const evidence = lifecycleEvidence
  evidence.artifact_sha256 = sha256(packed.mainTar)

  writeShim(binDir, 'kaboom-agentic-browser', packed.wrapper)
  writeShim(binDir, 'kaboom', packed.wrapper)
  writeShim(binDir, 'browser-agent', packed.wrapper)
  const envWithShims = {
    ...envBase,
    PATH: `${binDir}${path.delimiter}${process.env.PATH || ''}`
  }

  const port = await getFreePort()
  const pidFile = path.join(stateDir, 'run', `kaboom-${port}.pid`)
  cleanupHost = () => {
    stopDaemon(packed.wrapper, port, envWithShims)
    stopDaemon(oldBinary, port, envWithShims)
    fs.rmSync(tmpRoot, { recursive: true, force: true })
  }
  info(`using port ${port}`)

  let daemon = null
  try {
    currentStage = 'fresh_install_upgrade'
    info('stage 1: go wrapper version-mismatch recycle')
    fs.mkdirSync(path.dirname(pidFile), { recursive: true })
    fs.writeFileSync(pidFile, 'corrupt-service-registration')
    daemon = startDaemon(oldBinary, port, envWithShims)
    await expectDaemonIdentity(port, supportedPreviousVersion)
    if (readPidFile(pidFile) !== daemon.pid) fail('daemon did not repair the corrupt service registration')
    evidence.scenarios.push({ name: 'corrupt_service_registration_recovery', status: 'passed' })
    const oldPid = daemon.pid
    ensureMcpRoundTrip(packed.wrapper, port, envWithShims)
    await expectDaemonIdentity(port, version)
    await waitForChildExit(daemon, 12000)
    const newPid = readPidFile(pidFile)
    if (!newPid || newPid === oldPid) {
      fail(`expected respawned daemon pid after recycle`, `old=${oldPid} new=${newPid}`)
    }
    stopDaemon(packed.wrapper, port, envWithShims)
    evidence.scenarios.push({ name: 'fresh_install_upgrade', status: 'passed' })

    currentStage = 'lifecycle_recovery'
    info('stage 1a: corrupt state, extension suspension, browser restart, and daemon crash recovery')
    const doctorState = path.join(stateDir, 'doctor', 'incident-timeline.json')
    fs.mkdirSync(path.dirname(doctorState), { recursive: true })
    fs.writeFileSync(doctorState, '{corrupt lifecycle fixture')
    daemon = startDaemon(newBinary, port, envWithShims)
    await expectDaemonIdentity(port, version)
    evidence.scenarios.push({ name: 'corrupt_state_recovery', status: 'passed' })

    await syncExtension(port, 'packaged-lifecycle-session')
    await new Promise((resolve) => setTimeout(resolve, 150))
    await syncExtension(port, 'packaged-lifecycle-session')
    evidence.scenarios.push({ name: 'extension_suspension_recovery', status: 'passed' })

    stopDaemon(newBinary, port, envWithShims)
    await waitForChildExit(daemon, 12000)
    daemon = startDaemon(newBinary, port, envWithShims)
    await expectDaemonIdentity(port, version)
    await syncExtension(port, 'packaged-lifecycle-session')
    evidence.scenarios.push({ name: 'browser_restart_recovery', status: 'passed' })

    await crashDaemon(daemon)
    daemon = startDaemon(newBinary, port, envWithShims)
    await expectDaemonIdentity(port, version)
    await expectDoctorIncident(port, 'unclean_daemon_exit', `:${port}:`)
    evidence.scenarios.push({ name: 'daemon_crash_recovery', status: 'passed' })
    stopDaemon(newBinary, port, envWithShims)
    await waitForChildExit(daemon, 12000)

    currentStage = 'same_version_reinstall'
    info('stage 1b: same-version reinstall')
    run('npm', ['install', '--prefix', packed.installRoot, '--ignore-scripts', '--omit=optional', packed.mainTar], {
      env: envWithShims
    })
    validateCandidateVersion(newBinary, version, candidateChecksum, envWithShims)
    evidence.scenarios.push({ name: 'same_version_reinstall', status: 'passed' })

    currentStage = 'failed_readiness_rollback'
    info('stage 1c: failed replacement rolls back')
    const activeBinary = path.join(tmpRoot, `active-kaboom${exeSuffix}`)
    const backupBinary = path.join(tmpRoot, `active-kaboom.backup${exeSuffix}`)
    fs.copyFileSync(oldBinary, activeBinary)
    await exerciseFailedReadinessRollback(activeBinary, backupBinary, newBinary, candidateChecksum, envWithShims)
    if (binaryVersion(activeBinary, envWithShims) !== supportedPreviousVersion) {
      fail('rollback did not restore the prior executable')
    }
    evidence.scenarios.push({ name: 'failed_readiness_rollback', status: 'passed' })

    currentStage = 'npm_cleanup'
    info('stage 2: npm cleanup kills old daemon + pid file')
    daemon = startDaemon(oldBinary, port, envWithShims)
    await expectDaemonIdentity(port, supportedPreviousVersion)
    run('node', ['npm/kaboom-agentic-browser/lib/daemon/kill-daemon.js'], { env: envWithShims })
    await waitForChildExit(daemon, 12000)
    if (fs.existsSync(pidFile)) {
      fail(`npm cleanup did not remove pid file ${pidFile}`)
    }

    if (hasPypiPackage) {
      info('stage 3: pypi cleanup kills old daemon + pid file')
      daemon = startDaemon(oldBinary, port, envWithShims)
      await expectDaemonIdentity(port, supportedPreviousVersion)
      run(
        python,
        [
          '-c',
          "import os,sys;sys.path.insert(0,os.environ['KABOOM_PYPI_PATH']);from kaboom_agentic_browser import platform;platform.cleanup_old_processes()"
        ],
        {
          env: {
            ...envWithShims,
            KABOOM_PYPI_PATH: pypiPackageDir
          }
        }
      )
      await waitForChildExit(daemon, 12000)
      if (fs.existsSync(pidFile)) {
        fail(`pypi cleanup did not remove pid file ${pidFile}`)
      }
    } else {
      info('stage 3 skipped: pypi/kaboom-agentic-browser not present in this checkout')
      evidence.scenarios.push({ name: 'pypi_cleanup', status: 'skipped', reason: 'package_tree_absent' })
    }

    if (
      fs.readFileSync(identityPath, 'utf8') !== continuityBefore.identity ||
      fs.readFileSync(preferencesPath, 'utf8') !== continuityBefore.preferences
    ) {
      fail('install identity or user preferences changed during packaged lifecycle UAT')
    }
    info('all upgrade cleanup regressions passed')
    evidence.scenarios.push({ name: 'identity_continuity', status: 'passed' })

    currentStage = 'uninstall_cleanup'
    info('stage 4: uninstall removes packaged executables but preserves user state')
    stopDaemon(packed.wrapper, port, envWithShims)
    fs.rmSync(packed.installRoot, { recursive: true, force: true })
    if (fs.existsSync(packed.wrapper)) fail('uninstall left the packaged executable behind')
    if (
      fs.readFileSync(identityPath, 'utf8') !== continuityBefore.identity ||
      fs.readFileSync(preferencesPath, 'utf8') !== continuityBefore.preferences
    ) {
      fail('uninstall removed install identity or user preferences')
    }
    evidence.scenarios.push({ name: 'uninstall_cleanup', status: 'passed' })
  } finally {
    stopDaemon(packed.wrapper, port, envWithShims)
    stopDaemon(oldBinary, port, envWithShims)
    if (daemon && daemon.pid && pidAlive(daemon.pid)) {
      try {
        process.kill(daemon.pid)
      } catch {
        // Best effort cleanup.
      }
    }
    persistEvidence()
    cleanupHost()
    cleanupHost = () => {}
  }
}

main().catch((err) => {
  if (lifecycleEvidence) {
    lifecycleEvidence.failure = {
      stage: currentStage,
      message: 'Packaged lifecycle verification failed. Run replay_command locally for full diagnostics.'
    }
    persistEvidence()
  }
  cleanupHost()
  console.error(String(err && err.stack ? err.stack : err))
  process.exit(1)
})
