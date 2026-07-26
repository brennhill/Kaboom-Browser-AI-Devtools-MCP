#!/usr/bin/env node
// Purpose: Automate normalize-docs.js workflow behavior for repository tooling.
// Why: Keeps repetitive maintenance and verification steps deterministic.
// Docs: docs/DEVELOPMENT.md

import fs from 'node:fs'
import path from 'node:path'

const repoRoot = path.resolve(process.cwd())
const docsRoot = path.join(repoRoot, 'docs')
const featuresRoot = path.join(docsRoot, 'features')
const today = new Date().toISOString().slice(0, 10)

const featureDirPredicates = [
  (rel) => rel.startsWith('feature/'),
  (rel) => rel.startsWith('bug/'),
  (rel) => rel === 'draw-mode',
  (rel) => rel === 'file-upload',
  (rel) => rel === 'cli-interface',
  (rel) => rel === 'lifecycle-monitoring',
  (rel) => rel === 'npm-preinstall-fix',
  (rel) => rel === 'mcp-persistent-server',
  (rel) => rel === 'icon-regression'
]

const requiredFiles = ['product-spec.md', 'tech-spec.md', 'qa-plan.md']

function exists(p) {
  try {
    fs.accessSync(p)
    return true
  } catch {
    return false
  }
}

function isDir(p) {
  try {
    return fs.statSync(p).isDirectory()
  } catch {
    return false
  }
}

function readFileSafe(p) {
  try {
    return fs.readFileSync(p, 'utf8')
  } catch {
    return ''
  }
}

function parseFrontmatter(content) {
  if (!content.startsWith('---\n')) return {}
  const end = content.indexOf('\n---\n', 4)
  if (end === -1) return {}
  const fm = content.slice(4, end).split('\n')
  const out = {}
  for (const line of fm) {
    const idx = line.indexOf(':')
    if (idx <= 0) continue
    const key = line.slice(0, idx).trim()
    const value = line.slice(idx + 1).trim().replace(/^"|"$/g, '')
    out[key] = value
  }
  return out
}

function ensureRequiredFrontmatter(filePath, requiredMap) {
  const content = readFileSafe(filePath)
  if (!content) return false

  if (!content.startsWith('---\n')) {
    const fmLines = Object.entries(requiredMap).map(([k, v]) => `${k}: ${v}`)
    const next = `---\n${fmLines.join('\n')}\n---\n\n${content}`
    fs.writeFileSync(filePath, next, 'utf8')
    return true
  }

  const end = content.indexOf('\n---\n', 4)
  if (end === -1) return false

  const fmBlock = content.slice(4, end)
  const body = content.slice(end + 5)
  const lines = fmBlock.split('\n')
  const existing = new Set(
    lines
      .map((line) => {
        const idx = line.indexOf(':')
        return idx > 0 ? line.slice(0, idx).trim() : ''
      })
      .filter(Boolean)
  )

  const missing = Object.entries(requiredMap)
    .filter(([k]) => !existing.has(k))
    .map(([k, v]) => `${k}: ${v}`)

  if (missing.length === 0) return false

  const next = `---\n${lines.concat(missing).join('\n')}\n---\n${body}`
  fs.writeFileSync(filePath, next, 'utf8')
  return true
}

function toFeatureId(relDir) {
  return relDir.replaceAll(path.sep, '-').replace(/[^a-zA-Z0-9-]+/g, '-').toLowerCase()
}

function titleFromSlug(slug) {
  return slug
    .split('-')
    .filter(Boolean)
    .map((part) => part[0].toUpperCase() + part.slice(1))
    .join(' ')
}

function classify(relDir) {
  if (relDir.startsWith('bug/')) return 'bug'
  if (relDir.startsWith('feature/')) return 'feature'
  return 'feature'
}

function discoverFeatureDirs() {
  const out = []
  const stack = [featuresRoot]
  while (stack.length > 0) {
    const cur = stack.pop()
    if (!cur || !isDir(cur)) continue
    const entries = fs.readdirSync(cur)
    for (const entry of entries) {
      const full = path.join(cur, entry)
      if (!isDir(full)) continue
      stack.push(full)
      const rel = path.relative(featuresRoot, full)
      if (featureDirPredicates.some((fn) => fn(rel))) {
        out.push(full)
      }
    }
  }
  return out.sort((a, b) => a.localeCompare(b))
}

function makeSpecStub({ type, relDir: _relDir, featureId, title, status, tool, mode }) {
  const reqPrefix = featureId.replace(/[^a-z0-9]/g, '_').toUpperCase()
  const docTitle = type === 'product-spec.md' ? 'Product Spec' : type === 'tech-spec.md' ? 'Tech Spec' : 'QA Plan'
  return `---
doc_type: ${type.replace('.md', '')}
feature_id: ${featureId}
status: ${status}
owners: []
last_reviewed: ${today}
links:
  product: ./product-spec.md
  tech: ./tech-spec.md
  qa: ./qa-plan.md
  feature_index: ./index.md
---

# ${title} ${docTitle}

## TL;DR

- Status: ${status}
- Tool: ${tool}
- Mode/Action: ${mode}
- This document is a generated placeholder and should be completed.

## Linked Specs

- Product: [product-spec.md](./product-spec.md)
- Tech: [tech-spec.md](./tech-spec.md)
- QA: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- ${reqPrefix}_001
- ${reqPrefix}_002
- ${reqPrefix}_003

## Notes

Fill this file with feature-specific details and reference code/test paths used by this feature.
`
}

function makeFeatureIndex({ relDir, featureId, title, status, tool, mode }) {
  const reqPrefix = featureId.replace(/[^a-z0-9]/g, '_').toUpperCase()
  return `---
doc_type: feature_index
feature_id: ${featureId}
status: ${status}
feature_type: ${classify(relDir)}
owners: []
last_reviewed: ${today}
code_paths: []
test_paths: []
---

# ${title}

## TL;DR

- Status: ${status}
- Tool: ${tool}
- Mode/Action: ${mode}
- Location: \`docs/features/${relDir}\`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- ${reqPrefix}_001
- ${reqPrefix}_002
- ${reqPrefix}_003

## Code and Tests

Add concrete implementation and test links here as this feature evolves.
`
}

function normalize() {
  const featureDirs = discoverFeatureDirs()
  const rows = []
  let createdIndexes = 0
  let createdSpecs = 0
  let updatedFrontmatter = 0

  for (const dir of featureDirs) {
    const relDir = path.relative(featuresRoot, dir)
    const featureId = toFeatureId(relDir)
    const slug = path.basename(dir)
    const title = titleFromSlug(slug)

    const productPath = path.join(dir, 'product-spec.md')
    const techPath = path.join(dir, 'tech-spec.md')
    const qaPath = path.join(dir, 'qa-plan.md')

    const firstExisting = [productPath, techPath, qaPath].find((p) => exists(p))
    const fm = firstExisting ? parseFrontmatter(readFileSafe(firstExisting)) : {}

    const status = fm.status || 'proposed'
    const tool = fm.tool || 'tbd'
    const mode = fm.mode || 'tbd'

    for (const specName of requiredFiles) {
      const specPath = path.join(dir, specName)
      if (!exists(specPath)) {
        fs.writeFileSync(
          specPath,
          makeSpecStub({ type: specName, relDir, featureId, title, status, tool, mode }),
          'utf8'
        )
        createdSpecs += 1
      }

      const docType = specName.replace('.md', '')
      const changed = ensureRequiredFrontmatter(specPath, {
        doc_type: docType,
        feature_id: featureId,
        last_reviewed: today
      })
      if (changed) updatedFrontmatter += 1
    }

    const indexPath = path.join(dir, 'index.md')
    if (!exists(indexPath)) {
      fs.writeFileSync(indexPath, makeFeatureIndex({ relDir, featureId, title, status, tool, mode }), 'utf8')
      createdIndexes += 1
    }
    const indexChanged = ensureRequiredFrontmatter(indexPath, {
      doc_type: 'feature_index',
      feature_id: featureId,
      last_reviewed: today
    })
    if (indexChanged) updatedFrontmatter += 1

    rows.push({
      featureId,
      relDir,
      title,
      status,
      tool,
      mode
    })
  }

  rows.sort((a, b) => a.featureId.localeCompare(b.featureId))

  const statusCounts = rows.reduce((acc, row) => {
    acc[row.status] = (acc[row.status] || 0) + 1
    return acc
  }, {})

  const featureIndexPath = path.join(featuresRoot, 'feature-index.md')
  const featureIndex = `---
doc_type: docs_index
scope: features
last_reviewed: ${today}
---

# Feature Index

Canonical index of feature docs with direct links to per-feature spec bundles.

| Feature ID | Title | Status | Tool | Mode/Action | Path |
|---|---|---|---|---|---|
${rows
  .map((r) => `| ${r.featureId} | ${r.title} | ${r.status} | ${r.tool} | ${r.mode} | [docs/features/${r.relDir}](./${r.relDir}/index.md) |`)
  .join('\n')}

## Status Summary

| Status | Count |
|---|---|
${Object.entries(statusCounts)
  .sort((a, b) => a[0].localeCompare(b[0]))
  .map(([k, v]) => `| ${k} | ${v} |`)
  .join('\n')}
`
  fs.writeFileSync(featureIndexPath, featureIndex, 'utf8')

  const traceabilityPath = path.join(docsRoot, 'traceability', 'feature-map.md')
  const traceability = `---
doc_type: traceability_map
scope: features
last_reviewed: ${today}
---

# Feature Traceability Map

This map links each feature to its product/tech/qa bundle and requirement ID prefix.

| Feature ID | Requirement Prefix | Product | Tech | QA |
|---|---|---|---|---|
${rows
  .map((r) => {
    const prefix = r.featureId.replace(/[^a-z0-9]/g, '_').toUpperCase()
    return `| ${r.featureId} | ${prefix}_* | [product](../features/${r.relDir}/product-spec.md) | [tech](../features/${r.relDir}/tech-spec.md) | [qa](../features/${r.relDir}/qa-plan.md) |`
  })
  .join('\n')}
`
  fs.writeFileSync(traceabilityPath, traceability, 'utf8')

  console.log(
    JSON.stringify(
      {
        features: rows.length,
        createdIndexes,
        createdSpecs,
        updatedFrontmatter,
        wrote: [
          path.relative(repoRoot, featureIndexPath),
          path.relative(repoRoot, traceabilityPath)
        ]
      },
      null,
      2
    )
  )
}

normalize()
