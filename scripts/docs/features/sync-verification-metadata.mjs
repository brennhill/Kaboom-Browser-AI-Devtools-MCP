#!/usr/bin/env node

import { promises as fs } from 'node:fs'
import path from 'node:path'

const repoRoot = process.cwd()
const version = (await fs.readFile(path.join(repoRoot, 'VERSION'), 'utf8')).trim()
const today = new Date().toISOString().slice(0, 10)

const roots = [
  {
    dir: 'docs/features',
    requireFrontmatter: true,
    updateLastReviewed: true
  },
  {
    dir: 'docs/architecture',
    requireFrontmatter: true,
    updateLastReviewed: true
  },
  {
    dir: 'gokaboom.dev/src/content/docs',
    requireFrontmatter: false,
    updateLastReviewed: false
  }
]

function parseFrontmatter(content) {
  const match = content.match(/^---\r?\n([\s\S]*?)\r?\n---\r?\n?/)
  if (!match) {
    return { hasFrontmatter: false, frontmatter: '', body: content }
  }
  return {
    hasFrontmatter: true,
    frontmatter: match[1],
    body: content.slice(match[0].length)
  }
}

function escapeRegExp(value) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

function hasKey(frontmatter, key) {
  const re = new RegExp(`^${escapeRegExp(key)}:\\s*`, 'm')
  return re.test(frontmatter)
}

function upsertKey(frontmatter, key, value) {
  const line = `${key}: ${value}`
  const re = new RegExp(`^${escapeRegExp(key)}:\\s*.*$`, 'm')
  if (re.test(frontmatter)) {
    return frontmatter.replace(re, line)
  }
  const trimmed = frontmatter.replace(/\s+$/u, '')
  return trimmed.length > 0 ? `${trimmed}\n${line}\n` : `${line}\n`
}

async function collectMarkdownFiles(dir) {
  const out = []
  const entries = await fs.readdir(dir, { withFileTypes: true })
  for (const entry of entries) {
    const fullPath = path.join(dir, entry.name)
    if (entry.isDirectory()) {
      out.push(...(await collectMarkdownFiles(fullPath)))
      continue
    }
    if (/\.mdx?$/u.test(entry.name)) {
      out.push(fullPath)
    }
  }
  return out
}

let changed = 0

for (const root of roots) {
  const absRoot = path.join(repoRoot, root.dir)
  const files = await collectMarkdownFiles(absRoot)

  for (const file of files) {
    const raw = await fs.readFile(file, 'utf8')
    const parsed = parseFrontmatter(raw)
    let frontmatter = parsed.frontmatter
    const body = parsed.body

    if (!parsed.hasFrontmatter) {
      if (!root.requireFrontmatter) {
        continue
      }
      frontmatter = ''
    }

    if (root.requireFrontmatter) {
      if (!hasKey(frontmatter, 'doc_type') && !hasKey(frontmatter, 'status')) {
        frontmatter = upsertKey(frontmatter, 'doc_type', 'legacy_doc')
        frontmatter = upsertKey(frontmatter, 'status', 'reference')
      }
      if (root.updateLastReviewed) {
        frontmatter = upsertKey(frontmatter, 'last_reviewed', today)
      }
    }

    frontmatter = upsertKey(frontmatter, 'last_verified_version', version)
    frontmatter = upsertKey(frontmatter, 'last_verified_date', today)

    const next = `---\n${frontmatter.replace(/\s+$/u, '')}\n---\n\n${body.replace(/^\s*/u, '')}`

    if (next !== raw) {
      await fs.writeFile(file, next, 'utf8')
      changed += 1
    }
  }
}

console.log(`Updated verification metadata in ${changed} files (version ${version}, date ${today}).`)
