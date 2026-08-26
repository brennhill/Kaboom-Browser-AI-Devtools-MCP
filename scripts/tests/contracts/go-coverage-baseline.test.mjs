// go-coverage-baseline.test.mjs — Contract tests for the coverage baseline
// ratchet in run-go-coverage.sh: parse failures must be loud, a missing
// baseline floors to the historical minimum, and updates never lower the bar.

import assert from 'node:assert/strict';
import { mkdtemp, readFile, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { spawnSync } from 'node:child_process';
import test from 'node:test';

const shellPath = new URL('../../build/run-go-coverage.sh', import.meta.url);

// The shell script embeds its node programs as single-quoted `node -e`
// fragments ending in `' "$BASELINE_PATH"`. Extract them verbatim so these
// tests pin the shipped script, not a copy that can drift.
async function loadFragments() {
  const shell = await readFile(shellPath, 'utf8');
  const fragments = [...shell.matchAll(/node -e '\n([\s\S]*?)' "\$BASELINE_PATH"/g)].map((m) => m[1]);
  assert.equal(fragments.length, 2, 'run-go-coverage.sh must keep exactly two node -e baseline fragments');
  return { bootstrap: fragments[0], update: fragments[1], shell };
}

function runNode(fragment, ...args) {
  return spawnSync(process.execPath, ['-e', fragment, ...args], { encoding: 'utf8' });
}

async function fixture(name, contents) {
  const dir = await mkdtemp(join(tmpdir(), 'kaboom-covbaseline-'));
  const path = join(dir, name);
  if (contents !== undefined) await writeFile(path, contents);
  return path;
}

test('script still embeds the pinned floor awk program', async () => {
  const { shell } = await loadFragments();
  const floor = shell.match(/awk -v minimum="\$MINIMUM" -v baseline="\$BASELINE_COVERAGE" '(BEGIN \{[^']*})'/);
  assert.ok(floor, 'run-go-coverage.sh must keep the FLOOR max(minimum, baseline) awk program');
  const low = spawnSync('awk', ['-v', 'minimum=89', '-v', 'baseline=0', floor[1]], { encoding: 'utf8' });
  assert.equal(low.status, 0, low.stderr);
  assert.equal(low.stdout, '89\n');
  const high = spawnSync('awk', ['-v', 'minimum=89', '-v', 'baseline=92.5', floor[1]], { encoding: 'utf8' });
  assert.equal(high.stdout, '92.5\n');
});

test('bootstrap reads a valid baseline', async () => {
  const { bootstrap } = await loadFragments();
  const path = await fixture('baseline.json', '{"version":1,"go_total_percent":92.5}\n');
  const result = runNode(bootstrap, path);
  assert.equal(result.status, 0, result.stderr);
  assert.equal(result.stdout, '92.5\n');
});

test('bootstrap: missing baseline floors to the historical minimum (first run)', async () => {
  const { bootstrap } = await loadFragments();
  const path = await fixture('absent.json');
  const result = runNode(bootstrap, path);
  assert.equal(result.status, 0, result.stderr);
  assert.equal(result.stdout, '0\n');
});

test('bootstrap: corrupt baseline JSON fails loud, not silently to 0', async () => {
  const { bootstrap } = await loadFragments();
  const path = await fixture('corrupt.json', '{ not json');
  const result = runNode(bootstrap, path);
  assert.notEqual(result.status, 0, 'invalid JSON must fail the run');
  assert.equal(result.stdout, '', 'no silent 0 fallback may be printed');
  assert.match(result.stderr, /FAIL: coverage baseline .* is not valid JSON/);
});

test('bootstrap: string-typed go_total_percent fails loud', async () => {
  const { bootstrap } = await loadFragments();
  const path = await fixture('string-percent.json', '{"version":1,"go_total_percent":"92.5"}\n');
  const result = runNode(bootstrap, path);
  assert.notEqual(result.status, 0, 'a string percent must fail the run');
  assert.equal(result.stdout, '');
  assert.match(result.stderr, /FAIL: coverage baseline .* needs a finite numeric go_total_percent/);
});

test('bootstrap: wrong baseline version fails loud', async () => {
  const { bootstrap } = await loadFragments();
  const path = await fixture('wrong-version.json', '{"version":2,"go_total_percent":90}\n');
  const result = runNode(bootstrap, path);
  assert.notEqual(result.status, 0);
  assert.equal(result.stdout, '');
  assert.match(result.stderr, /FAIL: coverage baseline .* must have version 1/);
});

test('update: refuses to lower the baseline and leaves the file untouched', async () => {
  const { update } = await loadFragments();
  const original = '{"version":1,"go_total_percent":90}\n';
  const path = await fixture('higher.json', original);
  const result = runNode(update, path, '85');
  assert.equal(result.status, 0, result.stderr);
  assert.match(result.stdout, /refusing to lower coverage baseline from 90 to 85/);
  assert.doesNotMatch(result.stdout, /ratcheted/);
  assert.equal(await readFile(path, 'utf8'), original);
});

test('update: equal or higher values write normally', async () => {
  const { update } = await loadFragments();
  const path = await fixture('lower.json', '{"version":1,"go_total_percent":85}\n');

  const equal = runNode(update, path, '85');
  assert.equal(equal.status, 0, equal.stderr);
  assert.match(equal.stdout, /ratcheted to 85/);
  assert.equal(JSON.parse(await readFile(path, 'utf8')).go_total_percent, 85);

  const higher = runNode(update, path, '90.5');
  assert.equal(higher.status, 0, higher.stderr);
  assert.match(higher.stdout, /ratcheted to 90\.5/);
  assert.equal(JSON.parse(await readFile(path, 'utf8')).go_total_percent, 90.5);
});

test('update: creates the baseline on first run', async () => {
  const { update } = await loadFragments();
  const path = await fixture('first-run.json');
  const result = runNode(update, path, '89.3');
  assert.equal(result.status, 0, result.stderr);
  assert.match(result.stdout, /ratcheted to 89\.3/);
  const written = JSON.parse(await readFile(path, 'utf8'));
  assert.equal(written.version, 1);
  assert.equal(written.go_total_percent, 89.3);
});

test('update: corrupt existing baseline fails loud instead of being overwritten', async () => {
  const { update } = await loadFragments();
  const original = '{ not json';
  const path = await fixture('corrupt-update.json', original);
  const result = runNode(update, path, '90');
  assert.notEqual(result.status, 0, 'updating over a corrupt baseline must fail the run');
  assert.match(result.stderr, /FAIL: coverage baseline .* is not valid JSON/);
  assert.equal(await readFile(path, 'utf8'), original);
});
