// Tests for lib/health.js — reading the daemon /health snapshot and waiting for
// the browser extension to connect.

'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');

const http = require('node:http');
const net = require('node:net');
const {
  DEFAULT_PORT,
  summarizeHealth,
  defaultRequest,
  fetchHealth,
  waitForExtension,
  connectHint,
  connectWaitDisabled,
} = require('./health');

// Bind an ephemeral port, hand it to fn, then clean up. Returns fn's result.
async function withHttpServer(handler, fn) {
  const server = http.createServer(handler);
  await new Promise((res) => server.listen(0, '127.0.0.1', res));
  const port = server.address().port;
  try {
    return await fn(port);
  } finally {
    if (server.closeAllConnections) server.closeAllConnections();
    await new Promise((res) => server.close(res));
  }
}

test('DEFAULT_PORT matches the daemon default', () => {
  assert.equal(DEFAULT_PORT, 7890);
});

test('summarizeHealth reports a connected extension from a 200 body', () => {
  const body = JSON.stringify({
    version: '0.8.6',
    'service-name': 'kaboom-agentic-browser',
    capture: { extension_connected: true, extension_last_seen: '2026-07-24T00:00:00Z' },
  });
  const snap = summarizeHealth(200, body);
  assert.equal(snap.reachable, true);
  assert.equal(snap.ok, true);
  assert.equal(snap.extensionConnected, true);
  assert.equal(snap.extensionLastSeen, '2026-07-24T00:00:00Z');
  assert.equal(snap.version, '0.8.6');
  assert.equal(snap.serviceName, 'kaboom-agentic-browser');
});

test('summarizeHealth: daemon up but extension not connected', () => {
  const body = JSON.stringify({ version: '0.8.6', capture: { extension_connected: false } });
  const snap = summarizeHealth(200, body);
  assert.equal(snap.reachable, true);
  assert.equal(snap.ok, true);
  assert.equal(snap.extensionConnected, false);
  assert.equal(snap.extensionLastSeen, null);
});

test('summarizeHealth: no capture block means daemon up, extension unknown/false', () => {
  const snap = summarizeHealth(200, JSON.stringify({ version: '0.8.6' }));
  assert.equal(snap.reachable, true);
  assert.equal(snap.ok, true);
  assert.equal(snap.extensionConnected, false);
});

test('summarizeHealth: null status means the daemon never answered', () => {
  const snap = summarizeHealth(null, null);
  assert.equal(snap.reachable, false);
  assert.equal(snap.ok, false);
  assert.equal(snap.extensionConnected, false);
  assert.equal(snap.version, null);
  assert.equal(snap.raw, null);
});

test('summarizeHealth: non-200 counts as reachable but not ok', () => {
  const snap = summarizeHealth(503, 'Service Unavailable');
  assert.equal(snap.reachable, true);
  assert.equal(snap.ok, false);
  assert.equal(snap.extensionConnected, false);
});

test('summarizeHealth: invalid JSON body is reachable but not ok', () => {
  const snap = summarizeHealth(200, '{ not json');
  assert.equal(snap.reachable, true);
  assert.equal(snap.ok, false);
});

test('fetchHealth uses the injected request and summarizes the result', async () => {
  const request = async (port, timeoutMs) => {
    assert.equal(port, 7890);
    assert.ok(timeoutMs > 0);
    return { statusCode: 200, body: JSON.stringify({ capture: { extension_connected: true } }) };
  };
  const snap = await fetchHealth(7890, { request });
  assert.equal(snap.extensionConnected, true);
});

test('fetchHealth treats a thrown/absent request as unreachable', async () => {
  const throwing = async () => { throw new Error('ECONNREFUSED'); };
  assert.equal((await fetchHealth(7890, { request: throwing })).reachable, false);

  const nullish = async () => null;
  assert.equal((await fetchHealth(7890, { request: nullish })).reachable, false);
});

// A deterministic fake clock: sleep() advances virtual time so waitForExtension
// terminates without real timers.
function fakeClock() {
  let t = 0;
  return {
    now: () => t,
    sleep: (ms) => { t += ms; return Promise.resolve(); },
  };
}

test('waitForExtension resolves connected once the extension appears', async () => {
  const clock = fakeClock();
  // Poll 1: unreachable. Poll 2: up, no extension. Poll 3: connected.
  const queue = [
    null,
    { statusCode: 200, body: JSON.stringify({ capture: { extension_connected: false } }) },
    { statusCode: 200, body: JSON.stringify({ capture: { extension_connected: true } }) },
  ];
  const request = async () => queue.shift();
  const phases = [];
  const result = await waitForExtension({
    port: 7890,
    timeoutMs: 30000,
    pollMs: 500,
    request,
    now: clock.now,
    sleep: clock.sleep,
    onState: (s) => phases.push(s.phase),
  });
  assert.equal(result.connected, true);
  assert.equal(result.reason, 'connected');
  assert.equal(result.lastPhase, 'connected');
  assert.deepEqual(phases, ['daemon_unreachable', 'waiting_extension', 'connected']);
});

test('waitForExtension times out and reports the last phase (extension never loaded)', async () => {
  const clock = fakeClock();
  const request = async () => ({ statusCode: 200, body: JSON.stringify({ capture: { extension_connected: false } }) });
  const result = await waitForExtension({
    timeoutMs: 2000,
    pollMs: 750,
    request,
    now: clock.now,
    sleep: clock.sleep,
  });
  assert.equal(result.connected, false);
  assert.equal(result.reason, 'timeout');
  assert.equal(result.lastPhase, 'waiting_extension');
});

test('waitForExtension times out reporting daemon_unreachable when nothing answers', async () => {
  const clock = fakeClock();
  const request = async () => null;
  const result = await waitForExtension({
    timeoutMs: 1500,
    pollMs: 750,
    request,
    now: clock.now,
    sleep: clock.sleep,
  });
  assert.equal(result.connected, false);
  assert.equal(result.lastPhase, 'daemon_unreachable');
});

test('defaultRequest performs a real /health GET and returns status + body', async () => {
  await withHttpServer(
    (req, res) => {
      if (req.url === '/health') {
        res.writeHead(200, { 'content-type': 'application/json' });
        res.end(JSON.stringify({ version: '0.8.6', capture: { extension_connected: true } }));
      } else {
        res.writeHead(404);
        res.end('nope');
      }
    },
    async (port) => {
      const raw = await defaultRequest(port, 1000);
      assert.equal(raw.statusCode, 200);
      assert.equal(JSON.parse(raw.body).capture.extension_connected, true);
      // And end-to-end through fetchHealth (exercises defaultRequest + summarize).
      const snap = await fetchHealth(port);
      assert.equal(snap.extensionConnected, true);
      assert.equal(snap.version, '0.8.6');
    }
  );
});

test('defaultRequest returns null when the connection is refused', async () => {
  // Grab an ephemeral port, then close it so nothing is listening.
  const probe = net.createServer();
  await new Promise((res) => probe.listen(0, '127.0.0.1', res));
  const deadPort = probe.address().port;
  await new Promise((res) => probe.close(res));

  assert.equal(await defaultRequest(deadPort, 500), null);
  // fetchHealth turns that into an unreachable snapshot.
  assert.equal((await fetchHealth(deadPort, { timeoutMs: 500 })).reachable, false);
});

test('defaultRequest returns null when the server never responds (timeout)', async () => {
  await withHttpServer(
    () => { /* never write a response */ },
    async (port) => {
      assert.equal(await defaultRequest(port, 150), null);
    }
  );
});

test('waitForExtension returns reason "aborted" when the signal is already aborted', async () => {
  const controller = new AbortController();
  controller.abort();
  const result = await waitForExtension({
    signal: controller.signal,
    request: async () => null,
    now: () => 0,
    sleep: async () => {},
  });
  assert.equal(result.connected, false);
  assert.equal(result.reason, 'aborted');
});

test('waitForExtension stops within a poll cycle when aborted mid-wait', async () => {
  const clock = fakeClock();
  const controller = new AbortController();
  let polls = 0;
  const request = async () => {
    polls += 1;
    if (polls === 2) controller.abort(); // abort while waiting, extension never connects
    return { statusCode: 200, body: JSON.stringify({ capture: { extension_connected: false } }) };
  };
  const result = await waitForExtension({
    signal: controller.signal,
    request,
    now: clock.now,
    sleep: clock.sleep,
    timeoutMs: 1000000,
    pollMs: 100,
  });
  assert.equal(result.reason, 'aborted');
  assert.equal(polls, 2, 'must not poll again after the abort is observed');
});

test('connectWaitDisabled honors the opt-out env vars', () => {
  assert.equal(connectWaitDisabled({}), false);
  assert.equal(connectWaitDisabled({ KABOOM_NO_WAIT: '1' }), true);
  assert.equal(connectWaitDisabled({ KABOOM_INSTALL_NO_WAIT: 'true' }), true);
  assert.equal(connectWaitDisabled({ KABOOM_NO_WAIT: '0' }), false);
  assert.equal(connectWaitDisabled({ KABOOM_NO_WAIT: 'false' }), false);
});

test('connectHint gives a distinct, actionable message per stuck phase', () => {
  const unreachable = connectHint('daemon_unreachable', { port: 7890, extensionDir: '/x/ext' });
  const waiting = connectHint('waiting_extension', { port: 7890, extensionDir: '/x/ext' });
  assert.notEqual(unreachable, waiting);
  // Unreachable → tell them the server isn't running.
  assert.match(unreachable, /server|running|start/i);
  // Waiting → tell them to load the extension, and name the exact folder.
  assert.match(waiting, /extension/i);
  assert.match(waiting, /\/x\/ext/);
});
