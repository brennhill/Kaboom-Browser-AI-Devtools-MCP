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
    // Multi-package -coverpkg runs can repeat this block with a different end.
    'example/pkg/a.go:1.1,2.8 2 1',
    'example/pkg/b.go:3.1,4.2 1 1',
    'example/pkg/c.go:5.1,6.2 3 0',
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

test('projects differently shaped subprocess blocks onto the canonical profile', async () => {
  const dir = await mkdtemp(join(tmpdir(), 'kaboom-cover-'));
  const canonical = join(dir, 'packages.out');
  const subprocess = join(dir, 'subprocess.out');
  const merged = join(dir, 'merged.out');

  await writeFile(canonical, [
    'mode: set',
    'example/pkg/install.go:10.2,12.3 4 0',
    'example/pkg/install.go:13.2,14.3 2 1',
    '',
  ].join('\n'));
  await writeFile(subprocess, [
    'mode: set',
    // go build -cover can place the closing column differently from go test.
    'example/pkg/install.go:10.2,12.18 4 1',
    // A subprocess-only shape must not become a duplicate denominator block.
    'example/pkg/install.go:13.2,14.8 2 0',
    '',
  ].join('\n'));

  const result = spawnSync(process.execPath, [merger.pathname, merged, canonical, subprocess], {
    encoding: 'utf8',
  });
  assert.equal(result.status, 0, result.stderr);
  assert.equal(await readFile(merged, 'utf8'), [
    'mode: set',
    'example/pkg/install.go:10.2,12.3 4 1',
    'example/pkg/install.go:13.2,14.3 2 1',
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
