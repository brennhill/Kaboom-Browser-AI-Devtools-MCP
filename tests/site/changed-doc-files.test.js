// changed-doc-files.test.js — Verifies documentation gates inspect every Git change surface.

import assert from 'node:assert/strict';
import test from 'node:test';

import { collectChangedFiles } from '../../scripts/docs/site/changed-doc-files.mjs';

test('collectChangedFiles includes staged rename destinations', () => {
  const commands = [];
  const outputs = new Map([
    ['diff --name-only --diff-filter=ACMR HEAD~1..HEAD', 'docs/committed.md\n'],
    ['diff --name-only --diff-filter=ACMR', 'docs/unstaged.md\n'],
    ['diff --cached --name-only --diff-filter=ACMR', 'docs/renamed/index.md\n'],
    ['ls-files --others --exclude-standard', 'docs/untracked.md\n'],
  ]);

  const files = collectChangedFiles({
    env: {},
    envFilesKey: 'DOC_FILES',
    envRangeKey: 'DOC_RANGE',
    runGit(args) {
      const command = args.join(' ');
      commands.push(command);
      return outputs.get(command) ?? '';
    },
  });

  assert.deepEqual(files, [
    'docs/committed.md',
    'docs/renamed/index.md',
    'docs/unstaged.md',
    'docs/untracked.md',
  ]);
  assert.ok(commands.includes('diff --cached --name-only --diff-filter=ACMR'));
});

test('collectChangedFiles honors an explicit file list without invoking Git', () => {
  const files = collectChangedFiles({
    env: { DOC_FILES: 'docs/one.md,\ndocs/two.mdx' },
    envFilesKey: 'DOC_FILES',
    envRangeKey: 'DOC_RANGE',
    runGit() {
      assert.fail('Git must not run when the caller supplies an explicit file list');
    },
  });

  assert.deepEqual(files, ['docs/one.md', 'docs/two.mdx']);
});
