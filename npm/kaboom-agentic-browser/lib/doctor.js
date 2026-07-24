// Purpose: Implement doctor.js behavior for npm wrapper command flows.
// Why: Keeps distribution-channel behavior consistent and supportable.
// Docs: docs/features/feature/enhanced-cli-config/index.md

/**
 * Doctor diagnostics for the Kaboom MCP CLI
 * Checks config validity, binary availability, and provides repair suggestions
 */

const fs = require('fs');
const net = require('net');
const { execFileSync } = require('child_process');
const {
  CLIENT_DEFINITIONS,
  LEGACY_PATHS,
  MCP_SERVER_NAME,
  LEGACY_MCP_SERVER_NAMES,
  getClientConfigPath,
  isClientInstalled,
  commandExistsOnPath,
  readConfigFile,
  expandPath,
} = require('./config');
const { fetchHealth, DEFAULT_PORT } = require('./health');

// Node floor: the launcher and lib/*.js rely on modern Node built-ins
// (node: specifiers, structuredClone, fetch-free http usage). 18 is the oldest
// still-maintained line that clears that bar.
const MIN_NODE_MAJOR = 18;

function knownServerNames() {
  return [MCP_SERVER_NAME, ...LEGACY_MCP_SERVER_NAMES.filter((name) => name !== MCP_SERVER_NAME)];
}

/**
 * Check whether a port is free by trying to bind it. Cross-platform (net only,
 * no lsof), so it works identically on Windows/macOS/Linux. Never rejects.
 * @param {number} port Port to check
 * @returns {Promise<{available: boolean, error?: string}>}
 */
function checkPort(port) {
  return new Promise((resolve) => {
    const server = net.createServer();
    server.once('error', (err) => {
      if (err.code === 'EADDRINUSE') {
        resolve({ available: false, error: `Port ${port} is in use by another process` });
      } else {
        resolve({ available: false, error: err.message });
      }
    });
    server.once('listening', () => {
      server.close();
      resolve({ available: true });
    });
    server.listen(port, '127.0.0.1');
  });
}

/**
 * Test if the Kaboom binary is available and working
 * @returns {Object} {ok: bool, path?: string, version?: string, error?: string}
 */
function testBinary() {
  try {
    // Try to find the binary from node_modules
    const path = require('path');
    const os = require('os');
    const platform = os.platform();
    const arch = os.arch();

    const platformMap = {
      'darwin-arm64': '@brennhill/kaboom-agentic-browser-darwin-arm64',
      'darwin-x64': '@brennhill/kaboom-agentic-browser-darwin-x64',
      'linux-arm64': '@brennhill/kaboom-agentic-browser-linux-arm64',
      'linux-x64': '@brennhill/kaboom-agentic-browser-linux-x64',
      'win32-x64': '@brennhill/kaboom-agentic-browser-win32-x64',
    };

    const key = `${platform}-${arch}`;
    const pkg = platformMap[key];

    if (!pkg) {
      return {
        ok: false,
        error: `Unsupported platform: ${platform}-${arch}`,
      };
    }

    // Try to find binary
    const binaryName = platform === 'win32' ? 'kaboom-agentic-browser.exe' : 'kaboom-agentic-browser';
    const homeDir = os.homedir();

    // Check several locations
    const candidates = [
      path.join(homeDir, '.npm', '_npx', pkg, 'bin', binaryName),
      path.join(homeDir, 'node_modules', pkg, 'bin', binaryName),
      path.join(__dirname, '..', 'node_modules', pkg, 'bin', binaryName),
      path.join(__dirname, '..', '..', pkg, 'bin', binaryName),
      path.join(__dirname, '..', '..', '..', pkg, 'bin', binaryName),
    ];

    let binaryPath = null;
    for (const candidate of candidates) {
      if (fs.existsSync(candidate)) {
        binaryPath = candidate;
        break;
      }
    }

    if (!binaryPath) {
      return {
        ok: false,
        error: `Kaboom binary not found for platform ${key}`,
      };
    }

    // Test binary with --version
    try {
      const version = execFileSync(binaryPath, ['--version'], {
        encoding: 'utf8',
        stdio: ['pipe', 'pipe', 'pipe'],
      }).trim();

      return {
        ok: true,
        path: binaryPath,
        version: version || 'unknown',
      };
    } catch (e) {
      return {
        ok: false,
        path: binaryPath,
        error: 'Binary found but failed to execute',
      };
    }
  } catch (err) {
    return {
      ok: false,
      error: `Error testing binary: ${err.message}`,
    };
  }
}

/**
 * Evaluate a Node version string against the supported floor. Pure.
 * @param {string} versionString e.g. process.version ("v20.11.0")
 * @returns {{ok: boolean, version: string, major: number|null, minMajor: number}}
 */
function evaluateNodeVersion(versionString) {
  const match = /^v?(\d+)\./.exec(String(versionString || ''));
  const major = match ? Number(match[1]) : null;
  return {
    ok: major != null && major >= MIN_NODE_MAJOR,
    version: String(versionString || 'unknown'),
    major,
    minMajor: MIN_NODE_MAJOR,
  };
}

/** Diagnose the Node runtime the CLI itself is running under. */
function nodeCheck() {
  return evaluateNodeVersion(process.version);
}

/**
 * Live daemon/extension check via /health. Never throws.
 * @param {{port?: number, fetchHealthFn?: Function}} [opts]
 * @returns {Promise<{port: number, reachable: boolean, ok: boolean,
 *   version: string|null, extensionConnected: boolean, extensionLastSeen: any}>}
 */
async function checkDaemon(opts = {}) {
  const port = opts.port || DEFAULT_PORT;
  const fetchHealthFn = opts.fetchHealthFn || ((p) => fetchHealth(p, { timeoutMs: 800 }));
  const snap = await fetchHealthFn(port);
  return {
    port,
    reachable: !!(snap && snap.reachable),
    ok: !!(snap && snap.ok),
    version: (snap && snap.version) || null,
    extensionConnected: !!(snap && snap.extensionConnected),
    extensionLastSeen: (snap && snap.extensionLastSeen) || null,
  };
}

/**
 * Diagnose a single file-type client
 * @param {Object} def Client definition
 * @param {boolean} verbose
 * @returns {Object} Tool diagnostic
 */
function diagnoseFileClient(def, verbose) {
  const cfgPath = getClientConfigPath(def);
  const detected = isClientInstalled(def);

  const tool = {
    name: def.name,
    id: def.id,
    type: 'file',
    path: cfgPath,
    detected,
    status: 'error',
    issues: [],
    suggestions: [],
  };

  if (verbose) {
    console.log(`[DEBUG] Checking ${def.name} at ${cfgPath}`);
  }

  if (!detected) {
    tool.status = 'info';
    tool.issues.push('Not installed on this system');
    return tool;
  }

  if (!cfgPath) {
    tool.status = 'info';
    tool.issues.push('No config path for this platform');
    return tool;
  }

  if (!fs.existsSync(cfgPath)) {
    tool.status = 'error';
    tool.issues.push('Config file not found');
    tool.suggestions.push('Run: kaboom-agentic-browser --install');
    return tool;
  }

  const readResult = readConfigFile(cfgPath);
  if (!readResult.valid) {
    tool.issues.push('Invalid JSON');
    tool.suggestions.push('Fix the JSON syntax or run: kaboom-agentic-browser --install');
    return tool;
  }

  const configKey = def.configKey || 'mcpServers';
  const servers = readResult.data[configKey] || {};
  const matchedName = knownServerNames().find((name) => Object.prototype.hasOwnProperty.call(servers, name));
  if (!matchedName) {
    // Check legacy config keys (e.g. VS Code's old "mcpServers" key).
    for (const legacyKey of def.legacyConfigKeys || []) {
      const legacyServers = readResult.data[legacyKey] || {};
      const legacyMatch = knownServerNames().find((name) => Object.prototype.hasOwnProperty.call(legacyServers, name));
      if (legacyMatch) {
        tool.status = 'error';
        tool.issues.push(`MCP entry found under legacy "${legacyKey}" key; migrate to "${configKey}"`);
        tool.suggestions.push('Run: kaboom-agentic-browser --install');
        return tool;
      }
    }
    tool.issues.push(`${MCP_SERVER_NAME} entry missing from ${configKey}`);
    tool.suggestions.push('Run: kaboom-agentic-browser --install');
    return tool;
  }
  if (matchedName !== MCP_SERVER_NAME) {
    tool.status = 'error';
    tool.issues.push(`Legacy MCP server name detected (${matchedName}); migrate to ${MCP_SERVER_NAME}`);
    tool.suggestions.push('Run: kaboom-agentic-browser --install');
    return tool;
  }

  tool.status = 'ok';
  return tool;
}

/**
 * Diagnose a CLI-type client
 * @param {Object} def Client definition
 * @param {boolean} verbose
 * @returns {Object} Tool diagnostic
 */
function diagnoseCliClient(def, verbose) {
  const detected = isClientInstalled(def);

  const tool = {
    name: def.name,
    id: def.id,
    type: 'cli',
    detected,
    status: 'error',
    issues: [],
    suggestions: [],
  };

  if (verbose) {
    console.log(`[DEBUG] Checking ${def.name} (CLI: ${def.detectCommand})`);
  }

  if (!detected) {
    tool.status = 'info';
    tool.issues.push(`${def.detectCommand} CLI not found on PATH`);
    return tool;
  }

  // Try to check if Kaboom is configured via CLI
  let found = false;
  for (const serverName of knownServerNames()) {
    try {
      execFileSync(def.detectCommand, ['mcp', 'get', serverName], {
        stdio: ['pipe', 'pipe', 'pipe'],
        timeout: 10000,
        env: { ...process.env, CLAUDECODE: undefined },
      });
      found = true;
      break;
    } catch {
      // Try next known server name.
    }
  }
  if (found) {
    tool.status = 'ok';
  } else {
    tool.status = 'error';
    tool.issues.push(`${MCP_SERVER_NAME} not configured`);
    tool.suggestions.push('Run: kaboom-agentic-browser --install');
  }

  return tool;
}

/**
 * Check for legacy/orphaned config files at old paths
 * @returns {Array<Object>} Warnings for legacy paths found
 */
function checkLegacyPaths() {
  const warnings = [];
  for (const legacy of LEGACY_PATHS) {
    const expanded = expandPath(legacy.path);
    if (fs.existsSync(expanded)) {
      try {
        const readResult = readConfigFile(expanded);
        if (readResult.valid && readResult.data.mcpServers) {
          const hasKnownEntry = knownServerNames().some((name) => Object.prototype.hasOwnProperty.call(readResult.data.mcpServers, name));
          if (hasKnownEntry) {
            warnings.push({
              path: expanded,
              description: legacy.description,
              message: `Orphaned ${MCP_SERVER_NAME} config at old path: ${expanded}`,
            });
          }
        }
      } catch {
        // Ignore read errors on legacy paths
      }
    }
  }
  return warnings;
}

/**
 * Run full diagnostics on all client locations, plus live runtime checks
 * (Node version, daemon reachability, extension connectivity).
 * @param {boolean} verbose If true, log debug info
 * @param {{fetchHealthFn?: Function, clients?: Array}} [opts]
 *   Injectable /health and client list for hermetic tests (defaults hit the
 *   real client scan + daemon).
 * @returns {Promise<Object>} Diagnostic report with tools array and summary
 */
async function runDiagnostics(verbose = false, opts = {}) {
  const tools = [];
  const clients = opts.clients || CLIENT_DEFINITIONS;

  for (const def of clients) {
    if (def.type === 'cli') {
      tools.push(diagnoseCliClient(def, verbose));
    } else {
      tools.push(diagnoseFileClient(def, verbose));
    }
  }

  // Check binary availability
  const binary = testBinary();

  // Check default port availability (7890)
  const defaultPort = DEFAULT_PORT;
  const port = await checkPort(defaultPort);

  // Live runtime checks: Node version + whether the daemon/extension are up.
  const node = nodeCheck();
  const daemon = await checkDaemon({ port: defaultPort, fetchHealthFn: opts.fetchHealthFn });
  const extension = { connected: daemon.extensionConnected, lastSeen: daemon.extensionLastSeen };

  // Check for legacy paths
  const legacyWarnings = checkLegacyPaths();

  // Generate summary
  const okCount = tools.filter(t => t.status === 'ok').length;
  const errorCount = tools.filter(t => t.status === 'error').length;
  const infoCount = tools.filter(t => t.status === 'info').length;

  let summary = `Summary: ${okCount} client${okCount === 1 ? '' : 's'} ready`;
  if (errorCount > 0) {
    summary += `, ${errorCount} need${errorCount === 1 ? 's' : ''} repair`;
  }
  if (infoCount > 0) {
    summary += `, ${infoCount} not detected`;
  }

  return {
    tools,
    binary,
    port: { port: defaultPort, ...port },
    node,
    daemon,
    extension,
    legacyWarnings,
    summary,
  };
}

module.exports = {
  MIN_NODE_MAJOR,
  evaluateNodeVersion,
  nodeCheck,
  checkDaemon,
  checkPort,
  testBinary,
  runDiagnostics,
};
