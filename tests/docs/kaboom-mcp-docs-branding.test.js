// @ts-nocheck
/**
 * @fileoverview kaboom-mcp-docs-branding.test.js — Guards MCP integration docs branding.
 */

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

describe('kaboom mcp docs branding', () => {
  test('integration docs use Kaboom server identity and package names', () => {
    const files = [
      'docs/mcp-integration/index.md',
      'docs/mcp-integration/claude-code.md',
      'docs/mcp-integration/claude-desktop.md',
      'docs/mcp-integration/cursor.md',
      'docs/mcp-integration/windsurf.md',
      'docs/mcp-integration/zed.md',
      'docs/core/protocol/mcp-client-configs.md'
    ]

    for (const file of files) {
      const source = read(file)
      assert.match(source, /Kaboom|kaboom-browser-devtools|kaboom-agentic-browser|kaboom:\/\//)
      assert.doesNotMatch(
        source,
        /Gasoline|gasoline-browser-devtools|gasoline-mcp|gasoline:\/\//,
        `${file} still contains legacy MCP branding`
      )
    }
  })
})
