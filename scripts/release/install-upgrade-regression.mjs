#!/usr/bin/env node
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import net from 'node:net'
import { fileURLToPath } from 'node:url'
import { spawn, spawnSync } from 'node:child_process'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const repoRoot = path.resolve(__dirname, '..', '..')
const isWindows = process.platform === 'win32'
const exeSuffix = isWindows ? '.exe' : ''

function info(message) {
  process.stdout.write(`[upgrade-regression] ${message}\n`)
}

function fail(message, details = '') {
  const text = [`[upgrade-regression] ERROR: ${message}`, details].filter(Boolean).join('\n')
  throw new Error(text)
}

function run(cmd, args, options = {}) {
  const result = spawnSync(cmd, args, {
    cwd: repoRoot,
    encoding: 'utf8',
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
  const result = spawnSync(binaryPath, ['--port', String(port)], {
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

function resolveServiceName(health) {
  if (!health || typeof health !== 'object') {
    return ''
  }
  const dashed = typeof health['service-name'] === 'string' ? health['service-name'].trim() : ''
  if (dashed) {
    return dashed
  }
  return typeof health.service_name === 'string' ? health.service_name.trim() : ''
}

async function expectDaemonIdentity(port, expectedVersion, timeoutMs = 20000) {
  const health = await waitForPortHealth(port, timeoutMs)
  if (!health) {
    fail(`daemon on port ${port} did not become healthy`)
  }

  const serviceName = resolveServiceName(health)
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
  const tmpRoot = fs.realpathSync(
    fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-upgrade-regression-'))
  )
  const homeDir = path.join(tmpRoot, 'home')
  const binDir = path.join(tmpRoot, 'bin')
  fs.mkdirSync(homeDir, { recursive: true })
  fs.mkdirSync(binDir, { recursive: true })

  const oldBinary = path.join(tmpRoot, `kaboom-old${exeSuffix}`)
  const newBinary = path.join(tmpRoot, `kaboom-new${exeSuffix}`)

  const envBase = {
    ...process.env,
    HOME: homeDir,
    USERPROFILE: homeDir,
    KABOOM_RELEASES_URL: 'http://127.0.0.1:1/releases/latest'
  }

  const cmdPkg = process.env.KABOOM_CMD_PKG || './cmd/browser-agent'
  info('building old/new binaries')
  run('go', ['build', '-ldflags', '-X main.version=0.0.1', '-o', oldBinary, cmdPkg], {
    env: envBase
  })
  run('go', ['build', '-ldflags', `-X main.version=${version}`, '-o', newBinary, cmdPkg], {
    env: envBase
  })

  writeShim(binDir, 'kaboom-agentic-browser', newBinary)
  writeShim(binDir, 'kaboom', newBinary)
  writeShim(binDir, 'browser-agent', newBinary)
  const envWithShims = {
    ...envBase,
    PATH: `${binDir}${path.delimiter}${process.env.PATH || ''}`
  }

  const port = await getFreePort()
  const pidFile = path.join(homeDir, '.kaboom', 'run', `kaboom-${port}.pid`)
  info(`using port ${port}`)

  let daemon = null
  try {
    info('stage 1: go wrapper version-mismatch recycle')
    daemon = startDaemon(oldBinary, port, envWithShims)
    await expectDaemonIdentity(port, '0.0.1')
    const oldPid = daemon.pid
    ensureMcpRoundTrip(newBinary, port, envWithShims)
    await expectDaemonIdentity(port, version)
    await waitForChildExit(daemon, 12000)
    const newPid = readPidFile(pidFile)
    if (!newPid || newPid === oldPid) {
      fail(`expected respawned daemon pid after recycle`, `old=${oldPid} new=${newPid}`)
    }
    stopDaemon(newBinary, port, envWithShims)

    info('stage 2: npm cleanup kills old daemon + pid file')
    daemon = startDaemon(oldBinary, port, envWithShims)
    await expectDaemonIdentity(port, '0.0.1')
    run('node', ['npm/kaboom-agentic-browser/lib/kill-daemon.js'], { env: envWithShims })
    await waitForChildExit(daemon, 12000)
    if (fs.existsSync(pidFile)) {
      fail(`npm cleanup did not remove pid file ${pidFile}`)
    }

    if (hasPypiPackage) {
      info('stage 3: pypi cleanup kills old daemon + pid file')
      daemon = startDaemon(oldBinary, port, envWithShims)
      await expectDaemonIdentity(port, '0.0.1')
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
    }

    info('all upgrade cleanup regressions passed')
  } finally {
    stopDaemon(newBinary, port, envWithShims)
    stopDaemon(oldBinary, port, envWithShims)
    if (daemon && daemon.pid && pidAlive(daemon.pid)) {
      try {
        process.kill(daemon.pid)
      } catch {
        // Best effort cleanup.
      }
    }
  }
}

main().catch((err) => {
  console.error(String(err && err.stack ? err.stack : err))
  process.exit(1)
})
