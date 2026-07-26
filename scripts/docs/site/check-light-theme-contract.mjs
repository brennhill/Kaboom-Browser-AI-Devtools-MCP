#!/usr/bin/env node

import { promises as fs } from 'node:fs';
import path from 'node:path';

const repoRoot = process.cwd();
const astroConfigPath = 'gokaboom.dev/astro.config.mjs';
const themeProviderPath = 'gokaboom.dev/src/components/ThemeProvider.astro';
const themeSelectPath = 'gokaboom.dev/src/components/ThemeSelect.astro';

async function read(relativePath) {
  return fs.readFile(path.join(repoRoot, relativePath), 'utf8');
}

function has(content, pattern) {
  return pattern.test(content);
}

async function main() {
  const [astroConfig, themeProvider, themeSelect] = await Promise.all([
    read(astroConfigPath),
    read(themeProviderPath),
    read(themeSelectPath),
  ]);

  const violations = [];

  if (!has(astroConfig, /ThemeProvider:\s*['"]\.\/src\/components\/ThemeProvider\.astro['"]/)) {
    violations.push('astro.config.mjs must override Starlight ThemeProvider with src/components/ThemeProvider.astro.');
  }

  if (!has(astroConfig, /ThemeSelect:\s*['"]\.\/src\/components\/ThemeSelect\.astro['"]/)) {
    violations.push('astro.config.mjs must override Starlight ThemeSelect with src/components/ThemeSelect.astro.');
  }

  if (!has(themeProvider, /document\.documentElement\.dataset\.theme\s*=\s*['"]light['"]/)) {
    violations.push('ThemeProvider override must force document theme to light.');
  }

  if (!has(themeProvider, /localStorage\.setItem\(\s*['"]starlight-theme['"]\s*,\s*['"]light['"]\s*\)/)) {
    violations.push('ThemeProvider override must persist starlight-theme as light.');
  }

  if (!has(themeProvider, /window\.StarlightThemeProvider\s*=\s*\{/)) {
    violations.push('ThemeProvider override must provide window.StarlightThemeProvider for compatibility.');
  }

  if (!has(themeSelect, /Light-only mode: theme selector intentionally disabled\./)) {
    violations.push('ThemeSelect override should document why the selector is disabled.');
  }

  if (violations.length === 0) {
    console.log('Light theme contract: selector removal and forced light-mode behavior passed.');
    return;
  }

  console.error('Light theme contract violations found:\n');
  for (const violation of violations) {
    console.error(`- ${violation}`);
  }
  process.exit(1);
}

main().catch((error) => {
  console.error('Failed to run light theme contract check:', error);
  process.exit(1);
});
