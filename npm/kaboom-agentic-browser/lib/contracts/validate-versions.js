// Purpose: Implement validate-versions.js behavior for npm wrapper command flows.
// Why: Keeps distribution-channel behavior consistent and supportable.
// Docs: docs/features/feature/enhanced-cli-config/index.md

/**
 * Validate that optionalDependencies match the package version
 * Since npm package.json doesn't support variable substitution,
 * this script ensures they stay in sync during development.
 * Called automatically before releases via Makefile sync-version.
 */

const fs = require('fs');
const path = require('path');

const packagePath = path.join(__dirname, '..', '..', 'package.json');
const packageJson = JSON.parse(fs.readFileSync(packagePath, 'utf8'));

const mainVersion = packageJson.version;
const optionalDeps = packageJson.optionalDependencies || {};

let allMatch = true;
const mismatches = [];

for (const [pkg, version] of Object.entries(optionalDeps)) {
  if (version !== mainVersion) {
    allMatch = false;
    mismatches.push(`  ${pkg}: "${version}" (expected "${mainVersion}")`);
  }
}

if (!allMatch) {
  console.error('❌ Version mismatch in optionalDependencies:');
  console.error('');
  mismatches.forEach(m => console.error(m));
  console.error('');
  console.error('Fix by running: make sync-version');
  process.exit(1);
}

console.log(`✓ All optionalDependencies match version ${mainVersion}`);
