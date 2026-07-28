// Purpose: Implement config.js behavior for npm wrapper command flows.
// Why: Keeps distribution-channel behavior consistent and supportable.
// Docs: docs/features/feature/enhanced-cli-config/index.md

/**
 * Config file utilities for the Kaboom MCP CLI
 * Handles reading, writing, validating, and merging MCP configurations
 */

const fs = require('fs');
const path = require('path');
const os = require('os');
const { execFileSync } = require('child_process');
const {
  InvalidJSONError,
  PermissionError,
  ConfigValidationError,
  FileSizeError,
} = require('./errors');

const MAX_CONFIG_SIZE = 1024 * 1024; // 1MB
const MCP_SERVER_NAME = 'kaboom-browser-devtools';

/**
 * Resolve the managed Kaboom binary path from the installed npm package layout.
 * Falls back to command name when an absolute binary path cannot be discovered.
 * @returns {string} Absolute binary path when discoverable, else command name
 */
function resolveManagedBinaryPath(deps = {}) {
  const env = deps.env || process.env;
  const platformName = deps.platform || process.platform;
  const archName = deps.arch || process.arch;
  const existsFn = deps.existsFn || fs.existsSync;
  const packageRoot = deps.packageRoot || path.resolve(__dirname, '..');

  const envOverride = env.KABOOM_BINARY_PATH;
  if (envOverride && existsFn(envOverride)) {
    return path.resolve(envOverride);
  }

  const platformMap = { darwin: 'darwin', linux: 'linux', win32: 'win32' };
  const archMap = { x64: 'x64', arm64: 'arm64' };
  const platform = platformMap[platformName];
  const arch = archMap[archName];
  if (!platform || !arch) {
    return 'kaboom-agentic-browser';
  }

  const effectiveArch = platform === 'win32' ? 'x64' : arch;
  const ext = platform === 'win32' ? '.exe' : '';
  const platformKey = `${platform}-${effectiveArch}`;
  const binaryName = `kaboom-agentic-browser${ext}`;
  const pkgName = `@brennhill/kaboom-agentic-browser-${platformKey}`;

  const candidates = [
    path.join(packageRoot, 'node_modules', pkgName, 'bin', binaryName),
    path.join(packageRoot, '..', pkgName, 'bin', binaryName),
    path.join(packageRoot, '..', '..', pkgName, 'bin', binaryName),
  ];

  // Dev source tree only: when running from <repo>/npm/kaboom-agentic-browser
  // (parent dir "npm", never "node_modules"), prefer the freshly built repo-root
  // dist binary so --install and daemon-start use it instead of a stale global on
  // PATH. Skipped for an installed package, so a dist/ planted in a user's
  // project can never be resolved. Name matches the Makefile's dist output
  // ($(BINARY_NAME)-<platformKey>), not the old kaboom-<platformKey>.
  if (path.basename(path.dirname(packageRoot)) === 'npm') {
    candidates.push(path.resolve(packageRoot, '..', '..', 'dist', `kaboom-agentic-browser-${platformKey}${ext}`));
  }

  for (const candidate of candidates) {
    if (existsFn(candidate)) {
      return path.resolve(candidate);
    }
  }

  return 'kaboom-agentic-browser';
}

/**
 * Client definitions for all supported AI assistant clients.
 * Each entry describes detection, config path, and install strategy.
 */
const CLIENT_DEFINITIONS = [
  {
    id: 'claude-code',
    name: 'Claude Code',
    type: 'cli',
    detectCommand: 'claude',
    installArgs: ['mcp', 'add-json', '--scope', 'user', MCP_SERVER_NAME],
    removeArgs: ['mcp', 'remove', '--scope', 'user', MCP_SERVER_NAME],
    // Auto-approve: write `mcp__<server>` into ~/.claude/settings.json
    // permissions.allow (a bare server rule approves ALL its tools).
    autoApprove: { kind: 'claude-settings' },
  },
  {
    id: 'claude-desktop',
    name: 'Claude Desktop',
    type: 'file',
    // No config-file auto-approve exists — claude_desktop_config.json has no
    // official trust/autoApprove field (only third-party injection hacks add
    // one). Tool approval is a UI action ("Allow for this chat"/"Always allow").
    autoApprove: { kind: 'ui-only', note: 'Approve in-app; no config field' },
    // claude_desktop_config.json is a dedicated MCP config: safe to delete when emptied.
    dedicatedMcpFile: true,
    configPath: {
      darwin: '~/Library/Application Support/Claude/claude_desktop_config.json',
      win32: '%APPDATA%/Claude/claude_desktop_config.json',
    },
    detectDir: {
      darwin: '~/Library/Application Support/Claude',
      win32: '%APPDATA%/Claude',
    },
  },
  {
    id: 'cursor',
    name: 'Cursor',
    type: 'file',
    dedicatedMcpFile: true,
    // No config-file auto-approve: mcp.json has no trust field; auto-run is a
    // UI setting (Run Modes / "auto-run").
    autoApprove: { kind: 'ui-only', note: 'Enable auto-run in Cursor settings' },
    configPath: { all: '~/.cursor/mcp.json' },
    detectDir: { all: '~/.cursor' },
  },
  {
    id: 'windsurf',
    name: 'Windsurf',
    type: 'file',
    dedicatedMcpFile: true,
    // No config-file auto-approve: mcp_config.json has no trust field;
    // auto-execution is a UI setting (Cascade Turbo / "allow every tool").
    autoApprove: { kind: 'ui-only', note: 'Allow the server in Cascade (Turbo)' },
    configPath: { all: '~/.codeium/windsurf/mcp_config.json' },
    detectDir: { all: '~/.codeium/windsurf' },
  },
  {
    id: 'vscode',
    name: 'VS Code',
    type: 'file',
    dedicatedMcpFile: true,
    // No per-server config auto-approve: mcp.json has no trust field, and the
    // only file-based auto-approve (settings.json `chat.tools.global.autoApprove`)
    // trusts EVERY tool of EVERY server globally — out of scope for a
    // Kaboom-scoped installer, so we don't write it. Per-server = UI ("Always
    // Allow").
    autoApprove: { kind: 'ui-only', note: 'Use "Always Allow"; only a global auto-approve exists' },
    // VS Code's mcp.json uses a top-level "servers" key.
    configKey: 'servers',
    configPath: {
      darwin: '~/Library/Application Support/Code/User/mcp.json',
      win32: '%APPDATA%/Code/User/mcp.json',
      linux: '~/.config/Code/User/mcp.json',
    },
    detectDir: {
      darwin: '~/Library/Application Support/Code',
      win32: '%APPDATA%/Code',
      linux: '~/.config/Code',
    },
  },
  {
    id: 'gemini',
    name: 'Gemini CLI',
    type: 'file',
    // Auto-approve: `trust: true` on the server entry bypasses all tool-call
    // confirmations for that server.
    autoApprove: { kind: 'gemini-trust' },
    configPath: { all: '~/.gemini/settings.json' },
    detectDir: { all: '~/.gemini' },
  },
  {
    id: 'opencode',
    name: 'OpenCode',
    type: 'file',
    // Auto-approve: top-level `permission` map, `<server>_*` => "allow".
    autoApprove: { kind: 'opencode-permission' },
    configPath: { all: '~/.config/opencode/opencode.json' },
    detectDir: { all: '~/.config/opencode' },
    configKey: 'mcp',
    buildEntry: (envVars, binaryCommand = 'kaboom-agentic-browser') => {
      const entry = { type: 'local', command: [binaryCommand], enabled: true };
      if (envVars && Object.keys(envVars).length > 0) entry.env = envVars;
      return entry;
    },
  },
  {
    id: 'antigravity',
    name: 'Antigravity',
    type: 'file',
    dedicatedMcpFile: true,
    // No config-file auto-approve for the IDE: mcp_config.json has no trust
    // field, and auto-approve lives in a separate UI-managed permissions policy.
    autoApprove: { kind: 'ui-only', note: 'Approve in Antigravity UI; mcp_config.json has no trust field' },
    // Antigravity uses the home-dir path on every OS (matches the Go installer).
    configPath: { all: '~/.gemini/antigravity/mcp_config.json' },
    detectDir: { all: '~/.gemini/antigravity' },
  },
  {
    id: 'zed',
    name: 'Zed',
    type: 'file',
    // Auto-approve: agent.tool_permissions.tools["mcp:<server>:<tool>"] =
    // {default:"allow"} for each tool (Zed has no server-level wildcard).
    autoApprove: { kind: 'zed-tool-permissions' },
    configPath: { all: '~/.config/zed/settings.json' },
    detectDir: { all: '~/.config/zed' },
    configKey: 'context_servers',
    buildEntry: (envVars, binaryCommand = 'kaboom-agentic-browser') => {
      const entry = { source: 'custom', command: binaryCommand, args: [] };
      if (envVars && Object.keys(envVars).length > 0) entry.env = envVars;
      return entry;
    },
  },
  {
    id: 'codex',
    name: 'Codex CLI',
    type: 'file',
    // Codex config is TOML — handled by lib/codex-config.js, not the JSON path.
    format: 'toml',
    // config.toml is a shared settings file; never delete it.
    // Auto-approve: default_tools_approval_mode = "approve" trusts all tools.
    autoApprove: { kind: 'codex-toml' },
    // $CODEX_HOME overrides ~/.codex when set (honored via envHome below).
    envHome: 'CODEX_HOME',
    envHomeFile: 'config.toml',
    configPath: { all: '~/.codex/config.toml' },
    detectDir: { all: '~/.codex' },
  },
];

/**
 * Expand ~ and %APPDATA% in a path string
 * @param {string} p Path with ~ or %APPDATA%
 * @returns {string} Expanded path
 */
function expandPath(p) {
  if (!p) return p;
  let expanded = p.replace(/^~/, os.homedir());
  if (process.platform === 'win32' && expanded.includes('%APPDATA%')) {
    expanded = expanded.replace(/%APPDATA%/g, process.env.APPDATA || '');
  }
  return path.normalize(expanded);
}

/**
 * Resolve a client's env-var home override (e.g. $CODEX_HOME) when the
 * definition declares one and the env var is set. Returns null otherwise, so
 * all clients without `envHome` behave exactly as before.
 * @param {Object} def Client definition
 * @returns {string|null} Expanded override directory, or null
 */
function resolveEnvHome(def) {
  if (!def.envHome) return null;
  const val = process.env[def.envHome];
  return val ? expandPath(val) : null;
}

/**
 * Get resolved config path for a file-type client definition
 * @param {Object} def Client definition
 * @param {string} [platform] Platform override (defaults to os.platform())
 * @returns {string|null} Resolved path or null if not applicable
 */
function getClientConfigPath(def, platform) {
  if (def.type === 'cli') return null;
  const envHome = resolveEnvHome(def);
  if (envHome && def.envHomeFile) {
    return path.normalize(path.join(envHome, def.envHomeFile));
  }
  const plat = platform || os.platform();
  const raw = def.configPath[plat] || def.configPath.all || null;
  return raw ? expandPath(raw) : null;
}

/**
 * Get resolved detect directory for a file-type client definition
 * @param {Object} def Client definition
 * @param {string} [platform] Platform override
 * @returns {string|null} Resolved path or null
 */
function getClientDetectDir(def, platform) {
  if (def.type === 'cli') return null;
  const envHome = resolveEnvHome(def);
  if (envHome) return envHome;
  const plat = platform || os.platform();
  const raw = def.detectDir[plat] || def.detectDir.all || null;
  return raw ? expandPath(raw) : null;
}

/**
 * Check if a command exists on PATH
 * @param {string} cmd Command name
 * @returns {boolean}
 */
function commandExistsOnPath(cmd) {
  try {
    const checkCmd = process.platform === 'win32' ? 'where' : 'which';
    execFileSync(checkCmd, [cmd], { stdio: 'pipe', timeout: 3000 });
    return true;
  } catch {
    return false;
  }
}

/**
 * Check if a client is installed/detected on this system
 * @param {Object} def Client definition
 * @returns {boolean}
 */
function isClientInstalled(def) {
  if (def.type === 'cli') {
    return commandExistsOnPath(def.detectCommand);
  }
  const dir = getClientDetectDir(def);
  if (!dir) return false;
  try {
    return fs.statSync(dir).isDirectory();
  } catch {
    return false;
  }
}

/**
 * Get all detected (installed) clients
 * @returns {Array<Object>} Detected client definitions
 */
function getDetectedClients() {
  return CLIENT_DEFINITIONS.filter(def => isClientInstalled(def));
}

/**
 * Find a client definition by ID
 * @param {string} id Client ID
 * @returns {Object|undefined}
 */
function getClientById(id) {
  return CLIENT_DEFINITIONS.find(def => def.id === id);
}

/**
 * Short-name aliases for targeted install (maps to client IDs)
 */
const CLIENT_ALIASES = {
  'claude': 'claude-code',
  'claude-code': 'claude-code',
  'claude-desktop': 'claude-desktop',
  'desktop': 'claude-desktop',
  'cursor': 'cursor',
  'windsurf': 'windsurf',
  'vscode': 'vscode',
  'vs-code': 'vscode',
  'gemini': 'gemini',
  'opencode': 'opencode',
  'antigravity': 'antigravity',
  'zed': 'zed',
  'codex': 'codex',
};

/**
 * Look up a client definition by alias name
 * @param {string} alias Short name (e.g. 'gemini', 'cursor')
 * @returns {Object|null} Client definition or null if not found
 */
function getClientByAlias(alias) {
  const id = CLIENT_ALIASES[alias.toLowerCase()];
  if (!id) return null;
  return getClientById(id);
}

/**
 * Get all valid alias names (for error messages)
 * @returns {Array<string>} Unique alias names (one per client)
 */
function getValidAliases() {
  const seen = new Set();
  const aliases = [];
  for (const [alias, id] of Object.entries(CLIENT_ALIASES)) {
    if (!seen.has(id)) {
      seen.add(id);
      aliases.push(alias);
    }
  }
  return aliases;
}

/**
 * Read and parse a config file
 * @param {string} filePath Path to config file
 * @returns {Object} {valid: bool, data: obj, error: string, stats: {size, mtime}}
 */
function readConfigFile(filePath) {
  try {
    // Check file size
    const stats = fs.statSync(filePath);
    if (stats.size > MAX_CONFIG_SIZE) {
      throw new FileSizeError(filePath, stats.size);
    }

    // Read and parse
    const content = fs.readFileSync(filePath, 'utf8');
    let data;
    try {
      data = JSON.parse(content);
    } catch (parseErr) {
      // Try to find line number
      const lines = content.split('\n');
      let lineNumber = 1;
      for (let i = 0; i < lines.length; i++) {
        try {
          JSON.parse(lines.slice(0, i + 1).join('\n'));
        } catch (e) {
          lineNumber = i + 1;
          break;
        }
      }
      throw new InvalidJSONError(filePath, lineNumber, parseErr.message);
    }

    return {
      valid: true,
      data,
      error: null,
      stats: { size: stats.size, mtime: stats.mtime },
    };
  } catch (err) {
    if (err instanceof InvalidJSONError || err instanceof FileSizeError) {
      throw err;
    }
    // File doesn't exist or can't be read
    return {
      valid: false,
      data: null,
      error: err.message,
      stats: null,
    };
  }
}

/**
 * Write config file (with optional dry-run)
 * Atomic write: temp file + rename
 * @param {string} filePath Path to config file
 * @param {Object} data Config object to write
 * @param {boolean} dryRun If true, returns what would be written without writing
 * @returns {Object} {success: bool, message: string, path: string, before?: Object, after?: Object}
 */
function writeConfigFile(filePath, data, dryRun = false, options = {}) {
  try {
    // Validate data (skip for non-standard config formats like OpenCode)
    if (!options.skipValidation) {
      const errors = validateMCPConfig(data);
      if (errors.length > 0) {
        throw new ConfigValidationError(errors);
      }
    }

    const jsonStr = JSON.stringify(data, null, 2);

    if (dryRun) {
      return {
        success: true,
        message: `Would write to ${filePath}`,
        path: filePath,
        after: data,
      };
    }

    // Ensure directory exists
    const dir = path.dirname(filePath);
    fs.mkdirSync(dir, { recursive: true });

    // Atomic write: temp file + rename
    const tempPath = `${filePath}.tmp`;
    try {
      fs.writeFileSync(tempPath, jsonStr + '\n', 'utf8');
      fs.renameSync(tempPath, filePath);
    } catch (writeErr) {
      // Clean up temp file if it exists
      try {
        fs.unlinkSync(tempPath);
      } catch (e) {
        // Ignore cleanup errors
      }

      if (writeErr.code === 'EACCES') {
        throw new PermissionError(filePath);
      }
      throw writeErr;
    }

    return {
      success: true,
      message: `Wrote to ${filePath}`,
      path: filePath,
      after: data,
    };
  } catch (err) {
    if (err instanceof ConfigValidationError || err instanceof PermissionError) {
      throw err;
    }
    throw err;
  }
}

/**
 * Validate MCP config structure
 * @param {Object} data Config object to validate
 * @returns {Array<string>} Array of error messages (empty if valid)
 */
function validateMCPConfig(data) {
  const errors = [];

  if (!data || typeof data !== 'object') {
    errors.push('Config must be an object');
    return errors;
  }

  if (!data.mcpServers) {
    errors.push('Config must have "mcpServers" property');
  } else if (typeof data.mcpServers !== 'object' || Array.isArray(data.mcpServers)) {
    errors.push('"mcpServers" must be an object (not an array)');
  }

  return errors;
}

/**
 * Merge Kaboom config into existing config
 * @param {Object} existing Existing config object
 * @param {Object} kaboomEntry New Kaboom entry {command, args, env}
 * @param {Object} envVars Additional env vars to merge {KEY: VALUE}
 * @returns {Object} Merged config
 */
function mergeKaboomConfig(existing, kaboomEntry, envVars = {}) {
  const merged = JSON.parse(JSON.stringify(existing)); // Deep copy

  // Ensure mcpServers exists
  if (!merged.mcpServers) {
    merged.mcpServers = {};
  }

  // Merge Kaboom entry
  merged.mcpServers[MCP_SERVER_NAME] = {
    command: kaboomEntry.command,
    args: kaboomEntry.args || [],
  };

  // Add env vars if provided
  if (envVars && Object.keys(envVars).length > 0) {
    merged.mcpServers[MCP_SERVER_NAME].env = envVars;
  }

  return merged;
}

/**
 * Parse and validate env var string (KEY=VALUE)
 * @param {string} envStr Environment variable string
 * @returns {Object} {key: string, value: string} or throws InvalidEnvFormatError
 */
function parseEnvVar(envStr) {
  const { InvalidEnvFormatError } = require('./errors');
  const raw = String(envStr ?? '');
  // Split on the FIRST '=' only — values may legitimately contain '='
  // (e.g. TOKEN=abc=def, base64 payloads, URLs with query strings).
  const idx = raw.indexOf('=');
  if (idx <= 0) {
    throw new InvalidEnvFormatError(envStr);
  }
  const key = raw.slice(0, idx);
  const value = raw.slice(idx + 1);
  if (!value) {
    throw new InvalidEnvFormatError(envStr);
  }

  // Validate key (no null bytes or control chars)
  if (!/^[A-Z_][A-Z0-9_]*$/i.test(key)) {
    throw new InvalidEnvFormatError(envStr);
  }

  return { key, value };
}

/**
 * True when any of the given env keys is set to a truthy opt-in value.
 * Unset/empty/"0"/"false"/"no" all count as off. Shared source of truth for the
 * install-time opt-outs (auto-open, daemon-start, connect-wait) so their
 * accepted values never drift apart.
 * @param {object} env
 * @param {string[]} keys
 * @returns {boolean}
 */
function isEnvFlagSet(env, keys) {
  for (const key of keys) {
    const value = String((env && env[key]) || '').trim().toLowerCase();
    if (value && value !== '0' && value !== 'false' && value !== 'no') return true;
  }
  return false;
}

module.exports = {
  CLIENT_DEFINITIONS,
  CLIENT_ALIASES,
  MCP_SERVER_NAME,
  expandPath,
  getClientConfigPath,
  getClientDetectDir,
  commandExistsOnPath,
  isClientInstalled,
  getDetectedClients,
  getClientById,
  getClientByAlias,
  getValidAliases,
  readConfigFile,
  writeConfigFile,
  validateMCPConfig,
  mergeKaboomConfig,
  parseEnvVar,
  resolveManagedBinaryPath,
  isEnvFlagSet,
};
