const test = require('node:test')
const assert = require('node:assert')
const fs = require('node:fs')
const path = require('node:path')

const REPO_ROOT = path.resolve(__dirname, '../../..')

function read(relativePath) {
  return fs.readFileSync(path.join(REPO_ROOT, relativePath), 'utf8')
}

test('operator scripts use only canonical Kaboom binary names', () => {
  const stdioSilence = read('scripts/uat/protocol/test-stdio-silence.sh')
  const mcpComprehensive = read('scripts/uat/protocol/test-mcp-comprehensive.sh')
  const cursorSimulation = read('scripts/uat/protocol/test-cursor-simulation.sh')
  const rebuild = read('scripts/maintenance/rebuild.sh')
  const buildCrx = read('scripts/release/build-crx.js')
  const killTestServers = read('scripts/maintenance/kill-test-servers.sh')

  assert.match(stdioSilence, /WRAPPER="kaboom-agentic-browser"/)
  assert.doesNotMatch(stdioSilence, /gasoline-mcp|npm\/gasoline-mcp/)

  assert.match(mcpComprehensive, /WRAPPER="kaboom-agentic-browser"/)
  assert.doesNotMatch(mcpComprehensive, /gasoline-mcp/)

  assert.match(cursorSimulation, /WRAPPER="kaboom-agentic-browser"/)
  assert.match(cursorSimulation, /Cursor spawns kaboom-agentic-browser/)
  assert.doesNotMatch(cursorSimulation, /gasoline-mcp/)

  assert.match(rebuild, /VERSIONED_BIN_NAME="kaboom-agentic-browser-\$VERSION_TAG"/)
  assert.match(rebuild, /go build -o kaboom-agentic-browser/)
  assert.match(rebuild, /\/usr\/local\/bin\/kaboom-agentic-browser/)
  assert.doesNotMatch(rebuild, /\b(?:gasoline|strum|legacy)\b/i)

  assert.match(buildCrx, /\.kaboom\/extension-signing-key\.pem/)
  assert.match(buildCrx, /kaboom-extension-v\$\{VERSION\}\.crx/)
  assert.match(buildCrx, /https:\/\/gokaboom\.dev\/downloads\/kaboom-extension-v\$\{VERSION\}\.crx/)
  assert.doesNotMatch(buildCrx, /\.gasoline\/extension-signing-key\.pem|gasoline-extension-v|cookwithgasoline/)

  assert.match(killTestServers, /Kaboom test servers/)
  assert.match(killTestServers, /pgrep -f 'kaboom'/)
  assert.match(killTestServers, /\/tmp\/kaboom-.*\.pid/)
  assert.match(killTestServers, /\/tmp\/kaboom-test-.*\.jsonl/)
  assert.doesNotMatch(killTestServers, /gasoline/)
})
