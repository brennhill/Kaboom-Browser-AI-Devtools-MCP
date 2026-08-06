// documentation-link-parser.test.js — Verifies docs links ignore code examples without hiding real links.

import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { mkdtempSync, mkdirSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';
import test from 'node:test';

const linter = path.resolve('scripts/docs/lint-documentation.py');

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

test('documentation compatibility facades fail the lint gate', () => {
  const output = runLint(`---\ndoc_type: legacy_alias\nstatus: archived\nlast_reviewed: 2026-08-06\n---\n\n` +
    '# Legacy Alias\n\nCanonical: [real.md](real.md)\n');

  assert.match(output, /compatibility facade.*legacy_alias/i);
  assert.match(output, /Summary: 2 errors/);
});

test('alias-only prose fails even without legacy frontmatter', () => {
  const output = runLint(`---\ndoc_type: reference\nstatus: archived\nlast_reviewed: 2026-08-06\n---\n\n` +
    '# Old Location\n\nThis file exists only to preserve old links. See the canonical documentation.\n');

  assert.match(output, /documentation compatibility facade.*forwarding-only/i);
  assert.match(output, /Summary: 1 errors/);
});

test('the documentation linter has no legacy path compatibility map', () => {
  const source = execFileSync('python3', ['-c', `print(open(${JSON.stringify(linter)}, encoding="utf-8").read())`], {
    encoding: 'utf8',
  });

  assert.doesNotMatch(source, /LEGACY_CODE_PATH_MAP/);
});

test('strict documentation validation invokes the canonical linter', () => {
  const packageJson = JSON.parse(execFileSync('node', ['-e', 'process.stdout.write(require("fs").readFileSync("package.json"))'], {
    encoding: 'utf8',
  }));

  assert.match(packageJson.scripts['docs:check:strict'], /python3 scripts\/docs\/lint-documentation\.py docs/);
});
