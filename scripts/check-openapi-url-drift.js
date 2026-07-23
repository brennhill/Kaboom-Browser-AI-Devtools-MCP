#!/usr/bin/env node
// check-openapi-url-drift.js — Validates that every TS/JS call to the daemon
// targets a path documented in cmd/browser-agent/openapi.json.
// Catches HTTP-boundary drift that wire_*.go ↔ wire-*.ts struct checks miss:
// e.g. extension calling /api/extension-status when the spec has no such path.
//
// Usage: node scripts/check-openapi-url-drift.js [--json]
// Exit codes: 0 = no drift, 1 = drift detected.

import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const REPO_ROOT = path.resolve(__dirname, '..')

// URL-producing expressions that target the daemon. The check extracts the
// path literal that immediately follows these template heads.
const DAEMON_URL_HEADS = [
  '${serverUrl}',
  '${SERVER_URL}',
  '${this.serverUrl}',
  '${getServerUrl()}',
  '${deps.getServerUrl()}',
  '${getBrowserAgentUrl()}'
]

// Note: URLs built from non-daemon expressions — `${termUrl}` and
// `${getTerminalServerUrl(state.serverUrl)}` (the terminal sub-server) — are not
// part of the daemon spec. They need no explicit exclusion list: the scanner only
// matches DAEMON_URL_HEADS above, so those heads never produce a hit.

// Directories to scan for call sites. Only src/ — extension/ is the compiled
// output and would double-count.
const SCAN_DIRS = ['src']

// File extensions treated as source.
const EXTS = new Set(['.ts', '.tsx', '.mts', '.cts'])

// Paths that are intentionally unspec'd — baseline-skip allowlist mirroring
// the schemathesis baseline-skip list. Adding an entry here REQUIRES adding
// a matching row to docs/audits/openapi-drift-backlog.md with a linked issue.
// The goal is for this list to stay empty. Each entry blocks real drift
// detection for that path, so adding one is a deliberate tradeoff, not a
// silencer for noise.
const KNOWN_UNSPECD_PATHS = new Set([])

// ---------- IO helpers ----------

function walk(dir, out = []) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name)
    if (entry.isDirectory()) {
      if (entry.name === 'node_modules' || entry.name.startsWith('.')) continue
      walk(full, out)
    } else if (EXTS.has(path.extname(entry.name))) {
      out.push(full)
    }
  }
  return out
}

function loadSpecPaths(specPath) {
  const raw = fs.readFileSync(specPath, 'utf8')
  const spec = JSON.parse(raw)
  if (!spec.paths || typeof spec.paths !== 'object') {
    throw new Error(`openapi spec at ${specPath} has no .paths`)
  }
  return new Set(Object.keys(spec.paths))
}

// ---------- extraction ----------

/**
 * Extract daemon-path references from a single source file.
 * Returns an array of { path, line, snippet } where path is the pathname
 * portion of a URL that follows a known daemon URL head.
 */
function extractDaemonPaths(source, filePath) {
  const hits = []
  const lines = source.split('\n')
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i]
    for (const head of DAEMON_URL_HEADS) {
      let idx = 0
      while ((idx = line.indexOf(head, idx)) !== -1) {
        const rest = line.slice(idx + head.length)
        const pathLiteral = readPathLiteral(rest)
        if (pathLiteral) {
          hits.push({
            path: pathLiteral,
            line: i + 1,
            file: filePath,
            snippet: line.trim()
          })
        }
        idx += head.length
      }
    }
  }
  return hits
}

/**
 * Read a path literal that starts at the current position. Stops at the first
 * character that ends the literal (quote, backtick, whitespace, `?`, `#`).
 * Returns the path with `${...}` interpolations normalized to `{param}` so it
 * can be matched against spec path templates like `/clients/{id}`.
 */
function readPathLiteral(rest) {
  if (!rest.startsWith('/')) return null
  let out = ''
  let i = 0
  while (i < rest.length) {
    const ch = rest[i]
    if (ch === '`' || ch === "'" || ch === '"') break
    if (ch === '?' || ch === '#') break
    if (ch === ' ' || ch === '\t' || ch === ',' || ch === ')') break
    if (ch === '$' && rest[i + 1] === '{') {
      // Track brace depth so nested expressions like `${foo({a:1})}` close on
      // the correct `}` — a plain indexOf would stop at the inner `}` and
      // truncate the path suffix. Returns null if braces are unbalanced.
      let depth = 1
      let j = i + 2
      while (j < rest.length && depth > 0) {
        if (rest[j] === '{') depth++
        else if (rest[j] === '}') depth--
        j++
      }
      if (depth !== 0) return null
      out += '{param}'
      i = j
      continue
    }
    out += ch
    i++
  }
  // Strip trailing sentence punctuation that can trail a path when it's
  // embedded in an error message or comment (e.g., "failed to reach /foo.").
  // Path chars like `.` are legal internally (`/diagnostics.json`) so we only
  // trim them off the very end.
  out = out.replace(/[.,;:!?]+$/, '')
  return out.length > 1 ? out : null
}

// ---------- match ----------

/**
 * Check if a code path matches any spec path. Spec paths use `{name}`
 * placeholders; code paths have had `${...}` normalized to `{param}`.
 * Match is structural: same number of segments, literals match exactly,
 * any segment wrapped in `{...}` on either side is a wildcard.
 */
function pathMatchesSpec(codePath, specPaths) {
  if (specPaths.has(codePath)) return true
  const codeSegs = codePath.split('/')
  for (const spec of specPaths) {
    const specSegs = spec.split('/')
    if (specSegs.length !== codeSegs.length) continue
    let ok = true
    for (let i = 0; i < specSegs.length; i++) {
      const s = specSegs[i]
      const c = codeSegs[i]
      const sIsParam = s.startsWith('{') && s.endsWith('}')
      const cIsParam = c.startsWith('{') && c.endsWith('}')
      if (sIsParam || cIsParam) continue
      if (s !== c) { ok = false; break }
    }
    if (ok) return true
  }
  return false
}

// ---------- main ----------

function main() {
  const asJSON = process.argv.includes('--json')
  const specPaths = loadSpecPaths(path.join(REPO_ROOT, 'cmd/browser-agent/openapi.json'))

  const allHits = []
  for (const dir of SCAN_DIRS) {
    const full = path.join(REPO_ROOT, dir)
    if (!fs.existsSync(full)) continue
    for (const file of walk(full)) {
      const source = fs.readFileSync(file, 'utf8')
      allHits.push(...extractDaemonPaths(source, path.relative(REPO_ROOT, file)))
    }
  }

  const drifts = []
  for (const hit of allHits) {
    if (KNOWN_UNSPECD_PATHS.has(hit.path)) continue
    if (!pathMatchesSpec(hit.path, specPaths)) drifts.push(hit)
  }

  if (asJSON) {
    console.log(JSON.stringify({ scanned: allHits.length, drifts }, null, 2))
  } else {
    if (drifts.length === 0) {
      console.log(`OK: ${allHits.length} daemon URL call site(s) scanned, zero drift against ${specPaths.size} spec paths`)
    } else {
      console.error(`FAIL: ${drifts.length} daemon URL call site(s) hit paths not in openapi.json:\n`)
      for (const d of drifts) {
        console.error(`  ${d.file}:${d.line}  →  ${d.path}`)
        console.error(`    ${d.snippet}`)
      }
      console.error('\nFix options:')
      console.error('  1. Add the path to cmd/browser-agent/openapi.json (preferred)')
      console.error('  2. Remove the caller if the endpoint no longer exists')
      console.error('  3. Add the path to KNOWN_UNSPECD_PATHS in this script with an issue link')
    }
  }

  process.exit(drifts.length > 0 ? 1 : 0)
}

// Expose internals for testing; only run when invoked as a script.
export { extractDaemonPaths, pathMatchesSpec, readPathLiteral, loadSpecPaths }

if (import.meta.url === `file://${process.argv[1]}`) {
  main()
}
