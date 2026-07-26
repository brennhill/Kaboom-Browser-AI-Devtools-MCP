import assert from 'node:assert/strict'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import test from 'node:test'

import { checkFeatureBundles } from './check-feature-bundles.js'

function writeFeatureBundle(repoRoot, relDir, reviewedDate) {
  const bundleDir = path.join(repoRoot, 'docs', 'features', relDir)
  fs.mkdirSync(bundleDir, { recursive: true })
  const frontmatter = `---\ndoc_type: feature_index\nfeature_id: feature-${relDir.replaceAll('/', '-')}\nlast_reviewed: ${reviewedDate}\n---\n\n# Title\n`
  const files = ['index.md', 'product-spec.md', 'tech-spec.md', 'qa-plan.md']
  for (const file of files) {
    fs.writeFileSync(path.join(bundleDir, file), frontmatter, 'utf8')
  }
}

test('strict feature freshness gate fails stale docs/features bundles', () => {
  const repoRoot = fs.mkdtempSync(path.join(os.tmpdir(), 'feature-bundle-check-'))
  writeFeatureBundle(repoRoot, 'feature/sample-feature', '2026-01-01')

  const result = checkFeatureBundles({
    repoRoot,
    strictFrontmatter: true,
    enforceFeatureFreshness: true,
    freshnessWindowDays: 30,
    now: new Date('2026-03-03T00:00:00Z')
  })

  assert.ok(result.issues.length > 0, 'expected stale issues')
  assert.ok(result.issues.some((issue) => issue.includes("stale last_reviewed '2026-01-01'")))
})

test('feature freshness checks can be disabled for gradual rollout', () => {
  const repoRoot = fs.mkdtempSync(path.join(os.tmpdir(), 'feature-bundle-check-'))
  writeFeatureBundle(repoRoot, 'feature/sample-feature', '2026-01-01')

  const result = checkFeatureBundles({
    repoRoot,
    strictFrontmatter: true,
    enforceFeatureFreshness: false,
    freshnessWindowDays: 30,
    now: new Date('2026-03-03T00:00:00Z')
  })

  assert.deepEqual(result.issues, [])
})

test('strict feature freshness gate rejects invalid calendar dates', () => {
  const repoRoot = fs.mkdtempSync(path.join(os.tmpdir(), 'feature-bundle-check-'))
  writeFeatureBundle(repoRoot, 'feature/sample-feature', '2026-02-31')

  const result = checkFeatureBundles({
    repoRoot,
    strictFrontmatter: true,
    enforceFeatureFreshness: true,
    freshnessWindowDays: 30,
    now: new Date('2026-03-03T00:00:00Z')
  })

  assert.ok(result.issues.some((issue) => issue.includes("invalid last_reviewed value '2026-02-31'")))
})
