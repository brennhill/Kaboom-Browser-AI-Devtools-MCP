// check-npm-audit.test.mjs — Regression tests for the bounded build-tool vulnerability policy.
import assert from 'node:assert/strict'
import { mkdtempSync, readFileSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { spawnSync } from 'node:child_process'
import test from 'node:test'

const cleanAudit = { metadata: { vulnerabilities: { high: 0, critical: 0 } }, vulnerabilities: {} }
const validException = {
  package: 'builder',
  severity: 'high',
  advisories: ['42'],
  expires: '2026-08-31',
  issue: 'kaboom-1234',
  owner: 'security-team',
  scope: 'build_only',
  rationale: 'The package is used only while building static documentation and never ships in either runtime.'
}

function run({ audit = cleanAudit, productionAudit = cleanAudit, exceptions = [], today = '2026-08-03' }) {
  const directory = mkdtempSync(join(tmpdir(), 'kaboom-npm-audit-'))
  const auditPath = join(directory, 'audit.json')
  const productionPath = join(directory, 'production.json')
  const policyPath = join(directory, 'policy.json')
  writeFileSync(auditPath, JSON.stringify(audit))
  writeFileSync(productionPath, JSON.stringify(productionAudit))
  writeFileSync(policyPath, JSON.stringify({ exceptions }))
  return spawnSync(
    'node',
    [
      'scripts/security/check-npm-audit.mjs',
      '--audit-json',
      auditPath,
      '--production-audit-json',
      productionPath,
      '--policy',
      policyPath,
      '--today',
      today
    ],
    { cwd: process.cwd(), encoding: 'utf8' }
  )
}

test('accepts only named, unexpired, issue-linked build-tool exceptions', () => {
  const result = run({
    audit: {
      metadata: { vulnerabilities: { high: 1, critical: 0 } },
      vulnerabilities: { builder: { severity: 'high', isDirect: false, via: [{ source: 42 }] } }
    },
    exceptions: [validException]
  })
  assert.equal(result.status, 0, result.stderr)
})

test('rejects runtime, new, expired, and untracked high-risk vulnerabilities', () => {
  const highAudit = {
    metadata: { vulnerabilities: { high: 1, critical: 0 } },
    vulnerabilities: { builder: { severity: 'high', isDirect: false, via: [{ source: 42 }] } }
  }
  assert.notEqual(run({ productionAudit: highAudit }).status, 0)
  assert.notEqual(run({ audit: highAudit }).status, 0)
  assert.notEqual(
    run({
      audit: highAudit,
      exceptions: [
        { ...validException, expires: '2026-08-02' }
      ]
    }).status,
    0
  )
  assert.notEqual(
    run({
      audit: highAudit,
      exceptions: [{ ...validException, issue: '' }]
    }).status,
    0
  )
  assert.notEqual(
    run({
      audit: highAudit,
      exceptions: [
        { ...validException, advisories: ['different'] }
      ]
    }).status,
    0
  )
})

test('rejects exceptions without an owner, rationale, or build-only scope', () => {
  const highAudit = {
    metadata: { vulnerabilities: { high: 1, critical: 0 } },
    vulnerabilities: { builder: { severity: 'high', isDirect: false, via: [{ source: 42 }] } }
  }
  for (const invalid of [
    { ...validException, owner: '' },
    { ...validException, rationale: '' },
    { ...validException, scope: '' },
    { ...validException, scope: 'production' }
  ]) {
    assert.notEqual(run({ audit: highAudit, exceptions: [invalid] }).status, 0)
  }
})

test('scheduled CI invokes the canonical security gate', () => {
  const workflow = readFileSync('.github/workflows/ci.yml', 'utf8')
  assert.match(workflow, /schedule:\s*\n\s*- cron:/)
  assert.match(workflow, /name: Canonical security gate\s*\n\s*run: make security-check/)
})
