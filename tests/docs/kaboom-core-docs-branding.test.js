// @ts-nocheck

import { describe, test } from 'node:test'
import assert from 'node:assert'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const TEST_DIR = path.dirname(fileURLToPath(import.meta.url))
const REPO_ROOT = path.resolve(TEST_DIR, '../..')

function read(relativePath) {
  return fs.readFileSync(path.join(REPO_ROOT, relativePath), 'utf8')
}

describe('kaboom core docs branding', () => {
  test('core architecture and UAT docs use Kaboom naming', () => {
    const files = [
      'docs/core/protocol/mcp-correctness.md',
      'docs/core/architecture/server-architecture.md',
      'docs/core/product/version-checking.md',
      'docs/core/quality/comprehensive-uat-plan.md'
    ]

    for (const file of files) {
      const source = read(file)
      assert.match(source, /Kaboom|KaBOOM|kaboom/i)
      assert.doesNotMatch(
        source,
        /Gasoline|STRUM|gasoline-mcp|X-Gasoline|gasoline:\/\/|cookwithgasoline|getstrum/
      )
    }
  })
})
