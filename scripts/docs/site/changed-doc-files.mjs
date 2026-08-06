#!/usr/bin/env node

// changed-doc-files.mjs — Collects documentation changes across every Git diff surface.

import { execFileSync } from 'node:child_process';

function splitFileList(value) {
  return value
    .split(/[\n,]/)
    .map((item) => item.trim())
    .filter(Boolean);
}

function defaultRunGit(args) {
  return execFileSync('git', args, {
    encoding: 'utf8',
    stdio: ['ignore', 'pipe', 'pipe'],
  });
}

function addOutput(changed, output) {
  for (const file of splitFileList(output)) {
    changed.add(file);
  }
}

export function collectChangedFiles({
  env = process.env,
  envFilesKey,
  envRangeKey,
  runGit = defaultRunGit,
  requireRange = false,
}) {
  if (env[envFilesKey]) {
    return splitFileList(env[envFilesKey]).sort();
  }

  const changed = new Set();
  const ranges = [];
  if (env[envRangeKey]) {
    ranges.push(env[envRangeKey]);
  }
  if (env.GITHUB_BASE_REF) {
    ranges.push(`origin/${env.GITHUB_BASE_REF}...HEAD`);
  }
  ranges.push('HEAD~1..HEAD');

  let rangeSucceeded = false;
  for (const range of ranges) {
    try {
      addOutput(changed, runGit(['diff', '--name-only', '--diff-filter=ACMR', range]));
      rangeSucceeded = true;
    } catch {
      // EXPECTED_ABSENCE: CI clones may not contain a requested base ref. Each
      // configured fallback range is tried, and requireRange rejects total loss.
    }
  }
  if (requireRange && !rangeSucceeded) {
    throw new Error(
      'All git diff ranges failed. Set the explicit files variable or use fetch-depth: 0 in CI.'
    );
  }

  addOutput(changed, runGit(['diff', '--name-only', '--diff-filter=ACMR']));
  addOutput(changed, runGit(['diff', '--cached', '--name-only', '--diff-filter=ACMR']));
  addOutput(changed, runGit(['ls-files', '--others', '--exclude-standard']));
  return [...changed].sort();
}
