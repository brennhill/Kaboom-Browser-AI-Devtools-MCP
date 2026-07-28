const test = require('node:test')
const assert = require('node:assert')
const fs = require('node:fs')
const path = require('node:path')

const REPO_ROOT = path.resolve(__dirname, '../..')

test('clean-old-daemons script targets only canonical Kaboom processes and state', () => {
  const source = fs.readFileSync(path.join(REPO_ROOT, 'scripts/clean-old-daemons.sh'), 'utf8')

  assert.match(source, /Kaboom Daemon Cleanup/)
  assert.match(source, /Safe to install or upgrade Kaboom now:/)
  assert.match(source, /npm install -g kaboom-agentic-browser@latest/)

  assert.match(source, /kaboom-agentic-browser.*--daemon|kaboom-agentic-browser\.exe/)
  assert.match(source, /KABOOM_STATE_DIR/)
  assert.doesNotMatch(source, /\b(?:gasoline|strum|legacy)\b/i)
})
