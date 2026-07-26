#!/usr/bin/env node
// check-ts-json-casing.js — Enforce snake_case keys in JSON.stringify() calls in TypeScript.
//
// Scans src/**/*.ts for JSON.stringify({ ... }) object literals and flags any
// camelCase keys. This prevents wire-type mismatches where the extension sends
// camelCase but the Go server expects snake_case.
//
// Allowlist: Keys that are part of external specs (e.g., JSON-RPC "jsonrpc", "params")
// can be annotated with // WIRE-OK on the same line, or added to ALLOWED_KEYS below.
//
// Usage: node scripts/contracts/check-ts-json-casing.js
// Exit code: 0 if clean, 1 if violations found.

import fs from 'fs'
import path from 'path'
import { fileURLToPath } from 'url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const rootDir = path.resolve(__dirname, '..', '..')

// Keys exempt from snake_case requirement (external protocol fields)
const ALLOWED_KEYS = new Set([
  'jsonrpc',    // JSON-RPC 2.0 spec
  'id',         // JSON-RPC 2.0 spec
  'method',     // JSON-RPC 2.0 spec
  'params',     // JSON-RPC 2.0 spec
  'result',     // JSON-RPC 2.0 spec
  'error',      // JSON-RPC 2.0 spec
  'o', 'oy', 'ox', 'h', 'n', 'x', 'w',  // Compact style property shorthands
])

// Pattern: camelCase = lowercase letter immediately followed by uppercase letter
const CAMEL_CASE_RE = /[a-z][A-Z]/

/**
 * Find all .ts files under a directory recursively.
 */
function findTsFiles(dir) {
  const results = []
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const fullPath = path.join(dir, entry.name)
    if (entry.isDirectory() && entry.name !== 'node_modules' && entry.name !== 'types') {
      results.push(...findTsFiles(fullPath))
    } else if (entry.isFile() && entry.name.endsWith('.ts') && !entry.name.endsWith('.d.ts')) {
      results.push(fullPath)
    }
  }
  return results
}

/**
 * Extract the balanced content of a brace-delimited block starting at `start`.
 * Returns the content between (not including) the braces, or null.
 */
function extractBracedBlock(text, start) {
  if (text[start] !== '{') return null
  let depth = 1
  let pos = start + 1
  while (pos < text.length && depth > 0) {
    if (text[pos] === '{') depth++
    else if (text[pos] === '}') depth--
    pos++
  }
  if (depth !== 0) return null
  return { content: text.slice(start + 1, pos - 1), end: pos }
}

/**
 * Extract top-level object-literal keys from a block of code.
 * Only considers keys at brace-depth 0 (skips nested objects/arrays).
 */
function extractTopLevelKeys(block) {
  const keys = []
  let depth = 0
  let i = 0
  while (i < block.length) {
    const ch = block[i]
    if (ch === '{' || ch === '[') { depth++; i++; continue }
    if (ch === '}' || ch === ']') { depth--; i++; continue }
    if (depth > 0) { i++; continue }

    // Skip string literals
    if (ch === '\'' || ch === '"' || ch === '`') {
      i++
      while (i < block.length && block[i] !== ch) {
        if (block[i] === '\\') i++ // skip escaped char
        i++
      }
      i++ // skip closing quote
      continue
    }

    // Skip line comments
    if (ch === '/' && block[i + 1] === '/') {
      while (i < block.length && block[i] !== '\n') i++
      continue
    }

    // Skip block comments
    if (ch === '/' && block[i + 1] === '*') {
      i += 2
      while (i < block.length - 1 && !(block[i] === '*' && block[i + 1] === '/')) i++
      i += 2
      continue
    }

    // Match: identifier followed by : (but not ::)
    // This catches `key:` and `key :` patterns
    const keyMatch = block.slice(i).match(/^(\w+)\s*:(?!:)/)
    if (keyMatch) {
      const key = keyMatch[1]
      // Get the line for WIRE-OK annotation check
      const lineStart = block.lastIndexOf('\n', i) + 1
      const lineEnd = block.indexOf('\n', i)
      const line = block.slice(lineStart, lineEnd === -1 ? block.length : lineEnd)

      keys.push({ key, line: line.trim(), offset: i })
      i += keyMatch[0].length
      continue
    }

    i++
  }
  return keys
}

/**
 * Get line number for a character offset in source text.
 */
function getLineNumber(text, offset) {
  let line = 1
  for (let i = 0; i < offset && i < text.length; i++) {
    if (text[i] === '\n') line++
  }
  return line
}

// ============================================
// Main
// ============================================

const srcDir = path.join(rootDir, 'src')
const files = findTsFiles(srcDir)
let violations = 0
let checked = 0

for (const filePath of files) {
  const content = fs.readFileSync(filePath, 'utf-8')
  const relPath = path.relative(rootDir, filePath)

  // Find all JSON.stringify({ occurrences
  const re = /JSON\.stringify\(\s*\{/g
  let match
  while ((match = re.exec(content)) !== null) {
    // Find the opening brace position
    const bracePos = content.indexOf('{', match.index + 'JSON.stringify('.length)
    if (bracePos === -1) continue

    const block = extractBracedBlock(content, bracePos)
    if (!block) continue

    // Check if the JSON.stringify line itself has WIRE-OK (exempts entire block)
    const callLineStart = content.lastIndexOf('\n', match.index) + 1
    const callLineEnd = content.indexOf('\n', match.index)
    const callLine = content.slice(callLineStart, callLineEnd === -1 ? content.length : callLineEnd)
    if (callLine.includes('WIRE-OK')) {
      checked++
      continue
    }

    checked++
    const keys = extractTopLevelKeys(block.content)

    for (const { key, line } of keys) {
      // Skip allowed keys
      if (ALLOWED_KEYS.has(key)) continue

      // Skip WIRE-OK annotated lines
      if (line.includes('WIRE-OK')) continue

      // Check for camelCase
      if (CAMEL_CASE_RE.test(key)) {
        const lineNum = getLineNumber(content, bracePos)
        console.error(`VIOLATION: ${relPath}:${lineNum}`)
        console.error(`  Key '${key}' uses camelCase (expected snake_case)`)
        console.error(`  Context: ${line}`)
        console.error('')
        violations++
      }
    }
  }
}

if (violations > 0) {
  console.error(`FAIL: ${violations} camelCase key(s) in JSON.stringify() calls`)
  console.error('All JSON keys sent to the server must use snake_case.')
  console.error('Add // WIRE-OK to exempt external protocol fields.')
  process.exit(1)
} else {
  console.log(`OK: ${checked} JSON.stringify() calls checked, all keys are snake_case`)
}
