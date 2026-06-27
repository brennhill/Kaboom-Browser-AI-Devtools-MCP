// Purpose: Implement uninstall.js behavior for npm wrapper command flows.
// Why: Keeps distribution-channel behavior consistent and supportable.
// Docs: docs/features/feature/enhanced-cli-config/index.md

/**
 * Uninstall logic for Kaboom MCP CLI
 * Removes kaboom from all detected AI assistant clients
 */

const fs = require('fs');
const { execFileSync } = require('child_process');
const {
  CLIENT_DEFINITIONS,
  MCP_SERVER_NAME,
  LEGACY_MCP_SERVER_NAMES,
  getClientConfigPath,
  getClientLegacyConfigPaths,
  getDetectedClients,
  readConfigFile,
  writeConfigFile,
} = require('./config');
const { cleanupInstalledSkills } = require('./skills');

const LEGACY_UNINSTALL_SERVER_NAMES = [
  ...LEGACY_MCP_SERVER_NAMES,
  'strum-browser-devtools',
  'strum-agentic-browser',
  'strum',
];

function knownServerNames() {
  return [...new Set([MCP_SERVER_NAME, ...LEGACY_UNINSTALL_SERVER_NAMES])];
}

/**
 * Uninstall from a CLI-type client (e.g. Claude Code via `claude mcp remove`)
 * @param {Object} def Client definition
 * @param {Object} options {dryRun, verbose}
 * @returns {Object} {status, name, method, message}
 */
function uninstallViaCli(def, options) {
  const { dryRun = false, verbose = false } = options;
  const cmd = def.detectCommand;
  const canonicalArgs = [...def.removeArgs];

  if (dryRun) {
    if (verbose) {
      console.log(`[DEBUG] Would run: ${cmd} ${canonicalArgs.join(' ')}`);
    }
    return {
      status: 'removed',
      name: def.name,
      id: def.id,
      method: 'cli',
      message: `Would run: ${cmd} ${canonicalArgs.join(' ')}`,
    };
  }

  const env = { ...process.env };
  delete env.CLAUDECODE;
  const serverNames = knownServerNames();
  // Both the canonical name and one or more legacy names can be configured at
  // the same time — attempt removal for EVERY known name and collect results.
  const removedNames = [];
  let unexpectedErr = null;
  for (const serverName of serverNames) {
    const args = [...canonicalArgs];
    if (args.length > 0) {
      args[args.length - 1] = serverName;
    }
    try {
      execFileSync(cmd, args, {
        env,
        stdio: ['pipe', 'pipe', 'pipe'],
        timeout: 15000,
      });
      removedNames.push(serverName);
    } catch (err) {
      const stderr = err.stderr ? err.stderr.toString() : '';
      const notConfigured = stderr.includes('not found') || stderr.includes('does not exist');
      if (!notConfigured) {
        unexpectedErr = err;
      }
    }
  }
  if (removedNames.length > 0) {
    return {
      status: 'removed',
      name: def.name,
      id: def.id,
      method: 'cli',
      message: `Removed via ${cmd} CLI (${removedNames.join(', ')})`,
    };
  }
  if (unexpectedErr) {
    return {
      status: 'error',
      name: def.name,
      id: def.id,
      method: 'cli',
      message: `CLI uninstall failed: ${unexpectedErr.message}`,
    };
  }
  return {
    status: 'notConfigured',
    name: def.name,
    id: def.id,
    method: 'cli',
  };
}

/**
 * Remove managed Kaboom entries from one config file.
 * Cleans the client's primary config key plus any legacy keys (e.g. VS Code's
 * old "mcpServers" alongside the current "servers").
 * Only deletes the file itself when the client definition marks it as a
 * dedicated MCP config (def.dedicatedMcpFile) — shared settings files
 * (Zed/Gemini/OpenCode settings) are always written back, never unlinked.
 * @param {string} cfgPath Config file path
 * @param {Object} def Client definition
 * @param {Object} options {dryRun, verbose}
 * @returns {Object} {status: 'removed'|'notConfigured'|'error', message?}
 */
function removeManagedEntriesFromConfigFile(cfgPath, def, options) {
  const { dryRun = false, verbose = false } = options;

  if (!cfgPath || !fs.existsSync(cfgPath)) {
    return { status: 'notConfigured' };
  }

  const readResult = readConfigFile(cfgPath);
  if (!readResult.valid) {
    return {
      status: 'error',
      message: `${def.name}: Invalid JSON, cannot uninstall`,
    };
  }

  const configKeys = [def.configKey || 'mcpServers', ...(def.legacyConfigKeys || [])];
  const names = knownServerNames();
  const present = configKeys.some((key) => {
    const servers = readResult.data[key] || {};
    return names.some((name) => Object.prototype.hasOwnProperty.call(servers, name));
  });
  if (!present) {
    return { status: 'notConfigured' };
  }

  if (dryRun) {
    if (verbose) {
      console.log(`[DEBUG] Would remove kaboom from ${cfgPath}`);
    }
    return { status: 'removed' };
  }

  const modified = structuredClone(readResult.data);
  let remainingEntries = 0;
  for (const key of configKeys) {
    if (!modified[key] || typeof modified[key] !== 'object') continue;
    for (const name of names) {
      delete modified[key][name];
    }
    remainingEntries += Object.keys(modified[key]).length;
  }

  if (remainingEntries === 0 && def.dedicatedMcpFile) {
    fs.unlinkSync(cfgPath);
  } else {
    const primaryKey = def.configKey || 'mcpServers';
    const skipValidation = primaryKey !== 'mcpServers';
    writeConfigFile(cfgPath, modified, false, { skipValidation });
  }

  if (verbose) {
    console.log(`[DEBUG] Removed kaboom from ${cfgPath}`);
  }

  return { status: 'removed' };
}

/**
 * Uninstall from a file-type client
 * @param {Object} def Client definition
 * @param {Object} options {dryRun, verbose}
 * @returns {Object} {status, name, method, path}
 */
function uninstallViaFile(def, options) {
  const cfgPath = getClientConfigPath(def);
  const primary = removeManagedEntriesFromConfigFile(cfgPath, def, options);

  // Best-effort cleanup of paths older versions wrote to (e.g. the
  // Windows %APPDATA% Antigravity location).
  let legacyRemoved = false;
  for (const legacyPath of getClientLegacyConfigPaths(def)) {
    try {
      const legacyResult = removeManagedEntriesFromConfigFile(legacyPath, def, options);
      if (legacyResult.status === 'removed') {
        legacyRemoved = true;
      }
    } catch (_) {
      // Legacy path cleanup must never fail the uninstall.
    }
  }

  if (primary.status === 'error') {
    return { status: 'error', name: def.name, id: def.id, message: primary.message };
  }
  if (primary.status === 'removed' || legacyRemoved) {
    return {
      status: 'removed',
      name: def.name,
      id: def.id,
      method: 'file',
      path: cfgPath,
    };
  }
  return { status: 'notConfigured', name: def.name, id: def.id };
}

/**
 * Uninstall from a single client (dispatches by type)
 * @param {Object} def Client definition
 * @param {Object} options {dryRun, verbose}
 * @returns {Object} Result with status, name, method
 */
function uninstallFromClient(def, options) {
  if (def.type === 'cli') {
    return uninstallViaCli(def, options);
  }
  return uninstallViaFile(def, options);
}

/**
 * Execute uninstall across all detected clients
 * @param {Object} options {dryRun, verbose, _clientOverrides}
 * @returns {Object} {success, removed, notConfigured, errors}
 */
function executeUninstall(options = {}) {
  const { dryRun = false, verbose = false } = options;

  const clients = options._clientOverrides !== undefined
    ? options._clientOverrides
    : getDetectedClients();

  const result = {
    success: false,
    removed: [],
    notConfigured: [],
    errors: [],
  };

  for (const def of clients) {
    try {
      const r = uninstallFromClient(def, { dryRun, verbose });

      if (r.status === 'removed') {
        result.removed.push(r);
      } else if (r.status === 'notConfigured') {
        result.notConfigured.push(r.name);
      } else if (r.status === 'error') {
        result.errors.push(r.message || `${r.name}: unknown error`);
      }
    } catch (err) {
      result.errors.push(`${def.name}: ${err.message}`);
      if (verbose) {
        console.log(`[DEBUG] Error uninstalling from ${def.name}: ${err.message}`);
      }
    }
  }

  result.skillCleanup = cleanupInstalledSkills({
    dryRun,
    verbose,
    agents: options.skillAgents,
    scope: options.skillScope,
  });
  result.success = result.removed.length > 0 || result.skillCleanup.removed > 0;
  return result;
}

module.exports = {
  uninstallFromClient,
  executeUninstall,
};
