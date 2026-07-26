#!/usr/bin/env node

import { execSync } from 'node:child_process';
import { promises as fs } from 'node:fs';
import path from 'node:path';

const DOCS_PREFIX = 'gokaboom.dev/src/content/docs/';
const DOC_EXT_RE = /\.(md|mdx)$/;
const STYLE_TARGET_RE = /\/(articles|blog|guides)\//;

const ACRONYM_EXPANSIONS = [
  { acronym: 'API', expansions: ['Application Programming Interface'] },
  { acronym: 'CLI', expansions: ['Command Line Interface'] },
  { acronym: 'DOM', expansions: ['Document Object Model'] },
  { acronym: 'MCP', expansions: ['Model Context Protocol'] },
  { acronym: 'SEO', expansions: ['Search Engine Optimization'] },
  { acronym: 'QA', expansions: ['Quality Assurance'] },
  { acronym: 'UI', expansions: ['User Interface'] },
  { acronym: 'UX', expansions: ['User Experience'] },
  { acronym: 'LLM', expansions: ['Large Language Model'] },
  { acronym: 'ARIA', expansions: ['Accessible Rich Internet Applications'] },
  { acronym: 'SARIF', expansions: ['Static Analysis Results Interchange Format'] },
  { acronym: 'CSP', expansions: ['Content Security Policy'] },
  { acronym: 'WCAG', expansions: ['Web Content Accessibility Guidelines'] },
  { acronym: 'URL', expansions: ['Uniform Resource Locator'] },
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

function getChangedFiles() {
  if (process.env.STYLE_CONTRACT_FILES) {
    return splitEnvFileList(process.env.STYLE_CONTRACT_FILES);
  }

  const changed = new Set();
  const ranges = [];

  if (process.env.STYLE_CONTRACT_RANGE) {
    ranges.push(process.env.STYLE_CONTRACT_RANGE);
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

function getFrontmatterValue(frontmatter, key) {
  const match = frontmatter.match(new RegExp(`^${key}:\\s*(.+)$`, 'm'));
  if (!match) {
    return '';
  }
  return match[1].trim().replace(/^['"]|['"]$/g, '');
}

function normalizeProse(body) {
  return body
    .replace(/```[\s\S]*?```/g, ' ')
    .replace(/`[^`\n]+`/g, ' ')
    .replace(/<!--[\s\S]*?-->/g, ' ')
    .replace(/\[[^\]]+\]\([^)]+\)/g, ' ')
    .replace(/https?:\/\/\S+/g, ' ')
    .replace(/\s+/g, ' ');
}

function validateAcronymExpansions(relativePath, body, errors) {
  const text = normalizeProse(body);

  for (const item of ACRONYM_EXPANSIONS) {
    const regex = new RegExp(`\\b${item.acronym}\\b`);
    const match = regex.exec(text);
    if (!match) {
      continue;
    }

    const start = Math.max(0, match.index - 220);
    const end = Math.min(text.length, match.index + item.acronym.length + 220);
    const windowText = text.slice(start, end).toLowerCase();
    const hasExpansion = item.expansions.some((expansion) => windowText.includes(expansion.toLowerCase()));

    if (!hasExpansion) {
      errors.push(
        `First "${item.acronym}" mention should expand the term (for example: "${item.expansions[0]} (${item.acronym})").`
      );
    }
  }

  if (/\bWebSocket\b/.test(text) && !/persistent,?\s+two-way\b/i.test(text)) {
    errors.push('First "WebSocket" mention should include a plain-language definition (persistent, two-way connection).');
  }
}

function validateHowToContract(relativePath, frontmatter, body, errors) {
  const fileName = path.basename(relativePath);
  if (!fileName.startsWith('how-to-')) {
    return;
  }

  const title = getFrontmatterValue(frontmatter, 'title');
  if (!/^How to\b/.test(title)) {
    errors.push('How-to article title must start with "How to".');
  }

  const hasStepSection = /^##\s+Step\s+\d+/m.test(body);
  const hasNumberedSteps = /^\d+\.\s+/m.test(body);
  if (!hasStepSection && !hasNumberedSteps) {
    errors.push('How-to article must include step-by-step actions ("## Step 1" or numbered steps).');
  }

  validateAcronymExpansions(relativePath, body, errors);
}

function validateFile(relativePath, content) {
  const errors = [];
  const frontmatter = requireFrontmatterBlock(content);

  if (!frontmatter) {
    return ['Missing frontmatter block (--- ... ---).'];
  }

  const isArticle = relativePath.includes('/articles/');
  const isBlog = relativePath.includes('/blog/');

  if (isArticle || isBlog) {
    for (const key of ['date', 'authors', 'tags']) {
      if (!hasFrontmatterKey(frontmatter.raw, key)) {
        errors.push(`Missing frontmatter key: ${key}`);
      }
    }
  }

  if (isArticle && !/<!--\s*more\s*-->/.test(frontmatter.body)) {
    errors.push('Article must include excerpt marker: <!-- more -->');
  }

  if (isArticle) {
    validateHowToContract(relativePath, frontmatter.raw, frontmatter.body, errors);
  }

  return errors;
}

async function main() {
  const changed = getChangedFiles();
  const targets = changed.filter(
    (file) => file.startsWith(DOCS_PREFIX) && DOC_EXT_RE.test(file) && STYLE_TARGET_RE.test(file)
  );

  if (targets.length === 0) {
    console.log('Style contract: no changed articles/blog/guides files detected.');
    return;
  }

  const violations = [];

  for (const relativePath of targets) {
    const absolutePath = path.resolve(relativePath);
    const content = await fs.readFile(absolutePath, 'utf8');
    const fileErrors = validateFile(relativePath, content);

    if (fileErrors.length > 0) {
      violations.push({ file: relativePath, errors: fileErrors });
    }
  }

  if (violations.length === 0) {
    console.log(`Style contract: ${targets.length} changed file(s) passed.`);
    return;
  }

  console.error('Style contract violations found:\n');
  for (const violation of violations) {
    console.error(`- ${violation.file}`);
    for (const error of violation.errors) {
      console.error(`  - ${error}`);
    }
  }
  process.exit(1);
}

main().catch((error) => {
  console.error('Failed to run style contract check:', error);
  process.exit(1);
});
