#!/usr/bin/env node

import { execSync } from 'node:child_process';
import { promises as fs } from 'node:fs';
import path from 'node:path';

const DOCS_PREFIX = 'gokaboom.dev/src/content/docs/';
const DOC_EXT_RE = /\.(md|mdx)$/;
const VERSION_SURFACES = [
  {
    path: 'gokaboom.dev/src/components/Footer.astro',
    checks: [
      {
        pattern: /Docs version:\s*<strong>\{siteVersionLabel\}<\/strong>\s*\(\{siteReleaseChannel\}\)/,
        message:
          'Footer must render docs version label and latest channel from siteVersion utility.'
      }
    ]
  },
  {
    path: 'gokaboom.dev/src/pages/index.md.ts',
    checks: [
      { pattern: /from '\.\.\/utils\/siteVersion'/, message: 'Must import siteVersion utility.' },
      { pattern: /\\ndocs_version:\s*\$\{toYamlString\(siteVersionLabel\)\}/, message: 'Must set docs_version frontmatter key.' },
      { pattern: /\\ndocs_channel:\s*\$\{toYamlString\(siteReleaseChannel\)\}/, message: 'Must set docs_channel frontmatter key.' }
    ]
  },
  {
    path: 'gokaboom.dev/src/pages/[...slug].md.ts',
    checks: [
      { pattern: /from '\.\.\/utils\/siteVersion'/, message: 'Must import siteVersion utility.' },
      { pattern: /\\ndocs_version:\s*\$\{toYamlString\(siteVersionLabel\)\}/, message: 'Must set docs_version frontmatter key.' },
      { pattern: /\\ndocs_channel:\s*\$\{toYamlString\(siteReleaseChannel\)\}/, message: 'Must set docs_channel frontmatter key.' }
    ]
  },
  {
    path: 'gokaboom.dev/src/pages/markdown/[...slug].md.ts',
    checks: [
      { pattern: /from '\.\.\/\.\.\/utils\/siteVersion'/, message: 'Must import siteVersion utility.' },
      { pattern: /\\ndocs_version:\s*\$\{toYamlString\(siteVersionLabel\)\}/, message: 'Must set docs_version frontmatter key.' },
      { pattern: /\\ndocs_channel:\s*\$\{toYamlString\(siteReleaseChannel\)\}/, message: 'Must set docs_channel frontmatter key.' }
    ]
  },
  {
    path: 'gokaboom.dev/src/pages/llms.txt.ts',
    checks: [
      { pattern: /from '\.\.\/utils\/siteVersion'/, message: 'Must import siteVersion utility.' },
      {
        pattern: /# docs_version:\s*\$\{siteVersionLabel\}\s*\(\$\{siteReleaseChannel\}\)/,
        message: 'llms.txt must include docs version header line.'
      }
    ]
  },
  {
    path: 'gokaboom.dev/src/pages/llms-full.txt.ts',
    checks: [
      { pattern: /from '\.\.\/utils\/siteVersion'/, message: 'Must import siteVersion utility.' },
      {
        pattern: /# docs_version:\s*\$\{siteVersionLabel\}\s*\(\$\{siteReleaseChannel\}\)/,
        message: 'llms-full.txt must include docs version header line.'
      }
    ]
  }
];

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

function isShallowRepository() {
  try {
    const result = execSync('git rev-parse --is-shallow-repository', {
      encoding: 'utf8',
      stdio: ['ignore', 'pipe', 'ignore'],
    }).trim();
    return result === 'true';
  } catch {
    return false;
  }
}

function ensureSufficientDepth() {
  if (!isShallowRepository()) {
    return;
  }

  // Attempt to deepen the clone enough for diff ranges to work.
  try {
    execSync('git fetch --deepen=50', {
      encoding: 'utf8',
      stdio: ['ignore', 'pipe', 'ignore'],
    });
  } catch {
    throw new Error(
      'Content contract: shallow clone detected and git fetch --deepen=50 failed. ' +
      'Either use a full clone (fetch-depth: 0) or set CONTENT_CONTRACT_FILES explicitly.'
    );
  }

  // If still shallow after deepening, warn but continue — ranges may still fail.
  if (isShallowRepository()) {
    console.warn(
      'Content contract: repository is still shallow after deepening. ' +
      'Diff ranges may be incomplete. Consider fetch-depth: 0 in CI.'
    );
  }
}

function getChangedFiles() {
  if (process.env.CONTENT_CONTRACT_FILES) {
    return splitEnvFileList(process.env.CONTENT_CONTRACT_FILES);
  }

  ensureSufficientDepth();

  const changed = new Set();
  const ranges = [];

  if (process.env.CONTENT_CONTRACT_RANGE) {
    ranges.push(process.env.CONTENT_CONTRACT_RANGE);
  }

  if (process.env.GITHUB_BASE_REF) {
    ranges.push(`origin/${process.env.GITHUB_BASE_REF}...HEAD`);
  }

  ranges.push('HEAD~1..HEAD');

  let anyRangeSucceeded = false;

  for (const range of ranges) {
    try {
      const files = runGitDiff(range);
      for (const file of files) {
        changed.add(file);
      }
      anyRangeSucceeded = true;
    } catch {
      // Try next range.
    }
  }

  if (!anyRangeSucceeded && ranges.length > 0) {
    throw new Error(
      'Content contract: all git diff ranges failed. ' +
      'This may indicate a shallow clone without sufficient history. ' +
      'Set CONTENT_CONTRACT_FILES or use fetch-depth: 0 in CI.'
    );
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

function requireFrontmatterBlock(content) {
  const match = content.match(/^---\n([\s\S]*?)\n---\n?/);
  if (!match) {
    return null;
  }

  return {
    raw: match[1],
    body: content.slice(match[0].length),
  };
}

function hasFrontmatterKey(frontmatter, key) {
  return new RegExp(`^${key}:\\s*(.+)$`, 'm').test(frontmatter);
}

function hasHeading(body, headingLevel, headingText) {
  const escaped = headingText.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const pattern = new RegExp(`^${'#'.repeat(headingLevel)}\\s+${escaped}\\b`, 'm');
  return pattern.test(body);
}

function hasAnyH2(body) {
  return /^##\s+\S/m.test(body);
}

async function validateVersionSurfaceContracts() {
  const violations = [];
  for (const surface of VERSION_SURFACES) {
    const absolutePath = path.resolve(surface.path);
    const content = await fs.readFile(absolutePath, 'utf8');
    for (const check of surface.checks) {
      if (!check.pattern.test(content)) {
        violations.push(`${surface.path}: ${check.message}`);
      }
    }
  }
  return violations;
}

function validateFile(relativePath, content) {
  const errors = [];
  const frontmatter = requireFrontmatterBlock(content);

  if (!frontmatter) {
    return ['Missing frontmatter block (--- ... ---).'];
  }

  if (!hasFrontmatterKey(frontmatter.raw, 'title')) {
    errors.push('Missing frontmatter key: title');
  }

  if (!hasFrontmatterKey(frontmatter.raw, 'description')) {
    errors.push('Missing frontmatter key: description');
  }

  const isSplashTemplate = /^template:\s*splash\s*$/m.test(frontmatter.raw);

  if (!isSplashTemplate && !hasAnyH2(frontmatter.body)) {
    errors.push('Missing at least one H2 heading (## ...) in document body.');
  }

  if (relativePath.includes('/reference/')) {
    if (!hasHeading(frontmatter.body, 2, 'Quick Reference')) {
      errors.push('Reference page must include "## Quick Reference".');
    }

    if (!hasHeading(frontmatter.body, 2, 'Common Parameters')) {
      errors.push('Reference page must include "## Common Parameters".');
    }
  }

  if (relativePath.includes('/blog/')) {
    if (!hasFrontmatterKey(frontmatter.raw, 'date')) {
      errors.push('Blog post must include frontmatter key: date');
    }

    if (!hasFrontmatterKey(frontmatter.raw, 'authors')) {
      errors.push('Blog post must include frontmatter key: authors');
    }

    if (!hasFrontmatterKey(frontmatter.raw, 'tags')) {
      errors.push('Blog post must include frontmatter key: tags');
    }
  }

  return errors;
}

async function main() {
  const changed = getChangedFiles();
  const docsTargets = changed.filter((file) => file.startsWith(DOCS_PREFIX) && DOC_EXT_RE.test(file));
  const versionSurfaceViolations = await validateVersionSurfaceContracts();

  if (docsTargets.length === 0 && versionSurfaceViolations.length === 0) {
    console.log('Content contract: no changed docs/blog/articles files detected.');
    return;
  }

  const violations = [];

  for (const relativePath of docsTargets) {
    const absolutePath = path.resolve(relativePath);
    const content = await fs.readFile(absolutePath, 'utf8');
    const fileErrors = validateFile(relativePath, content);

    if (fileErrors.length > 0) {
      violations.push({ file: relativePath, errors: fileErrors });
    }
  }

  if (violations.length === 0) {
    if (versionSurfaceViolations.length === 0) {
      console.log(`Content contract: ${docsTargets.length} changed docs/blog/articles file(s) passed.`);
      return;
    }
  }

  if (versionSurfaceViolations.length > 0) {
    violations.push({
      file: 'version-surfaces',
      errors: versionSurfaceViolations
    });
  }

  if (violations.length === 0) {
    return;
  }

  console.error('Content contract violations found:\n');
  for (const violation of violations) {
    console.error(`- ${violation.file}`);
    for (const error of violation.errors) {
      console.error(`  - ${error}`);
    }
  }

  process.exit(1);
}

main().catch((error) => {
  console.error('Failed to run content contract check:', error);
  process.exit(1);
});
