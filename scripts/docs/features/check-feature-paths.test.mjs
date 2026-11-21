import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import test from 'node:test'

const repoRoot = path.resolve(import.meta.dirname, '..', '..', '..')
const featureRoot = path.join(repoRoot, 'docs', 'features', 'feature')

function frontmatterPathEntries(content) {
  const end = content.indexOf('\n---\n', 4)
  if (!content.startsWith('---\n') || end === -1) return []

  const entries = []
  let activeKey = ''
  for (const line of content.slice(4, end).split('\n')) {
    const keyMatch = /^(code_paths|test_paths):\s*$/.exec(line)
    if (keyMatch) {
      activeKey = keyMatch[1]
      continue
    }
    const entryMatch = /^ {2}- (.+)$/.exec(line)
    if (entryMatch && activeKey) {
      entries.push({ key: activeKey, value: entryMatch[1] })
      continue
    }
    if (/^[a-z_]+:/.test(line)) activeKey = ''
  }
  return entries
}

test('feature index code_paths and test_paths resolve to repository files', () => {
  const missing = []
  for (const feature of fs.readdirSync(featureRoot).sort()) {
    const indexPath = path.join(featureRoot, feature, 'index.md')
    if (!fs.existsSync(indexPath)) continue
    const content = fs.readFileSync(indexPath, 'utf8')
    for (const entry of frontmatterPathEntries(content)) {
      if (!fs.existsSync(path.join(repoRoot, entry.value))) {
        missing.push(`${path.relative(repoRoot, indexPath)}:${entry.key}:${entry.value}`)
      }
    }
  }

  assert.deepEqual(missing, [])
})

// Glossary references are backticked tokens: `path` (must exist) or
// `path:Identifier` / `path:Type.Member` (file must exist and declare or
// mention every dotted segment as a whole word).
const referencePattern = /`((?:\.?[\w-]+\/)*[\w.-]+\.(?:go|ts|json|md|cjs|mjs|js)|(?:[\w-]+\/)+)(?::([A-Za-z_][\w.]*))?`/g

export function brokenReferences(markdown, root) {
  const broken = []
  for (const match of markdown.matchAll(referencePattern)) {
    const [, relative, identifier] = match
    if (!relative.includes('/') && !identifier) continue
    const absolute = path.join(root, relative)
    if (!fs.existsSync(absolute)) {
      broken.push(`${relative}: missing path`)
      continue
    }
    if (!identifier) continue
    const source = fs.readFileSync(absolute, 'utf8')
    for (const segment of identifier.split('.')) {
      if (!new RegExp(`\\b${segment}\\b`).test(source)) broken.push(`${relative}:${identifier}: ${segment} not found`)
    }
  }
  return broken
}

test('glossary reference checker rejects missing paths and identifiers', () => {
  const sample =
    '`scripts/docs/features/check-feature-paths.test.mjs:brokenReferences` ' +
    '`scripts/docs/nope.md` `package.json:scripts.NoSuchSymbol`'
  assert.deepEqual(brokenReferences(sample, repoRoot), [
    'scripts/docs/nope.md: missing path',
    'package.json:scripts.NoSuchSymbol: NoSuchSymbol not found'
  ])
})

test('GLOSSARY.md and the architecture guides reference real paths and identifiers', () => {
  const documents = [
    'GLOSSARY.md',
    'docs/core/architecture/server-architecture.md',
    'docs/core/architecture/extension-architecture.md'
  ]
  const broken = documents.flatMap((document) =>
    brokenReferences(fs.readFileSync(path.join(repoRoot, document), 'utf8'), repoRoot).map(
      (entry) => `${document}: ${entry}`
    )
  )
  assert.deepEqual(broken, [])
})
