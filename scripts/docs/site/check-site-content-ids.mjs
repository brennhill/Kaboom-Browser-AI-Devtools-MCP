#!/usr/bin/env node

import { promises as fs } from 'node:fs'
import path from 'node:path'

const docsRoot = path.join(process.cwd(), 'gokaboom.dev', 'src', 'content', 'docs')

async function walk(dir) {
  const out = []
  const entries = await fs.readdir(dir, { withFileTypes: true })
  for (const entry of entries) {
    const fullPath = path.join(dir, entry.name)
    if (entry.isDirectory()) {
      out.push(...(await walk(fullPath)))
      continue
    }
    if (/\.mdx?$/u.test(entry.name)) {
      out.push(fullPath)
    }
  }
  return out
}

function toSlug(filePath) {
  const rel = path.relative(docsRoot, filePath).replace(/\\/g, '/')
  const noExt = rel.replace(/\.mdx?$/u, '')
  if (noExt === 'index') return 'index'
  if (noExt.endsWith('/index')) return noExt.slice(0, -('/index'.length))
  return noExt
}

const files = await walk(docsRoot)
const seen = new Map()
for (const file of files) {
  const slug = toSlug(file)
  if (!seen.has(slug)) seen.set(slug, [])
  seen.get(slug).push(file)
}

const duplicates = [...seen.entries()].filter(([, paths]) => paths.length > 1)
if (duplicates.length > 0) {
  console.error('Duplicate content IDs/slugs detected in gokaboom docs collection:\n')
  for (const [slug, paths] of duplicates) {
    console.error(`- slug "${slug}"`) 
    for (const file of paths) {
      console.error(`  - ${path.relative(process.cwd(), file)}`)
    }
  }
  process.exit(1)
}

console.log(`Content IDs check: ${files.length} docs scanned, no duplicate slugs.`)
