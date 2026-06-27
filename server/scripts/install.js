#!/usr/bin/env node

/**
 * Postinstall script to download the correct binary for the platform
 * Also handles cleanup of old Kaboom and legacy processes for clean upgrades
 */

const https = require('https')
const http = require('http')
const fs = require('fs')
const path = require('path')
const os = require('os')
const crypto = require('crypto')
const { spawnSync, spawn } = require('child_process')

const VERSION = require('../package.json').version
const GITHUB_REPO = 'brennhill/Kaboom-Browser-AI-Devtools-MCP'
const BINARY_NAME = 'kaboom-agentic-browser'
const EXPECTED_SERVICE_NAME = 'kaboom-browser-devtools'

// Anchored to full daemon binary names (current + legacy brands). Never use
// bare substrings like "gasoline" — they match unrelated command lines.
const DAEMON_NAME_PATTERN =
  '(kaboom|gasoline|strum)-(agentic-browser|agentic-devtools|browser-devtools|hooks|mcp)|\\.kaboom/bin/'

// Accepted /health identities for the kill path: current daemon plus legacy brand eras.
const KABOOM_SERVICE_IDENTITY_RE =
  /^(kaboom|gasoline|strum)(-browser-devtools|-agentic-browser|-agentic-devtools|-mcp)?$/

function printPanel(title, lines = []) {
  const border = '+----------------------------------------------------------+'
  console.log(border)
  const safeTitle = title.padEnd(56, ' ').slice(0, 56)
  console.log(`| ${safeTitle} |`)
  console.log(border)
  for (const line of lines) {
    const safeLine = String(line).padEnd(58, ' ').slice(0, 58)
    console.log(`| ${safeLine} |`)
  }
  console.log(border)
}

function printBanner() {
  console.log('')
  console.log('   ____                 _ _            ')
  console.log('  / ___| __ _ ___  ___ | (_)_ __   ___ ')
  console.log(" | |  _ / _` / __|/ _ \\| | | '_ \\ / _ \\")
  console.log(' | |_| | (_| \\__ \\ (_) | | | | | |  __/')
  console.log('  \\____|\\__,_|___/\\___/|_|_|_| |_|\\___|')
  console.log('')
  printPanel('KABOOM INSTALLER', [
    'Polished setup for binary + background server + extension guidance.',
    '',
    'Install flow:',
    '  1) Clean up old processes',
    '  2) Download + verify binary',
    '  3) Start local server',
    '  4) Show manual extension checklist'
  ])
}

/**
 * Check whether a /health payload identifies a Kaboom (or legacy-brand) daemon.
 * Used to gate by-port kills so unrelated dev servers are never killed.
 */
function isKaboomServiceIdentity(health) {
  if (!health) return false
  const serviceName = resolveServiceName(health).toLowerCase()
  return KABOOM_SERVICE_IDENTITY_RE.test(serviceName)
}

/**
 * Kill all running Kaboom and legacy processes to ensure clean upgrade.
 * deps: { spawnSync, readHealth } injection seams for tests.
 */
// #lizard forgives
async function cleanupOldProcesses(deps = {}) {
  const exec = deps.spawnSync || spawnSync
  const fetchHealth = deps.readHealth || readHealth

  if (process.platform === 'win32') {
    // Windows: Find and kill Kaboom and legacy processes
    try {
      for (const imagePattern of ['kaboom*', 'gasoline*', 'browser-agent*']) {
        const result = exec('tasklist', ['/FI', `IMAGENAME eq ${imagePattern}`, '/FO', 'CSV'], {
          encoding: 'utf8',
          windowsHide: true
        })
        if (result.stdout) {
          const lines = result.stdout.split('\n').slice(1) // Skip header
          for (const line of lines) {
            const match = line.match(/"([^"]+)","(\d+)"/)
            if (match) {
              const imageName = match[1]
              const pid = match[2]
              exec('taskkill', ['/F', '/PID', pid], { windowsHide: true })
              console.log(`Killed old ${imageName} process (PID: ${pid})`)
            }
          }
        }
      }
    } catch (e) {
      // Ignore errors - process might not exist
    }
  } else {
    // Unix: Find and kill Kaboom and legacy processes by full binary name only.
    try {
      // Method 1: pkill anchored to full daemon binary names (never bare substrings).
      exec('pkill', ['-f', DAEMON_NAME_PATTERN], {
        encoding: 'utf8'
      })

      // Method 2: Also check common ports (7890, 17890), but only kill the
      // listener when its /health endpoint identifies a Kaboom daemon.
      const ports = [7890, 17890]
      for (const port of ports) {
        const health = await fetchHealth(port, 800)
        if (!isKaboomServiceIdentity(health)) {
          if (health) {
            console.log(`Port ${port} is in use by a non-Kaboom service; leaving it alone`)
          }
          continue
        }
        const lsofResult = exec('lsof', ['-ti', `:${port}`], {
          encoding: 'utf8'
        })
        if (lsofResult.stdout && lsofResult.stdout.trim()) {
          const pids = lsofResult.stdout.trim().split('\n')
          for (const pid of pids) {
            if (pid) {
              exec('kill', ['-9', pid])
              console.log(`Killed Kaboom process on port ${port} (PID: ${pid})`)
            }
          }
        }
      }
    } catch (e) {
      // Ignore errors - processes might not exist
    }
  }
  return
}

/**
 * Verify the installed version matches expected
 */
function verifyVersion(binaryPath) {
  try {
    const result = spawnSync(binaryPath, ['--version'], { // nosemgrep: detect-child-process
      encoding: 'utf8',
      timeout: 5000
    })
    if (result.stdout) {
      const version = result.stdout.trim()
      if (version.includes(VERSION)) {
        console.log(`✓ Verified Kaboom version: ${version}`)
        return true
      } else {
        console.warn(`Warning: Expected version ${VERSION}, got: ${version}`)
        return false
      }
    }
  } catch (e) {
    console.warn(`Could not verify version: ${e.message}`)
  }
  return false
}

function normalizeVersion(raw) {
  return String(raw || '').trim().replace(/^v/i, '')
}

function resolveServiceName(health) {
  if (!health || typeof health !== 'object') return ''
  const dashed = typeof health['service-name'] === 'string' ? health['service-name'].trim() : ''
  if (dashed) return dashed
  return typeof health.service_name === 'string' ? health.service_name.trim() : ''
}

function readHealth(port, timeoutMs = 1000) {
  return new Promise((resolve) => {
    const req = http.request({ // nosemgrep: problem-based-packs.insecure-transport.js-node.http-request.http-request, problem-based-packs.insecure-transport.js-node.using-http-server.using-http-server -- localhost-only install-time health probe
      hostname: '127.0.0.1',
      port,
      path: '/health',
      method: 'GET',
      timeout: timeoutMs
    }, (res) => {
      let body = ''
      res.setEncoding('utf8')
      res.on('data', (chunk) => {
        body += chunk
      })
      res.on('end', () => {
        if (res.statusCode !== 200) {
          resolve(null)
          return
        }
        try {
          resolve(JSON.parse(body))
        } catch {
          resolve(null)
        }
      })
    })

    req.on('error', () => resolve(null))
    req.on('timeout', () => {
      req.destroy()
      resolve(null)
    })
    req.end()
  })
}

async function checkServerIdentity(port, expectedVersion) {
  const health = await readHealth(port, 800)
  if (!health) return false

  const serviceName = resolveServiceName(health).toLowerCase()
  if (serviceName !== EXPECTED_SERVICE_NAME) return false

  const runningVersion = normalizeVersion(health.version)
  return runningVersion === normalizeVersion(expectedVersion)
}

function downloadText(url) {
  return new Promise((resolve, reject) => {
    const request = (currentUrl) => {
      https
        .get(currentUrl, (response) => {
          if (response.statusCode === 301 || response.statusCode === 302) {
            request(response.headers.location)
            return
          }
          if (response.statusCode !== 200) {
            reject(new Error(`failed to download text (${response.statusCode})`))
            return
          }

          let data = ''
          response.setEncoding('utf8')
          response.on('data', (chunk) => {
            data += chunk
          })
          response.on('end', () => resolve(data))
        })
        .on('error', reject)
    }

    request(url)
  })
}

function extractExpectedChecksum(checksumManifest, binaryName) {
  const lines = String(checksumManifest || '').split('\n')
  for (const line of lines) {
    const trimmed = line.trim()
    if (!trimmed || trimmed.startsWith('#')) continue
    const parts = trimmed.split(/\s+/)
    if (parts.length < 2) continue
    const fileName = parts[parts.length - 1]
    if (fileName === binaryName) {
      return parts[0].toLowerCase()
    }
  }
  return ''
}

function sha256File(filePath) {
  const hash = crypto.createHash('sha256')
  // eslint-disable-next-line security/detect-non-literal-fs-filename -- installer paths derived from verified platform config
  hash.update(fs.readFileSync(filePath))
  return hash.digest('hex').toLowerCase()
}

function isStrictChecksumMode() {
  const raw = String(process.env.KABOOM_REQUIRE_CHECKSUM || '').trim().toLowerCase()
  return raw === '1' || raw === 'true' || raw === 'yes' || raw === 'on'
}

async function verifyDownloadedBinary(binaryPath, binaryName, deps = {}) {
  const fetchText = deps.downloadText || downloadText
  const checksumUrl = `https://github.com/${GITHUB_REPO}/releases/download/v${VERSION}/checksums.txt`
  let manifest
  try {
    manifest = await fetchText(checksumUrl)
  } catch (err) {
    // The binary itself downloaded fine — a missing checksums.txt is a
    // manifest problem, not a binary-availability problem.
    const unavailable = new Error(`checksum manifest unavailable (${checksumUrl}): ${err.message}`)
    unavailable.code = 'CHECKSUM_MANIFEST_UNAVAILABLE'
    throw unavailable
  }
  const expected = extractExpectedChecksum(manifest, binaryName)
  if (!expected) {
    throw new Error(`checksums.txt missing entry for ${binaryName}`)
  }
  const actual = sha256File(binaryPath)
  if (expected !== actual) {
    throw new Error(`checksum mismatch for ${binaryName}`)
  }
  console.log(`✓ Checksum verified for ${binaryName}`)
}

function installStagedBinary(stagedPath, livePath) {
  // eslint-disable-next-line security/detect-non-literal-fs-filename -- installer paths derived from verified platform config
  fs.rmSync(livePath, { force: true })
  try {
    // eslint-disable-next-line security/detect-non-literal-fs-filename -- installer paths derived from verified platform config
    fs.renameSync(stagedPath, livePath)
  } catch {
    // Cross-device fallback.
    // eslint-disable-next-line security/detect-non-literal-fs-filename -- installer paths derived from verified platform config
    fs.copyFileSync(stagedPath, livePath)
    // eslint-disable-next-line security/detect-non-literal-fs-filename -- installer paths derived from verified platform config
    fs.rmSync(stagedPath, { force: true })
  }
}

/**
 * Start the Kaboom server in the background
 * Returns true if server started successfully
 */
function autoStartServer(binaryPath, port = 7890) {
  return new Promise((resolve) => {
    console.log(`Starting Kaboom server on port ${port}...`)

    // Check if port is already in use
    const testServer = http.createServer() // nosemgrep: problem-based-packs.insecure-transport.js-node.using-http-server.using-http-server -- localhost-only health check, no sensitive data
    testServer.once('error', (err) => {
      if (err.code === 'EADDRINUSE') {
        checkServerIdentity(port, VERSION)
          .then((ok) => {
            if (ok) {
              console.log(`Port ${port} already in use by Kaboom ${VERSION}`)
              resolve(true)
              return
            }
            console.warn(`Port ${port} is in use by a non-matching service/version`)
            resolve(false)
          })
          .catch(() => resolve(false))
      } else {
        console.warn(`Port check failed: ${err.message}`)
        resolve(false)
      }
    })

    testServer.once('listening', () => {
      testServer.close(() => {
        // Port is free, start the server
        try {
          // Spawn detached process that survives npm exit
          const child = spawn(binaryPath, ['--port', String(port)], { // nosemgrep: detect-child-process
            detached: true,
            stdio: ['pipe', 'ignore', 'ignore'], // pipe stdin to keep it open
            windowsHide: true
          })

          // Unref so npm can exit
          child.unref()

          // Wait for server to be ready
          let attempts = 0
          const maxAttempts = 30 // 3 seconds
          const checkHealth = async () => {
            attempts++
            const ok = await checkServerIdentity(port, VERSION)
            if (ok) {
              console.log(`✓ Server started on http://127.0.0.1:${port}`)
              resolve(true)
            } else if (attempts < maxAttempts) {
              setTimeout(checkHealth, 100)
            } else {
              console.warn('Server failed identity/version validation within startup window')
              resolve(false)
            }
          }

          // Start health checking after a brief delay
          setTimeout(checkHealth, 100)

        } catch (e) {
          console.warn(`Failed to start server: ${e.message}`)
          resolve(false)
        }
      })
    })

    testServer.listen(port, '127.0.0.1')
  })
}

// Map Node.js platform/arch to binary names
function getBinaryName() {
  const platform = process.platform
  const arch = process.arch

  const platformMap = {
    darwin: 'darwin',
    linux: 'linux',
    win32: 'win32',
  }

  const archMap = {
    x64: 'x64',
    arm64: 'arm64',
  }

  // eslint-disable-next-line security/detect-object-injection -- bracket access on platform config object
  const p = platformMap[platform]
  // eslint-disable-next-line security/detect-object-injection -- bracket access on platform config object
  const a = archMap[arch]

  if (!p || !a) {
    console.error(`Unsupported platform: ${platform}-${arch}`)
    // eslint-disable-next-line n/no-process-exit -- installer exits with error status on platform detection failure
    process.exit(1)
  }

  const ext = platform === 'win32' ? '.exe' : ''
  return `${BINARY_NAME}-${p}-${a}${ext}`
}

// Download file from URL
function download(url, dest) {
  return new Promise((resolve, reject) => {
    // eslint-disable-next-line security/detect-non-literal-fs-filename -- installer paths derived from verified platform config
    const file = fs.createWriteStream(dest)

    const request = (url) => {
      https
        .get(url, (response) => {
          // Handle redirects
          if (response.statusCode === 301 || response.statusCode === 302) {
            request(response.headers.location)
            return
          }

          if (response.statusCode !== 200) {
            reject(new Error(`Failed to download: ${response.statusCode}`))
            return
          }

          response.pipe(file)
          file.on('finish', () => {
            file.close()
            resolve()
          })
        })
        .on('error', (err) => {
          // eslint-disable-next-line security/detect-non-literal-fs-filename -- installer paths derived from verified platform config
          fs.unlink(dest, () => { /* noop — best-effort cleanup */ })
          reject(err)
        })
    }

    request(url)
  })
}

async function main() {
  printBanner()

  const binDir = path.join(__dirname, '..', 'bin')
  const binaryName = getBinaryName()
  const binaryPath = path.join(binDir, binaryName)
  const stagedBinaryPath = path.join(binDir, `${binaryName}.tmp-${Date.now()}`)

  // Clean up any old Kaboom and legacy processes before installing new version
  console.log('Cleaning up old Kaboom and legacy processes...')
  await cleanupOldProcesses()

  // Ensure bin directory exists
  if (!fs.existsSync(binDir)) {
    fs.mkdirSync(binDir, { recursive: true })
  }

  // Download URL
  const downloadUrl = `https://github.com/${GITHUB_REPO}/releases/download/v${VERSION}/${binaryName}`

  console.log(`Downloading Kaboom binary for ${process.platform}-${process.arch}...`)
  console.log(`URL: ${downloadUrl}`)

  let usedLocalBinary = false
  let checksumSkipped = false
  try {
    await download(downloadUrl, stagedBinaryPath)
    await verifyDownloadedBinary(stagedBinaryPath, binaryName)
  } catch (err) {
    if (err && err.code === 'CHECKSUM_MANIFEST_UNAVAILABLE') {
      // The binary downloaded fine — only the checksums.txt manifest is missing.
      if (isStrictChecksumMode()) {
        console.error('Binary downloaded, but the release checksum manifest (checksums.txt) is unavailable.')
        console.error('KABOOM_REQUIRE_CHECKSUM is set, so the install cannot be verified. Aborting.')
        throw err
      }
      checksumSkipped = true
      console.warn(`Warning: ${err.message}`)
      console.warn('Binary downloaded successfully, but the checksum manifest could not be fetched.')
      console.warn('Continuing without checksum verification (set KABOOM_REQUIRE_CHECKSUM=1 to enforce).')
    } else if (/checksum/i.test(err.message || '')) {
      throw err
    } else {
      // If download fails, check if we're in development (binary might be local)
      console.warn(`Download failed: ${err.message}`)
      console.warn('Checking for local binary...')

      // Look for local build
      const localBinary = path.join(__dirname, '..', '..', 'dist', binaryName)
      // eslint-disable-next-line security/detect-non-literal-fs-filename -- installer paths derived from verified platform config
      if (fs.existsSync(localBinary)) {
        console.log('Using local binary from dist/')
        fs.copyFileSync(localBinary, stagedBinaryPath)
        usedLocalBinary = true
      } else {
        console.error('')
        console.error('╔════════════════════════════════════════════════════════════════╗')
        console.error('║  KABOOM BINARY NOT AVAILABLE                                   ║')
        console.error('╚════════════════════════════════════════════════════════════════╝')
        console.error('')
        console.error('No pre-built binary found for your platform.')
        console.error('')
        console.error('OPTION 1: Build from source (requires Go 1.21+)')
        console.error('  git clone https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP.git')
        console.error('  cd gasoline')
        console.error('  go build -o /usr/local/bin/kaboom-agentic-browser ./cmd/browser-agent')
        console.error('')
        console.error('OPTION 2: Run directly with Go')
        console.error('  go run ./cmd/browser-agent')
        console.error('')
        console.error('OPTION 3: Use MCP config with Go (Claude Code / Cursor)')
        console.error('  Add to .mcp.json:')
        console.error('  {"mcpServers":{"kaboom-browser-devtools":{"command":"go","args":["run","./cmd/browser-agent"]}}}')
        console.error('')
        console.error('For help: https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP#quick-start')
        console.error('')
        // eslint-disable-next-line n/no-process-exit -- CLI script exits with error status
        process.exit(1)
      }
    }
  }

  if (usedLocalBinary) {
    console.warn('⚠️  Skipping checksum verification for local dist binary fallback.')
  } else if (checksumSkipped) {
    console.warn('⚠️  Installed without checksum verification (manifest unavailable).')
  }
  installStagedBinary(stagedBinaryPath, binaryPath)
  // eslint-disable-next-line security/detect-non-literal-fs-filename -- installer temp file path is controlled by script
  fs.rmSync(stagedBinaryPath, { force: true })

  // Make binary executable (Unix only)
  if (process.platform !== 'win32') {
    // eslint-disable-next-line security/detect-non-literal-fs-filename -- installer paths derived from verified platform config
    fs.chmodSync(binaryPath, 0o755)
  }

  // bin/kaboom-agentic-browser is a Node.js launcher that works on all platforms.
  // No separate shim needed — npm's bin wiring + the node shebang handle it.

  // Verify the installed version
  verifyVersion(binaryPath)

  // Auto-start the server so extension reconnects immediately
  await autoStartServer(binaryPath)

  console.log('Kaboom installed successfully!')
  const extensionDir = process.env.KABOOM_EXTENSION_DIR || path.join(os.homedir(), 'KaboomAgenticDevtoolExtension')
  printPanel('MANUAL BROWSER CHECKLIST', [
    'The installer cannot click browser UI controls for you.',
    '',
    '1) Open chrome://extensions (or brave://extensions)',
    '2) Enable Developer mode',
    `3) Click Load unpacked and select: ${extensionDir}`,
    '4) Pin Kaboom in the toolbar (recommended)',
    '5) Open popup and click Track This Tab'
  ])
  return
}

if (require.main === module) {
  main().catch((err) => {
    console.error('Installation failed:', err.message)
    process.exit(1)
  })
}

module.exports = {
  cleanupOldProcesses,
  isKaboomServiceIdentity,
  resolveServiceName,
  verifyDownloadedBinary,
  extractExpectedChecksum,
  checkServerIdentity,
}
