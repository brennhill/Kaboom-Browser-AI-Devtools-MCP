const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')

const REPO_ROOT = path.resolve(__dirname, '..', '..')
// Flow maps were relocated out of the retired docs/architecture/flow-maps/ dir into
// docs/architecture/ (the flow-map system was retired; see the Docs Cross-Reference
// Contract in CLAUDE.md/AGENTS.md). These paths track that move.
const NEW_FLOW_MAP = 'docs/architecture/gokaboom-content-publishing-and-agent-markdown.md'
const OLD_FLOW_MAP = 'docs/architecture/cookwithgasoline-content-publishing-and-agent-markdown.md'
const DOCS_TO_SCAN = [
  NEW_FLOW_MAP,
  'docs/architecture/terminal-side-panel-host.md',
  'docs/architecture/tracked-tab-hover-quick-actions.md',
  'docs/features/feature/terminal/index.md',
  'docs/features/feature/terminal/product-spec.md',
  'docs/features/feature/terminal/tech-spec.md',
  'docs/features/feature/tab-tracking-ux/index.md'
]

function read(relativePath) {
  return fs.readFileSync(path.join(REPO_ROOT, relativePath), 'utf8')
}

test('kaboom docs flow maps point at gokaboom and kaboom naming', () => {
  assert.ok(fs.existsSync(path.join(REPO_ROOT, NEW_FLOW_MAP)))
  assert.ok(!fs.existsSync(path.join(REPO_ROOT, OLD_FLOW_MAP)))

  const contentMap = read(NEW_FLOW_MAP)
  assert.match(contentMap, /Gokaboom Content Publishing and Agent Markdown/)
  assert.doesNotMatch(contentMap, /Cookwithgasoline Content Publishing and Agent Markdown/)

  for (const relativePath of DOCS_TO_SCAN) {
    const source = read(relativePath)
    assert.doesNotMatch(
      source,
      /STRUM|Gasoline|cookwithgasoline|Cookwithgasoline|getstrum/
    )
  }

  const terminalIndex = read('docs/features/feature/terminal/index.md')
  assert.match(terminalIndex, /Kaboom work context/)

  const hoverFlowMap = read('docs/architecture/tracked-tab-hover-quick-actions.md')
  assert.match(hoverFlowMap, /Kaboom terminal side panel/)
  assert.match(hoverFlowMap, /Hide Kaboom Devtool/)
})
