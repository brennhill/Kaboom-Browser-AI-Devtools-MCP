// Purpose: Validate config-based whole-server MCP tool auto-approval writers.
// Why: Wrong permission/trust fields silently break a user's config; these lock
// the verified per-client mechanisms and the merge-safe round-trip.
// Docs: docs/features/feature/enhanced-cli-config/index.md

const test = require('node:test');
const assert = require('node:assert/strict');
const os = require('node:os');
const path = require('node:path');
const fs = require('node:fs');

const {
  KABOOM_TOOL_NAMES,
  CLAUDE_ALLOW_RULE,
  OPENCODE_PERMISSION_KEY,
  applyToConfig,
  removeFromConfig,
  autoApprovePresent,
  isSameFileKind,
  applyClaudeSettingsAllow,
  removeClaudeSettingsAllow,
} = require('./auto-approve');
const { getClientById } = require('./config');
const { installToClient } = require('./install');
const { uninstallFromClient } = require('./uninstall');

const SERVER = 'kaboom-browser-devtools';

function tmpdir() {
  return fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-autoapprove-'));
}

// --- constants ---

test('CLAUDE_ALLOW_RULE is the bare server rule (approves ALL tools of the server)', () => {
  assert.equal(CLAUDE_ALLOW_RULE, 'mcp__kaboom-browser-devtools');
});

test('OPENCODE_PERMISSION_KEY wildcards the whole server with the underscore form', () => {
  assert.equal(OPENCODE_PERMISSION_KEY, 'kaboom-browser-devtools_*');
});

test('KABOOM_TOOL_NAMES lists exactly the five MCP tools', () => {
  assert.deepEqual(KABOOM_TOOL_NAMES, ['observe', 'generate', 'configure', 'interact', 'analyze']);
});

// --- applyToConfig / removeFromConfig (pure) ---

test('applyToConfig gemini-trust sets trust:true on the server entry only', () => {
  const def = { autoApprove: { kind: 'gemini-trust' }, configKey: 'mcpServers' };
  const cfg = { mcpServers: { [SERVER]: { command: 'x', args: [] }, other: { command: 'y' } } };
  assert.equal(applyToConfig(def, cfg), true);
  assert.equal(cfg.mcpServers[SERVER].trust, true);
  assert.equal(cfg.mcpServers.other.trust, undefined, 'other servers untouched');
});

test('applyToConfig opencode-permission adds the wildcard allow and preserves existing permissions', () => {
  const def = { autoApprove: { kind: 'opencode-permission' } };
  const cfg = { mcp: { [SERVER]: {} }, permission: { bash: 'ask' } };
  assert.equal(applyToConfig(def, cfg), true);
  assert.equal(cfg.permission[OPENCODE_PERMISSION_KEY], 'allow');
  assert.equal(cfg.permission.bash, 'ask', 'existing permission keys preserved');
});

test('applyToConfig zed-tool-permissions enumerates all five tools and preserves user agent settings', () => {
  const def = { autoApprove: { kind: 'zed-tool-permissions' } };
  const cfg = { agent: { play_sound_when_agent_done: true } };
  assert.equal(applyToConfig(def, cfg), true);
  const tools = cfg.agent.tool_permissions.tools;
  for (const t of KABOOM_TOOL_NAMES) {
    assert.deepEqual(tools[`mcp:${SERVER}:${t}`], { default: 'allow' });
  }
  assert.equal(cfg.agent.play_sound_when_agent_done, true, 'user agent settings preserved');
});

test('applyToConfig zed-tool-permissions refuses to clobber a non-object agent', () => {
  const def = { autoApprove: { kind: 'zed-tool-permissions' } };
  const cfg = { agent: 'not-an-object' };
  assert.equal(applyToConfig(def, cfg), false);
  assert.equal(cfg.agent, 'not-an-object', 'malformed value left intact');
});

test('removeFromConfig opencode-permission removes the key and prunes an emptied permission map', () => {
  const def = { autoApprove: { kind: 'opencode-permission' } };
  const cfg = { permission: { [OPENCODE_PERMISSION_KEY]: 'allow' } };
  assert.equal(removeFromConfig(def, cfg), true);
  assert.equal(cfg.permission, undefined, 'empty permission map pruned');
});

test('removeFromConfig opencode-permission keeps other permission keys', () => {
  const def = { autoApprove: { kind: 'opencode-permission' } };
  const cfg = { permission: { [OPENCODE_PERMISSION_KEY]: 'allow', edit: 'deny' } };
  assert.equal(removeFromConfig(def, cfg), true);
  assert.equal(cfg.permission[OPENCODE_PERMISSION_KEY], undefined);
  assert.equal(cfg.permission.edit, 'deny');
});

test('removeFromConfig zed-tool-permissions removes tool refs and prunes empty containers', () => {
  const def = { autoApprove: { kind: 'zed-tool-permissions' } };
  const cfg = {};
  applyToConfig(def, cfg);
  assert.equal(removeFromConfig(def, cfg), true);
  assert.equal(cfg.agent, undefined, 'agent container pruned when we created it');
});

test('removeFromConfig zed-tool-permissions keeps user agent settings and other tool perms', () => {
  const def = { autoApprove: { kind: 'zed-tool-permissions' } };
  const cfg = {
    agent: {
      play_sound_when_agent_done: true,
      tool_permissions: { tools: { 'mcp:other:foo': { default: 'confirm' } } },
    },
  };
  applyToConfig(def, cfg);
  assert.equal(removeFromConfig(def, cfg), true);
  assert.equal(cfg.agent.play_sound_when_agent_done, true);
  assert.deepEqual(cfg.agent.tool_permissions.tools, { 'mcp:other:foo': { default: 'confirm' } });
});

test('autoApprovePresent detects opencode and zed auto-approve keys', () => {
  const oc = { autoApprove: { kind: 'opencode-permission' } };
  assert.equal(autoApprovePresent(oc, { permission: { [OPENCODE_PERMISSION_KEY]: 'allow' } }), true);
  assert.equal(autoApprovePresent(oc, { permission: {} }), false);
  const zed = { autoApprove: { kind: 'zed-tool-permissions' } };
  const cfg = {};
  applyToConfig(zed, cfg);
  assert.equal(autoApprovePresent(zed, cfg), true);
  assert.equal(autoApprovePresent(zed, {}), false);
});

test('isSameFileKind is true only for the JSON same-file kinds', () => {
  assert.equal(isSameFileKind({ autoApprove: { kind: 'gemini-trust' } }), true);
  assert.equal(isSameFileKind({ autoApprove: { kind: 'opencode-permission' } }), true);
  assert.equal(isSameFileKind({ autoApprove: { kind: 'zed-tool-permissions' } }), true);
  assert.equal(isSameFileKind({ autoApprove: { kind: 'claude-settings' } }), false);
  assert.equal(isSameFileKind({ autoApprove: { kind: 'ui-only' } }), false);
  assert.equal(isSameFileKind({}), false);
});

// --- Claude Code settings.json permissions.allow ---

test('applyClaudeSettingsAllow creates the file and adds the bare server allow rule', () => {
  const tmp = tmpdir();
  const settingsPath = path.join(tmp, '.claude', 'settings.json');
  const r = applyClaudeSettingsAllow({ settingsPath });
  assert.equal(r.status, 'applied');
  assert.equal(r.changed, true);
  const data = JSON.parse(fs.readFileSync(settingsPath, 'utf8'));
  assert.deepEqual(data.permissions.allow, ['mcp__kaboom-browser-devtools']);
  fs.rmSync(tmp, { recursive: true, force: true });
});

test('applyClaudeSettingsAllow merges into existing settings, dedupes, preserves other rules', () => {
  const tmp = tmpdir();
  const settingsPath = path.join(tmp, 'settings.json');
  fs.writeFileSync(settingsPath, JSON.stringify({
    model: 'opus',
    permissions: { allow: ['Bash(ls:*)', 'mcp__kaboom-browser-devtools'], deny: ['Read(./secrets/**)'] },
  }));
  const r = applyClaudeSettingsAllow({ settingsPath });
  assert.equal(r.status, 'unchanged', 'already present => no duplicate');
  const data = JSON.parse(fs.readFileSync(settingsPath, 'utf8'));
  assert.deepEqual(
    data.permissions.allow.filter((x) => x === 'mcp__kaboom-browser-devtools'),
    ['mcp__kaboom-browser-devtools'],
    'no duplicate rule'
  );
  assert.ok(data.permissions.allow.includes('Bash(ls:*)'), 'user allow rules preserved');
  assert.deepEqual(data.permissions.deny, ['Read(./secrets/**)'], 'deny rules preserved');
  assert.equal(data.model, 'opus', 'unrelated settings preserved');
  fs.rmSync(tmp, { recursive: true, force: true });
});

test('applyClaudeSettingsAllow appends without dropping existing allow entries', () => {
  const tmp = tmpdir();
  const settingsPath = path.join(tmp, 'settings.json');
  fs.writeFileSync(settingsPath, JSON.stringify({ permissions: { allow: ['Bash(git:*)'] } }));
  applyClaudeSettingsAllow({ settingsPath });
  const data = JSON.parse(fs.readFileSync(settingsPath, 'utf8'));
  assert.deepEqual(data.permissions.allow, ['Bash(git:*)', 'mcp__kaboom-browser-devtools']);
  fs.rmSync(tmp, { recursive: true, force: true });
});

test('applyClaudeSettingsAllow fails loud on malformed JSON (does not clobber)', () => {
  const tmp = tmpdir();
  const settingsPath = path.join(tmp, 'settings.json');
  fs.writeFileSync(settingsPath, '{ this is not json ');
  assert.throws(() => applyClaudeSettingsAllow({ settingsPath }));
  // File left untouched.
  assert.equal(fs.readFileSync(settingsPath, 'utf8'), '{ this is not json ');
  fs.rmSync(tmp, { recursive: true, force: true });
});

test('removeClaudeSettingsAllow removes the rule, prunes empties, keeps other rules/settings', () => {
  const tmp = tmpdir();
  const settingsPath = path.join(tmp, 'settings.json');
  fs.writeFileSync(settingsPath, JSON.stringify({
    model: 'opus',
    permissions: { allow: ['Bash(ls:*)', 'mcp__kaboom-browser-devtools', 'mcp__gasoline'] },
  }));
  const r = removeClaudeSettingsAllow({ settingsPath });
  assert.equal(r.status, 'removed');
  const data = JSON.parse(fs.readFileSync(settingsPath, 'utf8'));
  assert.deepEqual(data.permissions.allow, ['Bash(ls:*)'], 'canonical + legacy rules removed, user rule kept');
  assert.equal(data.model, 'opus');
  fs.rmSync(tmp, { recursive: true, force: true });
});

test('removeClaudeSettingsAllow prunes permissions when it becomes empty', () => {
  const tmp = tmpdir();
  const settingsPath = path.join(tmp, 'settings.json');
  fs.writeFileSync(settingsPath, JSON.stringify({
    permissions: { allow: ['mcp__kaboom-browser-devtools'] },
  }));
  removeClaudeSettingsAllow({ settingsPath });
  const data = JSON.parse(fs.readFileSync(settingsPath, 'utf8'));
  assert.equal(data.permissions, undefined, 'empty permissions object pruned');
  assert.equal(fs.existsSync(settingsPath), true, 'shared settings file never deleted');
  fs.rmSync(tmp, { recursive: true, force: true });
});

test('removeClaudeSettingsAllow returns notConfigured when the rule is absent', () => {
  const tmp = tmpdir();
  const settingsPath = path.join(tmp, 'settings.json');
  fs.writeFileSync(settingsPath, JSON.stringify({ permissions: { allow: ['Bash(ls:*)'] } }));
  const r = removeClaudeSettingsAllow({ settingsPath });
  assert.equal(r.status, 'notConfigured');
  fs.rmSync(tmp, { recursive: true, force: true });
});

// --- install/uninstall wiring with REAL client definitions ---

function withTempPath(id, tmp, fileName) {
  const base = getClientById(id);
  return { ...base, configPath: { all: path.join(tmp, fileName) }, detectDir: { all: tmp } };
}

test('installToClient (gemini) writes trust:true alongside the server entry', () => {
  const tmp = tmpdir();
  const def = withTempPath('gemini', tmp, 'settings.json');
  const res = installToClient(def, { dryRun: false, envVars: {}, binaryCommand: '/tmp/kb' });
  assert.equal(res.autoApprove, 'applied');
  const data = JSON.parse(fs.readFileSync(path.join(tmp, 'settings.json'), 'utf8'));
  assert.equal(data.mcpServers[SERVER].trust, true);
  fs.rmSync(tmp, { recursive: true, force: true });
});

test('installToClient (opencode) writes the permission wildcard allow', () => {
  const tmp = tmpdir();
  const def = withTempPath('opencode', tmp, 'opencode.json');
  const res = installToClient(def, { dryRun: false, envVars: {}, binaryCommand: '/tmp/kb' });
  assert.equal(res.autoApprove, 'applied');
  const data = JSON.parse(fs.readFileSync(path.join(tmp, 'opencode.json'), 'utf8'));
  assert.equal(data.permission[OPENCODE_PERMISSION_KEY], 'allow');
  assert.ok(data.mcp[SERVER], 'server entry still written');
  fs.rmSync(tmp, { recursive: true, force: true });
});

test('installToClient (zed) writes the five per-tool allow permissions', () => {
  const tmp = tmpdir();
  const def = withTempPath('zed', tmp, 'settings.json');
  const res = installToClient(def, { dryRun: false, envVars: {}, binaryCommand: '/tmp/kb' });
  assert.equal(res.autoApprove, 'applied');
  const data = JSON.parse(fs.readFileSync(path.join(tmp, 'settings.json'), 'utf8'));
  for (const t of KABOOM_TOOL_NAMES) {
    assert.deepEqual(data.agent.tool_permissions.tools[`mcp:${SERVER}:${t}`], { default: 'allow' });
  }
  fs.rmSync(tmp, { recursive: true, force: true });
});

test('installToClient (claude-code) registers via CLI AND writes settings.json auto-approve', () => {
  if (process.platform === 'win32') return; // fake CLI uses a unix shebang
  const tmp = tmpdir();
  const fakeCli = path.join(tmp, 'fake-claude');
  fs.writeFileSync(fakeCli, ['#!/usr/bin/env node', 'process.exit(0);', ''].join('\n'), { mode: 0o755 });
  const settingsPath = path.join(tmp, '.claude', 'settings.json');
  const def = { ...getClientById('claude-code'), detectCommand: fakeCli };

  const res = installToClient(def, { dryRun: false, envVars: {}, binaryCommand: '/tmp/kb', claudeSettingsPath: settingsPath });
  assert.equal(res.success, true);
  assert.equal(res.autoApprove, 'applied');
  const data = JSON.parse(fs.readFileSync(settingsPath, 'utf8'));
  assert.ok(data.permissions.allow.includes('mcp__kaboom-browser-devtools'));
  fs.rmSync(tmp, { recursive: true, force: true });
});

test('installToClient marks UI-only clients as ui-only (no config field invented)', () => {
  const tmp = tmpdir();
  // cursor is UI-only; installing must still write the server entry but never a
  // trust field, and report ui-only.
  const def = withTempPath('cursor', tmp, 'mcp.json');
  const res = installToClient(def, { dryRun: false, envVars: {}, binaryCommand: '/tmp/kb' });
  assert.equal(res.autoApprove, 'ui-only');
  const data = JSON.parse(fs.readFileSync(path.join(tmp, 'mcp.json'), 'utf8'));
  assert.ok(data.mcpServers[SERVER]);
  assert.equal(data.mcpServers[SERVER].trust, undefined, 'no invented trust field');
  assert.equal(data.permission, undefined);
  assert.equal(data.agent, undefined);
  fs.rmSync(tmp, { recursive: true, force: true });
});

// --- round-trip cleanliness ---

test('install then uninstall leaves no orphan auto-approve keys (opencode)', () => {
  const tmp = tmpdir();
  const def = withTempPath('opencode', tmp, 'opencode.json');
  const cfgPath = path.join(tmp, 'opencode.json');
  // Seed a user setting to prove the file is preserved and only kaboom is removed.
  fs.writeFileSync(cfgPath, JSON.stringify({ theme: 'dark', permission: { bash: 'ask' } }));
  installToClient(def, { dryRun: false, envVars: {}, binaryCommand: '/tmp/kb' });
  const r = uninstallFromClient(def, { dryRun: false });
  assert.equal(r.status, 'removed');
  const data = JSON.parse(fs.readFileSync(cfgPath, 'utf8'));
  assert.equal(data.mcp && data.mcp[SERVER], undefined, 'server entry removed');
  assert.equal(data.permission[OPENCODE_PERMISSION_KEY], undefined, 'auto-approve key removed');
  assert.equal(data.permission.bash, 'ask', 'user permission preserved');
  assert.equal(data.theme, 'dark', 'user settings preserved');
  fs.rmSync(tmp, { recursive: true, force: true });
});

test('install then uninstall leaves no orphan auto-approve keys (zed)', () => {
  const tmp = tmpdir();
  const def = withTempPath('zed', tmp, 'settings.json');
  const cfgPath = path.join(tmp, 'settings.json');
  fs.writeFileSync(cfgPath, JSON.stringify({ theme: 'one-dark' }));
  installToClient(def, { dryRun: false, envVars: {}, binaryCommand: '/tmp/kb' });
  const r = uninstallFromClient(def, { dryRun: false });
  assert.equal(r.status, 'removed');
  const data = JSON.parse(fs.readFileSync(cfgPath, 'utf8'));
  assert.equal(data.context_servers && data.context_servers[SERVER], undefined, 'server entry removed');
  assert.equal(data.agent, undefined, 'tool_permissions container pruned');
  assert.equal(data.theme, 'one-dark', 'user settings preserved');
  fs.rmSync(tmp, { recursive: true, force: true });
});

test('install then uninstall leaves no orphan trust (gemini)', () => {
  const tmp = tmpdir();
  const def = withTempPath('gemini', tmp, 'settings.json');
  const cfgPath = path.join(tmp, 'settings.json');
  fs.writeFileSync(cfgPath, JSON.stringify({ selectedAuthType: 'oauth' }));
  installToClient(def, { dryRun: false, envVars: {}, binaryCommand: '/tmp/kb' });
  const r = uninstallFromClient(def, { dryRun: false });
  assert.equal(r.status, 'removed');
  const data = JSON.parse(fs.readFileSync(cfgPath, 'utf8'));
  assert.equal(data.mcpServers && data.mcpServers[SERVER], undefined, 'server entry (with trust) removed');
  assert.equal(data.selectedAuthType, 'oauth', 'user settings preserved');
  fs.rmSync(tmp, { recursive: true, force: true });
});

test('uninstallFromClient (claude-code fake CLI) removes the settings.json allow rule', () => {
  if (process.platform === 'win32') return;
  const tmp = tmpdir();
  const fakeCli = path.join(tmp, 'fake-claude');
  // Fake CLI: nothing configured for `claude mcp remove` (so the removal signal
  // must come from the settings.json cleanup).
  fs.writeFileSync(
    fakeCli,
    ['#!/usr/bin/env node', "process.stderr.write('not found');", 'process.exit(1);', ''].join('\n'),
    { mode: 0o755 }
  );
  const settingsPath = path.join(tmp, '.claude', 'settings.json');
  fs.mkdirSync(path.dirname(settingsPath), { recursive: true });
  fs.writeFileSync(settingsPath, JSON.stringify({ permissions: { allow: ['mcp__kaboom-browser-devtools', 'Bash(ls:*)'] } }));
  const def = { ...getClientById('claude-code'), detectCommand: fakeCli };

  const r = uninstallFromClient(def, { dryRun: false, claudeSettingsPath: settingsPath });
  assert.equal(r.status, 'removed');
  const data = JSON.parse(fs.readFileSync(settingsPath, 'utf8'));
  assert.deepEqual(data.permissions.allow, ['Bash(ls:*)']);
  fs.rmSync(tmp, { recursive: true, force: true });
});
