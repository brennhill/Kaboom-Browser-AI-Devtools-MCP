#!/usr/bin/env node
// check-npm-audit.mjs — Enforce zero runtime vulnerabilities and bounded build-tool exceptions.
import { readFileSync } from 'node:fs'
import { spawnSync } from 'node:child_process'

function option(name) {
  const index = process.argv.indexOf(name)
  return index >= 0 ? process.argv[index + 1] : undefined
}

function parseReport(raw, label) {
  try {
    return JSON.parse(raw)
  } catch (error) {
    throw new Error(
      `${label} did not produce valid npm audit JSON: ${error instanceof Error ? error.message : String(error)}`,
      { cause: error }
    )
  }
}

function runAudit(args, label) {
  const result = spawnSync('npm', ['audit', '--json', ...args], { encoding: 'utf8' })
  if (result.error) throw new Error(`${label} could not run: ${result.error.message}`)
  return parseReport(result.stdout, label)
}

function loadReport(path, auditArgs, label) {
  return path ? parseReport(readFileSync(path, 'utf8'), label) : runAudit(auditArgs, label)
}

function highRisk(report) {
  return Object.entries(report.vulnerabilities ?? {}).filter(([, vulnerability]) =>
    ['high', 'critical'].includes(vulnerability.severity)
  )
}

function advisoryFingerprint(vulnerability) {
  return (vulnerability.via ?? []).map((via) => String(typeof via === 'string' ? via : via.source)).sort()
}

function sameValues(left, right) {
  return left.length === right.length && left.every((value, index) => value === right[index])
}

const today = option('--today') ?? new Date().toISOString().slice(0, 10)
const policyPath = option('--policy') ?? 'scripts/security/npm-audit-policy.json'
const production = loadReport(
  option('--production-audit-json'),
  ['--omit=dev', '--audit-level=high'],
  'production dependency audit'
)
const complete = loadReport(option('--audit-json'), ['--audit-level=high'], 'complete dependency audit')
const policy = parseReport(readFileSync(policyPath, 'utf8'), 'npm audit policy')
const failures = []

for (const [name, vulnerability] of highRisk(production)) {
  failures.push(`production dependency ${name} has ${vulnerability.severity} severity`)
}

const activePackages = new Set()
for (const [name, vulnerability] of highRisk(complete)) {
  activePackages.add(name)
  const exception = (policy.exceptions ?? []).find((candidate) => candidate.package === name)
  if (!exception) {
    failures.push(`new ${vulnerability.severity} build-tool vulnerability: ${name}`)
    continue
  }
  if (!/^kaboom-[a-z0-9]+$/.test(exception.issue ?? '')) failures.push(`${name} exception has no Beads issue`)
  if (typeof exception.owner !== 'string' || exception.owner.trim() === '') {
    failures.push(`${name} exception has no owner`)
  }
  if (exception.scope !== 'build_only') failures.push(`${name} exception scope must be build_only`)
  if (typeof exception.rationale !== 'string' || exception.rationale.trim().length < 20) {
    failures.push(`${name} exception has no meaningful rationale`)
  }
  if (!/^\d{4}-\d{2}-\d{2}$/.test(exception.expires ?? '') || exception.expires < today) {
    failures.push(`${name} exception expired on ${exception.expires ?? 'an invalid date'}`)
  }
  if (exception.severity !== vulnerability.severity) {
    failures.push(`${name} severity changed from ${exception.severity} to ${vulnerability.severity}`)
  }
  const expectedAdvisories = [...(exception.advisories ?? [])].map(String).sort()
  if (!sameValues(expectedAdvisories, advisoryFingerprint(vulnerability))) {
    failures.push(`${name} advisory set changed; review and update the bounded exception`)
  }
}

for (const exception of policy.exceptions ?? []) {
  if (!activePackages.has(exception.package))
    failures.push(`${exception.package} exception is stale and must be removed`)
}

if (failures.length > 0) {
  process.stderr.write(`npm dependency audit policy failed:\n${failures.map((failure) => `- ${failure}`).join('\n')}\n`)
  process.exit(1)
}

process.stdout.write(`npm dependency audit policy passed (${activePackages.size} bounded build-tool exceptions)\n`)
