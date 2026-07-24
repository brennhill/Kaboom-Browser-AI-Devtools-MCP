#!/usr/bin/env node
// Purpose: Validate docs/features bundles and strict metadata freshness requirements.
// Why: Keeps Kaboom feature docs complete and recent for CI quality gates and LLM retrieval quality.
// Docs: docs/features/feature/quality-gates/index.md

import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const requiredFiles = ['index.md', 'product-spec.md', 'tech-spec.md', 'qa-plan.md']
const requiredFrontmatterKeys = ['doc_type', 'feature_id', 'last_reviewed']

const featureDirPredicates = [
  (rel) => rel.startsWith('feature/'),
  (rel) => rel.startsWith('bug/'),
  (rel) =>
    [
      'draw-mode',
      'file-upload',
      'cli-interface',
      'lifecycle-monitoring',
      'npm-preinstall-fix',
      'mcp-persistent-server',
      'icon-regression'
    ].includes(rel)
]

function isDir(p) {
  try {
    return fs.statSync(p).isDirectory()
  } catch {
    return false
  }
}

export function parseFrontmatter(content) {
  if (!content.startsWith('---\n')) return {}
  const end = content.indexOf('\n---\n', 4)
  if (end === -1) return {}
  const lines = content.slice(4, end).split('\n')
  const out = {}
  for (const line of lines) {
    const idx = line.indexOf(':')
    if (idx <= 0) continue
    const key = line.slice(0, idx).trim()
    const value = line.slice(idx + 1).trim()
    out[key] = value
  }
  return out
}

function parseISODate(value) {
  if (typeof value !== 'string') return null
  const trimmed = value.trim()
  if (!/^\d{4}-\d{2}-\d{2}$/.test(trimmed)) return null
  const [yearRaw, monthRaw, dayRaw] = trimmed.split('-')
  const year = Number.parseInt(yearRaw, 10)
  const month = Number.parseInt(monthRaw, 10)
  const day = Number.parseInt(dayRaw, 10)
  const parsed = new Date(Date.UTC(year, month - 1, day))
  if (Number.isNaN(parsed.getTime())) return null
  if (parsed.getUTCFullYear() !== year || parsed.getUTCMonth() !== month - 1 || parsed.getUTCDate() !== day) {
    return null
  }
  return parsed
}

function isStaleReviewDate(date, now, freshnessWindowDays) {
  const dayMs = 24 * 60 * 60 * 1000
  return now.getTime() - date.getTime() > freshnessWindowDays * dayMs
}

export function discoverFeatureDirs(featuresRoot) {
  const dirs = []
  const stack = [featuresRoot]
  while (stack.length > 0) {
    const current = stack.pop()
    if (!current || !isDir(current)) continue
    for (const entry of fs.readdirSync(current)) {
      const full = path.join(current, entry)
      if (!isDir(full)) continue
      stack.push(full)
      const rel = path.relative(featuresRoot, full)
      if (featureDirPredicates.some((fn) => fn(rel))) {
        dirs.push(full)
      }
    }
  }
  return dirs.sort((a, b) => a.localeCompare(b))
}

export function checkFeatureBundles({
  repoRoot = process.cwd(),
  strictFrontmatter = process.env.DOCS_STRICT_FRONTMATTER === '1',
  enforceFeatureFreshness = process.env.DOCS_STRICT_FEATURE_FRESHNESS !== '0',
  // Annual review window. A 30-day window across 130+ feature bundles is not
  // hand-maintainable — it forces mass date-bumps that assert a review nobody
  // performed, which is worse than an honest yearly cadence. Override per-run
  // with DOCS_FEATURE_FRESHNESS_DAYS when a tighter check is wanted.
  freshnessWindowDays = (() => {
    const parsed = Number.parseInt(process.env.DOCS_FEATURE_FRESHNESS_DAYS || '365', 10)
    return Number.isFinite(parsed) && parsed > 0 ? parsed : 365
  })(),
  now = new Date()
} = {}) {
  const featuresRoot = path.join(repoRoot, 'docs', 'features')
  const dirs = discoverFeatureDirs(featuresRoot)
  const issues = []

  for (const dir of dirs) {
    const rel = path.relative(repoRoot, dir)
    for (const fileName of requiredFiles) {
      const filePath = path.join(dir, fileName)
      if (!fs.existsSync(filePath)) {
        issues.push(`${rel}: missing ${fileName}`)
        continue
      }

      const content = fs.readFileSync(filePath, 'utf8')
      const fm = parseFrontmatter(content)

      // Progressive rollout:
      // - Always require frontmatter metadata on index.md
      // - Require it on all files only when DOCS_STRICT_FRONTMATTER=1
      const shouldCheckFrontmatter = fileName === 'index.md' || strictFrontmatter
      if (!shouldCheckFrontmatter) continue

      for (const key of requiredFrontmatterKeys) {
        if (!fm[key]) {
          issues.push(`${rel}/${fileName}: missing frontmatter key '${key}'`)
        }
      }

      if (!enforceFeatureFreshness || !fm.last_reviewed) continue

      const reviewedAt = parseISODate(fm.last_reviewed)
      if (!reviewedAt) {
        issues.push(`${rel}/${fileName}: invalid last_reviewed value '${fm.last_reviewed}' (expected valid YYYY-MM-DD)`)
        continue
      }

      if (isStaleReviewDate(reviewedAt, now, freshnessWindowDays)) {
        issues.push(`${rel}/${fileName}: stale last_reviewed '${fm.last_reviewed}' (> ${freshnessWindowDays} days old)`)
      }
    }
  }

  return { dirs, issues }
}

export function runCheckFeatureBundlesCLI(options = {}) {
  const { dirs, issues } = checkFeatureBundles(options)
  if (issues.length > 0) {
    console.error(`feature bundle check failed: ${issues.length} issue(s)`)
    for (const issue of issues.slice(0, 200)) {
      console.error(`- ${issue}`)
    }
    if (issues.length > 200) {
      console.error(`... and ${issues.length - 200} more`)
    }
    process.exit(1)
  }

  console.log(`feature bundle check passed for ${dirs.length} feature directories`)
}

const isDirectRun = process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)
if (isDirectRun) {
  runCheckFeatureBundlesCLI()
}
