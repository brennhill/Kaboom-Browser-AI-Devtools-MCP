// @ts-nocheck
/**
 * @fileoverview kaboom-skills-branding.test.js — Guards bundled skill branding and smoke-test copy.
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

describe('kaboom bundled skill branding', () => {
  test('skill docs, skill manifest, and smoke test use Kaboom branding', () => {
    const files = [
      'npm/kaboom-agentic-browser/skills/api-validation/SKILL.md',
      'npm/kaboom-agentic-browser/skills/audit/SKILL.md',
      'npm/kaboom-agentic-browser/skills/automate/SKILL.md',
      'npm/kaboom-agentic-browser/skills/config-doctor/SKILL.md',
      'npm/kaboom-agentic-browser/skills/debug-triage/SKILL.md',
      'npm/kaboom-agentic-browser/skills/debug/SKILL.md',
      'npm/kaboom-agentic-browser/skills/demo/SKILL.md',
      'npm/kaboom-agentic-browser/skills/performance/SKILL.md',
      'npm/kaboom-agentic-browser/skills/regression-test/SKILL.md',
      'npm/kaboom-agentic-browser/skills/release-readiness/SKILL.md',
      'npm/kaboom-agentic-browser/skills/reliability/SKILL.md',
      'npm/kaboom-agentic-browser/skills/security-redaction/SKILL.md',
      'npm/kaboom-agentic-browser/skills/site-audit/SKILL.md',
      'npm/kaboom-agentic-browser/skills/test-coverage/SKILL.md',
      'npm/kaboom-agentic-browser/skills/ux-audit/SKILL.md',
      'npm/kaboom-agentic-browser/skills/skills.json',
      'scripts/uat/runners/smoke-test.sh'
    ]

    for (const file of files) {
      const source = read(file)
      assert.match(source, /Kaboom|kaboom/i)
      assert.doesNotMatch(source, /Gasoline|STRUM|getstrum|cookwithgasoline|\.gasoline/)
    }
  })
})
