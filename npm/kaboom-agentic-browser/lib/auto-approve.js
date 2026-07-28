// auto-approve.js — Config-based whole-server MCP tool auto-approval.
// Why: Kaboom is the user's own local tool on their own machine, so the
// installer trusts ALL of its tools by default (default-ON) in every client
// that exposes a config-file trust mechanism — no per-call approval prompt.
// Clients whose only auto-approve is a UI toggle (or a global "trust everything"
// switch) are left untouched and reported as UI-only; we never invent a field.
// Docs: docs/features/feature/enhanced-cli-config/index.md

const fs = require('fs');
const os = require('os');
const path = require('path');
const {
  MCP_SERVER_NAME,
  readConfigFile,
  writeConfigFile,
} = require('./config');

// The five Kaboom MCP tools. Needed where a client cannot wildcard a whole
// server and each tool must be named individually (Zed).
const KABOOM_TOOL_NAMES = ['observe', 'generate', 'configure', 'interact', 'analyze'];

// --- Claude Code: ~/.claude/settings.json permissions.allow ---
// A bare `mcp__<server>` (no tool suffix) wildcard-approves EVERY tool of that
// server. Verified: https://code.claude.com/docs/en/permissions
const CLAUDE_ALLOW_RULE = `mcp__${MCP_SERVER_NAME}`;

// --- OpenCode: opencode.json top-level `permission` object ---
// `<server>_*` covers every tool of the server.
// Verified: https://opencode.ai/docs/permissions/ + https://opencode.ai/docs/tools/
const OPENCODE_PERMISSION_KEY = `${MCP_SERVER_NAME}_*`;

// --- Zed: settings.json agent.tool_permissions.tools ---
// MCP tools are referenced as `mcp:<server>:<tool>`; Zed has no server-level
// wildcard, so each tool is enumerated.
// Verified: https://zed.dev/docs/ai/tool-permissions
function zedToolRefs(serverName) {
  return KABOOM_TOOL_NAMES.map((t) => `mcp:${serverName}:${t}`);
}

/**
 * Get parent[key] as a plain object, creating it when absent. Returns null when
 * an existing value is a non-object (malformed) so callers refuse to clobber it.
 */
function ensureObject(parent, key) {
  if (parent[key] === undefined || parent[key] === null) {
    parent[key] = {};
    return parent[key];
  }
  if (typeof parent[key] !== 'object' || Array.isArray(parent[key])) {
    return null;
  }
  return parent[key];
}

/**
 * Apply the same-file (JSON) auto-approve for a client into an already-parsed
 * config object (the same object that carries the freshly-merged server entry),
 * so the install writes server + trust in one atomic write.
 * @param {Object} def Client definition (reads def.autoApprove.kind + def.configKey)
 * @param {Object} configData Parsed config to mutate in place
 * @returns {boolean} true when auto-approve was written
 */
function applyToConfig(def, configData) {
  const kind = def.autoApprove && def.autoApprove.kind;

  if (kind === 'gemini-trust') {
    // trust lives on the server entry itself; the entry is created before this.
    const key = def.configKey || 'mcpServers';
    const servers = configData[key];
    if (servers && typeof servers === 'object' && servers[MCP_SERVER_NAME] && typeof servers[MCP_SERVER_NAME] === 'object') {
      servers[MCP_SERVER_NAME].trust = true;
      return true;
    }
    return false;
  }

  if (kind === 'opencode-permission') {
    const permission = ensureObject(configData, 'permission');
    if (!permission) return false; // malformed existing value — do not clobber
    permission[OPENCODE_PERMISSION_KEY] = 'allow';
    return true;
  }

  if (kind === 'zed-tool-permissions') {
    const agent = ensureObject(configData, 'agent');
    if (!agent) return false;
    const toolPermissions = ensureObject(agent, 'tool_permissions');
    if (!toolPermissions) return false;
    const tools = ensureObject(toolPermissions, 'tools');
    if (!tools) return false;
    for (const ref of zedToolRefs(MCP_SERVER_NAME)) {
      tools[ref] = { default: 'allow' };
    }
    return true;
  }

  return false;
}

/**
 * Remove the same-file (JSON) auto-approve entries for a client from a parsed
 * config object, cleaning up any empty containers we created. Symmetric with
 * applyToConfig. gemini-trust needs nothing here — its trust flag lives on the
 * server entry, which the server-name removal already deletes.
 * @returns {boolean} true when something was removed
 */
function removeFromConfig(def, configData) {
  const kind = def.autoApprove && def.autoApprove.kind;
  let changed = false;

  if (kind === 'opencode-permission') {
    const permission = configData.permission;
    if (permission && typeof permission === 'object' && !Array.isArray(permission)) {
      if (Object.prototype.hasOwnProperty.call(permission, OPENCODE_PERMISSION_KEY)) {
        delete permission[OPENCODE_PERMISSION_KEY];
        changed = true;
      }
      if (changed && Object.keys(permission).length === 0) delete configData.permission;
    }
    return changed;
  }

  if (kind === 'zed-tool-permissions') {
    const agent = configData.agent;
    const toolPermissions = agent && agent.tool_permissions;
    const tools = toolPermissions && toolPermissions.tools;
    if (tools && typeof tools === 'object' && !Array.isArray(tools)) {
      for (const ref of zedToolRefs(MCP_SERVER_NAME)) {
        if (Object.prototype.hasOwnProperty.call(tools, ref)) {
          delete tools[ref];
          changed = true;
        }
      }
      if (changed) {
        if (Object.keys(tools).length === 0) delete toolPermissions.tools;
        if (Object.keys(toolPermissions).length === 0) delete agent.tool_permissions;
        if (Object.keys(agent).length === 0) delete configData.agent;
      }
    }
    return changed;
  }

  return false;
}

/**
 * Whether a client's same-file auto-approve entries exist in a parsed config.
 * Used by uninstall so a config carrying only auto-approve keys (server entry
 * already gone) is still recognized as configured and cleaned.
 * @returns {boolean}
 */
function autoApprovePresent(def, configData) {
  const kind = def.autoApprove && def.autoApprove.kind;

  if (kind === 'opencode-permission') {
    const permission = configData.permission;
    if (permission && typeof permission === 'object') {
      return Object.prototype.hasOwnProperty.call(permission, OPENCODE_PERMISSION_KEY);
    }
    return false;
  }

  if (kind === 'zed-tool-permissions') {
    const tools = configData.agent && configData.agent.tool_permissions && configData.agent.tool_permissions.tools;
    if (tools && typeof tools === 'object') {
      return zedToolRefs(MCP_SERVER_NAME)
        .some((ref) => Object.prototype.hasOwnProperty.call(tools, ref));
    }
    return false;
  }

  return false;
}

/** Whether this kind of auto-approve is written into the same JSON config file. */
function isSameFileKind(def) {
  const kind = def.autoApprove && def.autoApprove.kind;
  return kind === 'gemini-trust' || kind === 'opencode-permission' || kind === 'zed-tool-permissions';
}

// --- Claude Code: separate ~/.claude/settings.json ---

/** Default user-scope Claude settings path (env override for hermetic tests). */
function defaultClaudeSettingsPath(homeDir) {
  if (process.env.KABOOM_CLAUDE_SETTINGS_PATH) return process.env.KABOOM_CLAUDE_SETTINGS_PATH;
  return path.join(homeDir || os.homedir(), '.claude', 'settings.json');
}

/**
 * Read a settings.json for Claude Code. Returns a plain object (empty when the
 * file is absent). Throws on malformed JSON or a non-object top-level value so
 * genuine problems surface instead of clobbering the user's file.
 */
function readClaudeSettings(settingsPath) {
  if (!fs.existsSync(settingsPath)) return {};
  const read = readConfigFile(settingsPath); // throws InvalidJSONError on bad JSON
  if (!read.valid) {
    throw new Error(`Cannot read Claude settings at ${settingsPath}: ${read.error}`);
  }
  const data = read.data;
  if (!data || typeof data !== 'object' || Array.isArray(data)) {
    throw new Error(`Refusing to modify malformed Claude settings at ${settingsPath}`);
  }
  return data;
}

/**
 * Add the Kaboom allow rule to ~/.claude/settings.json `permissions.allow`.
 * Merge-safe: preserves every existing setting and rule, dedupes, creates the
 * keys/file when absent.
 * @param {{settingsPath?:string, homeDir?:string, dryRun?:boolean}} [options]
 * @returns {{status:'applied'|'unchanged', path:string, changed:boolean, dryRun?:boolean}}
 */
function applyClaudeSettingsAllow(options = {}) {
  const settingsPath = options.settingsPath || defaultClaudeSettingsPath(options.homeDir);
  const dryRun = !!options.dryRun;
  const data = readClaudeSettings(settingsPath);

  const permissions = (data.permissions && typeof data.permissions === 'object' && !Array.isArray(data.permissions))
    ? data.permissions
    : {};
  const allow = Array.isArray(permissions.allow) ? permissions.allow : [];

  if (allow.includes(CLAUDE_ALLOW_RULE)) {
    return { status: 'unchanged', path: settingsPath, changed: false };
  }
  if (dryRun) {
    return { status: 'applied', path: settingsPath, changed: true, dryRun: true };
  }

  const nextData = {
    ...data,
    permissions: { ...permissions, allow: [...allow, CLAUDE_ALLOW_RULE] },
  };
  writeConfigFile(settingsPath, nextData, false, { skipValidation: true });
  return { status: 'applied', path: settingsPath, changed: true };
}

/**
 * Remove the Kaboom allow rule from ~/.claude/settings.json,
 * pruning now-empty `allow`/`permissions` containers. Never deletes the file
 * (it is a shared user settings file).
 * @returns {{status:'removed'|'notConfigured', path:string, changed:boolean, dryRun?:boolean}}
 */
function removeClaudeSettingsAllow(options = {}) {
  const settingsPath = options.settingsPath || defaultClaudeSettingsPath(options.homeDir);
  const dryRun = !!options.dryRun;
  if (!fs.existsSync(settingsPath)) {
    return { status: 'notConfigured', path: settingsPath, changed: false };
  }
  const data = readClaudeSettings(settingsPath);
  const permissions = data.permissions;
  if (!permissions || typeof permissions !== 'object' || !Array.isArray(permissions.allow)) {
    return { status: 'notConfigured', path: settingsPath, changed: false };
  }

  const nextAllow = permissions.allow.filter((r) => r !== CLAUDE_ALLOW_RULE);
  if (nextAllow.length === permissions.allow.length) {
    return { status: 'notConfigured', path: settingsPath, changed: false };
  }
  if (dryRun) {
    return { status: 'removed', path: settingsPath, changed: true, dryRun: true };
  }

  const nextPermissions = { ...permissions };
  if (nextAllow.length === 0) delete nextPermissions.allow;
  else nextPermissions.allow = nextAllow;

  const nextData = { ...data };
  if (Object.keys(nextPermissions).length === 0) delete nextData.permissions;
  else nextData.permissions = nextPermissions;

  writeConfigFile(settingsPath, nextData, false, { skipValidation: true });
  return { status: 'removed', path: settingsPath, changed: true };
}

module.exports = {
  KABOOM_TOOL_NAMES,
  CLAUDE_ALLOW_RULE,
  OPENCODE_PERMISSION_KEY,
  zedToolRefs,
  applyToConfig,
  removeFromConfig,
  autoApprovePresent,
  isSameFileKind,
  defaultClaudeSettingsPath,
  applyClaudeSettingsAllow,
  removeClaudeSettingsAllow,
};
