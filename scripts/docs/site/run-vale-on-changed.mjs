#!/usr/bin/env node

import { execFileSync } from 'node:child_process';
import path from 'node:path';

import { collectChangedFiles } from './changed-doc-files.mjs';

const DOCS_PREFIX = 'gokaboom.dev/src/content/docs/';
const DOC_EXT_RE = /\.(md|mdx)$/;

function getChangedFiles() {
  return collectChangedFiles({
    envFilesKey: 'VALE_FILES',
    envRangeKey: 'VALE_RANGE',
  });
}

function main() {
  const changedFiles = getChangedFiles();
  const targets = changedFiles.filter((file) => file.startsWith(DOCS_PREFIX) && DOC_EXT_RE.test(file));

  if (targets.length === 0) {
    console.log('Vale style gate: no changed gokaboom docs content files detected.');
    return;
  }

  try {
    execFileSync('vale', ['--version'], { stdio: ['ignore', 'pipe', 'pipe'] });
  } catch {
    console.error('Vale style gate failed: `vale` binary not found.');
    console.error('Install Vale from https://vale.sh/docs/vale-cli/installation/');
    process.exit(1);
  }

  const configPath = path.resolve('.vale.ini');
  const args = ['--config', configPath, ...targets];

  console.log(`Vale style gate: checking ${targets.length} file(s).`);
  execFileSync('vale', args, { stdio: 'inherit' });
}

try {
  main();
} catch (error) {
  if (typeof error.status === 'number') {
    process.exit(error.status);
  }
  console.error('Vale style gate failed:', error);
  process.exit(1);
}
