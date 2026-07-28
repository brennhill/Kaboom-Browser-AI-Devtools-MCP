// Purpose: Implement install.js behavior for npm wrapper command flows.
// Why: Keeps distribution-channel behavior consistent and supportable.
// Docs: docs/features/feature/enhanced-cli-config/index.md

/**
 * Install logic for the Kaboom MCP CLI
 * Handles installation to all detected AI assistant clients
 */

const { execFileSync } = require('child_process');
const {
  CLIENT_DEFINITIONS,
  MCP_SERVER_NAME,
  getClientConfigPath,
  getDetectedClients,
  getClientByAlias,
  getValidAliases,
  readConfigFile,
  writeConfigFile,
  resolveManagedBinaryPath,
} = require('./config');
const autoApprove = require('./auto-approve');
const codexConfig = require('./codex-config');
const { resolveExtensionDir } = require('./extension');

/**
 * Generate default MCP config for Kaboom
 * @returns {Object} Default Kaboom MCP config
 */
function generateDefaultConfig(options = {}) {
  const { binaryCommand = resolveManagedBinaryPath() } = options;
  return {
    mcpServers: {
      [MCP_SERVER_NAME]: {
        command: binaryCommand,
        args: [],
      },
    },
  };
}

/**
 * Build the MCP entry JSON string for CLI-based install
 * @param {Object} [envVars] Optional env vars
 * @returns {string} JSON string of the Kaboom MCP entry
 */
function buildMcpEntry(envVars = {}, options = {}) {
  const { binaryCommand = resolveManagedBinaryPath() } = options;
  const entry = { command: binaryCommand, args: [] };
  if (envVars && Object.keys(envVars).length > 0) {
    entry.env = envVars;
  }
  return JSON.stringify(entry);
}

/**
 * Install to a CLI-type client (e.g. Claude Code via `claude mcp add-json`)
 * @param {Object} def Client definition
 * @param {Object} options {dryRun, envVars}
 * @returns {Object} {success, name, method, message}
 */
function installViaCli(def, options) {
  const { dryRun = false, envVars = {}, binaryCommand = resolveManagedBinaryPath(), claudeSettingsPath } = options;
  const entryJson = buildMcpEntry(envVars, { binaryCommand });
  const cmd = def.detectCommand;
  // `claude mcp add-json <name> '<json>'` takes the server JSON as the FINAL
  // POSITIONAL argument. It was previously piped to stdin, so the CLI aborted
  // with "missing required argument 'json'". installArgs already ends with the
  // server name; append the JSON config after it.
  const baseArgs = [...def.installArgs];
  const args = [...baseArgs, entryJson];

  if (dryRun) {
    return {
      success: true,
      name: def.name,
      id: def.id,
      method: 'cli',
      autoApprove: def.autoApprove && def.autoApprove.kind === 'claude-settings' ? 'would-apply' : undefined,
      message: `Would run: ${cmd} ${baseArgs.join(' ')} '<json>'`,
    };
  }

  try {
    // Must unset CLAUDECODE env var to avoid nested-session error
    const env = { ...process.env };
    delete env.CLAUDECODE;

    execFileSync(cmd, args, {
      env,
      stdio: ['pipe', 'pipe', 'pipe'],
      timeout: 15000,
    });

    // Auto-approve: after the server is registered, trust ALL of its tools by
    // adding `mcp__<server>` to ~/.claude/settings.json permissions.allow. This
    // is a separate merge-safe write (the CLI has no flag for it). A failure
    // here is reported (not swallowed) and does not undo the successful server
    // registration.
    const result = {
      success: true,
      name: def.name,
      id: def.id,
      method: 'cli',
      message: `Installed via ${cmd} CLI`,
    };
    if (def.autoApprove && def.autoApprove.kind === 'claude-settings') {
      try {
        const aa = autoApprove.applyClaudeSettingsAllow({ settingsPath: claudeSettingsPath });
        result.autoApprove = aa.changed ? 'applied' : 'unchanged';
        result.autoApprovePath = aa.path;
      } catch (aaErr) {
        result.autoApprove = 'failed';
        result.autoApproveError = aaErr.message;
      }
    }
    return result;
  } catch (err) {
    return {
      success: false,
      name: def.name,
      id: def.id,
      method: 'cli',
      message: `CLI install failed: ${err.message}`,
      error: err.message,
    };
  }
}

/**
 * Install to a file-type client (config file write)
 * @param {Object} def Client definition
 * @param {Object} options {dryRun, envVars}
 * @returns {Object} {success, name, method, path, isNew, message}
 */
function installViaFile(def, options) {
  const { dryRun = false, envVars = {}, binaryCommand = resolveManagedBinaryPath() } = options;
  const cfgPath = getClientConfigPath(def);

  if (!cfgPath) {
    return {
      success: false,
      name: def.name,
      id: def.id,
      method: 'file',
      message: `No config path for this platform`,
    };
  }

  const configKey = def.configKey || 'mcpServers';

  // Build entry in the right format for this client
  let kaboomEntry;
  if (def.buildEntry) {
    kaboomEntry = def.buildEntry(envVars, binaryCommand);
  } else {
    kaboomEntry = { command: binaryCommand, args: [] };
    if (envVars && Object.keys(envVars).length > 0) {
      kaboomEntry.env = envVars;
    }
  }

  let configData;
  let isNew = false;

  const readResult = readConfigFile(cfgPath);
  if (readResult.valid) {
    configData = readResult.data;
  } else {
    configData = {};
    isNew = true;
  }

  // Merge Kaboom entry under the correct key
  if (!configData[configKey]) configData[configKey] = {};
  configData[configKey][MCP_SERVER_NAME] = kaboomEntry;

  // Fold whole-server tool auto-approve into the SAME atomic write (Gemini
  // trust, OpenCode permission, Zed tool_permissions). UI-only clients are
  // left untouched. Default-ON: Kaboom trusts its own tools so the user is
  // never prompted where a config mechanism exists.
  let autoApproveStatus = def.autoApprove
    ? (def.autoApprove.kind === 'ui-only' ? 'ui-only' : 'none')
    : undefined;
  if (autoApprove.isSameFileKind(def)) {
    const applied = autoApprove.applyToConfig(def, configData);
    autoApproveStatus = applied ? (dryRun ? 'would-apply' : 'applied') : 'skipped';
  }

  const skipValidation = configKey !== 'mcpServers';
  writeConfigFile(cfgPath, configData, dryRun, { skipValidation });

  return {
    success: true,
    name: def.name,
    id: def.id,
    method: 'file',
    path: cfgPath,
    isNew,
    autoApprove: autoApproveStatus,
    message: dryRun ? `Would write to ${cfgPath}` : `Wrote to ${cfgPath}`,
  };
}

/**
 * Install to a TOML-format client (Codex config.toml): server registration +
 * whole-server tool auto-approve, handled by lib/codex-config.js.
 * @param {Object} def Client definition
 * @param {Object} options {dryRun, envVars, binaryCommand}
 * @returns {Object} {success, name, method, path, isNew, autoApprove, message}
 */
function installViaToml(def, options) {
  const { dryRun = false, envVars = {}, binaryCommand = resolveManagedBinaryPath() } = options;
  const cfgPath = getClientConfigPath(def);
  if (!cfgPath) {
    return { success: false, name: def.name, id: def.id, method: 'file', message: 'No config path for this platform' };
  }
  const r = codexConfig.installCodex({ configPath: cfgPath, binaryCommand, envVars, dryRun });
  return {
    success: true,
    name: def.name,
    id: def.id,
    method: 'file',
    path: cfgPath,
    isNew: r.isNew,
    autoApprove: r.autoApprove,
    message: dryRun ? `Would write to ${cfgPath}` : `Wrote to ${cfgPath}`,
  };
}

/**
 * Install to a single client (dispatches by type)
 * @param {Object} def Client definition
 * @param {Object} options {dryRun, envVars}
 * @returns {Object} Result with success, name, method, etc.
 */
function installToClient(def, options) {
  if (def.type === 'cli') {
    return installViaCli(def, options);
  }
  if (def.format === 'toml') {
    return installViaToml(def, options);
  }
  return installViaFile(def, options);
}

/**
 * Execute install operation across all detected clients
 * @param {Object} options {dryRun, envVars, verbose, _clientOverrides}
 * @returns {Object} {success, installed, errors, total}
 */
function executeInstall(options = {}) {
  const {
    dryRun = false,
    envVars = {},
    verbose = false,
    targetTool,
    binaryCommand = resolveManagedBinaryPath(),
  } = options;

  // Targeted install: filter to a single client by alias
  let clients;
  if (options._clientOverrides !== undefined) {
    clients = options._clientOverrides;
  } else if (targetTool) {
    const def = getClientByAlias(targetTool);
    if (!def) {
      const valid = getValidAliases().join(', ');
      return {
        success: false,
        installed: [],
        errors: [{ name: targetTool, message: `Unknown tool: ${targetTool}. Valid tools: ${valid}` }],
        total: CLIENT_DEFINITIONS.length,
      };
    }
    clients = [def];
  } else {
    clients = getDetectedClients();
  }

  const result = {
    success: false,
    installed: [],
    errors: [],
    total: CLIENT_DEFINITIONS.length,
  };

  for (const def of clients) {
    try {
      const installResult = installToClient(def, { dryRun, envVars, binaryCommand, claudeSettingsPath: options.claudeSettingsPath });

      if (installResult.success) {
        result.installed.push(installResult);
      } else {
        result.errors.push(installResult);
      }

      if (verbose) {
        const status = installResult.success ? 'OK' : 'FAIL';
        console.log(`[DEBUG] ${def.name}: ${status} - ${installResult.message}`);
      }
    } catch (err) {
      result.errors.push({
        name: def.name,
        id: def.id,
        message: err.message,
        recovery: err.recovery,
      });

      if (verbose) {
        console.log(`[DEBUG] Error on ${def.name}: ${err.message}`);
      }
    }
  }

  result.success = result.installed.length > 0;

  // Tell the user the EXACT folder to load as an unpacked extension. The browser
  // step is the one part the installer cannot do, so surfacing the precise path
  // (and whether it is present yet) is what makes onboarding self-serve.
  const ext = resolveExtensionDir();
  result.extensionDir = ext.dir;
  result.extensionExists = ext.exists;

  return result;
}

module.exports = {
  generateDefaultConfig,
  buildMcpEntry,
  installToClient,
  executeInstall,
};
