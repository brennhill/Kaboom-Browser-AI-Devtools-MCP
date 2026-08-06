/**
 * @fileoverview Contract + behavioral tests for scripts/uninstall.sh and scripts/uninstall.ps1.
 * The uninstaller must reverse every artifact created by scripts/install.sh and
 * `kaboom-agentic-browser --install` (see docs/architecture/runtime/uninstall-and-cleanup.md).
 */

const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')
const { spawnSync } = require('node:child_process')

const REPO_ROOT = path.resolve(__dirname, '..', '..', '..')
const UNINSTALL_SH = path.join(REPO_ROOT, 'scripts', 'uninstall.sh')
const UNINSTALL_PS1 = path.join(REPO_ROOT, 'scripts', 'uninstall.ps1')

const KNOWN_SERVER_NAMES = [
  'kaboom-browser-devtools',
]

// ─────────────────────────────────────────────────────────────
// Static contract checks (bash)
// ─────────────────────────────────────────────────────────────

test('uninstall.sh exists and is executable', () => {
  assert.ok(fs.existsSync(UNINSTALL_SH), 'scripts/uninstall.sh must exist')
  const mode = fs.statSync(UNINSTALL_SH).mode
  assert.ok(mode & 0o111, 'scripts/uninstall.sh must be executable')
})

test('uninstall.sh uses strict shell mode and safe removal guards', () => {
  const script = fs.readFileSync(UNINSTALL_SH, 'utf8')
  assert.match(script, /set -euo pipefail/)
  assert.match(script, /safe_rm_rf/, 'must funnel recursive deletes through a guarded helper')
  assert.doesNotMatch(script, /rm -rf "?\$HOME"?\s*$/m, 'must never remove $HOME itself')
})

test('uninstall.sh covers every install.sh artifact', () => {
  const script = fs.readFileSync(UNINSTALL_SH, 'utf8')

  // Autostart registrations (install.sh section 9).
  assert.match(script, /com\.kaboom\.daemon/, 'must unload/remove the macOS LaunchAgent')
  assert.match(script, /kaboom\.service/, 'must disable/remove the systemd user service')
  assert.match(script, /kaboom\.desktop/, 'must remove the XDG autostart entry')

  // PATH registration marker written by install.sh register_path().
  assert.match(script, /# kaboom\$/, 'must strip rc-file PATH lines by their "# kaboom" marker')

  // Install/state/extension directories, including env overrides honored by installers.
  assert.match(script, /KABOOM_EXTENSION_DIR/)
  assert.match(script, /KaboomAgenticDevtoolExtension/)
  assert.match(script, /KABOOM_STATE_DIR/)
  assert.match(script, /XDG_STATE_HOME/)

  // MCP client configs written by native_install.go.
  for (const name of KNOWN_SERVER_NAMES) {
    assert.match(script, new RegExp(name), `must remove MCP server entry "${name}"`)
  }
  assert.match(script, /\.cursor\/mcp\.json/)
  assert.match(script, /windsurf\/mcp_config\.json/)
  assert.match(script, /\.gemini\/settings\.json/)
  assert.match(script, /antigravity\/mcp_config\.json/)
  assert.match(script, /opencode\.json/)
  assert.match(script, /zed\/settings\.json/)
  assert.match(script, /claude_desktop_config\.json/)
  assert.match(script, /Code\/User\/mcp\.json/)
  assert.match(script, /Code\/User\/mcp\.json" servers/, 'must clean the VS Code "servers" key')

  // Managed skills cleanup recognizes only the canonical marker.
  assert.match(script, /managed-skill/, 'must only delete marker-managed skill files')
  assert.match(script, /kaboom-managed-skill/)
  assert.doesNotMatch(script, /\b(?:gasoline|strum|legacy)\b/i)

  // Environment-only telemetry controls are not installed artifacts. The
  // uninstaller must not retain beacon or version-capture code after telemetry
  // is removed from the uninstall flow.
  assert.doesNotMatch(script, /KABOOM_TELEMETRY/)
  assert.doesNotMatch(script, /telemetry beacon/i)
  assert.doesNotMatch(script, /^\s*VERSION=/m)
})

test('uninstall.sh never deletes shared client config files outright', () => {
  const script = fs.readFileSync(UNINSTALL_SH, 'utf8')
  assert.doesNotMatch(
    script,
    /rm[^|\n]*settings\.json/,
    'shared settings files must be edited, never removed'
  )
  assert.doesNotMatch(
    script,
    /rm[^|\n]*mcp\.json/,
    'mcp config files must be edited, never removed'
  )
})

// ─────────────────────────────────────────────────────────────
// Behavioral checks (sandboxed $HOME)
// ─────────────────────────────────────────────────────────────

function writeStub(dir, name, body) {
  const p = path.join(dir, name)
  fs.writeFileSync(p, `#!/bin/sh\n${body}\n`)
  fs.chmodSync(p, 0o755)
}

function makeSandbox() {
  const home = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-uninstall-'))
  const stubBin = path.join(home, '.test-stubs')
  fs.mkdirSync(stubBin, { recursive: true })

  // Neutralize anything that could touch the real machine.
  writeStub(stubBin, 'pgrep', 'exit 1')
  writeStub(stubBin, 'pkill', 'exit 1')
  writeStub(stubBin, 'launchctl', 'exit 0')
  writeStub(stubBin, 'systemctl', 'exit 0')
  writeStub(stubBin, 'curl', 'exit 0')
  writeStub(stubBin, 'claude', `echo "$@" >> "${home}/.claude-calls"; exit 0`)

  const mk = (rel, content) => {
    const p = path.join(home, rel)
    fs.mkdirSync(path.dirname(p), { recursive: true })
    fs.writeFileSync(p, content)
  }

  // install.sh artifacts
  mk('.kaboom/bin/kaboom-agentic-browser', 'binary')
  mk('.kaboom/bin/kaboom-hooks', 'binary')
  mk('.kaboom/logs/kaboom.jsonl', '{}')
  mk('KaboomAgenticDevtoolExtension/manifest.json', '{}')
  mk('.zshrc', '# user content\nexport PATH="$HOME/.kaboom/bin:$PATH" # kaboom\nalias ll="ls -la"\n')
  mk('Library/LaunchAgents/com.kaboom.daemon.plist', '<plist/>')
  mk('.config/systemd/user/kaboom.service', '[Unit]')
  mk('.config/autostart/kaboom.desktop', '[Desktop Entry]')

  // native_install.go artifacts (MCP client configs)
  mk(
    '.cursor/mcp.json',
    JSON.stringify({ mcpServers: { 'kaboom-browser-devtools': { command: 'x' }, 'other-server': { command: 'y' } } }, null, 2)
  )
  mk(
    '.config/zed/settings.json',
    JSON.stringify({ theme: 'dark', context_servers: { 'kaboom-browser-devtools': { command: 'x' } } }, null, 2)
  )
  mk(
    '.gemini/settings.json',
    JSON.stringify({ mcpServers: { 'kaboom-browser-devtools': { command: 'x' } }, otherSetting: true }, null, 2)
  )
  const vscodeConfig = JSON.stringify(
    {
      servers: { 'kaboom-browser-devtools': { command: 'x' }, 'keep-me': { command: 'y' } },
    },
    null,
    2
  )
  mk('Library/Application Support/Code/User/mcp.json', vscodeConfig)
  mk('.config/Code/User/mcp.json', vscodeConfig)

  // Skills: managed (marker), the dedicated kaboom skill, and a user skill that must survive.
  mk('.claude/skills/kaboom/SKILL.md', '# kaboom skill')
  mk('.claude/skills/debug/SKILL.md', '<!-- kaboom-managed-skill id:debug version:1 -->\n# debug')
  mk('.claude/skills/my-custom/SKILL.md', '# mine, hands off')

  return { home, stubBin }
}

function runUninstall(home, stubBin, args) {
  const env = {
    HOME: home,
    PATH: `${stubBin}${path.delimiter}${process.env.PATH}`,
    KABOOM_TELEMETRY: 'off',
    TMPDIR: os.tmpdir(),
  }
  return spawnSync('bash', [UNINSTALL_SH, ...args], {
    env,
    encoding: 'utf8',
    stdio: ['pipe', 'pipe', 'pipe'],
    timeout: 30000,
  })
}

test('uninstall.sh --yes removes all install artifacts in a sandboxed HOME', () => {
  const { home, stubBin } = makeSandbox()
  const res = runUninstall(home, stubBin, ['--yes'])
  assert.equal(res.status, 0, `uninstall failed:\nstdout: ${res.stdout}\nstderr: ${res.stderr}`)

  const gone = (rel) => assert.ok(!fs.existsSync(path.join(home, rel)), `${rel} should be removed`)
  const kept = (rel) => assert.ok(fs.existsSync(path.join(home, rel)), `${rel} should be preserved`)

  gone('.kaboom')
  gone('KaboomAgenticDevtoolExtension')
  gone('.claude/skills/kaboom')
  gone('.claude/skills/debug')
  kept('.claude/skills/my-custom/SKILL.md')

  if (process.platform === 'darwin') {
    gone('Library/LaunchAgents/com.kaboom.daemon.plist')
  } else if (process.platform === 'linux') {
    gone('.config/systemd/user/kaboom.service')
    gone('.config/autostart/kaboom.desktop')
  }

  // PATH line removed, user content preserved.
  const zshrc = fs.readFileSync(path.join(home, '.zshrc'), 'utf8')
  assert.doesNotMatch(zshrc, /# kaboom$/m)
  assert.match(zshrc, /# user content/)
  assert.match(zshrc, /alias ll/)

  // MCP entries removed; files and unrelated entries preserved.
  const cursor = JSON.parse(fs.readFileSync(path.join(home, '.cursor/mcp.json'), 'utf8'))
  assert.equal(cursor.mcpServers['kaboom-browser-devtools'], undefined)
  assert.ok(cursor.mcpServers['other-server'], 'unrelated MCP servers must survive')

  const zed = JSON.parse(fs.readFileSync(path.join(home, '.config/zed/settings.json'), 'utf8'))
  assert.equal(zed.theme, 'dark', 'unrelated Zed settings must survive')
  assert.equal(zed.context_servers['kaboom-browser-devtools'], undefined)

  const gemini = JSON.parse(fs.readFileSync(path.join(home, '.gemini/settings.json'), 'utf8'))
  assert.equal(gemini.mcpServers['kaboom-browser-devtools'], undefined)
  assert.equal(gemini.otherSetting, true)

  // VS Code uses its canonical "servers" key.
  const vscodeRel =
    process.platform === 'darwin'
      ? 'Library/Application Support/Code/User/mcp.json'
      : '.config/Code/User/mcp.json'
  if (process.platform === 'darwin' || process.platform === 'linux') {
    const vscode = JSON.parse(fs.readFileSync(path.join(home, vscodeRel), 'utf8'))
    assert.equal(vscode.servers['kaboom-browser-devtools'], undefined)
    assert.ok(vscode.servers['keep-me'], 'unrelated VS Code servers must survive')
  }

  // Claude Code CLI removal attempted for the canonical server name.
  const claudeCalls = fs.readFileSync(path.join(home, '.claude-calls'), 'utf8')
  assert.match(claudeCalls, /mcp remove .*kaboom-browser-devtools/)

  fs.rmSync(home, { recursive: true, force: true })
})

test('uninstall.sh refuses to run non-interactively without --yes', () => {
  const { home, stubBin } = makeSandbox()
  const res = runUninstall(home, stubBin, [])
  assert.notEqual(res.status, 0, 'must exit non-zero without --yes when stdin is not a TTY')
  assert.ok(fs.existsSync(path.join(home, '.kaboom')), 'must not remove anything when aborting')
  fs.rmSync(home, { recursive: true, force: true })
})

test('uninstall.sh --dry-run removes nothing', () => {
  const { home, stubBin } = makeSandbox()
  const res = runUninstall(home, stubBin, ['--dry-run', '--yes'])
  assert.equal(res.status, 0, `dry-run failed:\nstdout: ${res.stdout}\nstderr: ${res.stderr}`)

  for (const rel of [
    '.kaboom/bin/kaboom-agentic-browser',
    'KaboomAgenticDevtoolExtension/manifest.json',
    'Library/LaunchAgents/com.kaboom.daemon.plist',
    '.claude/skills/debug/SKILL.md',
  ]) {
    assert.ok(fs.existsSync(path.join(home, rel)), `${rel} must survive a dry run`)
  }
  const zshrc = fs.readFileSync(path.join(home, '.zshrc'), 'utf8')
  assert.match(zshrc, /# kaboom$/m, 'PATH line must survive a dry run')
  const cursor = JSON.parse(fs.readFileSync(path.join(home, '.cursor/mcp.json'), 'utf8'))
  assert.ok(cursor.mcpServers['kaboom-browser-devtools'], 'MCP entries must survive a dry run')
  fs.rmSync(home, { recursive: true, force: true })
})

test('uninstall.sh --keep-data preserves state data but removes binaries', () => {
  const { home, stubBin } = makeSandbox()
  const res = runUninstall(home, stubBin, ['--keep-data', '--yes'])
  assert.equal(res.status, 0, `keep-data failed:\nstdout: ${res.stdout}\nstderr: ${res.stderr}`)

  assert.ok(!fs.existsSync(path.join(home, '.kaboom/bin')), 'binaries must be removed')
  assert.ok(fs.existsSync(path.join(home, '.kaboom/logs/kaboom.jsonl')), 'state data must be kept')
  assert.ok(!fs.existsSync(path.join(home, 'KaboomAgenticDevtoolExtension')), 'extension dir still removed')
  fs.rmSync(home, { recursive: true, force: true })
})

// ─────────────────────────────────────────────────────────────
// Static contract checks (PowerShell counterpart)
// ─────────────────────────────────────────────────────────────

test('uninstall.ps1 exists and mirrors the artifact coverage of install.ps1', () => {
  assert.ok(fs.existsSync(UNINSTALL_PS1), 'scripts/uninstall.ps1 must exist')
  const script = fs.readFileSync(UNINSTALL_PS1, 'utf8')

  assert.match(script, /\.kaboom/)
  assert.match(script, /KaboomAgenticDevtoolExtension/)
  assert.match(script, /KABOOM_EXTENSION_DIR/)
  assert.match(script, /claude_desktop_config\.json/)
  for (const name of KNOWN_SERVER_NAMES) {
    assert.match(script, new RegExp(name), `must remove MCP server entry "${name}"`)
  }
  assert.match(script, /kaboom-managed-skill/)
  assert.doesNotMatch(script, /\b(?:gasoline|strum|legacy)\b/i)
})
