// check-npm-audit.test.mjs — Regression tests for the bounded build-tool vulnerability policy.
import assert from 'node:assert/strict'
import { mkdtempSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { spawnSync } from 'node:child_process'
import test from 'node:test'

const cleanAudit = { metadata: { vulnerabilities: { high: 0, critical: 0 } }, vulnerabilities: {} }

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
    exceptions: [
      { package: 'builder', severity: 'high', advisories: ['42'], expires: '2026-08-31', issue: 'kaboom-1234' }
    ]
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
        { package: 'builder', severity: 'high', advisories: ['42'], expires: '2026-08-02', issue: 'kaboom-1234' }
      ]
    }).status,
    0
  )
  assert.notEqual(
    run({
      audit: highAudit,
      exceptions: [{ package: 'builder', severity: 'high', advisories: ['42'], expires: '2026-08-31', issue: '' }]
    }).status,
    0
  )
  assert.notEqual(
    run({
      audit: highAudit,
      exceptions: [
        { package: 'builder', severity: 'high', advisories: ['different'], expires: '2026-08-31', issue: 'kaboom-1234' }
      ]
    }).status,
    0
  )
})
