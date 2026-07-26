#!/usr/bin/env node

import { execFileSync, execSync } from 'node:child_process';
import path from 'node:path';

const DOCS_PREFIX = 'gokaboom.dev/src/content/docs/';
const DOC_EXT_RE = /\.(md|mdx)$/;

function splitEnvFileList(value) {
  return value
    .split(/[\n,]/)
    .map((item) => item.trim())
    .filter(Boolean);
}

function runGitDiff(range) {
  const output = execSync(`git diff --name-only --diff-filter=ACMR ${range}`, {
    encoding: 'utf8',
    stdio: ['ignore', 'pipe', 'ignore'],
  });

  return output
    .split('\n')
    .map((line) => line.trim())
    .filter(Boolean);
}

function getChangedFiles() {
  if (process.env.VALE_FILES) {
    return splitEnvFileList(process.env.VALE_FILES);
  }

  const changed = new Set();
  const ranges = [];

  if (process.env.VALE_RANGE) {
    ranges.push(process.env.VALE_RANGE);
  }

  if (process.env.GITHUB_BASE_REF) {
    ranges.push(`origin/${process.env.GITHUB_BASE_REF}...HEAD`);
  }

  ranges.push('HEAD~1..HEAD');

  for (const range of ranges) {
    try {
      const files = runGitDiff(range);
      for (const file of files) {
        changed.add(file);
      }
    } catch {
      // Try next range.
    }
  }

  try {
    const workingTree = execSync('git diff --name-only --diff-filter=ACMR', {
      encoding: 'utf8',
      stdio: ['ignore', 'pipe', 'ignore'],
    })
      .split('\n')
      .map((line) => line.trim())
      .filter(Boolean);

    for (const file of workingTree) {
      changed.add(file);
    }
  } catch {
    // Ignore and return gathered set.
  }

  try {
    const untracked = execSync('git ls-files --others --exclude-standard', {
      encoding: 'utf8',
      stdio: ['ignore', 'pipe', 'ignore'],
    })
      .split('\n')
      .map((line) => line.trim())
      .filter(Boolean);

    for (const file of untracked) {
      changed.add(file);
    }
  } catch {
    // Ignore and return gathered set.
  }

  return Array.from(changed);
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
