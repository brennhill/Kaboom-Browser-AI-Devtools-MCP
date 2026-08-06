// smoke-layout-contract.test.cjs — Guards change-coupled smoke-test ownership.

const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')

const REPO_ROOT = path.resolve(__dirname, '..', '..', '..')
const SMOKE_ROOT = path.join(REPO_ROOT, 'scripts', 'smoke-tests')
const RUNNER = path.join(REPO_ROOT, 'scripts', 'uat', 'runners', 'smoke-test.sh')

test('smoke modules live in bounded change-coupled owner folders', () => {
  const directFiles = fs.readdirSync(SMOKE_ROOT, { withFileTypes: true }).filter((entry) => entry.isFile())
  assert.deepStrictEqual(directFiles, [], 'scripts/smoke-tests must contain owner folders, not loose modules')

  for (const owner of fs.readdirSync(SMOKE_ROOT, { withFileTypes: true }).filter((entry) => entry.isDirectory())) {
    const files = fs.readdirSync(path.join(SMOKE_ROOT, owner.name), { withFileTypes: true })
      .filter((entry) => entry.isFile())
    assert.ok(files.length <= 10, `${owner.name} has ${files.length} direct files; maximum is 10`)
  }
})

test('the canonical runner names resolvable owner-qualified modules', () => {
  const source = fs.readFileSync(RUNNER, 'utf8')
  const moduleBlock = source.match(/MODULES=\(([\s\S]*?)\n\)/)
  assert.ok(moduleBlock, 'smoke runner must declare MODULES')

  const modules = [...moduleBlock[1].matchAll(/"([^"]+\.sh)"/g)].map((match) => match[1])
  assert.ok(modules.length > 0, 'smoke runner must declare at least one module')
  for (const modulePath of modules) {
    assert.match(modulePath, /^[a-z-]+\//, `${modulePath} must identify its owning feature family`)
    assert.ok(fs.existsSync(path.join(SMOKE_ROOT, modulePath)), `${modulePath} must resolve under scripts/smoke-tests`)
  }
})

test('framework fixtures build into the harness-owned embedded page tree', () => {
  const builder = fs.readFileSync(path.join(SMOKE_ROOT, 'framework', 'build-framework-fixtures.mjs'), 'utf8')
  assert.match(builder, /internal['"], 'testpages['"], 'pages['"]\)/)
  assert.match(builder, /path\.join\(harnessRootDir, 'frameworks'\)/)
  assert.doesNotMatch(builder, /cmd['"], 'browser-agent['"], 'testpages['"]/)
})
