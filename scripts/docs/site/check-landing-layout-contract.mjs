#!/usr/bin/env node

import { promises as fs } from 'node:fs';
import path from 'node:path';

const repoRoot = process.cwd();
const landingPath = 'gokaboom.dev/src/components/Landing.astro';
const stylesPath = 'gokaboom.dev/src/styles/custom.css';

async function read(relativePath) {
  return fs.readFile(path.join(repoRoot, relativePath), 'utf8');
}

function has(content, pattern) {
  return pattern.test(content);
}

async function main() {
  const [landingContent, stylesContent] = await Promise.all([read(landingPath), read(stylesPath)]);
  const violations = [];

  if (!has(landingContent, /solutions\.map\(\(solution,\s*index\)\s*=>/)) {
    violations.push('Landing solutions loop must expose index for alternating panel offsets.');
  }

  if (!has(landingContent, /gmodern-solution-panel--stagger-left/)) {
    violations.push('Landing solutions must include gmodern-solution-panel--stagger-left class.');
  }

  if (!has(landingContent, /gmodern-solution-panel--stagger-right/)) {
    violations.push('Landing solutions must include gmodern-solution-panel--stagger-right class.');
  }

  if (!has(stylesContent, /@media\s*\(min-width:\s*1200px\)\s*\{/)) {
    violations.push('custom.css must define a large-screen media query for staggered solution panels.');
  }

  if (!has(stylesContent, /--gmodern-panel-stagger:\s*clamp\(/)) {
    violations.push('custom.css must define --gmodern-panel-stagger for large-screen panel offsets.');
  }

  if (
    !has(
      stylesContent,
      /\.gmodern-solution-panel--stagger-left[\s\S]*?margin-right:\s*var\(--gmodern-panel-stagger\)/
    )
  ) {
    violations.push('Stagger-left panels must offset with right margin on large screens.');
  }

  if (
    !has(
      stylesContent,
      /\.gmodern-solution-panel--stagger-right[\s\S]*?margin-left:\s*var\(--gmodern-panel-stagger\)/
    )
  ) {
    violations.push('Stagger-right panels must offset with left margin on large screens.');
  }

  if (violations.length === 0) {
    console.log('Landing layout contract: large-screen staggered solution panel offsets passed.');
    return;
  }

  console.error('Landing layout contract violations found:\n');
  for (const violation of violations) {
    console.error(`- ${violation}`);
  }
  process.exit(1);
}

main().catch((error) => {
  console.error('Failed to run landing layout contract check:', error);
  process.exit(1);
});
