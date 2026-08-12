#!/usr/bin/env node
// snapshot-tool-responses.cjs — Capture the CURRENT response shape of every MCP mode.
//
// Purpose: MCP tool responses have no declared contract (internal/mcp/protocol.go
// declares InputSchema and nothing for output), so there is nothing for a response
// to drift from. This captures what the hardened tools actually return today, which
// becomes the declared contract rather than a redesign of 163 payloads.
//
// FAIL LOUD. A previous version of this recorded transport failures as response
// shapes and produced 158 rows of "no_text" against a daemon that was down — a
// contract derived from that would have been permanently wrong. Every failure
// mode here aborts the run instead of becoming a row.
//
// This is an HTTP client only. It never spawns a daemon: a stray daemon claiming
// the shared lock at ~/.kaboom/run/daemon.lock.json is what took the developer's
// own daemon offline while this was being written.
//
// Usage: node scripts/contracts/snapshot-tool-responses.cjs <surface.json> <out.json> [port]
'use strict'

const fs = require('node:fs')
const { execFileSync } = require('node:child_process')

const [surfacePath, outPath, portArg] = process.argv.slice(2)
const PORT = portArg || process.env.KABOOM_PORT || '7890'

function die(message) {
  console.error(`\nFATAL: ${message}`)
  console.error('No snapshot written. A contract derived from a partial capture is worse than none.')
  process.exit(1)
}

function curl(args, what) {
  try {
    return execFileSync('curl', args, { encoding: 'utf8', maxBuffer: 8e7 })
  } catch (err) {
    die(`${what} could not reach the daemon on port ${PORT} (curl exit ${err.status}). Is it running?`)
  }
}

function health() {
  const raw = curl(['-s', '--max-time', '8', `http://127.0.0.1:${PORT}/health`], 'health check')
  let parsed
  try {
    parsed = JSON.parse(raw)
  } catch {
    die(`health endpoint returned non-JSON: ${raw.slice(0, 120)}`)
  }
  return parsed
}

// Browser-mediated modes silently degrade to errors when the extension is gone,
// and those degraded responses look like legitimate shapes. Refuse to capture
// unless the extension is attached, and check again at the end so a mid-run
// disconnect invalidates the run rather than poisoning the contract.
function requireAttachedExtension(stage) {
  const h = health()
  if (h?.capture?.extension_connected !== true) {
    die(
      `extension is not attached to the daemon (${stage}). Browser-mediated modes would be captured in a degraded state.`
    )
  }
}

if (!surfacePath || !outPath) die('usage: snapshot-tool-responses.cjs <surface.json> <out.json> [port]')
const surface = JSON.parse(fs.readFileSync(surfacePath, 'utf8'))

// Reuse cat-33's argument table so the snapshot matches what the sweep sends.
const sweep = fs.readFileSync('scripts/tests/browser/cat-33-connected-action-coverage.sh', 'utf8')
const argsStart = sweep.indexOf('action_args() {')
if (argsStart === -1)
  die('cat-33 action_args table not found; the snapshot must send the same arguments the sweep does')
const argsBody = sweep.slice(argsStart, sweep.indexOf('\n}', argsStart))
const argTable = {}
for (const m of argsBody.matchAll(/^\s+([a-z_|/]+)\)\s+echo\s+'([^']+)'\s*;;/gm)) {
  for (const pair of m[1].split('|')) argTable[pair] = m[2]
}

// Excluded deliberately, each for a reason that is not "it is inconvenient".
const EXCLUDED = {
  screen_recording_start: 'requires an explicit browser user gesture',
  screen_recording_stop: 'requires an explicit browser user gesture',
  clipboard_read: 'browser permission gated; outcome is decided by the browser',
  setup_quality_gates: 'writes into the checked-out project',
  restart: 'restarts the daemon mid-capture'
}

function callMode(tool, args) {
  const body = JSON.stringify({
    jsonrpc: '2.0',
    id: 1,
    method: 'tools/call',
    params: { name: tool, arguments: JSON.parse(args) }
  })
  const raw = curl(
    [
      '-s',
      '--max-time',
      '30',
      '-X',
      'POST',
      `http://127.0.0.1:${PORT}/mcp`,
      '-H',
      'Content-Type: application/json',
      '-d',
      body
    ],
    `${tool} call`
  )
  if (!raw.trim()) die(`${tool} returned an empty body — the daemon accepted the connection but answered nothing`)
  try {
    return JSON.parse(raw)
  } catch {
    die(`${tool} returned unparseable JSON-RPC: ${raw.slice(0, 160)}`)
  }
}

// The body is prose followed by JSON, and the prose can itself contain braces —
// analyze/draw_session's recovery hint embeds {what:'draw_history'}, and slicing
// from the first '{' anywhere in the text parsed that instead of the payload.
// Take the first line that BEGINS with '{', matching the framework's own
// sed -n '/^{/,$p' convention.
function jsonBodyOf(text) {
  const lines = text.split('\n')
  const start = lines.findIndex((line) => line.startsWith('{'))
  return start === -1 ? null : lines.slice(start).join('\n')
}

/** Classifies a response into the shapes this server actually returns. */
function shapeOf(rpc, key) {
  if (rpc.error) return { kind: 'rpc_error', payload_keys: [], note: String(rpc.error.message || '').slice(0, 80) }
  const text = rpc?.result?.content?.[0]?.text
  if (typeof text !== 'string') die(`${key} produced no content[0].text — the response envelope itself is malformed`)

  const isError = rpc?.result?.isError === true
  const body = jsonBodyOf(text)
  if (body === null) return { kind: 'prose_only', payload_keys: [], is_error: isError, prose: text.slice(0, 60) }

  let parsed
  try {
    parsed = JSON.parse(body)
  } catch {
    return { kind: 'unparseable_body', payload_keys: [], is_error: isError, body: body.slice(0, 80) }
  }
  if (parsed === null || typeof parsed !== 'object' || Array.isArray(parsed)) {
    return { kind: 'non_object', payload_keys: [], is_error: isError }
  }

  const top = Object.keys(parsed).sort()

  // A browser-mediated mode answers with a lifecycle envelope. When the query
  // has not resolved yet the envelope carries no `result` at all, and treating
  // that as the payload would lock the queue bookkeeping in as the contract —
  // analyze/dom was captured that way and looked like a 12-field response whose
  // fields were correlation_id, queue_depth and friends. Resolve it instead.
  const lifecycle = top.includes('correlation_id') && top.includes('lifecycle_status')
  if (lifecycle && !('result' in parsed)) {
    const resolved = resolveQueuedCommand(parsed.correlation_id)
    if (!resolved) {
      return { kind: 'pending_unresolved', payload_keys: [], is_error: isError, envelope_keys: top }
    }
    return {
      kind: 'envelope',
      envelope_keys: top,
      resolved_via: 'command_result',
      ...payloadShape(resolved),
      is_error: isError
    }
  }
  if (lifecycle) {
    return { kind: 'envelope', envelope_keys: top, ...payloadShape(parsed.result), is_error: isError }
  }
  return { kind: 'direct', payload_keys: top, payload_type: 'object', is_error: isError }
}

function payloadShape(inner) {
  const isObject = inner && typeof inner === 'object' && !Array.isArray(inner)
  return {
    payload_keys: isObject ? Object.keys(inner).sort() : [],
    payload_type: Array.isArray(inner) ? 'array' : inner === null ? 'null' : typeof inner
  }
}

// Polls observe(command_result) until the queued query resolves. Browser round
// trips are not instant, and a snapshot that gives up immediately would declare
// every browser-mediated mode contract-less.
function resolveQueuedCommand(correlationID) {
  if (!correlationID) return null
  for (let attempt = 0; attempt < 8; attempt++) {
    execFileSync('sleep', ['0.75'])
    const rpc = callMode('observe', JSON.stringify({ what: 'command_result', correlation_id: correlationID }))
    const text = rpc?.result?.content?.[0]?.text
    if (typeof text !== 'string') continue
    const body = jsonBodyOf(text)
    if (!body) continue
    let parsed
    try {
      parsed = JSON.parse(body)
    } catch {
      continue
    }
    const status = parsed.lifecycle_status || parsed.status
    if (status && String(status).match(/complete|resolved|success|failed|error/i)) {
      return 'result' in parsed ? parsed.result : parsed
    }
  }
  return null
}

requireAttachedExtension('before capture')

const rows = []
for (const [tool, modes] of Object.entries(surface)) {
  for (const mode of modes) {
    const key = `${tool}/${mode}`
    if (EXCLUDED[mode]) {
      rows.push({ key, tool, mode, kind: 'excluded', reason: EXCLUDED[mode] })
      continue
    }
    const args = argTable[key] || `{"what":"${mode}"}`
    const rpc = callMode(tool, args)
    const bytes = (rpc?.result?.content?.[0]?.text || '').length
    rows.push({ key, tool, mode, args, response_bytes: bytes, ...shapeOf(rpc, key) })
    process.stderr.write('.')
  }
}
process.stderr.write('\n')

requireAttachedExtension('after capture')

fs.writeFileSync(outPath, JSON.stringify(rows, null, 2))

const count = (k) => rows.filter((r) => r.kind === k).length
console.log(`captured ${rows.length} modes -> ${outPath}`)
console.log(
  `  direct=${count('direct')} envelope=${count('envelope')} prose_only=${count('prose_only')} ` +
    `non_object=${count('non_object')} unparseable=${count('unparseable_body')} rpc_error=${count('rpc_error')} excluded=${count('excluded')}`
)
console.log(`  responses flagged isError: ${rows.filter((r) => r.is_error).length}`)

// The review list the owner asked for: current behaviour that may be an accident
// rather than a decision, and should be looked at before it is locked in.
const big = rows.filter((r) => (r.response_bytes || 0) > 10000).sort((a, b) => b.response_bytes - a.response_bytes)
console.log(`\nLARGEST RESPONSES — ${big.length} modes over 10KB:`)
for (const r of big) console.log(`  ${String(r.response_bytes).padStart(7)} bytes  ${r.key}`)

const thin = rows.filter(
  (r) => !r.kind.match(/excluded|rpc_error/) && !r.is_error && (r.payload_keys || []).length <= 1
)
console.log(`\nREVIEW BEFORE LOCKING — ${thin.length} modes return 0-1 payload fields:`)
for (const r of thin)
  console.log(`  ${r.key.padEnd(34)} kind=${r.kind.padEnd(12)} payload=${JSON.stringify(r.payload_keys)}`)
