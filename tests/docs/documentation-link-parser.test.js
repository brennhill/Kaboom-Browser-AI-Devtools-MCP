// documentation-link-parser.test.js — Verifies docs links ignore code examples without hiding real links.

import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { mkdtempSync, mkdirSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';
import test from 'node:test';

const linter = path.resolve('scripts/lint-documentation.py');

function runLint(markdown) {
  const root = mkdtempSync(path.join(tmpdir(), 'kaboom-doc-lint-'));
  mkdirSync(path.join(root, 'docs'));
  writeFileSync(path.join(root, 'docs', 'fixture.md'), markdown);
  try {
    return execFileSync('python3', [linter, 'docs'], {
      cwd: root,
      encoding: 'utf8',
      stdio: ['ignore', 'pipe', 'pipe'],
    });
  } catch (error) {
    return `${error.stdout ?? ''}${error.stderr ?? ''}`;
  }
}

test('inline and fenced code are not parsed as Markdown links', () => {
  const output = runLint(`---\ndoc_type: reference\nstatus: active\nlast_reviewed: 2026-08-06\n---\n\n` +
    '`parseArgs[Params](req, args)`\n\n```go\nparseArgs[Params](req, args)\n```\n');

  assert.match(output, /Summary: 0 errors/);
});

test('actual broken Markdown links still fail', () => {
  const output = runLint(`---\ndoc_type: reference\nstatus: active\nlast_reviewed: 2026-08-06\n---\n\n` +
    '[Missing](missing.md)\n');

  assert.match(output, /broken link to missing\.md/);
});
