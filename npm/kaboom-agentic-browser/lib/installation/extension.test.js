// Tests for lib/extension.js — resolving the extension folder and revealing it.

'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const os = require('node:os');
const path = require('node:path');
const fs = require('node:fs');

const {
  STAGED_DIR_NAME,
  isExtensionDir,
  resolveExtensionDir,
  autoOpenDisabled,
  revealCommand,
  openExtensionDir,
} = require('./extension');

function makeExtensionDir() {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-ext-'));
  fs.writeFileSync(path.join(dir, 'manifest.json'), '{"manifest_version":3}');
  return dir;
}

test('isExtensionDir requires a manifest.json', () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-noext-'));
  assert.equal(isExtensionDir(dir), false);
  fs.writeFileSync(path.join(dir, 'manifest.json'), '{}');
  assert.equal(isExtensionDir(dir), true);
  assert.equal(isExtensionDir(''), false);
});

test('resolveExtensionDir honors $KABOOM_EXTENSION_DIR when it has a manifest', () => {
  const dir = makeExtensionDir();
  const res = resolveExtensionDir({ KABOOM_EXTENSION_DIR: dir });
  assert.deepEqual(res, { dir, exists: true, source: 'env' });
});

test('resolveExtensionDir still returns the override path when it is not there yet', () => {
  // The path must be shown even before staging, so the user knows where to look.
  const res = resolveExtensionDir({ KABOOM_EXTENSION_DIR: '/does/not/exist/kaboom' });
  assert.equal(res.dir, '/does/not/exist/kaboom');
  assert.equal(res.exists, false);
  assert.equal(res.source, 'env');
});

test('resolveExtensionDir falls back to the bundled path with exists=false when nothing is staged', () => {
  // In the source tree the extension is copied in at build time, so no candidate
  // exists — the resolver must still name the bundled location, not throw. Inject
  // an empty home so a real ~/KaboomAgenticDevtoolExtension can't shadow the test.
  const emptyHome = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-home-'));
  const res = resolveExtensionDir({}, emptyHome);
  assert.equal(res.source, 'bundled');
  assert.equal(res.exists, false);
  assert.ok(res.dir.endsWith('extension'));
});

test('staged fallback name matches the installer contract', () => {
  assert.equal(STAGED_DIR_NAME, 'KaboomAgenticDevtoolExtension');
});

test('autoOpenDisabled respects opt-out env vars', () => {
  assert.equal(autoOpenDisabled({}), false);
  assert.equal(autoOpenDisabled({ KABOOM_NO_OPEN: '1' }), true);
  assert.equal(autoOpenDisabled({ KABOOM_INSTALL_NO_OPEN: 'true' }), true);
  assert.equal(autoOpenDisabled({ KABOOM_NO_OPEN: '0' }), false);
  assert.equal(autoOpenDisabled({ KABOOM_NO_OPEN: 'false' }), false);
  assert.equal(autoOpenDisabled({ KABOOM_NO_OPEN: '' }), false);
});

test('revealCommand maps each platform to its file manager, null otherwise', () => {
  assert.deepEqual(revealCommand('darwin', '/x'), { command: 'open', args: ['/x'] });
  assert.deepEqual(revealCommand('win32', 'C:/x'), { command: 'explorer', args: ['C:/x'] });
  assert.deepEqual(revealCommand('linux', '/x'), { command: 'xdg-open', args: ['/x'] });
  assert.equal(revealCommand('sunos', '/x'), null);
});

test('openExtensionDir launches the right opener and is guardable', () => {
  const dir = makeExtensionDir();
  const calls = [];
  const spawnFn = (command, args) => {
    calls.push({ command, args });
    return { on() {}, unref() {} };
  };

  // Happy path: existing extension dir, opt-in → launches the platform opener.
  assert.equal(openExtensionDir(dir, { platform: 'darwin', env: {}, spawnFn }), true);
  assert.deepEqual(calls[0], { command: 'open', args: [dir] });

  // Opt-out env → never launches.
  assert.equal(openExtensionDir(dir, { platform: 'darwin', env: { KABOOM_NO_OPEN: '1' }, spawnFn }), false);

  // Missing directory → never launches.
  assert.equal(openExtensionDir('/no/such/dir', { platform: 'darwin', env: {}, spawnFn }), false);

  // Unknown platform → never launches.
  assert.equal(openExtensionDir(dir, { platform: 'sunos', env: {}, spawnFn }), false);

  assert.equal(calls.length, 1, 'only the happy path should have spawned');
});
