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
