// assertion-falsifiability.test.cjs — Guards that every UAT test can actually fail.
//
// A UAT test that reports pass on every reachable branch is worse than no test:
// it consumes runtime and reports green while exercising nothing. Two concrete
// regressions motivated this contract:
//   - cat-16.3 asserted the server validates X-Kaboom-Client by checking the
//     rejection body parses as JSON. The 403 body IS valid JSON, so the test took
//     the "server is lenient" branch and passed — and the other branch passed too.
//   - cat-14/15/16 posted /sync payloads and asserted only `jq .` on the reply.
//     A 400/403/409 error body satisfies that, so malformed payloads read as green.
'use strict'

const assert = require('node:assert/strict')
const { readFileSync } = require('node:fs')
const { globSync } = require('node:fs')
const { describe, test } = require('node:test')

const CATEGORY_SCRIPTS = globSync('scripts/tests/*/cat-*.sh').sort()
// Categories delegate assertions to shared helpers (post_extension, expect_http_status,
// uat_* readiness gates), so reachability analysis has to follow into them too.
const SHARED_HELPER_SOURCES = globSync('scripts/tests/framework/*.sh').sort()

/** Removes comments so a commented-out `fail` never counts as a failure path. */
function stripComments(source) {
  return source
    .split('\n')
    .map((line) => line.replace(/(^|\s)#.*$/, ''))
    .join('\n')
}

/** Extracts `name() { ... }` bodies by brace matching. */
function localFunctions(source) {
  const functions = new Map()
  const pattern = /^([A-Za-z_][A-Za-z0-9_]*)\s*\(\)\s*\{/gm
  let match
  while ((match = pattern.exec(source)) !== null) {
    let depth = 1
    let index = match.index + match[0].length
    while (index < source.length && depth > 0) {
      const char = source[index]
      if (char === '{') depth++
      else if (char === '}') depth--
      index++
    }
    functions.set(match[1], source.slice(match.index + match[0].length, index - 1))
  }
  return functions
}

/** Splits a category script into per-test blocks keyed by the begin_test id. */
function testBlocks(source) {
  const lines = source.split('\n')
  const blocks = []
  let current = null
  for (const line of lines) {
    const started = line.match(/^\s*begin_test\s+"([^"]+)"/)
    if (started) {
      if (current) blocks.push(current)
      current = { id: started[1], body: [] }
      continue
    }
    if (/^\s*finish_category/.test(line)) {
      if (current) blocks.push(current)
      current = null
      continue
    }
    if (current) current.body.push(line)
  }
  if (current) blocks.push(current)
  return blocks.map((block) => ({ id: block.id, body: block.body.join('\n') }))
}

/** Function bodies shared by every category, merged into each script's own map. */
function sharedHelpers() {
  const merged = new Map()
  for (const file of SHARED_HELPER_SOURCES) {
    for (const [name, body] of localFunctions(stripComments(readFileSync(file, 'utf8')))) {
      if (!merged.has(name)) merged.set(name, body)
    }
  }
  return merged
}

const SHARED = sharedHelpers()

/** Category-local definitions win over shared ones of the same name. */
function functionsFor(source) {
  const merged = new Map(SHARED)
  for (const [name, body] of localFunctions(source)) merged.set(name, body)
  return merged
}

const CALLS_FAIL = /(^|[\s;&|(){}])fail\s+["'$]/
const CALLS_PASS = /(^|[\s;&|(){}])pass\s+["'$]/
const CHECKS_STATUS = /LAST_HTTP_STATUS|http_code|get_http_status|check_http_status/

/**
 * Reports whether `pattern` is reachable from a test block, following calls into
 * helper functions — category-local ones (cat-33's evaluate_* helpers) and shared
 * framework ones (post_extension, expect_http_status) alike. Without this the
 * analysis under-approximates and flags tests that do assert, just indirectly.
 */
function reaches(pattern, body, functions, seen = new Set()) {
  if (pattern.test(body)) return true
  for (const [name, functionBody] of functions) {
    if (seen.has(name)) continue
    const called = new RegExp(`(^|[\\s;&|(){}])${name}(\\s|$|;|\\)|")`)
    if (!called.test(body)) continue
    seen.add(name)
    if (reaches(pattern, functionBody, functions, seen)) return true
  }
  return false
}

describe('UAT assertion falsifiability', () => {
  test('every UAT test has a reachable failure path', () => {
    const unfalsifiable = []
    for (const file of CATEGORY_SCRIPTS) {
      const source = stripComments(readFileSync(file, 'utf8'))
      const functions = functionsFor(source)
      for (const block of testBlocks(source)) {
        // A block that can only skip is a precondition gate, not a false green:
        // it reports "skipped" rather than claiming the behaviour was verified.
        // The defect is a block that can report pass but never fail.
        if (!reaches(CALLS_PASS, block.body, functions)) continue
        if (!reaches(CALLS_FAIL, block.body, functions)) {
          unfalsifiable.push(`${file.replace('scripts/tests/', '')} ${block.id}`)
        }
      }
    }
    assert.deepEqual(
      unfalsifiable,
      [],
      `these UAT tests report pass on every branch and can never fail:\n  ${unfalsifiable.join('\n  ')}`
    )
  })

  test('tests that POST to /sync assert the HTTP status code', () => {
    // `jq .` succeeds on a 400/403/409 error body, so parsing the reply proves
    // nothing about whether the server accepted the payload.
    const unchecked = []
    for (const file of CATEGORY_SCRIPTS) {
      const source = stripComments(readFileSync(file, 'utf8'))
      const functions = functionsFor(source)
      for (const block of testBlocks(source)) {
        if (!/\/sync/.test(block.body)) continue
        if (!reaches(CHECKS_STATUS, block.body, functions)) {
          unchecked.push(`${file.replace('scripts/tests/', '')} ${block.id}`)
        }
      }
    }
    assert.deepEqual(
      unchecked,
      [],
      `these /sync tests never assert an HTTP status:\n  ${unchecked.join('\n  ')}`
    )
  })

  test('tool requests fit on one line so they reach the tool', () => {
    // send_mcp pipes the request through `echo` into a line-delimited JSON-RPC
    // reader, so a multi-line argument yields
    // {"code":-32700,"message":"Parse error: unexpected end of JSON input"}.
    // That envelope carries no .result.isError, so check_not_error reports
    // success and the test proceeds as though the call worked. Twenty tests
    // across five categories were green on requests that never reached a tool.
    const offenders = []
    for (const file of CATEGORY_SCRIPTS) {
      const lines = readFileSync(file, 'utf8').split('\n')
      lines.forEach((line, index) => {
        if (/^\s*#/.test(line)) return
        const opened = line.match(/(call_tool|send_mcp)\s+("[a-z_]+"\s+)?(['"])\{/)
        if (!opened) return
        const rest = line.slice(line.indexOf(opened[0]) + opened[0].length)
        if (rest.includes(`}${opened[3]}`)) return
        offenders.push(`${file.replace('scripts/tests/', '')}:${index + 1}`)
      })
    }
    assert.deepEqual(
      offenders,
      [],
      `these tool requests span multiple lines and will fail to parse:\n  ${offenders.join('\n  ')}`
    )
  })

  test('no pass message concedes that the behaviour was not verified', () => {
    // "Auto-detect query processed (output format TBD)" is a pass that admits
    // nothing was checked. Reporting green for unimplemented or unreached
    // behaviour is the failure mode this whole contract exists to prevent.
    const hollow = []
    for (const file of CATEGORY_SCRIPTS) {
      const lines = readFileSync(file, 'utf8').split('\n')
      lines.forEach((line, index) => {
        if (/^\s*#/.test(line)) return
        const message = line.match(/(?:^|[\s;&|(){}])pass\s+"([^"]*)"/)
        if (!message) return
        // Field names the test verified are quoted inside the message
        // ("...response with 'pending', 'completed', 'failed'"), and must not be
        // mistaken for a concession about the test's own outcome.
        const prose = message[1].replace(/'[^']*'/g, '')
        if (!/\b(TBD|pending|not yet|future enhancement|may vary|may differ|may limit|implementation may|planned)\b/i.test(prose))
          return
        hollow.push(`${file.replace('scripts/tests/', '')}:${index + 1}`)
      })
    }
    assert.deepEqual(
      hollow,
      [],
      `these pass messages concede the behaviour was never verified:\n  ${hollow.join('\n  ')}`
    )
  })

  test('no UAT test asserts only that a response parses as JSON', () => {
    // `jq . >/dev/null` as the sole assertion accepts every error envelope.
    const jsonOnly = []
    for (const file of CATEGORY_SCRIPTS) {
      const source = stripComments(readFileSync(file, 'utf8'))
      for (const block of testBlocks(source)) {
        if (!/\|\s*jq\s+\.\s*>\/dev\/null/.test(block.body)) continue
        jsonOnly.push(`${file.replace('scripts/tests/', '')} ${block.id}`)
      }
    }
    assert.deepEqual(
      jsonOnly,
      [],
      `these tests gate on bare JSON validity, which every error body satisfies:\n  ${jsonOnly.join('\n  ')}`
    )
  })
})
