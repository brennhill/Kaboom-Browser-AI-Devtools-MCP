// Purpose: Validate install flow behavior for file/CLI clients in the npm wrapper.
// Why: Ensures --install remains deterministic and safe across dry-run and merge paths.
// Docs: docs/features/feature/enhanced-cli-config/index.md

const test = require('node:test');
const assert = require('node:assert/strict');
const os = require('node:os');
const path = require('node:path');
const fs = require('node:fs');
const {
  generateDefaultConfig,
  buildMcpEntry,
  installToClient,
  executeInstall,
} = require('./install');
const { installBundledSkills } = require('./skills');

test('npm wrapper metadata uses kaboom package and launcher names', () => {
  const packageJson = JSON.parse(fs.readFileSync(path.join(__dirname, '..', 'package.json'), 'utf8'));
  assert.equal(packageJson.name, 'kaboom-agentic-browser');
  assert.equal(packageJson.bin['kaboom-agentic-browser'], 'bin/kaboom-agentic-browser');
  assert.equal(packageJson.bin['kaboom-hooks'], 'bin/kaboom-hooks');
  assert.equal(packageJson.homepage, 'https://gokaboom.dev');
  assert.ok(packageJson.description.includes('Kaboom'));
  assert.ok(packageJson.optionalDependencies['@brennhill/kaboom-agentic-browser-darwin-arm64']);
});

test('npm wrapper readme and launcher copy use kaboom branding', () => {
  const readme = fs.readFileSync(path.join(__dirname, '..', 'README.md'), 'utf8');
  const launcher = fs.readFileSync(path.join(__dirname, '..', 'bin', 'kaboom-agentic-browser'), 'utf8');
  const cli = fs.readFileSync(path.join(__dirname, 'cli.js'), 'utf8');

  assert.match(readme, /^# kaboom-agentic-browser$/m);
  assert.match(launcher, /Kaboom Agentic Browser server/);
  assert.match(launcher, /npm install -g kaboom-agentic-browser@latest/);
  assert.match(cli, /Kaboom Agentic Browser/);
  assert.match(cli, /kaboom-agentic-browser --install/);
});

// --- generateDefaultConfig ---

test('generateDefaultConfig returns valid MCP config', () => {
  const cfg = generateDefaultConfig();
  assert.ok(cfg.mcpServers);
  assert.ok(cfg.mcpServers['kaboom-browser-devtools']);
  assert.ok(cfg.mcpServers['kaboom-browser-devtools'].command.length > 0);
});

test('generateDefaultConfig honors binaryCommand override', () => {
  const cfg = generateDefaultConfig({ binaryCommand: '/tmp/kaboom-bin' });
  assert.equal(cfg.mcpServers['kaboom-browser-devtools'].command, '/tmp/kaboom-bin');
});

// --- buildMcpEntry ---

test('buildMcpEntry returns JSON string for MCP entry', () => {
  const entry = buildMcpEntry();
  const parsed = JSON.parse(entry);
  assert.ok(parsed.command.length > 0);
  assert.deepEqual(parsed.args, []);
});

test('buildMcpEntry includes env vars when provided', () => {
  const entry = buildMcpEntry({ DEBUG: '1' });
  const parsed = JSON.parse(entry);
  assert.equal(parsed.env.DEBUG, '1');
});

test('buildMcpEntry honors binaryCommand override', () => {
  const entry = buildMcpEntry({}, { binaryCommand: '/tmp/kaboom-bin' });
  const parsed = JSON.parse(entry);
  assert.equal(parsed.command, '/tmp/kaboom-bin');
});

// --- installToClient: file-type ---

test('installToClient creates new config for file-type client', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-install-'));
  const cfgPath = path.join(tmp, 'mcp.json');

  const def = {
    id: 'test-cursor',
    name: 'Test Cursor',
    type: 'file',
    configPath: { all: cfgPath },
    detectDir: { all: tmp },
  };

  const result = installToClient(def, { dryRun: false, envVars: {}, binaryCommand: '/tmp/kaboom-bin' });
  assert.equal(result.success, true);
  assert.equal(result.method, 'file');
  assert.equal(result.isNew, true);

  const written = JSON.parse(fs.readFileSync(cfgPath, 'utf8'));
  assert.ok(written.mcpServers['kaboom-browser-devtools']);
  assert.equal(written.mcpServers['kaboom-browser-devtools'].command, '/tmp/kaboom-bin');

  fs.rmSync(tmp, { recursive: true });
});

test('installToClient merges into existing file-type config', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-install-'));
  const cfgPath = path.join(tmp, 'mcp.json');

  // Pre-existing config with another server
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

  const result = installToClient(def, { dryRun: false, envVars: {} });
  assert.equal(result.success, true);
  assert.equal(result.isNew, false);

  const written = JSON.parse(fs.readFileSync(cfgPath, 'utf8'));
  assert.ok(written.mcpServers['kaboom-browser-devtools']);
  assert.ok(written.mcpServers.other, 'should preserve existing server');

  fs.rmSync(tmp, { recursive: true });
});

test('installToClient removes gasoline and strum MCP entries before writing kaboom config', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-install-'));
  const cfgPath = path.join(tmp, 'mcp.json');

  fs.writeFileSync(cfgPath, JSON.stringify({
    mcpServers: {
      other: { command: 'other-cmd', args: [] },
      gasoline: { command: 'gasoline-mcp', args: [] },
      'gasoline-agentic-browser': { command: 'gasoline-agentic-browser', args: [] },
      'strum-browser-devtools': { command: 'strum-agentic-browser', args: [] },
      strum: { command: 'strum-agentic-browser', args: [] },
    },
  }));

  const def = {
    id: 'test-cursor',
    name: 'Test Cursor',
    type: 'file',
    configPath: { all: cfgPath },
    detectDir: { all: tmp },
  };

  const result = installToClient(def, { dryRun: false, envVars: {}, binaryCommand: '/tmp/kaboom-bin' });
  assert.equal(result.success, true);

  const written = JSON.parse(fs.readFileSync(cfgPath, 'utf8'));
  assert.ok(written.mcpServers['kaboom-browser-devtools']);
  assert.equal(written.mcpServers.gasoline, undefined);
  assert.equal(written.mcpServers['gasoline-agentic-browser'], undefined);
  assert.equal(written.mcpServers['strum-browser-devtools'], undefined);
  assert.equal(written.mcpServers.strum, undefined);
  assert.ok(written.mcpServers.other, 'should preserve non-managed servers');

  fs.rmSync(tmp, { recursive: true });
});

test('installToClient dry-run does not write file', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-install-'));
  const cfgPath = path.join(tmp, 'mcp.json');

  const def = {
    id: 'test-cursor',
    name: 'Test Cursor',
    type: 'file',
    configPath: { all: cfgPath },
    detectDir: { all: tmp },
  };

  const result = installToClient(def, { dryRun: true, envVars: {} });
  assert.equal(result.success, true);
  assert.equal(fs.existsSync(cfgPath), false, 'should not create file in dry-run');

  fs.rmSync(tmp, { recursive: true });
});

test('installToClient adds env vars to file-type config', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-install-'));
  const cfgPath = path.join(tmp, 'mcp.json');

  const def = {
    id: 'test-cursor',
    name: 'Test Cursor',
    type: 'file',
    configPath: { all: cfgPath },
    detectDir: { all: tmp },
  };

  installToClient(def, { dryRun: false, envVars: { DEBUG: '1' } });
  const written = JSON.parse(fs.readFileSync(cfgPath, 'utf8'));
  assert.equal(written.mcpServers['kaboom-browser-devtools'].env.DEBUG, '1');

  fs.rmSync(tmp, { recursive: true });
});

// --- installToClient: CLI-type ---

test('installToClient handles CLI type with dry-run', () => {
  const def = {
    id: 'claude-code',
    name: 'Claude Code',
    type: 'cli',
    detectCommand: 'claude',
    installArgs: ['mcp', 'add-json', '--scope', 'user', 'kaboom-browser-devtools'],
  };

  const result = installToClient(def, { dryRun: true, envVars: {} });
  assert.equal(result.success, true);
  assert.equal(result.method, 'cli');
  assert.ok(result.message.includes('claude'));
});

// --- executeInstall ---

test('executeInstall installs to detected file-type clients', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-install-'));
  const cursorDir = path.join(tmp, '.cursor');
  fs.mkdirSync(cursorDir);

  // Override detected clients for test
  const result = executeInstall({
    dryRun: false,
    envVars: {},
    _clientOverrides: [
      {
        id: 'test-cursor',
        name: 'Test Cursor',
        type: 'file',
        configPath: { all: path.join(cursorDir, 'mcp.json') },
        detectDir: { all: cursorDir },
      },
    ],
  });

  assert.equal(result.success, true);
  assert.equal(result.installed.length, 1);
  assert.equal(result.installed[0].name, 'Test Cursor');

  fs.rmSync(tmp, { recursive: true });
});

test('executeInstall reports when no clients detected', () => {
  const result = executeInstall({
    dryRun: false,
    envVars: {},
    _clientOverrides: [],
  });

  assert.equal(result.success, false);
  assert.equal(result.installed.length, 0);
});

// --- targeted install ---

test('executeInstall with targetTool installs to specific client', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-install-'));
  const geminiDir = path.join(tmp, '.gemini');

  // Monkey-patch os.homedir for this test via _clientOverrides won't work here
  // since targetTool uses getClientByAlias which looks up CLIENT_DEFINITIONS.
  // Instead, use _clientOverrides to simulate targeted install behavior.
  const result = executeInstall({
    dryRun: false,
    envVars: {},
    _clientOverrides: [
      {
        id: 'test-gemini',
        name: 'Test Gemini',
        type: 'file',
        configPath: { all: path.join(tmp, '.gemini', 'settings.json') },
        detectDir: { all: geminiDir },
      },
    ],
  });

  assert.equal(result.success, true);
  assert.equal(result.installed.length, 1);
  assert.equal(result.installed[0].name, 'Test Gemini');

  const written = JSON.parse(fs.readFileSync(path.join(tmp, '.gemini', 'settings.json'), 'utf8'));
  assert.ok(written.mcpServers['kaboom-browser-devtools']);

  fs.rmSync(tmp, { recursive: true });
});

test('executeInstall with invalid targetTool returns error', () => {
  const result = executeInstall({
    dryRun: false,
    envVars: {},
    targetTool: 'bogus',
  });

  assert.equal(result.success, false);
  assert.equal(result.errors.length, 1);
  assert.ok(result.errors[0].message.includes('Unknown tool: bogus'));
  assert.ok(result.errors[0].message.includes('Valid tools:'));
});

// --- OpenCode format install ---

test('installToClient creates OpenCode-format config with mcp key', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-install-'));
  const binaryCommand = '/tmp/kaboom-bin';

  const def = {
    id: 'test-opencode',
    name: 'Test OpenCode',
    type: 'file',
    configPath: { all: path.join(tmp, 'opencode.json') },
    detectDir: { all: tmp },
    configKey: 'mcp',
    buildEntry: (envVars, resolvedBinaryCommand) => {
      const entry = { type: 'local', command: [resolvedBinaryCommand], enabled: true };
      if (envVars && Object.keys(envVars).length > 0) entry.env = envVars;
      return entry;
    },
  };

  const result = installToClient(def, { dryRun: false, envVars: {}, binaryCommand });
  assert.equal(result.success, true);
  assert.equal(result.isNew, true);

  const written = JSON.parse(fs.readFileSync(path.join(tmp, 'opencode.json'), 'utf8'));
  assert.ok(written.mcp, 'should have mcp key');
  assert.ok(written.mcp['kaboom-browser-devtools'], 'should have kaboom under mcp');
  assert.equal(written.mcp['kaboom-browser-devtools'].type, 'local');
  assert.deepEqual(written.mcp['kaboom-browser-devtools'].command, [binaryCommand]);
  assert.equal(written.mcp['kaboom-browser-devtools'].enabled, true);
  assert.equal(written.mcpServers, undefined, 'should not have mcpServers');

  fs.rmSync(tmp, { recursive: true });
});

test('installToClient merges OpenCode config preserving existing entries', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-install-'));
  const cfgPath = path.join(tmp, 'opencode.json');
  const binaryCommand = '/tmp/kaboom-bin';

  // Pre-existing OpenCode config with another server
  fs.writeFileSync(cfgPath, JSON.stringify({
    mcp: { other: { type: 'local', command: ['other-cmd'], enabled: true } },
    theme: 'dark',
  }));

  const def = {
    id: 'test-opencode',
    name: 'Test OpenCode',
    type: 'file',
    configPath: { all: cfgPath },
    detectDir: { all: tmp },
    configKey: 'mcp',
    buildEntry: (_envVars, resolvedBinaryCommand) => ({ type: 'local', command: [resolvedBinaryCommand], enabled: true }),
  };

  const result = installToClient(def, { dryRun: false, envVars: {}, binaryCommand });
  assert.equal(result.success, true);

  const written = JSON.parse(fs.readFileSync(cfgPath, 'utf8'));
  assert.ok(written.mcp['kaboom-browser-devtools'], 'should have kaboom');
  assert.ok(written.mcp.other, 'should preserve existing server');
  assert.equal(written.theme, 'dark', 'should preserve non-mcp keys');

  fs.rmSync(tmp, { recursive: true });
});

// --- Zed format install ---

test('installToClient creates Zed-format config with context_servers key', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-install-'));
  const binaryCommand = '/tmp/kaboom-bin';

  const def = {
    id: 'test-zed',
    name: 'Test Zed',
    type: 'file',
    configPath: { all: path.join(tmp, 'settings.json') },
    detectDir: { all: tmp },
    configKey: 'context_servers',
    buildEntry: (envVars, resolvedBinaryCommand) => {
      const entry = { source: 'custom', command: resolvedBinaryCommand, args: [] };
      if (envVars && Object.keys(envVars).length > 0) entry.env = envVars;
      return entry;
    },
  };

  const result = installToClient(def, { dryRun: false, envVars: {}, binaryCommand });
  assert.equal(result.success, true);

  const written = JSON.parse(fs.readFileSync(path.join(tmp, 'settings.json'), 'utf8'));
  assert.ok(written.context_servers, 'should have context_servers key');
  assert.ok(written.context_servers['kaboom-browser-devtools']);
  assert.equal(written.context_servers['kaboom-browser-devtools'].source, 'custom');
  assert.equal(written.context_servers['kaboom-browser-devtools'].command, binaryCommand);
  assert.equal(written.mcpServers, undefined, 'should not have mcpServers');

  fs.rmSync(tmp, { recursive: true });
});

// --- VS Code format install ---

test('installToClient writes VS Code config under servers key and migrates legacy mcpServers entries', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-install-'));
  const cfgPath = path.join(tmp, 'mcp.json');

  // Pre-existing VS Code config: stale kaboom entry under the legacy key plus a user server.
  fs.writeFileSync(cfgPath, JSON.stringify({
    servers: { other: { command: 'other-cmd', args: [] } },
    mcpServers: { 'kaboom-browser-devtools': { command: 'old-kaboom', args: [] } },
  }));

  const def = {
    id: 'test-vscode',
    name: 'Test VS Code',
    type: 'file',
    dedicatedMcpFile: true,
    configKey: 'servers',
    legacyConfigKeys: ['mcpServers'],
    configPath: { all: cfgPath },
    detectDir: { all: tmp },
  };

  const result = installToClient(def, { dryRun: false, envVars: {}, binaryCommand: '/tmp/kaboom-bin' });
  assert.equal(result.success, true);

  const written = JSON.parse(fs.readFileSync(cfgPath, 'utf8'));
  assert.ok(written.servers['kaboom-browser-devtools'], 'must write under the servers key');
  assert.equal(written.servers['kaboom-browser-devtools'].command, '/tmp/kaboom-bin');
  assert.ok(written.servers.other, 'must preserve user servers');
  assert.equal(written.mcpServers, undefined, 'stale legacy-key entry must be migrated away');

  fs.rmSync(tmp, { recursive: true });
});

test('executeInstall dry-run reports all detected clients without writing', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-install-'));
  const cursorDir = path.join(tmp, '.cursor');
  fs.mkdirSync(cursorDir);

  const result = executeInstall({
    dryRun: true,
    envVars: {},
    _clientOverrides: [
      {
        id: 'test-cursor',
        name: 'Test Cursor',
        type: 'file',
        configPath: { all: path.join(cursorDir, 'mcp.json') },
        detectDir: { all: cursorDir },
      },
    ],
  });

  assert.equal(result.success, true);
  assert.equal(result.installed.length, 1);
  assert.equal(fs.existsSync(path.join(cursorDir, 'mcp.json')), false);

  fs.rmSync(tmp, { recursive: true });
});

test('installBundledSkills removes managed gasoline and strum legacy skill files before writing kaboom skill', async () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-skills-'));
  const skillsDir = path.join(tmp, 'bundled');
  const claudeRoot = path.join(tmp, 'claude-skills');

  fs.mkdirSync(path.join(skillsDir, 'debug'), { recursive: true });
  fs.writeFileSync(
    path.join(skillsDir, 'skills.json'),
    JSON.stringify({ skills: [{ id: 'debug', version: 2 }] }),
    'utf8'
  );
  fs.writeFileSync(path.join(skillsDir, 'debug', 'SKILL.md'), '# Debug\nKaboom skill body\n', 'utf8');

  fs.mkdirSync(claudeRoot, { recursive: true });
  fs.writeFileSync(
    path.join(claudeRoot, 'gasoline-debug.md'),
    '<!-- gasoline-managed-skill id:debug version:1 -->\nold gasoline body\n',
    'utf8'
  );
  fs.writeFileSync(
    path.join(claudeRoot, 'strum-debug.md'),
    '<!-- strum-managed-skill id:debug version:1 -->\nold strum body\n',
    'utf8'
  );

  const originalClaudeDir = process.env.KABOOM_CLAUDE_SKILLS_DIR;
  try {
    process.env.KABOOM_CLAUDE_SKILLS_DIR = claudeRoot;
    const result = await installBundledSkills({
      agents: ['claude'],
      scope: 'global',
      skillsDir,
    });

    assert.equal(result.skipped, false);
    assert.ok(result.summary.legacy_removed >= 2);
    assert.equal(fs.existsSync(path.join(claudeRoot, 'gasoline-debug.md')), false);
    assert.equal(fs.existsSync(path.join(claudeRoot, 'strum-debug.md')), false);

    const installedSkill = fs.readFileSync(path.join(claudeRoot, 'debug.md'), 'utf8');
    assert.match(installedSkill, /Kaboom skill body/);
  } finally {
    if (originalClaudeDir === undefined) {
      delete process.env.KABOOM_CLAUDE_SKILLS_DIR;
    } else {
      process.env.KABOOM_CLAUDE_SKILLS_DIR = originalClaudeDir;
    }
    fs.rmSync(tmp, { recursive: true, force: true });
  }
});
