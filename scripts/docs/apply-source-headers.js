#!/usr/bin/env node
// Purpose: Automate apply-source-headers.js workflow behavior for repository tooling.
// Why: Keeps repetitive maintenance and verification steps deterministic.
// Docs: docs/DEVELOPMENT.md

import fs from 'node:fs'
import path from 'node:path'

const repoRoot = process.cwd()
const roots = ['src', 'cmd', 'internal']

function walk(dir, out) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name)
    if (entry.isDirectory()) {
      walk(full, out)
      continue
    }
    out.push(full)
  }
}

function isTarget(file) {
  const rel = path.relative(repoRoot, file)
  if (rel.includes('/testdata/')) return false
  if (rel.endsWith('_test.go')) return false
  if (rel.endsWith('.test.ts')) return false
  if (rel.endsWith('.d.ts')) return false
  if (rel.endsWith('/doc.go')) return false
  return rel.endsWith('.ts') || rel.endsWith('.go')
}

function inferDocs(rel) {
  const docs = new Set()
  const add = (slug) => docs.add(`docs/features/feature/${slug}/index.md`)

  if (rel.startsWith('cmd/browser-agent/bridge')) add('bridge-restart')
  if (rel.startsWith('cmd/browser-agent/upload')) add('file-upload')
  if (rel.startsWith('cmd/browser-agent/testgen')) add('test-generation')
  if (rel.startsWith('cmd/browser-agent/tools_configure')) add('config-profiles')
  if (rel.startsWith('cmd/browser-agent/recording_') || rel.startsWith('cmd/browser-agent/tools_recording_video')) {
    add('playback-engine')
  }

  if (rel.startsWith('cmd/browser-agent/tools_analyze')) add('analyze-tool')
  if (rel.startsWith('cmd/browser-agent/tools_interact')) add('interact-explore')
  if (rel.startsWith('cmd/browser-agent/tools_observe')) add('observe')
  if (rel.startsWith('cmd/browser-agent/tools_generate') || rel.includes('/testgen')) add('test-generation')
  if (rel.includes('reproduction')) add('reproduction-scripts')
  if (rel.startsWith('internal/bridge/')) add('bridge-restart')
  if (rel.startsWith('internal/buffers/')) add('ring-buffer')
  if (rel.startsWith('internal/mcp/')) add('query-service')
  if (rel.startsWith('internal/queries/')) add('query-service')
  if (rel.startsWith('internal/recording/')) add('playback-engine')
  if (rel.startsWith('internal/schema/analyze')) add('analyze-tool')
  if (rel.startsWith('internal/schema/interact')) add('interact-explore')
  if (rel.startsWith('internal/schema/observe')) add('observe')
  if (rel.startsWith('internal/schema/configure')) add('config-profiles')
  if (rel.startsWith('internal/schema/generate')) add('test-generation')
  if (rel.startsWith('internal/schema/schema.go')) {
    add('analyze-tool')
    add('interact-explore')
    add('observe')
    add('config-profiles')
    add('test-generation')
  }
  if (rel.startsWith('internal/tools/analyze/')) add('analyze-tool')
  if (rel.startsWith('internal/tools/interact/')) add('interact-explore')
  if (rel.startsWith('internal/tools/observe/')) add('observe')
  if (rel.startsWith('internal/tools/configure/')) add('config-profiles')
  if (rel.startsWith('internal/tools/generate/')) add('test-generation')
  if (rel.startsWith('internal/upload/')) add('file-upload')
  if (rel.startsWith('internal/testgen/')) add('test-generation')
  if (rel.startsWith('internal/pagination/')) add('pagination')
  if (rel.startsWith('internal/export/')) {
    add('har-export')
    add('sarif-export')
  }
  if (rel.startsWith('internal/redaction/')) add('redaction-patterns')
  if (rel.startsWith('internal/performance/')) add('performance-audit')
  if (rel.startsWith('internal/capture/')) add('backend-log-streaming')
  if (rel.startsWith('internal/observe/')) add('observe')
  if (rel.startsWith('internal/session/')) {
    add('observe')
    add('pagination')
  }
  if (rel.startsWith('src/lib/analysis/dom-queries')) add('query-dom')
  if (rel.startsWith('src/lib/analysis/link-health')) add('link-health')
  if (rel.startsWith('src/lib/analysis/perf') || rel.startsWith('src/lib/analysis/performance')) add('performance-audit')
  if (rel.startsWith('src/lib/net/network') || rel.startsWith('src/lib/net/websocket')) add('backend-log-streaming')
  if (rel.startsWith('src/background/')) {
    add('analyze-tool')
    add('interact-explore')
    add('observe')
  }
  if (rel.startsWith('src/content/')) {
    add('interact-explore')
    add('query-dom')
  }
  if (rel.startsWith('src/inject/')) {
    add('interact-explore')
    add('query-dom')
  }
  if (rel === 'src/background.ts' || rel === 'src/content.ts' || rel === 'src/inject.ts') {
    add('interact-explore')
    add('analyze-tool')
  }

  return Array.from(docs)
}

function inferPurpose(rel) {
  if (rel.startsWith('cmd/browser-agent/bridge')) return 'Implements bridge transport lifecycle, forwarding, and reconnect behavior.'
  if (rel.startsWith('cmd/browser-agent/upload')) return 'Implements upload command handling, validation, and OS automation wiring.'
  if (rel.startsWith('cmd/browser-agent/testgen')) return 'Implements test generation, classification, and healing command handlers.'
  if (rel.startsWith('cmd/browser-agent/tools_configure')) return 'Implements configure tool handlers for policy, profiles, and session controls.'
  if (rel.startsWith('cmd/browser-agent/recording_') || rel.startsWith('cmd/browser-agent/tools_recording_video')) {
    return 'Implements recording and playback command handlers for captured browser sessions.'
  }
  if (rel.startsWith('cmd/browser-agent/tools_analyze')) return 'Implements analyze tool handlers and response shaping.'
  if (rel.startsWith('cmd/browser-agent/tools_interact')) return 'Implements interact tool handlers and browser action orchestration.'
  if (rel.startsWith('cmd/browser-agent/tools_observe')) return 'Implements observe tool queries against captured runtime buffers.'
  if (rel.startsWith('cmd/browser-agent/tools_generate')) return 'Implements generate tool formats and output assembly.'
  if (rel.startsWith('internal/bridge/')) return 'Implements framed stdio transport, timeouts, and bridge connection lifecycle.'
  if (rel.startsWith('internal/buffers/')) return 'Implements ring buffer storage primitives and cursor-safe access patterns.'
  if (rel.startsWith('internal/export/')) return 'Implements export serializers and format-specific output builders.'
  if (rel.startsWith('internal/mcp/')) return 'Defines MCP protocol types, validation, and structured error response helpers.'
  if (rel.startsWith('internal/pagination/')) return 'Implements cursor pagination over captured telemetry collections.'
  if (rel.startsWith('internal/redaction/')) return 'Implements redaction rules for sensitive data in captured telemetry.'
  if (rel.startsWith('internal/performance/')) return 'Implements performance metric diffing and threshold evaluation.'
  if (rel.startsWith('internal/queries/')) return 'Implements async command/query dispatch and correlation state tracking.'
  if (rel.startsWith('internal/recording/')) return 'Implements recording storage, replay engine execution, and diffing helpers.'
  if (rel.startsWith('internal/schema/')) return 'Defines JSON schema contracts for tool arguments and responses.'
  if (rel.startsWith('internal/session/')) return 'Implements session lifecycle, snapshots, and diff state management.'
  if (rel.startsWith('internal/testgen/')) return 'Implements prompt-driven test generation, healing, and classification helpers.'
  if (rel.startsWith('internal/tools/analyze/')) return 'Provides analyze tool implementation helpers shared by command handlers.'
  if (rel.startsWith('internal/tools/configure/')) return 'Provides configure tool implementation helpers for policy and rewrite flows.'
  if (rel.startsWith('internal/tools/generate/')) return 'Provides generate tool implementation helpers for emitted artifacts.'
  if (rel.startsWith('internal/tools/interact/')) return 'Provides interact tool implementation helpers for selectors and workflows.'
  if (rel.startsWith('internal/tools/observe/')) return 'Provides observe tool implementation helpers for filtering and storage queries.'
  if (rel.startsWith('internal/upload/')) return 'Implements upload validation, security checks, and automation support paths.'
  if (rel.startsWith('src/background/')) return 'Handles extension background coordination and message routing.'
  if (rel.startsWith('src/content/')) return 'Handles content-script message relay between background and inject contexts.'
  if (rel.startsWith('src/inject/')) return 'Executes in-page actions and query handlers within the page context.'
  if (rel.startsWith('src/lib/')) return 'Provides shared runtime utilities used by extension and server workflows.'
  return ''
}

function hasPurposeDocs(content) {
  const head = content.split('\n').slice(0, 40).join('\n')
  return /Purpose:\s*\S/.test(head) && /Docs:\s*docs\/features\/feature\/[a-z0-9-]+\/index\.md/.test(head)
}

function tsHeader(rel) {
  const purpose = inferPurpose(rel)
  const docsList = inferDocs(rel)
  if (!purpose || docsList.length === 0) return ''
  const docs = docsList.map((d) => ` * Docs: ${d}`).join('\n')
  return `/**\n * Purpose: ${purpose}\n${docs}\n */\n`
}

function goHeader(rel) {
  const purpose = inferPurpose(rel)
  const docsList = inferDocs(rel)
  if (!purpose || docsList.length === 0) return ''
  const docs = docsList.map((d) => `// Docs: ${d}`).join('\n')
  return `// Purpose: ${purpose}\n${docs}\n`
}

function insertHeader(rel, content) {
  const isGo = rel.endsWith('.go')
  const header = isGo ? goHeader(rel) : tsHeader(rel)
  if (!header) return content
  if (content.startsWith('#!')) {
    const nl = content.indexOf('\n')
    if (nl !== -1) {
      return `${content.slice(0, nl + 1)}${header}${content.slice(nl + 1)}`
    }
  }
  return `${header}\n${content}`
}

function main() {
  const files = []
  for (const r of roots) {
    const dir = path.join(repoRoot, r)
    if (fs.existsSync(dir)) walk(dir, files)
  }
  const targets = files.filter(isTarget).sort((a, b) => a.localeCompare(b))
  let updated = 0

  for (const file of targets) {
    const rel = path.relative(repoRoot, file)
    const content = fs.readFileSync(file, 'utf8')
    if (!hasPurposeDocs(content)) {
      const withHeader = insertHeader(rel, content)
      if (withHeader !== content) {
        fs.writeFileSync(file, withHeader, 'utf8')
        updated += 1
      }
    }
  }

  console.log(`updated ${updated} file(s) with source headers`)
}

main()
