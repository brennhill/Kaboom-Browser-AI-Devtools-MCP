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
const skillModule = 'installation/skills.js';
const staleSkillArtifactsPattern =
  /const STALE_SKILL_ARTIFACTS = Object\.freeze\(\{\r?\n[\s\S]*?\r?\n\}\);\r?\n/;

function sourceWithoutStaleSkillArtifacts(filename, source) {
  if (filename !== skillModule) return source;
  assert.match(source, staleSkillArtifactsPattern, 'skill migration data must stay explicitly bounded');
  return source.replace(staleSkillArtifactsPattern, '');
}

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
    const canonicalSource = sourceWithoutStaleSkillArtifacts(filename, source);
    assert.doesNotMatch(canonicalSource, /\b(?:gasoline|strum)\b/i, `${filename} retains an old-brand shim`);
    assert.doesNotMatch(source, /\bLEGACY_/i, `${filename} retains a compatibility branch`);
    assert.doesNotMatch(source, /\blegacyConfig(?:Keys|Paths)\b/, `${filename} retains an old config boundary`);
    assert.doesNotMatch(
      source,
      /\blegacy_(?:removed|warnings)\b|\blegacyWarnings\b/i,
      `${filename} retains obsolete legacy reporting`
    );
  }
});

test('retired skill identities are bounded, non-exported artifact cleanup data', () => {
  const source = fs.readFileSync(path.join(libRoot, skillModule), 'utf8');
  const artifactData = source.match(staleSkillArtifactsPattern)?.[0] || '';

  assert.match(artifactData, /\bgasoline\b/i);
  assert.match(artifactData, /\bstrum\b/i);
  assert.doesNotMatch(source.match(/module\.exports = \{[\s\S]*?\n\};/)?.[0] || '', /STALE_SKILL_ARTIFACTS/);
  assert.match(source, /function removeStaleFlatSkillFiles\(/);
  assert.match(source, /fs\.unlinkSync\(stalePath\)/);
});

test('retired skill artifact boundaries are portable across checkout line endings', () => {
  const source = fs.readFileSync(path.join(libRoot, skillModule), 'utf8');
  const windowsSource = source.replace(/\r?\n/g, '\r\n');

  assert.doesNotMatch(
    sourceWithoutStaleSkillArtifacts(skillModule, windowsSource),
    /\b(?:gasoline|strum)\b/i
  );
});
