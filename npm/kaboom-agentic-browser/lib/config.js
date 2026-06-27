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
const LEGACY_MCP_SERVER_NAMES = [
  'kaboom-browser-devtools',
  'kaboom-agentic-browser',
  'kaboom',
  'gasoline-browser-devtools',
  'gasoline-agentic-browser',
  'gasoline',
  'strum-browser-devtools',
  'strum-agentic-browser',
  'strum',
];

/**
 * Resolve the managed Kaboom binary path from the installed npm package layout.
 * Falls back to command name when an absolute binary path cannot be discovered.
 * @returns {string} Absolute binary path when discoverable, else command name
 */
function resolveManagedBinaryPath() {
  const envOverride = process.env.KABOOM_BINARY_PATH;
  if (envOverride && fs.existsSync(envOverride)) {
    return path.resolve(envOverride);
  }

  const platformMap = { darwin: 'darwin', linux: 'linux', win32: 'win32' };
  const archMap = { x64: 'x64', arm64: 'arm64' };
  const platform = platformMap[process.platform];
  const arch = archMap[process.arch];
  if (!platform || !arch) {
    return 'kaboom-agentic-browser';
  }

  const effectiveArch = platform === 'win32' ? 'x64' : arch;
  const ext = platform === 'win32' ? '.exe' : '';
  const platformKey = `${platform}-${effectiveArch}`;
  const binaryName = `kaboom-agentic-browser${ext}`;
  const pkgName = `@brennhill/kaboom-agentic-browser-${platformKey}`;
  const packageRoot = path.resolve(__dirname, '..');

  const localDistCandidate = path.resolve(packageRoot, 'dist', `kaboom-${platformKey}${ext}`);
  const candidates = [
    path.join(packageRoot, 'node_modules', pkgName, 'bin', binaryName),
    path.join(packageRoot, '..', pkgName, 'bin', binaryName),
    path.join(packageRoot, '..', '..', pkgName, 'bin', binaryName),
  ];
  // Only include the local dist fallback if it resolves within the package boundary
  if (path.resolve(localDistCandidate).startsWith(packageRoot + path.sep)) {
    candidates.push(localDistCandidate);
  }

  for (const candidate of candidates) {
    if (fs.existsSync(candidate)) {
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
  },
  {
    id: 'claude-desktop',
    name: 'Claude Desktop',
    type: 'file',
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
    configPath: { all: '~/.cursor/mcp.json' },
    detectDir: { all: '~/.cursor' },
  },
  {
    id: 'windsurf',
    name: 'Windsurf',
    type: 'file',
    dedicatedMcpFile: true,
    configPath: { all: '~/.codeium/windsurf/mcp_config.json' },
    detectDir: { all: '~/.codeium/windsurf' },
  },
  {
    id: 'vscode',
    name: 'VS Code',
    type: 'file',
    dedicatedMcpFile: true,
    // VS Code's mcp.json uses a top-level "servers" key; "mcpServers" entries
    // were written by older Kaboom versions and must still be cleaned up.
    configKey: 'servers',
    legacyConfigKeys: ['mcpServers'],
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
    configPath: { all: '~/.gemini/settings.json' },
    detectDir: { all: '~/.gemini' },
  },
  {
    id: 'opencode',
    name: 'OpenCode',
    type: 'file',
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
    // Antigravity uses the home-dir path on every OS (matches the Go installer).
    configPath: { all: '~/.gemini/antigravity/mcp_config.json' },
    detectDir: { all: '~/.gemini/antigravity' },
    // Older npm versions wrote to %APPDATA% on Windows; keep cleaning that path.
    legacyConfigPaths: { win32: '%APPDATA%/.gemini/antigravity/mcp_config.json' },
  },
  {
    id: 'zed',
    name: 'Zed',
    type: 'file',
    configPath: { all: '~/.config/zed/settings.json' },
    detectDir: { all: '~/.config/zed' },
    configKey: 'context_servers',
    buildEntry: (envVars, binaryCommand = 'kaboom-agentic-browser') => {
      const entry = { source: 'custom', command: binaryCommand, args: [] };
      if (envVars && Object.keys(envVars).length > 0) entry.env = envVars;
      return entry;
    },
  },
];

/**
 * Legacy paths that may contain orphaned configs from older versions.
 * Used by doctor to warn users.
 */
const LEGACY_PATHS = [
  { path: '~/.codeium/mcp.json', description: 'Old Windsurf/Codeium path' },
  { path: '~/.vscode/claude.mcp.json', description: 'Old VS Code path' },
  { path: '~/.claude.json', description: 'Old Claude Code path (now uses CLI)' },
  { path: '%APPDATA%/.gemini/antigravity/mcp_config.json', description: 'Old Antigravity path (Windows %APPDATA% location)' },
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
 * Get resolved config path for a file-type client definition
 * @param {Object} def Client definition
 * @param {string} [platform] Platform override (defaults to os.platform())
 * @returns {string|null} Resolved path or null if not applicable
 */
function getClientConfigPath(def, platform) {
  if (def.type === 'cli') return null;
  const plat = platform || os.platform();
  const raw = def.configPath[plat] || def.configPath.all || null;
  return raw ? expandPath(raw) : null;
}

/**
 * Get resolved legacy config paths for a file-type client definition.
 * These are paths older versions wrote to; uninstall/doctor still clean them.
 * @param {Object} def Client definition
 * @param {string} [platform] Platform override (defaults to os.platform())
 * @returns {Array<string>} Resolved legacy paths (empty when none apply)
 */
function getClientLegacyConfigPaths(def, platform) {
  if (def.type === 'cli' || !def.legacyConfigPaths) return [];
  const plat = platform || os.platform();
  const raw = def.legacyConfigPaths[plat] || def.legacyConfigPaths.all || null;
  if (!raw) return [];
  const list = Array.isArray(raw) ? raw : [raw];
  return list.map((p) => expandPath(p)).filter(Boolean);
}

/**
 * Get resolved detect directory for a file-type client definition
 * @param {Object} def Client definition
 * @param {string} [platform] Platform override
 * @returns {string|null} Resolved path or null
 */
function getClientDetectDir(def, platform) {
  if (def.type === 'cli') return null;
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
 * Backward-compat: returns config file paths for detected file-type clients.
 * @returns {Array<string>} Array of config file paths
 */
function getConfigCandidates() {
  return CLIENT_DEFINITIONS
    .filter(def => def.type === 'file')
    .map(def => getClientConfigPath(def))
    .filter(Boolean);
}

/**
 * Backward-compat: get tool name from config path
 * @param {string} configPath Path to config file
 * @returns {string} Tool name
 */
function getToolNameFromPath(configPath) {
  const normalized = path.normalize(configPath);
  for (const def of CLIENT_DEFINITIONS) {
    if (def.type !== 'file') continue;
    const cfgPath = getClientConfigPath(def);
    if (cfgPath && normalized === path.normalize(cfgPath)) {
      return def.name;
    }
  }
  // Fallback: substring matching for legacy paths
  if (normalized.includes('.cursor')) return 'Cursor';
  if (normalized.includes(path.join('.codeium', 'windsurf'))) return 'Windsurf';
  if (normalized.includes('.codeium')) return 'Windsurf';
  if (normalized.includes('Claude')) return 'Claude Desktop';
  if (normalized.includes(path.join('.gemini', 'antigravity'))) return 'Antigravity';
  if (normalized.includes('.gemini')) return 'Gemini CLI';
  if (normalized.includes(path.join('.config', 'opencode'))) return 'OpenCode';
  if (normalized.includes(path.join('.config', 'zed'))) return 'Zed';
  if (normalized.includes('Code')) return 'VS Code';
  return 'Unknown';
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

  // Remove legacy keys before writing canonical key.
  for (const legacyName of LEGACY_MCP_SERVER_NAMES) {
    if (legacyName !== MCP_SERVER_NAME) {
      delete merged.mcpServers[legacyName];
    }
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

module.exports = {
  CLIENT_DEFINITIONS,
  CLIENT_ALIASES,
  LEGACY_PATHS,
  MCP_SERVER_NAME,
  LEGACY_MCP_SERVER_NAMES,
  expandPath,
  getClientConfigPath,
  getClientLegacyConfigPaths,
  getClientDetectDir,
  commandExistsOnPath,
  isClientInstalled,
  getDetectedClients,
  getClientById,
  getClientByAlias,
  getValidAliases,
  getConfigCandidates,
  getToolNameFromPath,
  readConfigFile,
  writeConfigFile,
  validateMCPConfig,
  mergeKaboomConfig,
  parseEnvVar,
  resolveManagedBinaryPath,
};
