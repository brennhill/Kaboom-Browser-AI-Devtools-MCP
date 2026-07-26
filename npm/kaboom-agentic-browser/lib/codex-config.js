// codex-config.js — OpenAI Codex CLI (~/.codex/config.toml) MCP registration
// plus whole-server tool auto-approve.
// Why: Codex config is TOML, not JSON, so this module does a conservative,
// comment-preserving SECTION edit (replace/remove only the `[mcp_servers.<name>]`
// block) instead of re-serializing the whole file — the rest of the user's
// config, including comments, is left byte-for-byte intact.
// Auto-approve: `default_tools_approval_mode = "approve"` trusts every tool of
// the server (never prompts). `auto`/`writes` can still prompt for risk-hinted
// tools, so only `approve` guarantees no prompt.
// Verified: https://developers.openai.com/codex/mcp (config.toml schema).
// Docs: docs/features/feature/enhanced-cli-config/index.md

const fs = require('fs');
const path = require('path');
const { MCP_SERVER_NAME, LEGACY_MCP_SERVER_NAMES } = require('./config');

// Only `approve` unconditionally suppresses tool-call prompts.
const APPROVAL_MODE = 'approve';
const APPROVAL_FIELD = 'default_tools_approval_mode';

function knownServerNames() {
  return [...new Set([MCP_SERVER_NAME, ...LEGACY_MCP_SERVER_NAMES])];
}

/** Escape a value for a double-quoted TOML basic string. */
function tomlString(value) {
  return `"${String(value).replace(/\\/g, '\\\\').replace(/"/g, '\\"')}"`;
}

/** A bare TOML key allows [A-Za-z0-9_-]; anything else must be quoted. */
function tomlKey(name) {
  return /^[A-Za-z0-9_-]+$/.test(name) ? name : tomlString(name);
}

/**
 * Return the dotted inner path of a TOML table header line (e.g.
 * `[mcp_servers.foo.env]` -> `mcp_servers.foo.env`), or null when the line is
 * not a table header. Key/value lines start with the key, never `[`.
 */
function headerPath(line) {
  const m = /^\s*\[\[?\s*([^\]]+?)\s*\]\]?\s*(#.*)?$/.exec(line);
  return m ? m[1] : null;
}

/** Whether a table path is `mcp_servers.<name>` (or a sub-table) for a known name. */
function tableTargetsKnownServer(tablePath, names) {
  const prefix = 'mcp_servers.';
  if (!tablePath.startsWith(prefix)) return false;
  const rest = tablePath.slice(prefix.length);
  const m = /^("([^"]*)"|[A-Za-z0-9_-]+)/.exec(rest);
  if (!m) return false;
  const name = m[2] !== undefined ? m[2] : m[1];
  return names.includes(name);
}

/**
 * Drop every `[mcp_servers.<name>]` block (and its sub-tables) for the given
 * server names from TOML text, preserving all other content and comments.
 * @returns {{text:string, changed:boolean}}
 */
function stripServerBlocks(content, names) {
  const lines = content.split('\n');
  const out = [];
  let skipping = false;
  let changed = false;
  for (const line of lines) {
    const tablePath = headerPath(line);
    if (tablePath !== null) {
      skipping = tableTargetsKnownServer(tablePath, names);
    }
    if (skipping) {
      changed = true;
      continue;
    }
    out.push(line);
  }
  return { text: out.join('\n'), changed };
}

/** Build the managed `[mcp_servers.kaboom-browser-devtools]` block. */
function buildServerBlock(binaryCommand, envVars) {
  const key = tomlKey(MCP_SERVER_NAME);
  const lines = [
    `[mcp_servers.${key}]`,
    '# Managed by Kaboom. default_tools_approval_mode="approve" trusts all Kaboom tools.',
    `command = ${tomlString(binaryCommand)}`,
    `${APPROVAL_FIELD} = ${tomlString(APPROVAL_MODE)}`,
  ];
  const env = envVars && Object.keys(envVars).length > 0 ? envVars : null;
  if (env) {
    lines.push(`[mcp_servers.${key}.env]`);
    for (const [k, v] of Object.entries(env)) {
      lines.push(`${tomlKey(k)} = ${tomlString(v)}`);
    }
  }
  return lines.join('\n');
}

/** Atomic text write (temp + rename), mirroring config.writeConfigFile. */
function writeAtomic(filePath, content) {
  const tmp = `${filePath}.tmp`;
  try {
    fs.writeFileSync(tmp, content, 'utf8');
    fs.renameSync(tmp, filePath);
  } catch (err) {
    try {
      fs.unlinkSync(tmp);
    } catch (cleanupErr) {
      // Best-effort temp cleanup; the original write error is what matters.
      void cleanupErr;
    }
    throw err;
  }
}

/**
 * Register (or refresh) the Kaboom MCP server in Codex config.toml with
 * whole-server tool auto-approve. Merge-safe: replaces only our block, keeps
 * everything else. Creates the file/dir when absent.
 * @param {{configPath:string, binaryCommand:string, envVars?:object, dryRun?:boolean}} opts
 * @returns {{success:boolean, path:string, isNew:boolean, autoApprove:string, dryRun?:boolean}}
 */
function installCodex(opts) {
  const { configPath, binaryCommand, envVars = {}, dryRun = false } = opts;
  const existing = fs.existsSync(configPath) ? fs.readFileSync(configPath, 'utf8') : '';
  const isNew = existing === '';
  const { text: stripped } = stripServerBlocks(existing, knownServerNames());
  const block = buildServerBlock(binaryCommand, envVars);
  const head = stripped.replace(/\s*$/, '');
  const next = head.length > 0 ? `${head}\n\n${block}\n` : `${block}\n`;

  if (dryRun) {
    return { success: true, path: configPath, isNew, autoApprove: 'would-apply', dryRun: true };
  }
  fs.mkdirSync(path.dirname(configPath), { recursive: true });
  writeAtomic(configPath, next);
  return { success: true, path: configPath, isNew, autoApprove: 'applied' };
}

/**
 * Remove the Kaboom (and legacy) MCP server blocks from Codex config.toml.
 * Never deletes the file (config.toml is a shared settings file).
 * @param {{configPath:string, dryRun?:boolean}} opts
 * @returns {{status:'removed'|'notConfigured', path:string}}
 */
function uninstallCodex(opts) {
  const { configPath, dryRun = false } = opts;
  if (!fs.existsSync(configPath)) return { status: 'notConfigured', path: configPath };
  const existing = fs.readFileSync(configPath, 'utf8');
  const { text, changed } = stripServerBlocks(existing, knownServerNames());
  if (!changed) return { status: 'notConfigured', path: configPath };
  if (dryRun) return { status: 'removed', path: configPath };
  // Collapse blank-line runs left by the removed block; keep a trailing newline.
  const collapsed = text.replace(/\n{3,}/g, '\n\n').replace(/^\n+/, '').replace(/\s*$/, '');
  writeAtomic(configPath, collapsed.length ? `${collapsed}\n` : '');
  return { status: 'removed', path: configPath };
}

/**
 * Diagnose whether the Kaboom (or a legacy) server table exists in config.toml.
 * @param {string} configPath
 * @returns {{configured:boolean, exists:boolean, matchedName?:string, error?:string}}
 */
function codexServerConfigured(configPath) {
  if (!fs.existsSync(configPath)) return { configured: false, exists: false };
  let content;
  try {
    content = fs.readFileSync(configPath, 'utf8');
  } catch (err) {
    return { configured: false, exists: true, error: err.message };
  }
  const names = knownServerNames();
  for (const line of content.split('\n')) {
    const tablePath = headerPath(line);
    if (tablePath === null) continue;
    for (const name of names) {
      if (tablePath === `mcp_servers.${name}` || tablePath === `mcp_servers.${tomlString(name)}`) {
        return { configured: true, exists: true, matchedName: name };
      }
    }
  }
  return { configured: false, exists: true };
}

module.exports = {
  APPROVAL_MODE,
  APPROVAL_FIELD,
  installCodex,
  uninstallCodex,
  codexServerConfigured,
  // exported for tests
  stripServerBlocks,
  buildServerBlock,
  tomlString,
  tomlKey,
};
