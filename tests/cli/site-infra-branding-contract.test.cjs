const test = require('node:test')
const assert = require('node:assert')
const fs = require('node:fs')
const path = require('node:path')

const REPO_ROOT = path.resolve(__dirname, '../..')

const FILES = [
  'scripts/docs/site/run-vale-on-changed.mjs',
  'scripts/docs/reference/check-reference-schema-sync.mjs',
  'scripts/docs/site/check-content-style-contract.mjs',
  'scripts/docs/site/check-site-content-ids.mjs',
  'scripts/docs/site/check-landing-layout-contract.mjs',
  'scripts/docs/features/sync-verification-metadata.mjs',
  'scripts/docs/site/normalize-site-tags.mjs',
  'scripts/docs/site/check-downloads-page-contract.mjs',
  'scripts/docs/site/check-light-theme-contract.mjs',
  'scripts/docs/features/check-docs-quality-gates.mjs',
  'scripts/docs/reference/generate-reference-executable-examples.mjs',
  'scripts/docs/features/check-feature-bundles.js',
  'gokaboom.dev/package.json',
  'docs/standards/content-style-voice-guide.md',
  'docs/standards/docs-quality-ci-gates.md',
  'docs/robots.txt'
]

test('site infrastructure files use Kaboom and gokaboom naming', () => {
  for (const relativePath of FILES) {
    const source = fs.readFileSync(path.join(REPO_ROOT, relativePath), 'utf8')
    assert.doesNotMatch(
      source,
      /Gasoline|cookwithgasoline|getstrum|STRUM|gasoline-ci/,
      `${relativePath} still contains legacy site branding`
    )
    assert.match(source, /gokaboom|Kaboom|KaBOOM|kaboom/, `${relativePath} is missing Kaboom branding`)
  }
})
