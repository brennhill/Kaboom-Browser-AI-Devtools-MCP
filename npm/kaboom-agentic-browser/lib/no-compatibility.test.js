// Purpose: Prevent npm installer compatibility shims from returning.
// Docs: docs/features/feature/enhanced-cli-config/index.md

const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');

const canonicalModules = [
  'auto-approve.js',
  'codex-config.js',
  'config.js',
  'doctor.js',
  'install.js',
  'kill-daemon.js',
  'skills.js',
  'uninstall.js',
];

test('npm installer modules contain only canonical Kaboom identities', () => {
  for (const filename of canonicalModules) {
    const source = fs.readFileSync(path.join(__dirname, filename), 'utf8');
    assert.doesNotMatch(source, /\b(?:gasoline|strum)\b/i, `${filename} retains an old-brand shim`);
    assert.doesNotMatch(source, /\bLEGACY_/i, `${filename} retains a compatibility branch`);
    assert.doesNotMatch(source, /\blegacyConfig(?:Keys|Paths)\b/, `${filename} retains an old config boundary`);
  }
});
