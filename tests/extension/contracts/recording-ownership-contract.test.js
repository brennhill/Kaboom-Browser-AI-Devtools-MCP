// @ts-nocheck
/**
 * @fileoverview recording-ownership-contract.test.js — Live screen recording
 * storage keys have exactly one authoritative writer (CLAUDE.md rule 18).
 * The background recording module (src/background/recording/) owns
 * kaboom_recording and kaboom_pending_recording; other contexts may only read
 * them. See docs/features/feature/tab-recording/index.md "Recording Ownership".
 */

import { test, describe } from 'node:test'
import assert from 'node:assert'
import ts from 'typescript'
import { relative } from 'node:path'
import { listTsFiles, parseSource, SRC_ROOT } from './source-contract-utils.js'

const OWNED_KEYS = ['RECORDING', 'PENDING_RECORDING']
const OWNER_DIR = 'background/recording/'
const WRITERS = new Set(['setLocal', 'setLocals', 'removeLocal', 'removeLocals'])

function findForeignWrites(file) {
  const sf = parseSource(file)
  const hits = []
  const visit = (node) => {
    if (ts.isCallExpression(node) && ts.isIdentifier(node.expression) && WRITERS.has(node.expression.text)) {
      const text = node.arguments.map((a) => a.getText(sf)).join(',')
      for (const key of OWNED_KEYS) {
        if (new RegExp(`StorageKey\\.${key}\\b`).test(text)) hits.push(`${node.expression.text}(StorageKey.${key})`)
      }
    }
    ts.forEachChild(node, visit)
  }
  visit(sf)
  return hits
}

describe('live screen recording storage ownership', () => {
  test('only src/background/recording/ writes kaboom_recording / kaboom_pending_recording', () => {
    const violations = []
    for (const file of listTsFiles(['background', 'popup', 'content', 'offscreen', 'lib'])) {
      const rel = relative(SRC_ROOT, file)
      if (rel.startsWith(OWNER_DIR)) continue
      for (const hit of findForeignWrites(file)) violations.push(`${rel}: ${hit}`)
    }
    assert.deepStrictEqual(violations, [], 'Route writes through the background recording owner')
  })
})
