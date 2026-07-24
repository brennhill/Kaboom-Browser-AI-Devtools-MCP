// Tests for lib/daemon.js — starting the Kaboom daemon during npm --install so
// the browser extension has something to connect to.

'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');

const {
  DEFAULT_PORT,
  daemonSpawnArgs,
  daemonStartDisabled,
  startDaemon,
  ensureDaemon,
} = require('./daemon');

test('daemonSpawnArgs matches the flags the Go installer uses', () => {
  assert.deepEqual(daemonSpawnArgs(7890), ['--daemon', '--port', '7890']);
  assert.deepEqual(daemonSpawnArgs(7999), ['--daemon', '--port', '7999']);
});

test('daemonStartDisabled honors KABOOM_NO_DAEMON', () => {
  assert.equal(daemonStartDisabled({}), false);
  assert.equal(daemonStartDisabled({ KABOOM_NO_DAEMON: '1' }), true);
  assert.equal(daemonStartDisabled({ KABOOM_NO_DAEMON: '0' }), false);
});

test('startDaemon spawns the binary detached and unrefs it', () => {
  const calls = [];
  let unrefd = false;
  const spawnFn = (command, args, options) => {
    calls.push({ command, args, options });
    return { on() {}, unref() { unrefd = true; } };
  };
  const res = startDaemon({ binaryPath: '/opt/kaboom', port: 7890, spawnFn, env: {} });
  assert.equal(res.started, true);
  assert.equal(res.binaryPath, '/opt/kaboom');
  assert.deepEqual(calls[0].command, '/opt/kaboom');
  assert.deepEqual(calls[0].args, ['--daemon', '--port', '7890']);
  assert.equal(calls[0].options.detached, true);
  assert.equal(unrefd, true);
});

test('startDaemon respects the opt-out and never spawns', () => {
  let spawned = false;
  const spawnFn = () => { spawned = true; return { on() {}, unref() {} }; };
  const res = startDaemon({ binaryPath: '/opt/kaboom', spawnFn, env: { KABOOM_NO_DAEMON: '1' } });
  assert.equal(res.started, false);
  assert.equal(spawned, false);
});

test('startDaemon reports started=false when spawn throws', () => {
  const spawnFn = () => { throw new Error('ENOENT'); };
  const res = startDaemon({ binaryPath: '/opt/kaboom', spawnFn, env: {} });
  assert.equal(res.started, false);
});

test('ensureDaemon reuses a daemon that is already answering', async () => {
  let started = false;
  const res = await ensureDaemon({
    port: 7890,
    fetchHealthFn: async () => ({ reachable: true }),
    startFn: () => { started = true; return { started: true }; },
  });
  assert.equal(res.alreadyRunning, true);
  assert.equal(res.started, false);
  assert.equal(started, false, 'must not start a duplicate daemon');
});

test('ensureDaemon starts the daemon when nothing is answering', async () => {
  let started = false;
  const res = await ensureDaemon({
    port: 7890,
    fetchHealthFn: async () => ({ reachable: false }),
    startFn: () => { started = true; return { started: true, binaryPath: '/opt/kaboom' }; },
  });
  assert.equal(res.alreadyRunning, false);
  assert.equal(res.started, true);
  assert.equal(started, true);
});
