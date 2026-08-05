// Purpose: Prevent npm installer compatibility shims from returning.
// Docs: docs/features/feature/enhanced-cli-config/index.md

const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');

const canonicalModules = [
  'config/auto-approve.js',
  'cli/cli.js',
  'config/codex-config.js',
  'config/config.js',
  'daemon/doctor.js',
  'installation/install.js',
  'daemon/kill-daemon.js',
  'cli/output.js',
  'installation/postinstall-skills.js',
  'installation/skills.js',
  'installation/uninstall.js',
];

const libRoot = path.resolve(__dirname, '..');

test('npm launcher modules are grouped into folders of at most ten files', () => {
  const pending = [libRoot];
  while (pending.length > 0) {
    const directory = pending.pop();
    const entries = fs.readdirSync(directory, { withFileTypes: true });
    const files = entries.filter((entry) => entry.isFile());
    assert.ok(files.length <= 10, `${path.relative(libRoot, directory) || 'lib'} has ${files.length} files`);
    pending.push(...entries.filter((entry) => entry.isDirectory()).map((entry) => path.join(directory, entry.name)));
  }
});

test('npm installer modules contain only canonical Kaboom identities', () => {
  for (const filename of canonicalModules) {
    const source = fs.readFileSync(path.join(libRoot, filename), 'utf8');
    assert.doesNotMatch(source, /\b(?:gasoline|strum)\b/i, `${filename} retains an old-brand shim`);
    assert.doesNotMatch(source, /\bLEGACY_/i, `${filename} retains a compatibility branch`);
    assert.doesNotMatch(source, /\blegacyConfig(?:Keys|Paths)\b/, `${filename} retains an old config boundary`);
    assert.doesNotMatch(
      source,
      /\blegacy_(?:removed|warnings)\b|\blegacyWarnings\b/i,
      `${filename} retains obsolete legacy reporting`
    );
  }
});
