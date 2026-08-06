/**
 * @fileoverview Regression guard for installer extension staging behavior.
 * Ensures fallback pulls from STABLE branch and validates CSP-safe bootstrap files.
 */

import { test } from 'node:test'
import assert from 'node:assert'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const REPO_ROOT = path.resolve(__dirname, '../../..')
const INSTALL_SH = path.join(REPO_ROOT, 'scripts', 'setup', 'install.sh')
const INSTALL_PS1 = path.join(REPO_ROOT, 'scripts', 'setup', 'install.ps1')
const SERVER_INSTALL_JS = path.join(REPO_ROOT, 'server', 'scripts', 'install.js')

test('bash installer validates the canonical bundled layout and falls back to STABLE source zip', () => {
  const script = fs.readFileSync(INSTALL_SH, 'utf8')

  assert.match(
    script,
    /\[ -f "\$base_dir\/early-patch\.bundled\.js" \]/,
    'install.sh must require the canonical early-patch bundle'
  )
  assert.match(
    script,
    /archive\/refs\/heads\/STABLE\.zip/,
    'install.sh fallback must use STABLE branch source zip'
  )
  assert.doesNotMatch(
    script,
    /archive\/refs\/tags\/v\$VERSION\.zip/,
    'install.sh must not fall back to version tag source zip for extension staging'
  )
})

test('bash installer uses staged extension promotion and supports strict checksum mode', () => {
  const script = fs.readFileSync(INSTALL_SH, 'utf8')

  assert.match(
    script,
    /STAGE_EXT_DIR="\$INSTALL_DIR\/\.extension-stage-\$\$"/,
    'install.sh should stage extension files in a temp directory before promotion'
  )
  assert.match(
    script,
    /promote_extension_stage\(\)/,
    'install.sh should promote validated staged extension atomically'
  )
  assert.match(
    script,
    /STRICT_CHECKSUM="\$\{KABOOM_INSTALL_STRICT:-0\}"/,
    'install.sh should expose strict checksum mode via KABOOM_INSTALL_STRICT'
  )
  assert.match(
    script,
    /Strict checksum mode enabled/,
    'install.sh should surface strict checksum mode in installer output'
  )
  assert.match(
    script,
    /EXT_DIR="\$\{KABOOM_EXTENSION_DIR:-\$HOME\/KaboomAgenticDevtoolExtension\}"/,
    'install.sh should default extension install location to a visible home-directory folder'
  )
})

test('powershell installer requires the canonical bundled layout and falls back to STABLE source zip', () => {
  const script = fs.readFileSync(INSTALL_PS1, 'utf8')

  assert.match(
    script,
    /manifest\.json/,
    'install.ps1 must always require manifest.json in staged extension'
  )
  for (const file of ['background.js', 'content.bundled.js', 'inject.bundled.js', 'early-patch.bundled.js']) {
    assert.match(script, new RegExp(file.replaceAll('.', '\\.')), `install.ps1 must require ${file}`)
  }
  assert.doesNotMatch(script, /background\\init\.js|content\\script-injection\.js|inject\\index\.js|theme-bootstrap\.js/)
  assert.match(
    script,
    /archive\/refs\/heads\/STABLE\.zip/,
    'install.ps1 fallback must use STABLE branch source zip'
  )
  assert.doesNotMatch(
    script,
    /archive\/refs\/tags\/v\$VERSION\.zip/,
    'install.ps1 must not fall back to version tag source zip for extension staging'
  )
})

test('powershell installer uses unique temp paths, staged promotion, and strict checksum mode', () => {
  const script = fs.readFileSync(INSTALL_PS1, 'utf8')

  assert.match(
    script,
    /\$TEMP_TOKEN = \[Guid\]::NewGuid\(\)\.ToString\("N"\)/,
    'install.ps1 should derive unique temp token per run'
  )
  assert.match(
    script,
    /kaboom-ext-\$TEMP_TOKEN\.zip/,
    'install.ps1 should use unique extension zip temp path'
  )
  assert.match(
    script,
    /function Promote-ExtensionStage/,
    'install.ps1 should promote validated staged extension atomically'
  )
  assert.match(
    script,
    /\$STRICT_CHECKSUM = \$env:KABOOM_INSTALL_STRICT -eq "1"/,
    'install.ps1 should support strict checksum mode via KABOOM_INSTALL_STRICT'
  )
  assert.match(
    script,
    /\$EXT_DIR = if \(\$env:KABOOM_EXTENSION_DIR\) \{ \$env:KABOOM_EXTENSION_DIR \} else \{ Join-Path \$HOME "KaboomAgenticDevtoolExtension" \}/,
    'install.ps1 should default extension install location to a visible home-directory folder'
  )
})

test('powershell installer force-stops stale server and prints manual recovery warning', () => {
  const script = fs.readFileSync(INSTALL_PS1, 'utf8')

  assert.match(
    script,
    /Stop-KaboomServerProcesses/,
    'install.ps1 must include explicit server stop logic before binary replacement'
  )
  assert.match(
    script,
    /taskkill\s+\/F\s+\/PID/,
    'install.ps1 must escalate to taskkill for stubborn running processes'
  )
  assert.match(
    script,
    /INSTALL WARNING: MANUAL ACTION REQUIRED/,
    'install.ps1 must print a high-visibility warning block when server replacement fails'
  )
  assert.match(
    script,
    /Get-Process kaboom-agentic-browser -ErrorAction SilentlyContinue \| Stop-Process -Force/,
    'install.ps1 must provide manual process kill instructions'
  )
})

test('node installer checklist defaults to visible extension path and supports env override', () => {
  const script = fs.readFileSync(SERVER_INSTALL_JS, 'utf8')

  assert.match(
    script,
    /const extensionDir = process\.env\.KABOOM_EXTENSION_DIR \|\| path\.join\(os\.homedir\(\), 'KaboomAgenticDevtoolExtension'\)/,
    'server install script should print visible extension directory by default and respect KABOOM_EXTENSION_DIR override'
  )
})
