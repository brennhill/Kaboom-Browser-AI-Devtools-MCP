const test = require('node:test')
const assert = require('node:assert')
const fs = require('node:fs')
const path = require('node:path')

const REPO_ROOT = path.resolve(__dirname, '../..')

test('clean-old-daemons script uses Kaboom copy while targeting legacy processes', () => {
  const source = fs.readFileSync(path.join(REPO_ROOT, 'scripts/clean-old-daemons.sh'), 'utf8')

  assert.match(source, /Kaboom Daemon Cleanup/)
  assert.match(source, /Safe to install or upgrade Kaboom now:/)
  assert.match(source, /npm install -g kaboom-agentic-browser@latest/)

  assert.match(source, /gasoline.*--daemon|gasoline\.exe|lsof -c gasoline/)
  assert.match(source, /strum.*--daemon|strum\.exe|lsof -c strum/)
  // Pin the coverage, not the exact list. The script has since added `kaboom`
  // to this sweep so it also clears stale current-brand PID files, which is a
  // strict improvement — but the old exact-match assertion failed on it. This
  // still goes red if gasoline or strum is dropped from the loop.
  assert.match(source, /for legacy_name in [^;]*\bgasoline\b[^;]*\bstrum\b[^;]*; do/)
  assert.match(source, /\.\\?\$\{legacy_name\}-\$port\.pid/)

  assert.doesNotMatch(source, /STRUM Daemon Cleanup/)
  assert.doesNotMatch(source, /Safe to install or upgrade STRUM now:/)
  assert.doesNotMatch(source, /npm install -g gasoline-mcp@latest/)
})
