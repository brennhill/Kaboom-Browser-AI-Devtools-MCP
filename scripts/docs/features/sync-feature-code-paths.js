#!/usr/bin/env node
// Purpose: Automate sync-feature-code-paths.js workflow behavior for repository tooling.
// Why: Keeps repetitive maintenance and verification steps deterministic.
// Docs: docs/DEVELOPMENT.md

import fs from 'node:fs'
import path from 'node:path'

const repoRoot = process.cwd()
const featuresRoot = path.join(repoRoot, 'docs', 'features', 'feature')
const roots = ['src', 'cmd', 'internal']

function walk(dir, out) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name)
    if (entry.isDirectory()) {
      walk(full, out)
      continue
    }
    out.push(full)
  }
}

function isTarget(file) {
  const rel = path.relative(repoRoot, file)
  if (rel.includes('/testdata/')) return false
  if (rel.endsWith('_test.go')) return false
  if (rel.endsWith('.test.ts')) return false
  if (rel.endsWith('.d.ts')) return false
  if (rel.endsWith('/doc.go')) return false
  return rel.endsWith('.ts') || rel.endsWith('.go')
}

function collectFeatureLinks() {
  const files = []
  for (const r of roots) {
    const dir = path.join(repoRoot, r)
    if (fs.existsSync(dir)) walk(dir, files)
  }

  const map = new Map()
  const rx = /Docs:\s*docs\/features\/feature\/([a-z0-9-]+)\/index\.md/g
  for (const file of files.filter(isTarget)) {
    const rel = path.relative(repoRoot, file)
    const head = fs.readFileSync(file, 'utf8').split('\n').slice(0, 60).join('\n')
    for (const m of head.matchAll(rx)) {
      const slug = m[1]
      if (!map.has(slug)) map.set(slug, [])
      map.get(slug).push(rel)
    }
  }

  for (const [k, v] of map.entries()) {
    map.set(k, Array.from(new Set(v)).sort((a, b) => a.localeCompare(b)))
  }
  return map
}

function updateIndex(indexPath, paths) {
  const original = fs.readFileSync(indexPath, 'utf8')
  const end = original.indexOf('\n---\n', 4)
  if (!original.startsWith('---\n') || end === -1) return false

  const fm = original.slice(4, end).split('\n')
  const body = original.slice(end + 5)
  const out = []
  let replaced = false
  for (let i = 0; i < fm.length; i += 1) {
    const line = fm[i]
    if (line.startsWith('code_paths:')) {
      replaced = true
      out.push('code_paths:')
      for (const p of paths) out.push(`  - ${p}`)
      while (i + 1 < fm.length && /^\s+-\s+/.test(fm[i + 1])) i += 1
    } else {
      out.push(line)
    }
  }
  if (!replaced) {
    out.push('code_paths:')
    for (const p of paths) out.push(`  - ${p}`)
  }
  const next = `---\n${out.join('\n')}\n---\n${body}`
  if (next === original) return false
  fs.writeFileSync(indexPath, next, 'utf8')
  return true
}

function main() {
  const links = collectFeatureLinks()
  let updated = 0
  if (!fs.existsSync(featuresRoot)) return

  for (const slug of fs.readdirSync(featuresRoot)) {
    const dir = path.join(featuresRoot, slug)
    if (!fs.statSync(dir).isDirectory()) continue
    const indexPath = path.join(dir, 'index.md')
    if (!fs.existsSync(indexPath)) continue
    const paths = links.get(slug) || []
    if (updateIndex(indexPath, paths)) updated += 1
  }

  console.log(`updated ${updated} feature index file(s)`)
}

main()
