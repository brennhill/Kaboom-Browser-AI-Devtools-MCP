// Tests for lib/doctor.js — the live diagnosis added on top of config checks:
// Node version, daemon reachability, and extension connectivity.

'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');

const net = require('node:net');
const {
  MIN_NODE_MAJOR,
  evaluateNodeVersion,
  nodeCheck,
  checkDaemon,
  checkPort,
  runDiagnostics,
} = require('./doctor');

test('evaluateNodeVersion flags versions below the floor', () => {
  assert.equal(evaluateNodeVersion('v20.11.0').ok, true);
  assert.equal(evaluateNodeVersion('v18.0.0').ok, true);
  const old = evaluateNodeVersion('v16.20.0');
  assert.equal(old.ok, false);
  assert.equal(old.major, 16);
  assert.equal(old.minMajor, MIN_NODE_MAJOR);
  // Unparseable input must not crash — treat as unknown, not ok.
  assert.equal(evaluateNodeVersion('not-a-version').ok, false);
});

test('nodeCheck describes the running Node', () => {
  const res = nodeCheck();
  assert.match(res.version, /^v\d+\./);
  assert.equal(typeof res.ok, 'boolean');
});

test('checkDaemon reports a reachable daemon with a connected extension', async () => {
  const fetchHealthFn = async (port) => {
    assert.equal(port, 7890);
    return { reachable: true, ok: true, version: '0.8.6', extensionConnected: true, extensionLastSeen: 'now' };
  };
  const res = await checkDaemon({ port: 7890, fetchHealthFn });
  assert.equal(res.reachable, true);
  assert.equal(res.version, '0.8.6');
  assert.equal(res.extensionConnected, true);
  assert.equal(res.port, 7890);
});

test('checkDaemon reports an unreachable daemon without throwing', async () => {
  const res = await checkDaemon({ port: 7890, fetchHealthFn: async () => ({ reachable: false }) });
  assert.equal(res.reachable, false);
  assert.equal(res.extensionConnected, false);
});

test('checkPort reports a bound port as unavailable and a free one as available (cross-platform)', async () => {
  const server = net.createServer();
  await new Promise((res) => server.listen(0, '127.0.0.1', res));
  const busyPort = server.address().port;

  const busy = await checkPort(busyPort);
  assert.equal(busy.available, false);
  assert.ok(busy.error, 'busy port should carry an error message');

  await new Promise((res) => server.close(res));
  const free = await checkPort(busyPort);
  assert.equal(free.available, true);
});

test('runDiagnostics is async and includes node, daemon, and extension sections', async () => {
  const report = await runDiagnostics(false, {
    // Empty client list keeps the test hermetic — no real `claude mcp get`
    // subprocess scan. We only assert the new runtime-check wiring here.
    clients: [],
    fetchHealthFn: async () => ({ reachable: true, ok: true, version: '0.8.6', extensionConnected: false }),
  });
  assert.ok(Array.isArray(report.tools));
  assert.ok(report.node && typeof report.node.ok === 'boolean');
  assert.equal(report.daemon.reachable, true);
  assert.equal(report.extension.connected, false);
  assert.ok(typeof report.summary === 'string');
});
