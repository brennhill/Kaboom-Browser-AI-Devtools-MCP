// Tests for lib/browser.js — detecting an installed Chromium-family browser and
// opening its extensions page so "Load unpacked" is one click away.

'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const path = require('node:path');

const {
  KNOWN_BROWSERS,
  extensionsUrl,
  detectExtensionsTarget,
  openExtensionsPage,
} = require('./browser');

test('extensionsUrl uses each brand internal scheme', () => {
  assert.equal(extensionsUrl('chrome'), 'chrome://extensions/');
  assert.equal(extensionsUrl('brave'), 'brave://extensions/');
  assert.equal(extensionsUrl('edge'), 'edge://extensions/');
});

test('KNOWN_BROWSERS covers the browsers we advertise', () => {
  const ids = KNOWN_BROWSERS.map((b) => b.id);
  for (const id of ['chrome', 'brave', 'edge', 'arc', 'chromium']) {
    assert.ok(ids.includes(id), `expected ${id} in KNOWN_BROWSERS`);
  }
});

test('darwin: detects an installed app and builds an `open -a` launch', () => {
  const hasApp = (name) => name === 'Brave Browser';
  const target = detectExtensionsTarget({ platform: 'darwin', hasApp });
  assert.equal(target.id, 'brave');
  assert.equal(target.url, 'brave://extensions/');
  assert.deepEqual(target.launch, { command: 'open', args: ['-a', 'Brave Browser', 'brave://extensions/'] });
});

test('darwin: Chrome wins when several are installed (declaration order)', () => {
  const hasApp = () => true; // everything present
  const target = detectExtensionsTarget({ platform: 'darwin', hasApp });
  assert.equal(target.id, 'chrome');
  assert.equal(target.url, 'chrome://extensions/');
});

test('linux: detects a browser on PATH and launches it with the URL', () => {
  const onPath = (cmd) => cmd === 'google-chrome';
  const target = detectExtensionsTarget({ platform: 'linux', onPath });
  assert.equal(target.id, 'chrome');
  assert.deepEqual(target.launch, { command: 'google-chrome', args: ['chrome://extensions/'] });
});

test('win32: finds an exe under a base dir and returns its absolute path', () => {
  const env = { ProgramFiles: 'C:/PF' };
  const edgeExe = path.join('C:/PF', 'Microsoft', 'Edge', 'Application', 'msedge.exe');
  const fileExists = (p) => p === edgeExe;
  const target = detectExtensionsTarget({ platform: 'win32', env, fileExists });
  assert.equal(target.id, 'edge');
  assert.equal(target.url, 'edge://extensions/');
  assert.equal(target.launch.command, edgeExe);
  assert.deepEqual(target.launch.args, ['edge://extensions/']);
});

test('returns null when no supported browser is found', () => {
  assert.equal(detectExtensionsTarget({ platform: 'darwin', hasApp: () => false }), null);
  assert.equal(detectExtensionsTarget({ platform: 'linux', onPath: () => false }), null);
  assert.equal(detectExtensionsTarget({ platform: 'win32', env: {}, fileExists: () => false }), null);
  assert.equal(detectExtensionsTarget({ platform: 'sunos' }), null);
});

test('openExtensionsPage launches the target, respects opt-out, and tolerates a null target', () => {
  const calls = [];
  const spawnFn = (command, args) => {
    calls.push({ command, args });
    return { on() {}, unref() {} };
  };
  const target = { id: 'chrome', name: 'Google Chrome', url: 'chrome://extensions/', launch: { command: 'open', args: ['-a', 'Google Chrome', 'chrome://extensions/'] } };

  assert.equal(openExtensionsPage(target, { env: {}, spawnFn }), true);
  assert.deepEqual(calls[0], { command: 'open', args: ['-a', 'Google Chrome', 'chrome://extensions/'] });

  // Opt-out env shared with the folder auto-open.
  assert.equal(openExtensionsPage(target, { env: { KABOOM_NO_OPEN: '1' }, spawnFn }), false);
  // Nothing detected → nothing launched.
  assert.equal(openExtensionsPage(null, { env: {}, spawnFn }), false);

  assert.equal(calls.length, 1, 'only the happy path should have spawned');
});
