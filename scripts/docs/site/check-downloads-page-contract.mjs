#!/usr/bin/env node

import { promises as fs } from 'node:fs';
import path from 'node:path';

const repoRoot = process.cwd();
const downloadsPath = 'gokaboom.dev/src/content/docs/downloads.md';
const stylesPath = 'gokaboom.dev/src/styles/custom.css';

async function read(relativePath) {
  return fs.readFile(path.join(repoRoot, relativePath), 'utf8');
}

function hasLine(content, pattern) {
  return pattern.test(content);
}

async function main() {
  const [downloadsContent, stylesContent] = await Promise.all([read(downloadsPath), read(stylesPath)]);

  const violations = [];

  if (
    !hasLine(
      downloadsContent,
      /^- \*\*Runtime:\*\* Native Go binary \(no Node\.js required for standalone binary installs\)$/m
    )
  ) {
    violations.push('Downloads requirements must explicitly state native binary runtime without Node.js.');
  }

  if (!hasLine(downloadsContent, /^- \*\*Node\.js:\*\* 18\+ \(optional, only if you install via npm\)$/m)) {
    violations.push('Downloads requirements must keep Node.js marked optional and npm-scoped.');
  }

  if (hasLine(downloadsContent, /Node\.js:\s*18\+\s*\(for CLI tools\)/)) {
    violations.push('Legacy mandatory Node.js requirement copy must not reappear in downloads.md.');
  }

  if (
    !hasLine(stylesContent, /:root\[data-theme='light'\]\s+\.expressive-code\s+\.frame\s+\.header\s*\{/)
  ) {
    violations.push('custom.css must retain the expressive-code light-theme header override for downloads code frames.');
  }

  if (
    !hasLine(
      stylesContent,
      /Downloads page: avoid flat slate header slabs on expressive-code frames\./
    )
  ) {
    violations.push('custom.css must retain the downloads expressive-code header intent comment.');
  }

  if (violations.length === 0) {
    console.log('Downloads page contract: requirements copy and expressive-code visual guardrails passed.');
    return;
  }

  console.error('Downloads page contract violations found:\n');
  for (const violation of violations) {
    console.error(`- ${violation}`);
  }

  process.exit(1);
}

main().catch((error) => {
  console.error('Failed to run downloads page contract check:', error);
  process.exit(1);
});
