// @ts-nocheck
/**
 * @fileoverview kaboom-audit-workflow.test.js — Guards the Phase 1 audit command and bundled skill contract.
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

describe('kaboom audit workflow assets', () => {
  test('command and bundled audit skill expose the Phase 1 audit contract', () => {
    const command = read('plugin/kaboom-workflows/commands/audit.md')
    const auditSkill = read('npm/kaboom-agentic-browser/skills/audit/SKILL.md')
    const manifest = read('npm/kaboom-agentic-browser/skills/skills.json')

    assert.match(command, /^name:\s+kaboom\/audit/m)
    assert.match(command, /Functionality/)
    assert.match(command, /UX Polish/)
    assert.match(command, /Accessibility/)
    assert.match(command, /Performance/)
    assert.match(command, /Release Risk/)
    assert.match(command, /SEO/)
    assert.match(command, /Fast Wins/)
    assert.match(command, /Ship Blockers/)

    assert.match(auditSkill, /tracked site|current tracked page/i)
    assert.match(auditSkill, /Functionality/)
    assert.match(auditSkill, /UX Polish/)
    assert.match(auditSkill, /Accessibility/)
    assert.match(auditSkill, /Performance/)
    assert.match(auditSkill, /Release Risk/)
    assert.match(auditSkill, /SEO/)
    assert.match(auditSkill, /Fast Wins/)
    assert.match(auditSkill, /Ship Blockers/)

    assert.match(manifest, /"id": "audit"/)
    assert.doesNotMatch(manifest, /"id": "qa"/)
  })
})
