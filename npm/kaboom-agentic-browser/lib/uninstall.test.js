// Purpose: Validate uninstall behavior for npm wrapper-managed MCP config entries.
// Why: Ensures cleanup removes only Kaboom-managed entries while preserving user config state.
// Docs: docs/features/feature/enhanced-cli-config/index.md

const test = require('node:test');
const assert = require('node:assert/strict');
const os = require('node:os');
const path = require('node:path');
const fs = require('node:fs');
const { uninstallFromClient, executeUninstall } = require('./uninstall');

test('npm wrapper no longer exposes gasoline launcher aliases', () => {
  const packageJson = JSON.parse(fs.readFileSync(path.join(__dirname, '..', 'package.json'), 'utf8'));
  const hooksLauncher = fs.readFileSync(path.join(__dirname, '..', 'bin', 'kaboom-hooks'), 'utf8');

  assert.equal(packageJson.bin['gasoline-agentic-browser'], undefined);
  assert.equal(packageJson.bin['gasoline-hooks'], undefined);
  assert.equal(packageJson.bin['kaboom-agentic-browser'], 'bin/kaboom-agentic-browser');
  assert.equal(packageJson.bin['kaboom-hooks'], 'bin/kaboom-hooks');
  assert.match(hooksLauncher, /kaboom-hooks binary not found/);
  assert.match(hooksLauncher, /npm install -g kaboom-agentic-browser@latest/);
});

// --- uninstallFromClient: file-type ---

test('uninstallFromClient removes canonical entry and preserves other servers', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-uninstall-'));
  const cfgPath = path.join(tmp, 'mcp.json');

  fs.writeFileSync(cfgPath, JSON.stringify({
    mcpServers: {
      'kaboom-browser-devtools': { command: 'kaboom-agentic-browser', args: [] },
      other: { command: 'other-cmd', args: [] },
    },
  }));

  const def = {
    id: 'test-cursor',
    name: 'Test Cursor',
    type: 'file',
    configPath: { all: cfgPath },
    detectDir: { all: tmp },
  };

  const result = uninstallFromClient(def, { dryRun: false });
  assert.equal(result.status, 'removed');

  const written = JSON.parse(fs.readFileSync(cfgPath, 'utf8'));
  assert.equal(written.mcpServers['kaboom-browser-devtools'], undefined);
  assert.ok(written.mcpServers.other, 'should preserve other servers');

  fs.rmSync(tmp, { recursive: true });
});

test('uninstallFromClient deletes dedicated MCP config file when managed entries are the only servers', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-uninstall-'));
  const cfgPath = path.join(tmp, 'mcp.json');

  fs.writeFileSync(cfgPath, JSON.stringify({
    mcpServers: { 'kaboom-browser-devtools': { command: 'kaboom-agentic-browser', args: [] } },
  }));

  const def = {
    id: 'test-cursor',
    name: 'Test Cursor',
    type: 'file',
    dedicatedMcpFile: true,
    configPath: { all: cfgPath },
    detectDir: { all: tmp },
  };

  const result = uninstallFromClient(def, { dryRun: false });
  assert.equal(result.status, 'removed');
  assert.equal(fs.existsSync(cfgPath), false, 'should delete dedicated MCP config file');

  fs.rmSync(tmp, { recursive: true });
});

test('uninstallFromClient never deletes shared settings files (Zed)', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-uninstall-'));
  const cfgPath = path.join(tmp, 'settings.json');

  // Zed-style shared settings: user theme + only a kaboom context server.
  fs.writeFileSync(cfgPath, JSON.stringify({
    theme: 'one-dark',
    context_servers: {
      'kaboom-browser-devtools': { source: 'custom', command: 'kaboom-agentic-browser', args: [] },
    },
  }));

  const def = {
    id: 'test-zed',
    name: 'Test Zed',
    type: 'file',
    configPath: { all: cfgPath },
    detectDir: { all: tmp },
    configKey: 'context_servers',
  };

  const result = uninstallFromClient(def, { dryRun: false });
  assert.equal(result.status, 'removed');
  assert.equal(fs.existsSync(cfgPath), true, 'shared settings file must NOT be deleted');

  const written = JSON.parse(fs.readFileSync(cfgPath, 'utf8'));
  assert.equal(written.theme, 'one-dark', 'user settings must be preserved');
  assert.equal(written.context_servers['kaboom-browser-devtools'], undefined, 'kaboom entry must be removed');

  fs.rmSync(tmp, { recursive: true });
});

test('uninstallFromClient never deletes shared settings files (Gemini-style mcpServers key)', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-uninstall-'));
  const cfgPath = path.join(tmp, 'settings.json');

  fs.writeFileSync(cfgPath, JSON.stringify({
    selectedAuthType: 'oauth',
    mcpServers: { 'kaboom-browser-devtools': { command: 'kaboom-agentic-browser', args: [] } },
  }));

  const def = {
    id: 'test-gemini',
    name: 'Test Gemini',
    type: 'file',
    configPath: { all: cfgPath },
    detectDir: { all: tmp },
  };

  const result = uninstallFromClient(def, { dryRun: false });
  assert.equal(result.status, 'removed');
  assert.equal(fs.existsSync(cfgPath), true, 'shared settings file must NOT be deleted');

  const written = JSON.parse(fs.readFileSync(cfgPath, 'utf8'));
  assert.equal(written.selectedAuthType, 'oauth', 'user settings must be preserved');
  assert.equal(written.mcpServers['kaboom-browser-devtools'], undefined, 'kaboom entry must be removed');

  fs.rmSync(tmp, { recursive: true });
});

test('uninstallFromClient cleans the canonical VS Code servers key', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-uninstall-'));
  const cfgPath = path.join(tmp, 'mcp.json');

  fs.writeFileSync(cfgPath, JSON.stringify({
    servers: {
      'kaboom-browser-devtools': { command: 'kaboom-agentic-browser', args: [] },
      other: { command: 'other-cmd', args: [] },
    },
    extensionSetting: true,
  }));

  const def = {
    id: 'test-vscode',
    name: 'Test VS Code',
    type: 'file',
    dedicatedMcpFile: true,
    configKey: 'servers',
    configPath: { all: cfgPath },
    detectDir: { all: tmp },
  };

  const result = uninstallFromClient(def, { dryRun: false });
  assert.equal(result.status, 'removed');
  assert.equal(fs.existsSync(cfgPath), true, 'file kept because unrelated servers remain');

  const written = JSON.parse(fs.readFileSync(cfgPath, 'utf8'));
  assert.equal(written.servers['kaboom-browser-devtools'], undefined);
  assert.ok(written.servers.other, 'unrelated entries under servers must survive');
  assert.equal(written.extensionSetting, true, 'unrelated settings must survive');

  fs.rmSync(tmp, { recursive: true });
});

test('uninstallFromClient returns notConfigured when file does not exist', () => {
  const def = {
    id: 'test-cursor',
    name: 'Test Cursor',
    type: 'file',
    configPath: { all: '/tmp/nonexistent-kaboom-test-12345/mcp.json' },
    detectDir: { all: '/tmp/nonexistent-kaboom-test-12345' },
  };

  const result = uninstallFromClient(def, { dryRun: false });
  assert.equal(result.status, 'notConfigured');
});

test('uninstallFromClient returns notConfigured when no managed entries are in config', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-uninstall-'));
  const cfgPath = path.join(tmp, 'mcp.json');

  fs.writeFileSync(cfgPath, JSON.stringify({
    mcpServers: { other: { command: 'other-cmd', args: [] } },
  }));

  const def = {
    id: 'test-cursor',
    name: 'Test Cursor',
    type: 'file',
    configPath: { all: cfgPath },
    detectDir: { all: tmp },
  };

  const result = uninstallFromClient(def, { dryRun: false });
  assert.equal(result.status, 'notConfigured');

  fs.rmSync(tmp, { recursive: true });
});

test('uninstallFromClient dry-run does not modify file', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-uninstall-'));
  const cfgPath = path.join(tmp, 'mcp.json');

  const original = {
    mcpServers: { 'kaboom-browser-devtools': { command: 'kaboom-agentic-browser', args: [] } },
  };
  fs.writeFileSync(cfgPath, JSON.stringify(original));

  const def = {
    id: 'test-cursor',
    name: 'Test Cursor',
    type: 'file',
    configPath: { all: cfgPath },
    detectDir: { all: tmp },
  };

  const result = uninstallFromClient(def, { dryRun: true });
  assert.equal(result.status, 'removed');

  const still = JSON.parse(fs.readFileSync(cfgPath, 'utf8'));
  assert.ok(still.mcpServers['kaboom-browser-devtools'], 'should not modify in dry-run');

  fs.rmSync(tmp, { recursive: true });
});

// --- uninstallFromClient: CLI-type ---

test('uninstallFromClient removes the canonical server via CLI', () => {
  if (process.platform === 'win32') return; // fake CLI uses a unix shebang
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-uninstall-cli-'));
  const logPath = path.join(tmp, 'calls.log');
  const fakeCli = path.join(tmp, 'fake-claude');

  fs.writeFileSync(
    fakeCli,
    [
      '#!/usr/bin/env node',
      "const fs = require('fs');",
      'const name = process.argv[process.argv.length - 1];',
      `fs.appendFileSync(${JSON.stringify(logPath)}, name + '\\n');`,
      "if (name === 'kaboom-browser-devtools') process.exit(0);",
      "process.stderr.write('No MCP server found: not found');",
      'process.exit(1);',
      '',
    ].join('\n'),
    { mode: 0o755 }
  );

  const def = {
    id: 'claude-code',
    name: 'Claude Code',
    type: 'cli',
    detectCommand: fakeCli,
    removeArgs: ['mcp', 'remove', '--scope', 'user', 'kaboom-browser-devtools'],
  };

  const result = uninstallFromClient(def, { dryRun: false });
  assert.equal(result.status, 'removed');

  const attempts = fs.readFileSync(logPath, 'utf8').trim().split('\n');
  assert.deepEqual(attempts, ['kaboom-browser-devtools']);

  fs.rmSync(tmp, { recursive: true, force: true });
});

test('uninstallFromClient returns notConfigured when no CLI server names exist', () => {
  if (process.platform === 'win32') return;
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-uninstall-cli-'));
  const fakeCli = path.join(tmp, 'fake-claude');
  fs.writeFileSync(
    fakeCli,
    [
      '#!/usr/bin/env node',
      "process.stderr.write('No MCP server found: not found');",
      'process.exit(1);',
      '',
    ].join('\n'),
    { mode: 0o755 }
  );

  const def = {
    id: 'claude-code',
    name: 'Claude Code',
    type: 'cli',
    detectCommand: fakeCli,
    removeArgs: ['mcp', 'remove', '--scope', 'user', 'kaboom-browser-devtools'],
  };

  const result = uninstallFromClient(def, { dryRun: false });
  assert.equal(result.status, 'notConfigured');

  fs.rmSync(tmp, { recursive: true, force: true });
});

test('uninstallFromClient handles CLI type with dry-run', () => {
  const def = {
    id: 'claude-code',
    name: 'Claude Code',
    type: 'cli',
    detectCommand: 'claude',
    removeArgs: ['mcp', 'remove', '--scope', 'user', 'kaboom-browser-devtools'],
  };

  const result = uninstallFromClient(def, { dryRun: true });
  assert.equal(result.status, 'removed');
  assert.equal(result.method, 'cli');
});

// --- executeUninstall ---

test('executeUninstall removes from detected file-type clients', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-uninstall-'));
  const cfgPath = path.join(tmp, 'mcp.json');

  fs.writeFileSync(cfgPath, JSON.stringify({
    mcpServers: {
      'kaboom-browser-devtools': { command: 'kaboom-agentic-browser', args: [] },
      other: { command: 'other-cmd', args: [] },
    },
  }));

  const result = executeUninstall({
    dryRun: false,
    _clientOverrides: [
      {
        id: 'test-cursor',
        name: 'Test Cursor',
        type: 'file',
        configPath: { all: cfgPath },
        detectDir: { all: tmp },
      },
    ],
  });

  assert.equal(result.success, true);
  assert.equal(result.removed.length, 1);

  fs.rmSync(tmp, { recursive: true });
});

test('executeUninstall removes canonical managed skill files', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-uninstall-'));
  const claudeRoot = path.join(tmp, 'claude-skills');
  fs.mkdirSync(claudeRoot, { recursive: true });
  fs.writeFileSync(
    path.join(claudeRoot, 'debug.md'),
    '<!-- kaboom-managed-skill id:debug version:2 -->\ncurrent kaboom skill\n',
    'utf8'
  );

  const originalClaudeDir = process.env.KABOOM_CLAUDE_SKILLS_DIR;
  try {
    process.env.KABOOM_CLAUDE_SKILLS_DIR = claudeRoot;
    const result = executeUninstall({
      dryRun: false,
      _clientOverrides: [],
      skillAgents: ['claude'],
      skillScope: 'global',
    });

    assert.equal(result.success, true);
    assert.ok(result.skillCleanup);
    assert.equal(result.skillCleanup.removed, 1);
    assert.equal(fs.existsSync(path.join(claudeRoot, 'debug.md')), false);
  } finally {
    if (originalClaudeDir === undefined) {
      delete process.env.KABOOM_CLAUDE_SKILLS_DIR;
    } else {
      process.env.KABOOM_CLAUDE_SKILLS_DIR = originalClaudeDir;
    }
    fs.rmSync(tmp, { recursive: true, force: true });
  }
});
