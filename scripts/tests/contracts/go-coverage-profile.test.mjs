// go-coverage-profile.test.mjs — Contract tests for honest Go profile merging.

import assert from 'node:assert/strict';
import { mkdtemp, readFile, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { spawnSync } from 'node:child_process';
import test from 'node:test';

const merger = new URL('../../build/merge-go-coverage.mjs', import.meta.url);

test('merges duplicate blocks by maximum execution count', async () => {
  const dir = await mkdtemp(join(tmpdir(), 'kaboom-cover-'));
  const first = join(dir, 'first.out');
  const second = join(dir, 'second.out');
  const merged = join(dir, 'merged.out');

  await writeFile(first, [
    'mode: set',
    'example/pkg/a.go:1.1,2.2 2 0',
    'example/pkg/b.go:3.1,4.2 1 1',
    '',
  ].join('\n'));
  await writeFile(second, [
    'mode: set',
    'example/pkg/a.go:1.1,2.2 2 1',
    'example/pkg/c.go:5.1,6.2 3 0',
    '',
  ].join('\n'));

  const result = spawnSync(process.execPath, [merger.pathname, merged, first, second], {
    encoding: 'utf8',
  });
  assert.equal(result.status, 0, result.stderr);
  assert.equal(await readFile(merged, 'utf8'), [
    'mode: set',
    'example/pkg/a.go:1.1,2.2 2 1',
    'example/pkg/b.go:3.1,4.2 1 1',
    'example/pkg/c.go:5.1,6.2 3 0',
    '',
  ].join('\n'));
});

test('rejects incompatible profile modes', async () => {
  const dir = await mkdtemp(join(tmpdir(), 'kaboom-cover-'));
  const first = join(dir, 'first.out');
  const second = join(dir, 'second.out');
  const merged = join(dir, 'merged.out');
  await writeFile(first, 'mode: set\nexample/pkg/a.go:1.1,2.2 2 1\n');
  await writeFile(second, 'mode: atomic\nexample/pkg/a.go:1.1,2.2 2 1\n');

  const result = spawnSync(process.execPath, [merger.pathname, merged, first, second], {
    encoding: 'utf8',
  });
  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /profile mode mismatch/);
});
