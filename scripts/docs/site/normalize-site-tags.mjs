#!/usr/bin/env node

import { promises as fs } from 'node:fs'
import path from 'node:path'

const docsRoot = path.join(process.cwd(), 'gokaboom.dev/src/content/docs')

const synonyms = {
  'bug-triage': ['bug repro', 'bug reproduction', 'repro', 'triage', 'incident triage'],
  debugging: ['debug', 'troubleshoot', 'investigate error'],
  automation: ['workflow automation', 'auto-run', 'automate flow'],
  accessibility: ['a11y', 'wcag', 'screen reader checks'],
  'api-validation': ['contract testing', 'schema validation', 'response validation'],
  websocket: ['real-time', 'socket debugging', 'ws debugging'],
  performance: ['core web vitals', 'slow page', 'latency regression'],
  security: ['security audit', 'headers check', 'privacy checks'],
  annotations: ['ui notes', 'visual feedback', 'design feedback'],
  demos: ['demo recording', 'click-through demo', 'sales demo']
}

const aliasToCanonical = new Map()
for (const [canonical, aliases] of Object.entries(synonyms)) {
  aliasToCanonical.set(canonical, canonical)
  for (const alias of aliases) aliasToCanonical.set(alias, canonical)
}

const STOP_WORDS = new Set(['a', 'an', 'and', 'for', 'from', 'how', 'in', 'of', 'on', 'the', 'to', 'with', 'your'])

function normalizeTag(input) {
  const raw = input.trim().toLowerCase().replace(/[_\s]+/g, '-')
  return aliasToCanonical.get(raw) ?? raw
}

function parseFrontmatter(content) {
  const match = content.match(/^---\r?\n([\s\S]*?)\r?\n---\r?\n?/)
  if (!match) return null
  return {
    raw: match[0],
    frontmatter: match[1],
    body: content.slice(match[0].length)
  }
}

function slugFromPath(filePath) {
  const rel = path.relative(docsRoot, filePath).replace(/\\/g, '/')
  const noExt = rel.replace(/\.mdx?$/u, '')
  if (noExt === 'index') return 'index'
  if (noExt.endsWith('/index')) return noExt.slice(0, -('/index'.length))
  return noExt
}

function extractTags(frontmatter) {
  const tags = []

  const inlineMatch = frontmatter.match(/^tags:\s*\[(.*)\]\s*$/m)
  if (inlineMatch) {
    const inlineValues = inlineMatch[1]
      .split(',')
      .map((value) => value.trim().replace(/^['"]|['"]$/g, ''))
      .filter(Boolean)
    tags.push(...inlineValues)
  }

  const scalarMatch = frontmatter.match(/^tags:\s*['"]?([a-zA-Z0-9 _-]+)['"]?\s*$/m)
  if (scalarMatch) tags.push(scalarMatch[1].trim())

  const lines = frontmatter.split(/\r?\n/)
  const tagsIndex = lines.findIndex((line) => /^tags:\s*$/u.test(line))
  if (tagsIndex >= 0) {
    for (let i = tagsIndex + 1; i < lines.length; i += 1) {
      const line = lines[i]
      if (!/^\s*-\s+/.test(line)) break
      tags.push(line.replace(/^\s*-\s+/, '').trim().replace(/^['"]|['"]$/g, ''))
    }
  }

  return tags.filter(Boolean)
}

function collectSlugTags(slug) {
  const pieces = slug.split('/').flatMap((part) => part.split('-'))
  return pieces.filter((part) => part && part !== 'index')
}

function upsertNormalizedTags(frontmatter, tags) {
  const tagLine = `normalized_tags: [${tags.map((tag) => `'${tag}'`).join(', ')}]`
  const cleaned = frontmatter
    .replace(/^normalized_tags:[^\n]*(?:\n\s*-\s.*)*/gm, '')
    .replace(/\n{3,}/g, '\n\n')
    .replace(/\s+$/u, '')
  return `${cleaned}\n${tagLine}\n`
}

async function walk(dir) {
  const out = []
  const entries = await fs.readdir(dir, { withFileTypes: true })
  for (const entry of entries) {
    const fullPath = path.join(dir, entry.name)
    if (entry.isDirectory()) {
      out.push(...(await walk(fullPath)))
      continue
    }
    if (/\.mdx?$/u.test(entry.name)) out.push(fullPath)
  }
  return out
}

const files = await walk(docsRoot)
let changed = 0

for (const file of files) {
  const raw = await fs.readFile(file, 'utf8')
  const parsed = parseFrontmatter(raw)
  if (!parsed) continue

  const slug = slugFromPath(file)
  const extracted = extractTags(parsed.frontmatter)
  const slugTags = collectSlugTags(slug)
  const merged = [...new Set([...extracted, ...slugTags])]
    .map((value) => normalizeTag(value))
    .filter((value) => value.length >= 2 && !STOP_WORDS.has(value))

  const normalized = [...new Set(merged)].slice(0, 14)
  if (normalized.length === 0) continue

  const updatedFrontmatter = upsertNormalizedTags(parsed.frontmatter, normalized)
  const next = `---\n${updatedFrontmatter.replace(/\s+$/u, '')}\n---\n\n${parsed.body.replace(/^\s*/u, '')}`

  if (next !== raw) {
    await fs.writeFile(file, next, 'utf8')
    changed += 1
  }
}

console.log(`Normalized tags updated in ${changed} docs pages.`)
