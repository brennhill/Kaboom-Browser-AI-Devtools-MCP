// Purpose: Query the running Kaboom daemon's /health endpoint and wait for the
// browser extension to connect.
// Why: After install the one remaining manual step is loading the unpacked
// extension. Polling /health lets us confirm it actually connected — or tell the
// user exactly what is still missing — instead of leaving them guessing.
// Docs: docs/features/feature/enhanced-cli-config/index.md
//
// NOTE: kill-daemon.js has its own narrow /health reader (readHealthIdentity)
// that only inspects service identity and must stay conservative for the
// process-killing path. This module is the canonical health client for
// user-facing install/doctor/connect flows; keep the two intentionally separate.

'use strict';

const http = require('node:http');
const { isEnvFlagSet } = require('./config');

const DEFAULT_PORT = 7890;
// One /health read should be quick on localhost; cap it so a hung socket can
// never wedge the connect loop.
const DEFAULT_REQUEST_TIMEOUT_MS = 800;

/**
 * @typedef {Object} HealthSnapshot
 * @property {boolean} reachable          Daemon answered on the port at all.
 * @property {boolean} ok                 HTTP 200 with a parseable JSON object.
 * @property {boolean} extensionConnected capture.extension_connected === true.
 * @property {string|number|null} extensionLastSeen
 * @property {string|null} version
 * @property {string|null} serviceName
 * @property {object|null} raw            The parsed body (when ok).
 */

function emptySnapshot(reachable) {
  return {
    reachable,
    ok: false,
    extensionConnected: false,
    extensionLastSeen: null,
    version: null,
    serviceName: null,
    raw: null,
  };
}

/**
 * Normalize a raw /health response into a HealthSnapshot. Pure.
 * @param {number|null} statusCode  null means the daemon never answered.
 * @param {string|null} body
 * @returns {HealthSnapshot}
 */
function summarizeHealth(statusCode, body) {
  if (statusCode == null) return emptySnapshot(false);
  const snap = emptySnapshot(true);
  if (statusCode !== 200) return snap;

  let data;
  try {
    data = JSON.parse(body);
  } catch {
    return snap;
  }
  if (!data || typeof data !== 'object') return snap;

  snap.ok = true;
  snap.raw = data;
  snap.version = typeof data.version === 'string' ? data.version : null;
  for (const key of ['service-name', 'service_name', 'name']) {
    if (typeof data[key] === 'string' && data[key].trim()) {
      snap.serviceName = data[key].trim();
      break;
    }
  }
  const cap = data.capture && typeof data.capture === 'object' ? data.capture : null;
  if (cap) {
    snap.extensionConnected = cap.extension_connected === true;
    snap.extensionLastSeen = cap.extension_last_seen == null ? null : cap.extension_last_seen;
  }
  return snap;
}

/**
 * Real /health request. Resolves { statusCode, body } or null on any error or
 * timeout — never rejects, so a down daemon is just "unreachable".
 * @param {number} port
 * @param {number} timeoutMs
 * @returns {Promise<{statusCode: number, body: string}|null>}
 */
function defaultRequest(port, timeoutMs) {
  return new Promise((resolve) => {
    let settled = false;
    const finish = (value) => {
      if (settled) return;
      settled = true;
      resolve(value);
    };
    let req;
    try {
      req = http.get({ hostname: '127.0.0.1', port, path: '/health', timeout: timeoutMs }, (res) => {
        let body = '';
        res.setEncoding('utf8');
        res.on('data', (chunk) => {
          body += chunk;
          // Guard against an unbounded body from a misbehaving server.
          if (body.length > 256 * 1024) {
            try { req.destroy(); } catch { /* already gone */ }
          }
        });
        res.on('end', () => finish({ statusCode: res.statusCode, body }));
        res.on('error', () => finish(null));
      });
    } catch {
      finish(null);
      return;
    }
    req.on('timeout', () => {
      try { req.destroy(); } catch { /* already gone */ }
      finish(null);
    });
    req.on('error', () => finish(null));
  });
}

/**
 * Fetch and normalize the daemon /health snapshot. Never rejects.
 * @param {number} [port]
 * @param {{timeoutMs?: number, request?: Function}} [opts]
 *   request(port, timeoutMs) is injectable for tests.
 * @returns {Promise<HealthSnapshot>}
 */
async function fetchHealth(port = DEFAULT_PORT, opts = {}) {
  const request = opts.request || defaultRequest;
  const timeoutMs = opts.timeoutMs || DEFAULT_REQUEST_TIMEOUT_MS;
  let res;
  try {
    res = await request(port, timeoutMs);
  } catch {
    res = null;
  }
  if (!res) return summarizeHealth(null, null);
  return summarizeHealth(res.statusCode, res.body);
}

/** Classify a snapshot into a connect phase. */
function phaseOf(snapshot) {
  if (snapshot.extensionConnected) return 'connected';
  if (snapshot.reachable) return 'waiting_extension';
  return 'daemon_unreachable';
}

/**
 * Poll /health until the extension connects or the deadline passes. Reports each
 * poll via onState so a caller can render live progress. Deterministic under an
 * injected clock (now/sleep) — no real timers required for tests.
 *
 * @param {Object} opts
 * @param {number} [opts.port]
 * @param {number} [opts.timeoutMs]  Overall budget (default 30s).
 * @param {number} [opts.pollMs]     Delay between polls (default 750ms).
 * @param {number} [opts.perPollTimeoutMs]
 * @param {Function} [opts.request]  Injected /health request.
 * @param {Function} [opts.now]      () => ms.
 * @param {Function} [opts.sleep]    (ms) => Promise.
 * @param {Function} [opts.onState]  ({phase, snapshot, elapsedMs}) => void.
 * @returns {Promise<{connected: boolean, reason: 'connected'|'timeout',
 *   lastPhase: string, snapshot: HealthSnapshot, elapsedMs: number}>}
 */
async function waitForExtension(opts = {}) {
  const port = opts.port || DEFAULT_PORT;
  const timeoutMs = opts.timeoutMs != null ? opts.timeoutMs : 30000;
  const pollMs = opts.pollMs || 750;
  const perPollTimeoutMs = opts.perPollTimeoutMs || DEFAULT_REQUEST_TIMEOUT_MS;
  const request = opts.request;
  const now = opts.now || (() => Date.now());
  const sleep = opts.sleep || ((ms) => new Promise((r) => setTimeout(r, ms)));
  const onState = typeof opts.onState === 'function' ? opts.onState : () => {};

  const start = now();
  let snapshot = emptySnapshot(false);
  let lastPhase = 'daemon_unreachable';

  for (;;) {
    snapshot = await fetchHealth(port, { request, timeoutMs: perPollTimeoutMs });
    lastPhase = phaseOf(snapshot);
    const elapsedMs = now() - start;
    onState({ phase: lastPhase, snapshot, elapsedMs });

    if (lastPhase === 'connected') {
      return { connected: true, reason: 'connected', lastPhase, snapshot, elapsedMs };
    }
    if (elapsedMs >= timeoutMs) {
      return { connected: false, reason: 'timeout', lastPhase, snapshot, elapsedMs };
    }
    await sleep(pollMs);
  }
}

/**
 * Targeted next-step message for a connect loop that did not finish. Pure.
 * @param {string} lastPhase 'daemon_unreachable' | 'waiting_extension'
 * @param {{port?: number, extensionDir?: string}} [ctx]
 * @returns {string}
 */
function connectHint(lastPhase, ctx = {}) {
  const port = ctx.port || DEFAULT_PORT;
  if (lastPhase === 'waiting_extension') {
    const where = ctx.extensionDir ? `\n     Folder to load: ${ctx.extensionDir}` : '';
    return (
      `The Kaboom server is running on port ${port}, but the browser extension has not connected yet.\n` +
      `   → Open chrome://extensions, enable "Developer mode", click "Load unpacked", and select the\n` +
      `     extension folder — then make sure the toggle is on.${where}`
    );
  }
  // daemon_unreachable (or anything unexpected)
  return (
    `The Kaboom server is not answering on port ${port} yet.\n` +
    `   → Start your AI client (Claude, Cursor, …) so it launches Kaboom, or run\n` +
    `     "kaboom-agentic-browser --install", then re-run "kaboom-agentic-browser --connect".`
  );
}

/** True when the user opted out of the install-time connect wait. */
function connectWaitDisabled(env = process.env) {
  return isEnvFlagSet(env, ['KABOOM_NO_WAIT', 'KABOOM_INSTALL_NO_WAIT']);
}

module.exports = {
  DEFAULT_PORT,
  summarizeHealth,
  fetchHealth,
  waitForExtension,
  connectHint,
  connectWaitDisabled,
  phaseOf,
};
