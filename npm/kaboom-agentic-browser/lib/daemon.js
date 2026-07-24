// Purpose: Start the Kaboom Go daemon during npm --install so the browser
// extension has a server to connect to (parity with the curl|sh installer,
// which already starts it via startDaemonSilently).
// Why: Without a running daemon, "load the extension" has nothing to connect to
// and the post-install connect check can never succeed.
// Docs: docs/features/feature/enhanced-cli-config/index.md

'use strict';

const { spawn } = require('node:child_process');
const { resolveManagedBinaryPath, isEnvFlagSet } = require('./config');
const { fetchHealth, DEFAULT_PORT } = require('./health');

/** The daemon flags — must match the Go installer's startDaemonSilently. */
function daemonSpawnArgs(port) {
  return ['--daemon', '--port', String(port)];
}

/** True when the user opted out of the install-time daemon start. */
function daemonStartDisabled(env = process.env) {
  return isEnvFlagSet(env, ['KABOOM_NO_DAEMON']);
}

/**
 * Best-effort: launch the Kaboom binary in daemon mode, detached, on `port`.
 * Never throws. Returns whether a launch was attempted and the resolved binary.
 * @param {{binaryPath?: string, port?: number, spawnFn?: Function, env?: object}} [opts]
 * @returns {{started: boolean, binaryPath: string}}
 */
function startDaemon(opts = {}) {
  const env = opts.env || process.env;
  const port = opts.port || DEFAULT_PORT;
  const binaryPath = opts.binaryPath || resolveManagedBinaryPath();
  const spawnFn = opts.spawnFn || spawn;

  if (daemonStartDisabled(env)) return { started: false, binaryPath };

  try {
    const child = spawnFn(binaryPath, daemonSpawnArgs(port), { stdio: 'ignore', detached: true });
    if (child && typeof child.on === 'function') child.on('error', () => {});
    if (child && typeof child.unref === 'function') child.unref();
    return { started: true, binaryPath };
  } catch {
    return { started: false, binaryPath };
  }
}

/**
 * Ensure a daemon is answering on `port`: reuse an existing healthy one, or
 * start a new one. Reusing avoids two instances contending for the port.
 * @param {{port?: number, env?: object, fetchHealthFn?: Function, startFn?: Function}} [opts]
 * @returns {Promise<{alreadyRunning: boolean, started: boolean, binaryPath: string|null}>}
 */
async function ensureDaemon(opts = {}) {
  const port = opts.port || DEFAULT_PORT;
  const env = opts.env || process.env;
  const fetchHealthFn = opts.fetchHealthFn || ((p) => fetchHealth(p, { timeoutMs: 600 }));
  const startFn = opts.startFn || (() => startDaemon({ port, env }));

  const snapshot = await fetchHealthFn(port);
  if (snapshot && snapshot.reachable) {
    return { alreadyRunning: true, started: false, binaryPath: null };
  }
  const result = startFn();
  return { alreadyRunning: false, started: !!(result && result.started), binaryPath: (result && result.binaryPath) || null };
}

module.exports = {
  DEFAULT_PORT,
  daemonSpawnArgs,
  daemonStartDisabled,
  startDaemon,
  ensureDaemon,
};
