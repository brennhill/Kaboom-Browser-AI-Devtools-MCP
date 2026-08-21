// postinstall-shims.test.js — Windows shim rewiring contract.

const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const os = require('os');
const path = require('path');

const { wireWindowsShims } = require('./postinstall-shims');

function layout() {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-shims-'));
  const pkg = path.join(root, 'node_modules', 'kaboom-agentic-browser');
  fs.mkdirSync(path.join(root, 'node_modules', '.bin'), { recursive: true });
  fs.mkdirSync(pkg, { recursive: true });
  return { root, pkg, binDir: path.join(root, 'node_modules', '.bin') };
}

test('does nothing on POSIX, where the sh exec shim is the point', () => {
  const r = wireWindowsShims({ platform: 'darwin' });
  assert.equal(r.skipped, true);
  assert.deepEqual(r.written, []);
});

test('rewrites both .cmd and .ps1 shims on win32', () => {
  const { pkg, binDir } = layout();
  const r = wireWindowsShims({ platform: 'win32', packageRoot: pkg });
  assert.equal(r.skipped, false);
  assert.deepEqual(r.failures, []);
  for (const f of ['kaboom-agentic-browser.cmd', 'kaboom-agentic-browser.ps1', 'kaboom-hooks.cmd', 'kaboom-hooks.ps1']) {
    assert.ok(fs.existsSync(path.join(binDir, f)), `${f} not written`);
  }
});

test('the replacement shims never invoke sh', () => {
  // The whole reason for this step: npm reads the `#!/bin/sh` shebang of the bin
  // entry and generates a shim that runs `sh`, which Windows may not have.
  const { pkg, binDir } = layout();
  wireWindowsShims({ platform: 'win32', packageRoot: pkg });
  for (const f of fs.readdirSync(binDir)) {
    const body = fs.readFileSync(path.join(binDir, f), 'utf8');
    assert.doesNotMatch(body, /\bsh\.exe\b|"\s*sh\s*"|\bsh\s+"/, `${f} still routes through sh`);
    assert.match(body, /bin[\\/]kaboom-(agentic-browser|hooks)\.cmd/, `${f} does not point at our batch launcher`);
  }
});

test('the replacement shims put no Node runtime in front of the binary', () => {
  const { pkg, binDir } = layout();
  wireWindowsShims({ platform: 'win32', packageRoot: pkg });
  const cmd = fs.readFileSync(path.join(binDir, 'kaboom-agentic-browser.cmd'), 'utf8');
  assert.doesNotMatch(cmd, /\bnode\b/);
});

test('skips cleanly when npm created no .bin directory', () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-shims-'));
  const pkg = path.join(root, 'node_modules', 'kaboom-agentic-browser');
  fs.mkdirSync(pkg, { recursive: true });
  const r = wireWindowsShims({ platform: 'win32', packageRoot: pkg });
  assert.equal(r.skipped, true);
  assert.match(r.reason, /no \.bin directory/);
});

test('reports write failures instead of swallowing them', () => {
  const { pkg } = layout();
  const r = wireWindowsShims({
    platform: 'win32',
    packageRoot: pkg,
    fsImpl: {
      existsSync: () => true,
      writeFileSync: () => { throw new Error('EACCES'); },
    },
  });
  assert.equal(r.failures.length, 4);
  assert.match(r.failures[0].error, /EACCES/);
});

test('forwarders use CRLF line endings', () => {
  const { pkg, binDir } = layout();
  wireWindowsShims({ platform: 'win32', packageRoot: pkg });
  const raw = fs.readFileSync(path.join(binDir, 'kaboom-hooks.cmd'), 'latin1');
  assert.ok(raw.includes('\r\n'));
  assert.doesNotMatch(raw, /[^\r]\n/);
});
