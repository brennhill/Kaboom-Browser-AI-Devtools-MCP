#!/usr/bin/env node
// Purpose: Automate check-wire-drift.js workflow behavior for repository tooling.
// Why: Keeps repetitive maintenance and verification steps deterministic.
// Docs: docs/setup/DEVELOPMENT.md

// check-wire-drift.js — Validates Go and TypeScript wire types stay in sync.
// Compares json tags in wire_*.go files against interface fields in wire-*.ts files.
// Exits non-zero if drift is detected.
//
// Usage: node scripts/contracts/check-wire-drift.js

import fs from 'fs'
import path from 'path'
import { fileURLToPath } from 'url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

// ============================================
// Configuration
// ============================================

const WIRE_PAIRS = [
  {
    go: 'internal/types/wire_enhanced_action.go',
    ts: 'src/types/wire/wire-enhanced-action.ts',
    types: [{ go: 'WireEnhancedAction', ts: 'WireEnhancedAction' }]
  },
  {
    go: 'internal/types/wire_network.go',
    ts: 'src/types/wire/wire-network.ts',
    types: [
      { go: 'WireNetworkBody', ts: 'WireNetworkBody' },
      { go: 'WireNetworkWaterfallEntry', ts: 'WireNetworkWaterfallEntry' },
      { go: 'WireServerTiming', ts: 'WireServerTiming' },
      { go: 'WireNetworkWaterfallPayload', ts: 'WireNetworkWaterfallPayload' }
    ]
  },
  {
    go: 'internal/types/wire_network.go',
    ts: 'src/types/wire/wire-websocket-event.ts',
    types: [{ go: 'WireWebSocketEvent', ts: 'WireWebSocketEvent' }]
  },
  {
    go: 'internal/performance/wire_performance.go',
    ts: 'src/types/wire/wire-performance-snapshot.ts',
    types: [
      { go: 'WirePerformanceTiming', ts: 'WirePerformanceTiming' },
      { go: 'WireNetworkSummary', ts: 'WireNetworkSummary' },
      { go: 'WireLongTaskMetrics', ts: 'WireLongTaskMetrics' },
      { go: 'WirePerformanceSnapshot', ts: 'WirePerformanceSnapshot' },
      { go: 'WireTypeSummary', ts: 'WireTypeSummary' },
      { go: 'WireSlowRequest', ts: 'WireSlowRequest' },
      { go: 'WireUserTimingEntry', ts: 'WireUserTimingEntry' },
      { go: 'WireUserTimingData', ts: 'WireUserTimingData' }
    ]
  },
  {
    go: 'internal/qafixture/wire_fixture.go',
    ts: 'src/types/wire/wire-qa-fixture.ts',
    types: [
      { go: 'WireQAFixture', ts: 'WireQAFixture' },
      { go: 'WireQATarget', ts: 'WireQATarget' },
      { go: 'WireQAViewport', ts: 'WireQAViewport' },
      { go: 'WireQANetwork', ts: 'WireQANetwork' },
      { go: 'WireQACookie', ts: 'WireQACookie' }
    ]
  },
  {
    go: 'internal/types/wire_log.go',
    ts: 'src/types/wire/wire-extension-log.ts',
    types: [{ go: 'ExtensionLog', ts: 'ExtensionLog' }]
  },
  {
    go: 'internal/capture/syncruntime/wire_sync.go',
    ts: 'src/types/wire/wire-sync.ts',
    types: [
      { go: 'SyncRequest', ts: 'SyncRequest' },
      { go: 'SyncSettings', ts: 'SyncSettings' },
      { go: 'SyncCommandResult', ts: 'SyncCommandResult' },
      { go: 'SyncInProgress', ts: 'SyncInProgress' },
      { go: 'SyncFeaturesUsed', ts: 'SyncFeaturesUsed' },
      { go: 'SyncResponse', ts: 'SyncResponse' },
      { go: 'SyncCommand', ts: 'SyncCommand' }
    ]
  }
]

// ============================================
// Go Parser
// ============================================

/**
 * Extract json field names from a Go struct definition.
 * Returns a Set of field names (without omitempty).
 */
function extractGoFields(content, typeName) {
  // Match: type TypeName struct { ... }
  const structRegex = new RegExp(`type\\s+${typeName}\\s+struct\\s*\\{([^}]*)\\}`, 's')
  const match = content.match(structRegex)
  if (!match) return null

  const body = match[1]
  const fields = new Set()

  // Match json tags: `json:"field_name"` or `json:"field_name,omitempty"`
  const tagRegex = /`json:"([^"]+)"`/g
  let tagMatch
  while ((tagMatch = tagRegex.exec(body)) !== null) {
    const tag = tagMatch[1]
    // Strip omitempty and other options
    const fieldName = tag.split(',')[0]
    if (fieldName && fieldName !== '-') {
      fields.add(fieldName)
    }
  }

  return fields
}

// ============================================
// TypeScript Parser
// ============================================

/**
 * Extract the body of the interface whose opening brace is at `braceStart`.
 */
function interfaceBody(content, braceStart) {
  let depth = 1
  let pos = braceStart + 1
  while (pos < content.length && depth > 0) {
    if (content[pos] === '{') depth++
    if (content[pos] === '}') depth--
    pos++
  }
  return content.slice(braceStart + 1, pos - 1)
}

/**
 * Net brace depth delta across one line.
 */
function lineDepthDelta(line) {
  let delta = 0
  for (const ch of line) {
    if (ch === '{') delta++
    if (ch === '}') delta--
  }
  return delta
}

/**
 * True when a trimmed interface line is a comment, blank, or closing brace.
 */
function isNonFieldLine(trimmed) {
  return trimmed.startsWith('//') || trimmed.startsWith('*') || trimmed.startsWith('/*') || !trimmed || trimmed === '}'
}

/**
 * Extract field names from a TypeScript interface definition.
 * Returns a Set of field names.
 */
function extractTsFields(content, typeName) {
  // Match: export interface TypeName { ... }
  // Handle multi-line with nested types
  const interfaceStart = content.search(new RegExp(`interface\\s+${typeName}\\s*\\{`))
  if (interfaceStart === -1) return null

  // Find the opening brace
  const braceStart = content.indexOf('{', interfaceStart)
  if (braceStart === -1) return null

  const body = interfaceBody(content, braceStart)
  const fields = new Set()

  // Match field declarations at the top level only (depth 0)
  // readonly field_name: type or readonly field_name?: type
  let lineDepth = 0
  for (const line of body.split('\n')) {
    lineDepth += lineDepthDelta(line)
    if (lineDepth > 0) continue

    if (isNonFieldLine(line.trim())) continue

    const fieldMatch = line.trim().match(/^\s*(?:readonly\s+)?(\w+)\??\s*:/)
    if (fieldMatch) {
      fields.add(fieldMatch[1])
    }
  }

  return fields
}

// ============================================
// Main
// ============================================

const rootDir = path.resolve(__dirname, '..', '..')
let errors = 0
let checked = 0

for (const pair of WIRE_PAIRS) {
  const goPath = path.join(rootDir, pair.go)
  const tsPath = path.join(rootDir, pair.ts)

  if (!fs.existsSync(goPath)) {
    console.error(`MISSING: ${pair.go}`)
    errors++
    continue
  }
  if (!fs.existsSync(tsPath)) {
    console.error(`MISSING: ${pair.ts}`)
    errors++
    continue
  }

  const goContent = fs.readFileSync(goPath, 'utf-8')
  const tsContent = fs.readFileSync(tsPath, 'utf-8')

  for (const typePair of pair.types) {
    const goFields = extractGoFields(goContent, typePair.go)
    const tsFields = extractTsFields(tsContent, typePair.ts)

    if (!goFields) {
      console.error(`NOT FOUND: Go type ${typePair.go} in ${pair.go}`)
      errors++
      continue
    }
    if (!tsFields) {
      console.error(`NOT FOUND: TS type ${typePair.ts} in ${pair.ts}`)
      errors++
      continue
    }

    // Compare fields — Go is the source of truth
    const goOnly = [...goFields].filter((f) => !tsFields.has(f))
    const tsOnly = [...tsFields].filter((f) => !goFields.has(f))

    if (goOnly.length > 0 || tsOnly.length > 0) {
      console.error(`DRIFT: ${typePair.go} ↔ ${typePair.ts}`)
      if (goOnly.length > 0) {
        console.error(`  Go-only fields: ${goOnly.join(', ')}`)
      }
      if (tsOnly.length > 0) {
        console.error(`  TS-only fields: ${tsOnly.join(', ')}`)
      }
      errors++
    } else {
      checked++
    }
  }
}

if (errors > 0) {
  console.error(`\nFAIL: ${errors} wire type drift(s) detected`)
  process.exit(1)
} else {
  console.log(`OK: ${checked} wire type pairs verified, zero drift`)
}
