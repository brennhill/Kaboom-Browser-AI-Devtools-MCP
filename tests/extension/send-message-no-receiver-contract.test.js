// @ts-nocheck
/**
 * @fileoverview send-message-no-receiver-contract.test.js
 *
 * "Never breaks again" guard for #status-update-noise.
 *
 * chrome.runtime.sendMessage / chrome.tabs.sendMessage reject with
 * "Could not establish connection. Receiving end does not exist." whenever the
 * receiver (a closed popup, or a tab with no content script) is absent. That is
 * the expected steady state — logging it as console.error/warn spams the console.
 *
 * This source-contract test statically scans every background source file and
 * fails if a `.sendMessage(...)` rejection handler logs via console.error/warn
 * WITHOUT first guarding the benign case through `isNoReceiverError`. Silent
 * catches (`.catch(() => {})`) and guarded catches both pass.
 */

import { test, describe } from 'node:test'
import assert from 'node:assert'
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join } from 'node:path'

const BACKGROUND_ROOT = 'src/background'

/** Recursively collect .ts source files (excluding tests). */
function collectSourceFiles(dir) {
  const out = []
  for (const entry of readdirSync(dir)) {
    const full = join(dir, entry)
    const st = statSync(full)
    if (st.isDirectory()) {
      if (entry === '__tests__') continue
      out.push(...collectSourceFiles(full))
    } else if (entry.endsWith('.ts') && !entry.endsWith('.test.ts')) {
      out.push(full)
    }
  }
  return out
}

/** Walk from the index of a `(` to its matching `)`, returning the inner body. */
function extractBalanced(text, openParenIndex) {
  let depth = 0
  for (let i = openParenIndex; i < text.length; i++) {
    const ch = text[i]
    if (ch === '(') depth++
    else if (ch === ')') {
      depth--
      if (depth === 0) return text.slice(openParenIndex + 1, i)
    }
  }
  return text.slice(openParenIndex + 1)
}

/**
 * Find every `.catch(` handler body in `text` whose preceding context contains a
 * `.sendMessage(` call (i.e. it is the rejection handler for a message send).
 */
function findSendMessageCatchBodies(text) {
  const bodies = []
  const catchToken = '.catch('
  let searchFrom = 0
  while (true) {
    const catchIdx = text.indexOf(catchToken, searchFrom)
    if (catchIdx === -1) break
    searchFrom = catchIdx + catchToken.length
    // Look back a bounded window for the message send that this catch handles.
    const preceding = text.slice(Math.max(0, catchIdx - 600), catchIdx)
    if (!preceding.includes('.sendMessage(')) continue
    const body = extractBalanced(text, catchIdx + catchToken.length - 1)
    bodies.push(body)
  }
  return bodies
}

describe('sendMessage rejection handlers must guard the benign no-receiver case', () => {
  const files = collectSourceFiles(BACKGROUND_ROOT)

  test('at least one background file was scanned', () => {
    assert.ok(files.length > 0, 'expected to find background source files')
  })

  test('no sendMessage .catch logs to console.error/warn without an isNoReceiverError guard', () => {
    const violations = []
    for (const file of files) {
      const text = readFileSync(file, 'utf8')
      if (!text.includes('.sendMessage(')) continue
      for (const body of findSendMessageCatchBodies(text)) {
        const logsError = /console\.(error|warn)\s*\(/.test(body)
        const hasGuard = body.includes('isNoReceiverError')
        if (logsError && !hasGuard) {
          violations.push(`${file}: a sendMessage .catch logs to console without an isNoReceiverError guard`)
        }
      }
    }
    assert.deepStrictEqual(
      violations,
      [],
      `Unguarded sendMessage rejection logging found:\n${violations.join('\n')}\n\n` +
        'Guard the benign closed-popup / no-content-script case with isNoReceiverError ' +
        '(from src/lib/error-utils.ts) before logging, or swallow it silently.'
    )
  })
})
