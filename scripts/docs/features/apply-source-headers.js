#!/usr/bin/env node
// Purpose: Automate apply-source-headers.js workflow behavior for repository tooling.
// Why: Keeps repetitive maintenance and verification steps deterministic.
// Docs: docs/setup/DEVELOPMENT.md

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

const DOC_RULES = [
  { prefixes: ['cmd/browser-agent/bridge'], docs: ['bridge-restart'] },
  { prefixes: ['cmd/browser-agent/upload'], docs: ['file-upload'] },
  { prefixes: ['cmd/browser-agent/testgen'], docs: ['test-generation'] },
  { prefixes: ['cmd/browser-agent/tools_configure'], docs: ['config-profiles'] },
  {
    prefixes: ['cmd/browser-agent/recording_', 'cmd/browser-agent/tools_recording_video'],
    docs: ['playback-engine']
  },
  { prefixes: ['cmd/browser-agent/tools_analyze'], docs: ['analyze-tool'] },
  { prefixes: ['cmd/browser-agent/tools_interact'], docs: ['interact-explore'] },
  { prefixes: ['cmd/browser-agent/tools_observe'], docs: ['observe'] },
  { prefixes: ['cmd/browser-agent/tools_generate'], includes: ['/testgen'], docs: ['test-generation'] },
  { includes: ['reproduction'], docs: ['reproduction-scripts'] },
  { prefixes: ['internal/bridge/'], docs: ['bridge-restart'] },
  { prefixes: ['internal/buffers/'], docs: ['ring-buffer'] },
  { prefixes: ['internal/mcp/'], docs: ['query-service'] },
  { prefixes: ['internal/queries/'], docs: ['query-service'] },
  { prefixes: ['internal/recording/'], docs: ['playback-engine'] },
  { prefixes: ['internal/schema/analyze'], docs: ['analyze-tool'] },
  { prefixes: ['internal/schema/interact'], docs: ['interact-explore'] },
  { prefixes: ['internal/schema/observe'], docs: ['observe'] },
  { prefixes: ['internal/schema/configure'], docs: ['config-profiles'] },
  { prefixes: ['internal/schema/generate'], docs: ['test-generation'] },
  {
    prefixes: ['internal/schema/schema.go'],
    docs: ['analyze-tool', 'interact-explore', 'observe', 'config-profiles', 'test-generation']
  },
  { prefixes: ['internal/tools/analyze/'], docs: ['analyze-tool'] },
  { prefixes: ['internal/tools/interact/'], docs: ['interact-explore'] },
  { prefixes: ['internal/tools/observe/'], docs: ['observe'] },
  { prefixes: ['internal/tools/configure/'], docs: ['config-profiles'] },
  { prefixes: ['internal/tools/generate/'], docs: ['test-generation'] },
  { prefixes: ['internal/upload/'], docs: ['file-upload'] },
  { prefixes: ['internal/testgen/'], docs: ['test-generation'] },
  { prefixes: ['internal/pagination/'], docs: ['pagination'] },
  { prefixes: ['internal/export/'], docs: ['har-export', 'sarif-export'] },
  { prefixes: ['internal/redaction/'], docs: ['redaction-patterns'] },
  { prefixes: ['internal/performance/'], docs: ['performance-audit'] },
  { prefixes: ['internal/capture/'], docs: ['backend-log-streaming'] },
  { prefixes: ['internal/observe/'], docs: ['observe'] },
  { prefixes: ['internal/session/'], docs: ['observe', 'pagination'] },
  { prefixes: ['src/lib/analysis/dom-queries'], docs: ['query-dom'] },
  { prefixes: ['src/lib/analysis/link-health'], docs: ['link-health'] },
  { prefixes: ['src/lib/analysis/perf', 'src/lib/analysis/performance'], docs: ['performance-audit'] },
  { prefixes: ['src/lib/net/network', 'src/lib/net/websocket'], docs: ['backend-log-streaming'] },
  { prefixes: ['src/background/'], docs: ['analyze-tool', 'interact-explore', 'observe'] },
  { prefixes: ['src/content/'], docs: ['interact-explore', 'query-dom'] },
  { prefixes: ['src/inject/'], docs: ['interact-explore', 'query-dom'] },
  {
    exact: ['src/background.ts', 'src/content.ts', 'src/inject.ts'],
    docs: ['interact-explore', 'analyze-tool']
  }
]

const PURPOSE_RULES = [
  { prefixes: ['cmd/browser-agent/bridge'], purpose: 'Implements bridge transport lifecycle, forwarding, and reconnect behavior.' },
  { prefixes: ['cmd/browser-agent/upload'], purpose: 'Implements upload command handling, validation, and OS automation wiring.' },
  { prefixes: ['cmd/browser-agent/testgen'], purpose: 'Implements test generation, classification, and healing command handlers.' },
  {
    prefixes: ['cmd/browser-agent/tools_configure'],
    purpose: 'Implements configure tool handlers for policy, profiles, and session controls.'
  },
  {
    prefixes: ['cmd/browser-agent/recording_', 'cmd/browser-agent/tools_recording_video'],
    purpose: 'Implements recording and playback command handlers for captured browser sessions.'
  },
  { prefixes: ['cmd/browser-agent/tools_analyze'], purpose: 'Implements analyze tool handlers and response shaping.' },
  { prefixes: ['cmd/browser-agent/tools_interact'], purpose: 'Implements interact tool handlers and browser action orchestration.' },
  { prefixes: ['cmd/browser-agent/tools_observe'], purpose: 'Implements observe tool queries against captured runtime buffers.' },
  { prefixes: ['cmd/browser-agent/tools_generate'], purpose: 'Implements generate tool formats and output assembly.' },
  { prefixes: ['internal/bridge/'], purpose: 'Implements framed stdio transport, timeouts, and bridge connection lifecycle.' },
  { prefixes: ['internal/buffers/'], purpose: 'Implements ring buffer storage primitives and cursor-safe access patterns.' },
  { prefixes: ['internal/export/'], purpose: 'Implements export serializers and format-specific output builders.' },
  { prefixes: ['internal/mcp/'], purpose: 'Defines MCP protocol types, validation, and structured error response helpers.' },
  { prefixes: ['internal/pagination/'], purpose: 'Implements cursor pagination over captured telemetry collections.' },
  { prefixes: ['internal/redaction/'], purpose: 'Implements redaction rules for sensitive data in captured telemetry.' },
  { prefixes: ['internal/performance/'], purpose: 'Implements performance metric diffing and threshold evaluation.' },
  { prefixes: ['internal/queries/'], purpose: 'Implements async command/query dispatch and correlation state tracking.' },
  { prefixes: ['internal/recording/'], purpose: 'Implements recording storage, replay engine execution, and diffing helpers.' },
  { prefixes: ['internal/schema/'], purpose: 'Defines JSON schema contracts for tool arguments and responses.' },
  { prefixes: ['internal/session/'], purpose: 'Implements session lifecycle, snapshots, and diff state management.' },
  { prefixes: ['internal/testgen/'], purpose: 'Implements prompt-driven test generation, healing, and classification helpers.' },
  { prefixes: ['internal/tools/analyze/'], purpose: 'Provides analyze tool implementation helpers shared by command handlers.' },
  { prefixes: ['internal/tools/configure/'], purpose: 'Provides configure tool implementation helpers for policy and rewrite flows.' },
  { prefixes: ['internal/tools/generate/'], purpose: 'Provides generate tool implementation helpers for emitted artifacts.' },
  { prefixes: ['internal/tools/interact/'], purpose: 'Provides interact tool implementation helpers for selectors and workflows.' },
  { prefixes: ['internal/tools/observe/'], purpose: 'Provides observe tool implementation helpers for filtering and storage queries.' },
  { prefixes: ['internal/upload/'], purpose: 'Implements upload validation, security checks, and automation support paths.' },
  { prefixes: ['src/background/'], purpose: 'Handles extension background coordination and message routing.' },
  { prefixes: ['src/content/'], purpose: 'Handles content-script message relay between background and inject contexts.' },
  { prefixes: ['src/inject/'], purpose: 'Executes in-page actions and query handlers within the page context.' },
  { prefixes: ['src/lib/'], purpose: 'Provides shared runtime utilities used by extension and server workflows.' }
]

function ruleMatches(rel, rule) {
  if (rule.exact) return rule.exact.includes(rel)
  return (
    (rule.prefixes || []).some((prefix) => rel.startsWith(prefix)) ||
    (rule.includes || []).some((needle) => rel.includes(needle))
  )
}

function inferDocs(rel) {
  const docs = new Set()
  for (const rule of DOC_RULES) {
    if (ruleMatches(rel, rule)) {
      for (const slug of rule.docs) docs.add(`docs/features/feature/${slug}/index.md`)
    }
  }
  return Array.from(docs)
}

function inferPurpose(rel) {
  for (const rule of PURPOSE_RULES) {
    if (ruleMatches(rel, rule)) return rule.purpose
  }
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
