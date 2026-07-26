// Purpose: Validate Codex CLI (config.toml) MCP registration + tool auto-approve.
// Why: Codex config is TOML; a wrong table name or approval field silently
// breaks the user's config. Locks the verified schema and the comment-preserving
// section edit / round-trip.
// Docs: docs/features/feature/enhanced-cli-config/index.md

const test = require('node:test');
const assert = require('node:assert/strict');
const os = require('node:os');
const path = require('node:path');
const fs = require('node:fs');

const codex = require('./codex-config');
const { getClientById } = require('./config');
const { installToClient } = require('./install');
const { uninstallFromClient } = require('./uninstall');
const { runDiagnostics } = require('./doctor');

const SERVER = 'kaboom-browser-devtools';

function tmpdir() {
  return fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-codex-'));
}

// --- TOML primitives ---

test('tomlString escapes backslashes and quotes', () => {
  assert.equal(codex.tomlString('/usr/bin/kb'), '"/usr/bin/kb"');
  assert.equal(codex.tomlString('C:\\kb\\bin.exe'), '"C:\\\\kb\\\\bin.exe"');
  assert.equal(codex.tomlString('a"b'), '"a\\"b"');
});

test('tomlKey leaves bare-safe keys unquoted and quotes others', () => {
  assert.equal(codex.tomlKey('kaboom-browser-devtools'), 'kaboom-browser-devtools');
  assert.equal(codex.tomlKey('has space'), '"has space"');
});

test('buildServerBlock uses the mcp_servers table + approve mode', () => {
  const block = codex.buildServerBlock('/tmp/kb', {});
  assert.match(block, /^\[mcp_servers\.kaboom-browser-devtools\]$/m);
  assert.match(block, /command = "\/tmp\/kb"/);
  assert.match(block, /default_tools_approval_mode = "approve"/);
});

test('buildServerBlock adds an env sub-table when env vars are present', () => {
  const block = codex.buildServerBlock('/tmp/kb', { DEBUG: '1' });
  assert.match(block, /\[mcp_servers\.kaboom-browser-devtools\.env\]/);
  assert.match(block, /DEBUG = "1"/);
});

// --- stripServerBlocks ---

test('stripServerBlocks removes only our block and preserves other tables + comments', () => {
  const content = [
    '# user preamble',
    'model = "gpt-5"',
    '',
    '[mcp_servers.other]',
    'command = "other-cmd"',
    '',
    '[mcp_servers.kaboom-browser-devtools]',
    '# Managed by Kaboom.',
    'command = "/tmp/kb"',
    'default_tools_approval_mode = "approve"',
    '',
    '[tools]',
    'web_search = true',
    '',
  ].join('\n');
  const { text, changed } = codex.stripServerBlocks(content, [SERVER]);
  assert.equal(changed, true);
  assert.match(text, /# user preamble/);
  assert.match(text, /model = "gpt-5"/);
  assert.match(text, /\[mcp_servers\.other\]/);
  assert.match(text, /\[tools\]/);
  assert.match(text, /web_search = true/);
  assert.doesNotMatch(text, /kaboom-browser-devtools/);
  assert.doesNotMatch(text, /default_tools_approval_mode/);
});

test('stripServerBlocks removes the env sub-table and handles quoted table names', () => {
  const content = [
    '[mcp_servers."kaboom-browser-devtools"]',
    'command = "/tmp/kb"',
    '[mcp_servers."kaboom-browser-devtools".env]',
    'TOKEN = "abc"',
    '',
    '[other]',
    'keep = 1',
  ].join('\n');
  const { text, changed } = codex.stripServerBlocks(content, [SERVER]);
  assert.equal(changed, true);
  assert.doesNotMatch(text, /kaboom-browser-devtools/);
  assert.doesNotMatch(text, /TOKEN/);
  assert.match(text, /\[other\]/);
  assert.match(text, /keep = 1/);
});

test('stripServerBlocks reports no change when our block is absent', () => {
  const content = '[mcp_servers.other]\ncommand = "x"\n';
  const { changed } = codex.stripServerBlocks(content, [SERVER]);
  assert.equal(changed, false);
});

// --- installCodex ---

test('installCodex creates config.toml with the server + approve mode', () => {
  const tmp = tmpdir();
  const configPath = path.join(tmp, 'config.toml');
  const r = codex.installCodex({ configPath, binaryCommand: '/tmp/kb', envVars: {} });
  assert.equal(r.success, true);
  assert.equal(r.isNew, true);
  assert.equal(r.autoApprove, 'applied');
  const text = fs.readFileSync(configPath, 'utf8');
  assert.match(text, /\[mcp_servers\.kaboom-browser-devtools\]/);
  assert.match(text, /default_tools_approval_mode = "approve"/);
  fs.rmSync(tmp, { recursive: true, force: true });
});

test('installCodex preserves existing config and is idempotent (replaces our block)', () => {
  const tmp = tmpdir();
  const configPath = path.join(tmp, 'config.toml');
  fs.writeFileSync(configPath, '# my codex config\nmodel = "gpt-5"\n\n[mcp_servers.other]\ncommand = "o"\n');
  codex.installCodex({ configPath, binaryCommand: '/tmp/kb', envVars: {} });
  codex.installCodex({ configPath, binaryCommand: '/tmp/kb2', envVars: {} }); // re-run
  const text = fs.readFileSync(configPath, 'utf8');
  assert.match(text, /# my codex config/);
  assert.match(text, /model = "gpt-5"/);
  assert.match(text, /\[mcp_servers\.other\]/);
  // Exactly one kaboom block, pointing at the latest binary.
  assert.equal((text.match(/\[mcp_servers\.kaboom-browser-devtools\]/g) || []).length, 1);
  assert.match(text, /command = "\/tmp\/kb2"/);
  assert.doesNotMatch(text, /"\/tmp\/kb"\n/);
  fs.rmSync(tmp, { recursive: true, force: true });
});

test('installCodex dry-run does not write the file', () => {
  const tmp = tmpdir();
  const configPath = path.join(tmp, 'config.toml');
  const r = codex.installCodex({ configPath, binaryCommand: '/tmp/kb', envVars: {}, dryRun: true });
  assert.equal(r.autoApprove, 'would-apply');
  assert.equal(fs.existsSync(configPath), false);
  fs.rmSync(tmp, { recursive: true, force: true });
});

// --- uninstallCodex ---

test('uninstallCodex removes our block, preserves others, never deletes the file', () => {
  const tmp = tmpdir();
  const configPath = path.join(tmp, 'config.toml');
  const original = '# my codex config\nmodel = "gpt-5"\n\n[mcp_servers.other]\ncommand = "o"\n';
  fs.writeFileSync(configPath, original);
  codex.installCodex({ configPath, binaryCommand: '/tmp/kb', envVars: {} });
  const r = codex.uninstallCodex({ configPath });
  assert.equal(r.status, 'removed');
  assert.equal(fs.existsSync(configPath), true, 'shared config.toml must not be deleted');
  const text = fs.readFileSync(configPath, 'utf8');
  assert.match(text, /model = "gpt-5"/);
  assert.match(text, /\[mcp_servers\.other\]/);
  assert.doesNotMatch(text, /kaboom-browser-devtools/);
  fs.rmSync(tmp, { recursive: true, force: true });
});

test('uninstallCodex returns notConfigured when file is absent or block missing', () => {
  const tmp = tmpdir();
  const configPath = path.join(tmp, 'config.toml');
  assert.equal(codex.uninstallCodex({ configPath }).status, 'notConfigured');
  fs.writeFileSync(configPath, '[mcp_servers.other]\ncommand = "o"\n');
  assert.equal(codex.uninstallCodex({ configPath }).status, 'notConfigured');
  fs.rmSync(tmp, { recursive: true, force: true });
});

test('install then uninstall restores the user config (no kaboom residue)', () => {
  const tmp = tmpdir();
  const configPath = path.join(tmp, 'config.toml');
  fs.writeFileSync(configPath, 'model = "gpt-5"\n\n[tools]\nweb_search = true\n');
  codex.installCodex({ configPath, binaryCommand: '/tmp/kb', envVars: {} });
  codex.uninstallCodex({ configPath });
  const text = fs.readFileSync(configPath, 'utf8');
  assert.match(text, /model = "gpt-5"/);
  assert.match(text, /\[tools\]/);
  assert.match(text, /web_search = true/);
  assert.doesNotMatch(text, /kaboom/);
  fs.rmSync(tmp, { recursive: true, force: true });
});

// --- codexServerConfigured ---

test('codexServerConfigured reports canonical, legacy, and missing states', () => {
  const tmp = tmpdir();
  const configPath = path.join(tmp, 'config.toml');
  assert.equal(codex.codexServerConfigured(configPath).exists, false);

  fs.writeFileSync(configPath, '[mcp_servers.kaboom-browser-devtools]\ncommand = "x"\n');
  let res = codex.codexServerConfigured(configPath);
  assert.equal(res.configured, true);
  assert.equal(res.matchedName, 'kaboom-browser-devtools');

  fs.writeFileSync(configPath, '[mcp_servers.gasoline]\ncommand = "x"\n');
  res = codex.codexServerConfigured(configPath);
  assert.equal(res.configured, true);
  assert.equal(res.matchedName, 'gasoline');

  fs.writeFileSync(configPath, '[mcp_servers.other]\ncommand = "x"\n');
  assert.equal(codex.codexServerConfigured(configPath).configured, false);
  fs.rmSync(tmp, { recursive: true, force: true });
});

// --- install/uninstall wiring via the real codex client def + $CODEX_HOME ---

test('installToClient/uninstallFromClient drive Codex via $CODEX_HOME', () => {
  const tmp = tmpdir();
  const prev = process.env.CODEX_HOME;
  try {
    process.env.CODEX_HOME = tmp;
    const def = getClientById('codex');
    const res = installToClient(def, { dryRun: false, envVars: {}, binaryCommand: '/tmp/kb' });
    assert.equal(res.success, true);
    assert.equal(res.autoApprove, 'applied');
    const configPath = path.join(tmp, 'config.toml');
    assert.match(fs.readFileSync(configPath, 'utf8'), /default_tools_approval_mode = "approve"/);

    const un = uninstallFromClient(def, { dryRun: false });
    assert.equal(un.status, 'removed');
    assert.doesNotMatch(fs.readFileSync(configPath, 'utf8'), /kaboom-browser-devtools/);
  } finally {
    if (prev === undefined) delete process.env.CODEX_HOME;
    else process.env.CODEX_HOME = prev;
    fs.rmSync(tmp, { recursive: true, force: true });
  }
});

// --- doctor understands the TOML client (no false "Invalid JSON") ---

test('runDiagnostics reports Codex ok when the TOML server table is present', async () => {
  const tmp = tmpdir();
  const configPath = path.join(tmp, 'config.toml');
  fs.writeFileSync(configPath, '[mcp_servers.kaboom-browser-devtools]\ncommand = "x"\ndefault_tools_approval_mode = "approve"\n');
  const def = {
    id: 'codex',
    name: 'Codex CLI',
    type: 'file',
    format: 'toml',
    configPath: { all: configPath },
    detectDir: { all: tmp },
  };
  const report = await runDiagnostics(false, {
    clients: [def],
    fetchHealthFn: async () => ({ reachable: false }),
  });
  const codexTool = report.tools.find((t) => t.id === 'codex');
  assert.equal(codexTool.status, 'ok', 'TOML client must not be misread as invalid JSON');
  fs.rmSync(tmp, { recursive: true, force: true });
});

test('runDiagnostics reports Codex error when the server table is missing', async () => {
  const tmp = tmpdir();
  const configPath = path.join(tmp, 'config.toml');
  fs.writeFileSync(configPath, 'model = "gpt-5"\n');
  const def = {
    id: 'codex',
    name: 'Codex CLI',
    type: 'file',
    format: 'toml',
    configPath: { all: configPath },
    detectDir: { all: tmp },
  };
  const report = await runDiagnostics(false, {
    clients: [def],
    fetchHealthFn: async () => ({ reachable: false }),
  });
  const codexTool = report.tools.find((t) => t.id === 'codex');
  assert.equal(codexTool.status, 'error');
  assert.ok(codexTool.issues.some((i) => i.includes('missing')));
  fs.rmSync(tmp, { recursive: true, force: true });
});
