const test = require('node:test')
const assert = require('node:assert')
const fs = require('node:fs')
const path = require('node:path')

const REPO_ROOT = path.resolve(__dirname, '../../..')

const FILES = [
  'scripts/test-all-split.sh',
  'scripts/test-install-hooks-only.sh',
  'tests/regression/run-all.sh',
  'tests/regression/lib/common.sh',
  'tests/e2e/helpers/extension.js',
  'tests/e2e/helpers/global-setup.js',
  'tests/e2e/package.json',
  'tests/e2e/package-lock.json'
]

test('build and test harness entry points use Kaboom naming', () => {
  for (const relativePath of FILES) {
    const source = fs.readFileSync(path.join(REPO_ROOT, relativePath), 'utf8')
    assert.doesNotMatch(
      source,
      /STRUM|Gasoline|gasoline-agentic-devtools|dist\/gasoline|gasoline-e2e-tests|gasoline-hooks|gasoline-mcp-uat-cover|gasoline-uat-phase|gasoline-test-\$\$|\.gasoline-\$\{?PORT\}?\.pid/,
      `${relativePath} still contains legacy harness branding`
    )
  }
})

test('core harness files point at Kaboom binary names', () => {
  const makefile = fs.readFileSync(path.join(REPO_ROOT, 'Makefile'), 'utf8')
  const splitScript = fs.readFileSync(path.join(REPO_ROOT, 'scripts/test-all-split.sh'), 'utf8')
  const hooksInstallScript = fs.readFileSync(path.join(REPO_ROOT, 'scripts/test-install-hooks-only.sh'), 'utf8')
  const regressionRunner = fs.readFileSync(path.join(REPO_ROOT, 'tests/regression/run-all.sh'), 'utf8')
  const regressionCommon = fs.readFileSync(path.join(REPO_ROOT, 'tests/regression/lib/common.sh'), 'utf8')
  const e2eExtension = fs.readFileSync(path.join(REPO_ROOT, 'tests/e2e/helpers/extension.js'), 'utf8')
  const e2eSetup = fs.readFileSync(path.join(REPO_ROOT, 'tests/e2e/helpers/global-setup.js'), 'utf8')
  const e2ePackage = fs.readFileSync(path.join(REPO_ROOT, 'tests/e2e/package.json'), 'utf8')
  const e2eLock = fs.readFileSync(path.join(REPO_ROOT, 'tests/e2e/package-lock.json'), 'utf8')

  assert.match(makefile, /^BINARY_NAME := kaboom-agentic-browser$/m)
  assert.match(makefile, /^HOOKS_BINARY_NAME := kaboom-hooks$/m)
  assert.match(splitScript, /KABOOM UAT TEST SUITE/)
  assert.match(splitScript, /kaboom-agentic-browser-uat-cover/)
  assert.match(hooksInstallScript, /kaboom-hooks/)
  assert.match(regressionRunner, /Kaboom External Regression Tests/)
  assert.match(regressionRunner, /dist\/kaboom-agentic-browser/)
  assert.match(regressionCommon, /dist\/kaboom-agentic-browser/)
  assert.match(regressionCommon, /\/tmp\/kaboom-test-\$\$\.jsonl/)
  assert.match(e2eExtension, /dist', 'kaboom-agentic-browser'/)
  assert.match(e2eSetup, /dist', 'kaboom-agentic-browser'/)
  assert.match(e2ePackage, /"name": "kaboom-e2e-tests"/)
  assert.match(e2eLock, /"name": "kaboom-e2e-tests"/)
})
