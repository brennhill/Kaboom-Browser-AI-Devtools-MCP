// @ts-nocheck
/**
 * @fileoverview kaboom-root-docs-branding.test.js — Guards root install and troubleshooting docs branding.
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

describe('kaboom root docs branding', () => {
  test('root install and troubleshooting docs use Kaboom naming', () => {
    const files = [
      'docs/README.md',
      'docs/setup/getting-started.md',
      'docs/setup/mcp-install-guide.md',
      'docs/setup/agent-install-guide.md',
      'docs/setup/troubleshooting.md'
    ]

    for (const file of files) {
      const source = read(file)
      assert.match(source, /Kaboom|KaBOOM|gokaboom|kaboom/i)
      assert.doesNotMatch(
        source,
        /Gasoline|STRUM|getstrum|cookwithgasoline|gasoline-mcp|gasoline-agentic-browser|gasoline-agentic-browser-devtools-mcp|gasoline-browser-devtools|\.gasoline|GasolineAgenticDevtoolExtension/
      )
    }
  })
})
