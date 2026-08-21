// resolve-binary.test.js — Contract for platform binary resolution.
// The resolver must never consult PATH: it resolves an explicit override, the
// source-tree dist build, or the platform optionalDependency — or it throws.

const test = require('node:test');
const assert = require('node:assert');
const path = require('path');

const {
  BinaryNotFoundError,
  detectPlatform,
  resolveBinary,
  SERVER_BINARY,
  HOOKS_BINARY,
} = require('./resolve-binary');

const INSTALLED_ROOT = path.join(path.sep, 'proj', 'node_modules', 'kaboom-agentic-browser');
const SOURCE_ROOT = path.join(path.sep, 'repo', 'npm', 'kaboom-agentic-browser');

function resolve(overrides = {}) {
  return resolveBinary({
    spec: SERVER_BINARY,
    env: {},
    platform: 'darwin',
    arch: 'arm64',
    packageRoot: INSTALLED_ROOT,
    existsFn: () => false,
    ...overrides,
  });
}

// --- detectPlatform ---

test('detectPlatform maps win32/arm64 onto the emulated x64 build', () => {
  const info = detectPlatform({ platform: 'win32', arch: 'arm64' });
  assert.equal(info.platformKey, 'win32-x64');
  assert.equal(info.ext, '.exe');
});

test('detectPlatform returns null for an unsupported platform', () => {
  assert.equal(detectPlatform({ platform: 'sunos', arch: 'sparc' }), null);
});

// --- the removed PATH fallback ---

test('resolveBinary throws instead of falling back to PATH when nothing is installed', () => {
  assert.throws(() => resolve(), BinaryNotFoundError);
});

test('resolveBinary never returns a bare command name', () => {
  // Regression: the old resolver returned the string 'kaboom-agentic-browser',
  // which MCP clients then resolved through PATH at launch time.
  try {
    resolve();
    assert.fail('expected BinaryNotFoundError');
  } catch (e) {
    assert.ok(e instanceof BinaryNotFoundError);
    assert.notEqual(e.message, SERVER_BINARY.command);
  }
});

test('resolveBinary throws on an unsupported platform rather than returning a command name', () => {
  assert.throws(
    () => resolve({ platform: 'sunos', arch: 'sparc' }),
    (e) => e instanceof BinaryNotFoundError && /Unsupported platform/.test(e.message)
  );
});

test('the failure names the missing optionalDependency and how to repair it', () => {
  try {
    resolve();
    assert.fail('expected BinaryNotFoundError');
  } catch (e) {
    assert.match(e.message, /@brennhill\/kaboom-agentic-browser-darwin-arm64/);
    assert.match(e.message, /npm install/);
  }
});

// --- explicit override ---

test('resolveBinary honors an explicit KABOOM_BINARY_PATH override', () => {
  const p = resolve({
    env: { KABOOM_BINARY_PATH: '/opt/kb/kaboom' },
    existsFn: (f) => f === '/opt/kb/kaboom',
  });
  assert.equal(p, path.resolve('/opt/kb/kaboom'));
});

test('a set-but-missing override fails loudly instead of silently falling through', () => {
  // Rule 25: a wrong explicit override is a real failure, not a recoverable state.
  const installed = path.join(INSTALLED_ROOT, 'node_modules', '@brennhill/kaboom-agentic-browser-darwin-arm64', 'bin', 'kaboom-agentic-browser');
  assert.throws(
    () => resolve({
      env: { KABOOM_BINARY_PATH: '/opt/kb/missing' },
      existsFn: (f) => f === installed, // a valid install exists and must NOT rescue the bad override
    }),
    (e) => e instanceof BinaryNotFoundError && /KABOOM_BINARY_PATH/.test(e.message)
  );
});

test('hooks resolution uses its own override key', () => {
  const p = resolveBinary({
    spec: HOOKS_BINARY,
    env: { KABOOM_HOOKS_BINARY_PATH: '/opt/kb/hooks' },
    platform: 'darwin',
    arch: 'arm64',
    packageRoot: INSTALLED_ROOT,
    existsFn: (f) => f === '/opt/kb/hooks',
  });
  assert.equal(p, path.resolve('/opt/kb/hooks'));
});

// --- node_modules platform package ---

test('resolveBinary finds the installed platform optionalDependency', () => {
  const expected = path.join(INSTALLED_ROOT, 'node_modules', '@brennhill/kaboom-agentic-browser-darwin-arm64', 'bin', 'kaboom-agentic-browser');
  assert.equal(resolve({ existsFn: (f) => f === expected }), path.resolve(expected));
});

test('resolveBinary finds a hoisted platform optionalDependency', () => {
  const expected = path.join(INSTALLED_ROOT, '..', '@brennhill/kaboom-agentic-browser-darwin-arm64', 'bin', 'kaboom-agentic-browser');
  assert.equal(resolve({ existsFn: (f) => f === expected }), path.resolve(expected));
});

// --- source-tree dist ---

test('the source-tree dist build wins over an installed platform package', () => {
  // The comment on the old resolver claimed it preferred a fresh dist build, but
  // the code appended dist LAST so node_modules always won. Order is now real.
  const distBin = path.resolve(SOURCE_ROOT, '..', '..', 'dist', 'kaboom-agentic-browser-darwin-arm64');
  const installed = path.join(SOURCE_ROOT, 'node_modules', '@brennhill/kaboom-agentic-browser-darwin-arm64', 'bin', 'kaboom-agentic-browser');
  const p = resolve({ packageRoot: SOURCE_ROOT, existsFn: (f) => f === distBin || f === installed });
  assert.equal(p, distBin);
});

test('a dist/ planted beside an installed package is never resolved', () => {
  // Supply-chain boundary: parent dir is "node_modules", not "npm".
  const distBin = path.resolve(INSTALLED_ROOT, '..', '..', 'dist', 'kaboom-agentic-browser-darwin-arm64');
  assert.throws(() => resolve({ existsFn: (f) => f === distBin }), BinaryNotFoundError);
});

test('the hooks dist build is not platform-suffixed', () => {
  const distBin = path.resolve(SOURCE_ROOT, '..', '..', 'dist', 'kaboom-hooks');
  const p = resolveBinary({
    spec: HOOKS_BINARY,
    env: {},
    platform: 'darwin',
    arch: 'arm64',
    packageRoot: SOURCE_ROOT,
    existsFn: (f) => f === distBin,
  });
  assert.equal(p, distBin);
});

test('win32 dist resolution carries the .exe extension', () => {
  const distBin = path.resolve(SOURCE_ROOT, '..', '..', 'dist', 'kaboom-agentic-browser-win32-x64.exe');
  const p = resolve({ platform: 'win32', arch: 'arm64', packageRoot: SOURCE_ROOT, existsFn: (f) => f === distBin });
  assert.equal(p, distBin);
});
