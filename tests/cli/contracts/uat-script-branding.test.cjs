const test = require('node:test')
const assert = require('node:assert')
const fs = require('node:fs')
const path = require('node:path')

const REPO_ROOT = path.resolve(__dirname, '../../..')

function read(relativePath) {
  return fs.readFileSync(path.join(REPO_ROOT, relativePath), 'utf8')
}

test('UAT entry scripts use Kaboom-facing copy and binary names', () => {
  const comprehensive = read('scripts/test-all-tools-comprehensive.sh')
  const stdioValidation = read('scripts/validate-test-generation-stdio.sh')
  const multiTier = read('scripts/test-multi-tier.sh')
  const newUat = read('scripts/test-new-uat.sh')
  const newUatConservative = read('scripts/test-new-uat-conservative.sh')

  assert.match(comprehensive, /Kaboom MCP — COMPREHENSIVE UAT/)
  assert.match(comprehensive, /kaboom-agentic-browser/)
  assert.doesNotMatch(comprehensive, /STRUM|gasoline-mcp/)

  assert.match(stdioValidation, /Kaboom Test Generation Validation/)
  assert.match(stdioValidation, /dist\/kaboom-agentic-browser/)
  assert.match(stdioValidation, /Kaboom extension connected/)
  assert.doesNotMatch(stdioValidation, /STRUM|dist\/gasoline/)

  assert.match(multiTier, /KABOOM UAT 6-TIER PARALLEL TEST SUITE/)
  assert.match(multiTier, /\/tmp\/kaboom-uat-multitier-/)
  assert.doesNotMatch(multiTier, /STRUM|\/tmp\/gasoline-uat-multitier-/)

  assert.match(newUat, /\/tmp\/kaboom-uat-new-/)
  assert.match(newUat, /KABOOM_UAT_FAIL_ON_RESULT_INTEGRITY/)
  assert.match(newUat, /KABOOM_UAT_SUMMARY_FILE/)
  assert.doesNotMatch(newUat, /\/tmp\/gasoline-uat-new-|GASOLINE_UAT_/)

  assert.match(newUatConservative, /\/tmp\/kaboom-uat-new-conservative-/)
  assert.doesNotMatch(newUatConservative, /\/tmp\/gasoline-uat-new-conservative-/)
})
