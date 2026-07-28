// Purpose: Validate client registry and config-path behaviors in the npm wrapper.
// Why: Prevents install/doctor regressions across supported MCP client targets.
// Docs: docs/features/feature/enhanced-cli-config/index.md

const test = require('node:test');
const assert = require('node:assert/strict');
const os = require('node:os');
const path = require('node:path');
const fs = require('node:fs');
const {
  CLIENT_DEFINITIONS,
  CLIENT_ALIASES,
  getClientConfigPath,
  isClientInstalled,
  getDetectedClients,
  commandExistsOnPath,
  getClientById,
  getClientByAlias,
  getValidAliases,
  resolveManagedBinaryPath,
} = require('./config');

// --- resolveManagedBinaryPath ---

test('resolveManagedBinaryPath honors KABOOM_BINARY_PATH when it exists', () => {
  const p = resolveManagedBinaryPath({
    env: { KABOOM_BINARY_PATH: '/opt/kb/kaboom' },
    existsFn: (f) => f === '/opt/kb/kaboom',
  });
  assert.equal(p, path.resolve('/opt/kb/kaboom'));
});

test('resolveManagedBinaryPath finds the installed node_modules platform binary', () => {
  const root = path.join(path.sep, 'proj', 'node_modules', 'kaboom-agentic-browser');
  const expected = path.join(root, 'node_modules', '@brennhill/kaboom-agentic-browser-darwin-arm64', 'bin', 'kaboom-agentic-browser');
  const p = resolveManagedBinaryPath({
    env: {}, platform: 'darwin', arch: 'arm64', packageRoot: root,
    existsFn: (f) => f === expected,
  });
  assert.equal(p, path.resolve(expected));
});

test('resolveManagedBinaryPath prefers the repo-root dist with the CORRECT name in the source tree', () => {
  // Regression: the dev path previously looked for dist/kaboom-<key> and never
  // matched the actual build output dist/kaboom-agentic-browser-<key>.
  const root = path.join(path.sep, 'repo', 'npm', 'kaboom-agentic-browser'); // parent dir "npm"
  const distBin = path.resolve(root, '..', '..', 'dist', 'kaboom-agentic-browser-darwin-arm64');
  const wrongOldName = path.resolve(root, '..', '..', 'dist', 'kaboom-darwin-arm64');
  const p = resolveManagedBinaryPath({
    env: {}, platform: 'darwin', arch: 'arm64', packageRoot: root,
    existsFn: (f) => f === distBin, // the old wrong name would NOT satisfy this
  });
  assert.equal(p, distBin);
  // And the old name must not resolve.
  assert.notEqual(p, wrongOldName);
});

test('resolveManagedBinaryPath never resolves a repo-root dist for an installed package', () => {
  // parent dir "node_modules" → not the source tree; a dist/ in the user's
  // project must be ignored (supply-chain boundary).
  const root = path.join(path.sep, 'proj', 'node_modules', 'kaboom-agentic-browser');
  const distBin = path.resolve(root, '..', '..', 'dist', 'kaboom-agentic-browser-darwin-arm64');
  const p = resolveManagedBinaryPath({
    env: {}, platform: 'darwin', arch: 'arm64', packageRoot: root,
    existsFn: (f) => f === distBin, // even though it "exists", it must be skipped
  });
  assert.equal(p, 'kaboom-agentic-browser');
});

test('resolveManagedBinaryPath maps win32/arm64 to the x64 .exe dist name', () => {
  const root = path.join(path.sep, 'repo', 'npm', 'kaboom-agentic-browser');
  const distBin = path.resolve(root, '..', '..', 'dist', 'kaboom-agentic-browser-win32-x64.exe');
  const p = resolveManagedBinaryPath({
    env: {}, platform: 'win32', arch: 'arm64', packageRoot: root,
    existsFn: (f) => f === distBin,
  });
  assert.equal(p, distBin);
});

test('resolveManagedBinaryPath falls back to the command name on an unknown platform', () => {
  const p = resolveManagedBinaryPath({ env: {}, platform: 'sunos', arch: 'sparc', existsFn: () => false });
  assert.equal(p, 'kaboom-agentic-browser');
});

// --- CLIENT_DEFINITIONS ---

test('CLIENT_DEFINITIONS contains all 10 clients (Codex added)', () => {
  const ids = CLIENT_DEFINITIONS.map(c => c.id);
  assert.deepEqual(ids, [
    'claude-code', 'claude-desktop', 'cursor', 'windsurf', 'vscode',
    'gemini', 'opencode', 'antigravity', 'zed', 'codex',
  ]);
});

test('each client definition has required fields', () => {
  for (const def of CLIENT_DEFINITIONS) {
    assert.ok(def.id, `missing id`);
    assert.ok(def.name, `missing name for ${def.id}`);
    assert.ok(['cli', 'file'].includes(def.type), `invalid type for ${def.id}`);
    if (def.type === 'cli') {
      assert.ok(def.detectCommand, `missing detectCommand for ${def.id}`);
      assert.ok(Array.isArray(def.installArgs), `missing installArgs for ${def.id}`);
      assert.ok(Array.isArray(def.removeArgs), `missing removeArgs for ${def.id}`);
    } else {
      assert.ok(def.configPath, `missing configPath for ${def.id}`);
      assert.ok(def.detectDir, `missing detectDir for ${def.id}`);
    }
  }
});

test('claude-code is CLI type with correct detect command', () => {
  const cc = CLIENT_DEFINITIONS.find(c => c.id === 'claude-code');
  assert.equal(cc.type, 'cli');
  assert.equal(cc.detectCommand, 'claude');
});

test('cursor uses correct config path', () => {
  const cursor = CLIENT_DEFINITIONS.find(c => c.id === 'cursor');
  assert.equal(cursor.type, 'file');
  assert.ok(cursor.configPath.all.includes('.cursor/mcp.json'));
});

test('windsurf uses correct config path (not .codeium/mcp.json)', () => {
  const ws = CLIENT_DEFINITIONS.find(c => c.id === 'windsurf');
  assert.ok(ws.configPath.all.includes('.codeium/windsurf/mcp_config.json'));
});

// --- getClientById ---

test('getClientById returns definition by id', () => {
  const cursor = getClientById('cursor');
  assert.equal(cursor.name, 'Cursor');
});

test('getClientById returns undefined for unknown id', () => {
  assert.equal(getClientById('nonexistent'), undefined);
});

// --- getClientConfigPath ---

// expandPath runs path.normalize, which emits backslashes on a Windows host even
// for a darwin/linux platform arg. Compare on forward slashes so these assertions
// hold on every runner, not just POSIX ones.
const fwd = (p) => String(p).replace(/\\/g, '/');

test('getClientConfigPath returns platform-specific path for claude-desktop on darwin', () => {
  const def = CLIENT_DEFINITIONS.find(c => c.id === 'claude-desktop');
  const result = getClientConfigPath(def, 'darwin');
  assert.ok(fwd(result).includes('Library/Application Support/Claude/claude_desktop_config.json'));
});

test('getClientConfigPath returns platform-specific path for vscode on linux', () => {
  const def = CLIENT_DEFINITIONS.find(c => c.id === 'vscode');
  const result = getClientConfigPath(def, 'linux');
  assert.ok(fwd(result).includes('.config/Code/User/mcp.json'));
});

test('getClientConfigPath returns "all" path for cursor', () => {
  const def = CLIENT_DEFINITIONS.find(c => c.id === 'cursor');
  const result = getClientConfigPath(def);
  assert.ok(fwd(result).includes('.cursor/mcp.json'));
});

test('getClientConfigPath returns null for CLI type', () => {
  const def = CLIENT_DEFINITIONS.find(c => c.id === 'claude-code');
  const result = getClientConfigPath(def);
  assert.equal(result, null);
});

test('getClientConfigPath returns null for unsupported platform', () => {
  const def = CLIENT_DEFINITIONS.find(c => c.id === 'claude-desktop');
  // claude-desktop only has darwin + win32
  const result = getClientConfigPath(def, 'linux');
  assert.equal(result, null);
});

// --- isClientInstalled ---

test('isClientInstalled detects existing directory for file-type client', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'gasoline-test-'));
  const cursorDir = path.join(tmp, '.cursor');
  fs.mkdirSync(cursorDir);

  const def = {
    id: 'test-cursor',
    type: 'file',
    detectDir: { all: path.join(tmp, '.cursor') },
    configPath: { all: path.join(tmp, '.cursor', 'mcp.json') },
  };

  assert.equal(isClientInstalled(def), true);
  fs.rmSync(tmp, { recursive: true });
});

test('isClientInstalled returns false when directory does not exist', () => {
  const def = {
    id: 'test-missing',
    type: 'file',
    detectDir: { all: '/tmp/nonexistent-gasoline-test-dir-12345' },
    configPath: { all: '/tmp/nonexistent-gasoline-test-dir-12345/mcp.json' },
  };

  assert.equal(isClientInstalled(def), false);
});

test('isClientInstalled checks detectCommand for CLI type', () => {
  // 'node' should be on PATH
  const def = {
    id: 'test-cli',
    type: 'cli',
    detectCommand: 'node',
  };
  assert.equal(isClientInstalled(def), true);
});

test('isClientInstalled returns false for missing CLI command', () => {
  const def = {
    id: 'test-cli-missing',
    type: 'cli',
    detectCommand: 'nonexistent-command-gasoline-test-12345',
  };
  assert.equal(isClientInstalled(def), false);
});

// --- commandExistsOnPath ---

test('commandExistsOnPath finds node', () => {
  assert.equal(commandExistsOnPath('node'), true);
});

test('commandExistsOnPath returns false for missing command', () => {
  assert.equal(commandExistsOnPath('nonexistent-command-gasoline-test-12345'), false);
});

// --- getDetectedClients ---

test('getDetectedClients returns only installed clients', () => {
  const detected = getDetectedClients();
  assert.ok(Array.isArray(detected));
  // Each should have isDetected = true implicitly (they passed the filter)
  for (const d of detected) {
    assert.ok(d.id, 'each detected client should have an id');
  }
});

// --- Gemini CLI client ---

test('gemini uses correct config path', () => {
  const gemini = CLIENT_DEFINITIONS.find(c => c.id === 'gemini');
  assert.equal(gemini.type, 'file');
  assert.ok(gemini.configPath.all.includes('.gemini/settings.json'));
});

// --- OpenCode client ---

test('opencode uses correct config path and configKey', () => {
  const oc = CLIENT_DEFINITIONS.find(c => c.id === 'opencode');
  assert.equal(oc.type, 'file');
  assert.ok(oc.configPath.all.includes('.config/opencode/opencode.json'));
  assert.equal(oc.configKey, 'mcp');
});

test('opencode buildEntry produces correct format', () => {
  const oc = CLIENT_DEFINITIONS.find(c => c.id === 'opencode');
  const entry = oc.buildEntry({});
  assert.deepEqual(entry, { type: 'local', command: ['kaboom-agentic-browser'], enabled: true });
});

test('opencode buildEntry includes env vars', () => {
  const oc = CLIENT_DEFINITIONS.find(c => c.id === 'opencode');
  const entry = oc.buildEntry({ DEBUG: '1' });
  assert.equal(entry.env.DEBUG, '1');
  assert.equal(entry.type, 'local');
  assert.deepEqual(entry.command, ['kaboom-agentic-browser']);
});

// --- Antigravity client ---

test('antigravity uses the home-dir config path on every platform', () => {
  const ag = CLIENT_DEFINITIONS.find(c => c.id === 'antigravity');
  assert.equal(ag.type, 'file');
  assert.ok(ag.configPath.all.includes('.gemini/antigravity/mcp_config.json'));
  // Regression: win32 must NOT drift to %APPDATA% — Antigravity uses ~ on all OSes.
  for (const plat of ['darwin', 'linux', 'win32']) {
    const resolved = getClientConfigPath(ag, plat);
    assert.ok(resolved.includes(path.join('.gemini', 'antigravity', 'mcp_config.json')), `bad path on ${plat}`);
    assert.ok(!resolved.includes('%APPDATA%'), `must not use %APPDATA% on ${plat}`);
  }
});

test('antigravity keeps the old %APPDATA% path tracked for stale cleanup', () => {
  const ag = CLIENT_DEFINITIONS.find(c => c.id === 'antigravity');
  assert.ok(ag.legacyConfigPaths, 'must declare legacy config paths');
  assert.ok(ag.legacyConfigPaths.win32.includes('%APPDATA%'), 'legacy win32 path must be the %APPDATA% location');
});

test('getClientLegacyConfigPaths resolves declared legacy paths per platform', () => {
  const { getClientLegacyConfigPaths } = require('./config');
  const def = {
    id: 'test',
    type: 'file',
    configPath: { all: '~/x.json' },
    detectDir: { all: '~' },
    legacyConfigPaths: { all: '~/legacy/x.json' },
  };
  const paths = getClientLegacyConfigPaths(def);
  assert.equal(paths.length, 1);
  assert.ok(paths[0].includes(path.join('legacy', 'x.json')));
  assert.deepEqual(getClientLegacyConfigPaths({ id: 'no-legacy', type: 'file', configPath: {}, detectDir: {} }), []);
});

// --- VS Code client ---

test('vscode uses the "servers" config key with "mcpServers" as legacy key', () => {
  const vs = CLIENT_DEFINITIONS.find(c => c.id === 'vscode');
  assert.equal(vs.configKey, 'servers', 'VS Code mcp.json uses a top-level "servers" key');
  assert.deepEqual(vs.legacyConfigKeys, ['mcpServers'], 'legacy mcpServers entries must still be cleaned');
  assert.equal(vs.dedicatedMcpFile, true);
});

// --- Dedicated vs shared config files ---

test('shared settings files are never flagged as dedicated MCP configs', () => {
  for (const id of ['gemini', 'opencode', 'zed']) {
    const def = CLIENT_DEFINITIONS.find(c => c.id === id);
    assert.ok(!def.dedicatedMcpFile, `${id} writes into a shared settings file and must never be deleted`);
  }
  for (const id of ['claude-desktop', 'cursor', 'windsurf', 'vscode', 'antigravity']) {
    const def = CLIENT_DEFINITIONS.find(c => c.id === id);
    assert.equal(def.dedicatedMcpFile, true, `${id} uses a dedicated MCP config file`);
  }
});

// --- Zed client ---

test('zed uses correct config path and configKey', () => {
  const zed = CLIENT_DEFINITIONS.find(c => c.id === 'zed');
  assert.equal(zed.type, 'file');
  assert.ok(zed.configPath.all.includes('.config/zed/settings.json'));
  assert.equal(zed.configKey, 'context_servers');
});

test('zed buildEntry produces correct format', () => {
  const zed = CLIENT_DEFINITIONS.find(c => c.id === 'zed');
  const entry = zed.buildEntry({});
  assert.deepEqual(entry, { source: 'custom', command: 'kaboom-agentic-browser', args: [] });
});

// --- Codex client ---

test('codex is a TOML-format client at ~/.codex/config.toml', () => {
  const codex = CLIENT_DEFINITIONS.find(c => c.id === 'codex');
  assert.equal(codex.type, 'file');
  assert.equal(codex.format, 'toml');
  assert.ok(codex.configPath.all.includes('.codex/config.toml'));
  assert.equal(codex.autoApprove.kind, 'codex-toml');
  // config.toml is shared — must never be flagged for deletion.
  assert.ok(!codex.dedicatedMcpFile);
});

test('codex honors $CODEX_HOME for config path and detection', () => {
  const codex = CLIENT_DEFINITIONS.find(c => c.id === 'codex');
  const prev = process.env.CODEX_HOME;
  try {
    const home = path.join(os.tmpdir(), 'kaboom-codex-home-test');
    process.env.CODEX_HOME = home;
    const cfg = getClientConfigPath(codex);
    assert.equal(cfg, path.normalize(path.join(home, 'config.toml')));
  } finally {
    if (prev === undefined) delete process.env.CODEX_HOME;
    else process.env.CODEX_HOME = prev;
  }
});

test('codex falls back to ~/.codex when CODEX_HOME is unset', () => {
  const codex = CLIENT_DEFINITIONS.find(c => c.id === 'codex');
  const prev = process.env.CODEX_HOME;
  try {
    delete process.env.CODEX_HOME;
    const cfg = getClientConfigPath(codex);
    assert.ok(fwd(cfg).includes('.codex/config.toml'));
  } finally {
    if (prev !== undefined) process.env.CODEX_HOME = prev;
  }
});

test('getClientByAlias resolves codex', () => {
  assert.equal(getClientByAlias('codex').id, 'codex');
});

// --- getClientByAlias ---

test('getClientByAlias returns client for valid alias', () => {
  assert.equal(getClientByAlias('gemini').id, 'gemini');
  assert.equal(getClientByAlias('cursor').id, 'cursor');
  assert.equal(getClientByAlias('claude').id, 'claude-code');
  assert.equal(getClientByAlias('opencode').id, 'opencode');
  assert.equal(getClientByAlias('vscode').id, 'vscode');
  assert.equal(getClientByAlias('antigravity').id, 'antigravity');
  assert.equal(getClientByAlias('zed').id, 'zed');
});

test('getClientByAlias is case-insensitive', () => {
  assert.equal(getClientByAlias('Gemini').id, 'gemini');
  assert.equal(getClientByAlias('CURSOR').id, 'cursor');
});

test('getClientByAlias returns null for unknown alias', () => {
  assert.equal(getClientByAlias('bogus'), null);
});

// --- getValidAliases ---

test('getValidAliases returns one alias per client', () => {
  const aliases = getValidAliases();
  assert.ok(aliases.includes('gemini'));
  assert.ok(aliases.includes('opencode'));
  assert.ok(aliases.includes('claude'));
  assert.ok(aliases.includes('antigravity'));
  assert.ok(aliases.includes('zed'));
  // Should have exactly one per unique client ID
  assert.equal(aliases.length, CLIENT_DEFINITIONS.length);
});
