#!/usr/bin/env node
// Purpose: Automate generate-wire-types.js workflow behavior for repository tooling.
// Why: Keeps repetitive maintenance and verification steps deterministic.
// Docs: docs/DEVELOPMENT.md

// generate-wire-types.js — Generates TypeScript wire type interfaces from Go wire type structs.
// Go structs are the source of truth; this script produces matching TS interfaces.
//
// Usage:
//   node scripts/build/generate-wire-types.js          # Generate TS files
//   node scripts/build/generate-wire-types.js --check   # Check for drift (exit non-zero if different)

import fs from 'fs'
import path from 'path'
import { fileURLToPath } from 'url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const ROOT = path.resolve(__dirname, '..', '..')

// ============================================
// Configuration
// ============================================

const WIRE_PAIRS = [
  { go: 'internal/types/wire_enhanced_action.go', ts: 'src/types/wire/wire-enhanced-action.ts' },
  { go: 'internal/types/wire_network.go', ts: 'src/types/wire/wire-network.ts' },
  { go: 'internal/types/wire_websocket_event.go', ts: 'src/types/wire/wire-websocket-event.ts' },
  { go: 'internal/performance/wire_performance.go', ts: 'src/types/wire/wire-performance-snapshot.ts' }
]

/**
 * Per-file field overrides. Keys are "StructName.json_field_name".
 * Values override the generated TS type string for that field.
 */
const TYPE_OVERRIDES = {
  // direction is a string in Go but a union type in TS
  'WireWebSocketEvent.direction': "'incoming' | 'outgoing'"
}

/**
 * Per-file field optionality overrides. Keys are "StructName.json_field_name".
 * When true, the field is forced optional (?) even if Go has no omitempty.
 */
const OPTIONAL_OVERRIDES = {
  // Extension may omit these timing fields even though Go struct has no omitempty
  'WireNetworkWaterfallEntry.fetch_start': true,
  'WireNetworkWaterfallEntry.response_end': true
}

/**
 * Per-file @fileoverview description overrides. Keys are Go file basenames.
 * 'overview' is the one-liner topic. 'description' is the second sentence.
 */
const FILE_DESCRIPTIONS = {
  'wire_enhanced_action.go': {
    overview: 'Wire type for enhanced actions',
    description: 'This is the canonical TypeScript definition for the EnhancedAction HTTP payload.'
  },
  'wire_network.go': {
    overview: 'Wire types for network telemetry',
    description: 'Canonical TypeScript definitions for NetworkBody and NetworkWaterfall HTTP payloads.'
  },
  'wire_websocket_event.go': {
    overview: 'Wire type for WebSocket events',
    description: 'Canonical TypeScript definition for the WebSocketEvent HTTP payload.'
  },
  'wire_performance.go': {
    overview: 'Wire types for performance snapshots',
    description: 'Canonical TypeScript definitions for the PerformanceSnapshot HTTP payload.'
  }
}

/**
 * Per-struct JSDoc comment overrides. Keys are struct names.
 * When present, overrides the Go comment entirely.
 */
const STRUCT_COMMENT_OVERRIDES = {
  WireEnhancedAction:
    'WireEnhancedAction is the JSON shape sent over HTTP between extension and Go daemon.\n * All fields use snake_case to match the Go json tags.',
  WireNetworkBody: 'WireNetworkBody is the JSON shape for captured network request/response bodies.',
  WireNetworkWaterfallEntry:
    'WireNetworkWaterfallEntry is the JSON shape for a single PerformanceResourceTiming entry.',
  WireNetworkWaterfallPayload: 'WireNetworkWaterfallPayload is the top-level shape POSTed to /network-waterfall.',
  WireWebSocketEvent: 'WireWebSocketEvent is the JSON shape sent over HTTP for captured WebSocket events.',
  WirePerformanceTiming: 'WirePerformanceTiming holds navigation timing metrics.',
  WireTypeSummary: 'WireTypeSummary holds per-type resource metrics.',
  WireSlowRequest: 'WireSlowRequest represents one of the slowest network requests.',
  WireNetworkSummary: 'WireNetworkSummary holds aggregated network resource metrics.',
  WireLongTaskMetrics: 'WireLongTaskMetrics holds accumulated long task data.',
  WireUserTimingEntry: 'WireUserTimingEntry represents a single performance mark or measure.',
  WireUserTimingData: 'WireUserTimingData holds captured performance.mark() and performance.measure() entries.',
  WirePerformanceSnapshot: 'WirePerformanceSnapshot is the JSON shape sent over HTTP for performance data.'
}

/**
 * Per-file server-only comments. Keys are TS interface names.
 * Values are the comment lines to append inside the interface.
 */
const SERVER_ONLY_COMMENTS = {
  WireEnhancedAction: [
    '// server-only: test_ids — added by Go daemon for test boundary correlation',
    '// server-only: source — added by Go daemon ("human" or "ai")'
  ],
  WireNetworkBody: [
    '// server-only: ts — server-side timestamp',
    '// server-only: response_headers, has_auth_header, binary_format, format_confidence, test_ids'
  ],
  WireWebSocketEvent: ['// server-only: sampled, binary_format, format_confidence, tab_id, test_ids'],
  WirePerformanceSnapshot: ['// server-only: resources — added by Go daemon for causal diffing']
}

// ============================================
// Go Parser
// ============================================

/**
 * Parse a single Go struct from file content.
 * Returns { name, fields: [{ goName, goType, jsonTag, omitempty, comment }] }
 */
function parseGoStruct(content, typeName) {
  const structRegex = new RegExp(`type\\s+${typeName}\\s+struct\\s*\\{([^}]*)\\}`, 's')
  const match = content.match(structRegex)
  if (!match) return null

  const body = match[1]
  const fields = []

  for (const line of body.split('\n')) {
    const trimmed = line.trim()
    if (!trimmed || trimmed.startsWith('//')) continue

    // Match: FieldName  GoType  `json:"json_name,omitempty"` // optional comment
    const fieldMatch = trimmed.match(/^(\w+)\s+([\w.*[\]]+(?:[[\w.]+]\w+)?)\s+`json:"([^"]+)"`(?:\s*\/\/\s*(.*))?$/)
    if (!fieldMatch) continue

    const [, goName, goType, jsonTagRaw, comment] = fieldMatch
    const jsonParts = jsonTagRaw.split(',')
    const jsonTag = jsonParts[0]
    const omitempty = jsonParts.includes('omitempty')

    fields.push({ goName, goType: goType.trim(), jsonTag, omitempty, comment: comment || '' })
  }

  return { name: typeName, fields }
}

/**
 * Parse ALL structs from a Go file.
 * Returns array of { name, fields }.
 */
function parseAllGoStructs(content) {
  const structs = []
  const structNameRegex = /type\s+(\w+)\s+struct\s*\{/g
  let match
  while ((match = structNameRegex.exec(content)) !== null) {
    const parsed = parseGoStruct(content, match[1])
    if (parsed) structs.push(parsed)
  }
  return structs
}

// ============================================
// Type Mapping
// ============================================

/**
 * Map a Go type to a TypeScript type.
 * @param {string} goType - The Go type string
 * @param {string} structName - The parent struct name (for override lookup)
 * @param {string} jsonTag - The JSON tag (for override lookup)
 * @returns {{ tsType: string, nullable: boolean }}
 */
function mapGoTypeToTS(goType, structName, jsonTag) {
  const overrideKey = `${structName}.${jsonTag}`
  if (TYPE_OVERRIDES[overrideKey]) {
    return { tsType: TYPE_OVERRIDES[overrideKey], nullable: false }
  }

  // Pointer types
  if (goType.startsWith('*')) {
    const inner = goType.slice(1)
    const mapped = mapGoTypeToTS(inner, structName, jsonTag)
    if (inner === 'float64' || inner === 'int' || inner === 'int64') {
      return { tsType: `${mapped.tsType} | null`, nullable: true }
    }
    // Pointer to struct — becomes optional (handled via omitempty), type stays as struct name
    return { tsType: mapped.tsType, nullable: false }
  }

  // Slice types
  if (goType.startsWith('[]')) {
    const inner = goType.slice(2)
    const mapped = mapGoTypeToTS(inner, structName, jsonTag)
    return { tsType: `readonly ${mapped.tsType}[]`, nullable: false }
  }

  // Map types
  const mapMatch = goType.match(/^map\[(\w+)\](.+)$/)
  if (mapMatch) {
    const [, keyType, valueType] = mapMatch
    const mappedKey = mapGoTypeToTS(keyType, structName, jsonTag)
    const mappedValue = mapGoTypeToTS(valueType, structName, jsonTag)
    return { tsType: `Readonly<Record<${mappedKey.tsType}, ${mappedValue.tsType}>>`, nullable: false }
  }

  // Primitive types
  switch (goType) {
    case 'string':
      return { tsType: 'string', nullable: false }
    case 'int':
      return { tsType: 'number', nullable: false }
    case 'int64':
      return { tsType: 'number', nullable: false }
    case 'float64':
      return { tsType: 'number', nullable: false }
    case 'bool':
      return { tsType: 'boolean', nullable: false }
    case 'any':
      return { tsType: 'unknown', nullable: false }
    default:
      // Assume it's a struct reference (e.g., WireTypeSummary)
      return { tsType: goType, nullable: false }
  }
}

// ============================================
// TS Code Generation
// ============================================

/**
 * Generate a TypeScript interface from a parsed Go struct.
 */
function generateInterface(goStruct, goComment) {
  const lines = []

  // JSDoc comment — prefer override, fall back to Go comment
  const comment = STRUCT_COMMENT_OVERRIDES[goStruct.name] || goComment
  if (comment) {
    lines.push('/**')
    lines.push(` * ${comment}`)
    lines.push(' */')
  }

  lines.push(`export interface ${goStruct.name} {`)

  for (const field of goStruct.fields) {
    const { tsType } = mapGoTypeToTS(field.goType, goStruct.name, field.jsonTag)
    const overrideOptional = OPTIONAL_OVERRIDES[`${goStruct.name}.${field.jsonTag}`]
    const isOptional = field.omitempty || overrideOptional || false

    // For pointer-to-struct with omitempty, the field is optional
    const isPointerToStruct =
      field.goType.startsWith('*') && !['float64', 'int', 'int64', 'bool', 'string'].includes(field.goType.slice(1))

    const optMarker = isOptional || isPointerToStruct ? '?' : ''
    lines.push(`  readonly ${field.jsonTag}${optMarker}: ${tsType}`)
  }

  // Add server-only comments if configured
  const serverComments = SERVER_ONLY_COMMENTS[goStruct.name]
  if (serverComments) {
    for (const comment of serverComments) {
      lines.push(`  ${comment}`)
    }
  }

  lines.push('}')

  return lines.join('\n')
}

/**
 * Extract the single-line doc comment preceding a struct definition.
 * Returns the Go comment with struct name included.
 */
function extractStructComment(content, structName) {
  const lines = content.split('\n')
  for (let i = 0; i < lines.length; i++) {
    if (lines[i].match(new RegExp(`^type\\s+${structName}\\s+struct`))) {
      // Look at the line immediately before
      if (i > 0 && lines[i - 1].trim().startsWith('//')) {
        const comment = lines[i - 1].trim().replace(/^\/\/\s*/, '')
        return comment
      }
    }
  }
  return null
}

/**
 * Generate the full TS file content for a Go source file.
 */
function generateTSFile(goContent, goPath, _tsPath) {
  const structs = parseAllGoStructs(goContent)
  if (structs.length === 0) {
    throw new Error(`No structs found in ${goPath}`)
  }

  const lines = []

  // Generated header
  lines.push('// THIS FILE IS GENERATED — do not edit by hand.')
  lines.push('// Source: ' + goPath)
  lines.push('// Generator: scripts/build/generate-wire-types.js')
  lines.push('')

  // @fileoverview block
  const goBase = path.basename(goPath)
  const fileDesc = FILE_DESCRIPTIONS[goBase]
  const overview = fileDesc ? `${fileDesc.overview} — matches ${goPath}` : `Wire types — matches ${goPath}`
  const description = fileDesc ? fileDesc.description : 'Canonical TypeScript definitions for the wire payloads.'

  lines.push('/**')
  lines.push(` * @fileoverview ${overview}`)
  lines.push(' *')
  lines.push(` * ${description}`)
  lines.push(' * Changes here MUST be mirrored in the Go counterpart. Run `make check-wire-drift`.')
  lines.push(' */')

  // Generate each interface
  for (const goStruct of structs) {
    lines.push('')
    const comment = extractStructComment(goContent, goStruct.name)
    const iface = generateInterface(goStruct, comment)
    lines.push(iface)
  }

  // Trailing newline
  lines.push('')

  return lines.join('\n')
}

// ============================================
// Main
// ============================================

const isCheck = process.argv.includes('--check')
let driftCount = 0
let generatedCount = 0

for (const pair of WIRE_PAIRS) {
  const goPath = path.join(ROOT, pair.go)
  const tsPath = path.join(ROOT, pair.ts)

  if (!fs.existsSync(goPath)) {
    console.error(`MISSING: ${pair.go}`)
    process.exit(1)
  }

  const goContent = fs.readFileSync(goPath, 'utf-8')
  const generated = generateTSFile(goContent, pair.go, pair.ts)

  if (isCheck) {
    // Compare against existing file
    if (!fs.existsSync(tsPath)) {
      console.error(`MISSING: ${pair.ts} (would be generated from ${pair.go})`)
      driftCount++
      continue
    }

    const existing = fs.readFileSync(tsPath, 'utf-8')
    if (generated !== existing) {
      console.error(`DRIFT: ${pair.ts} differs from generated output`)
      // Show a summary diff
      const genLines = generated.split('\n')
      const existLines = existing.split('\n')
      const maxLen = Math.max(genLines.length, existLines.length)
      for (let i = 0; i < maxLen; i++) {
        const g = genLines[i] ?? '<missing>'
        const e = existLines[i] ?? '<missing>'
        if (g !== e) {
          console.error(`  line ${i + 1}:`)
          console.error(`    expected: ${g}`)
          console.error(`    actual:   ${e}`)
        }
      }
      driftCount++
    } else {
      console.log(`OK: ${pair.ts}`)
    }
  } else {
    // Write the generated file
    fs.writeFileSync(tsPath, generated)
    console.log(`GENERATED: ${pair.ts}`)
    generatedCount++
  }
}

if (isCheck) {
  if (driftCount > 0) {
    console.error(`\nFAIL: ${driftCount} file(s) have wire type drift`)
    console.error('Run `node scripts/build/generate-wire-types.js` to regenerate')
    process.exit(1)
  } else {
    console.log(`\nOK: ${WIRE_PAIRS.length} wire type files verified, zero drift`)
  }
} else {
  console.log(`\nGenerated ${generatedCount} TypeScript wire type file(s)`)
}
