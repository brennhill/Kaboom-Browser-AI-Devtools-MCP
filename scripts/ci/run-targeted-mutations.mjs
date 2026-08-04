#!/usr/bin/env node
// run-targeted-mutations.mjs — Executes reproducible semantic mutants in an isolated worktree.
import { spawnSync } from 'node:child_process'
import { cpSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { basename, dirname, join, resolve } from 'node:path'

function option(name, fallback) {
  const index = process.argv.indexOf(name)
  return index === -1 ? fallback : process.argv[index + 1]
}

const root = resolve(option('--root', process.cwd()))
const configPath = resolve(root, option('--config', 'scripts/ci/mutation-cases.json'))
const outputPath = resolve(root, option('--output', 'artifacts/mutation/report.json'))
const config = JSON.parse(readFileSync(configPath, 'utf8'))

if (config.version !== 1 || !Array.isArray(config.cases) || config.cases.length === 0) {
  throw new Error('mutation config must declare version 1 and at least one case')
}

const workspaceParent = mkdtempSync(join(tmpdir(), 'kaboom-mutations-'))
const workspace = join(workspaceParent, 'repo')
const excluded = new Set(['.git', 'node_modules', 'artifacts'])
cpSync(root, workspace, {
  recursive: true,
  filter(source) {
    return !excluded.has(basename(source))
  }
})

const results = []
try {
  for (const packageName of new Set(config.cases.map((mutation) => mutation.package))) {
    const baseline = spawnSync('go', ['test', packageName, '-count=1'], {
      cwd: workspace,
      encoding: 'utf8',
      env: { ...process.env, KABOOM_TELEMETRY_DISABLED: '1' },
      timeout: config.timeout_ms ?? 120_000
    })
    if (baseline.status !== 0) {
      throw new Error(`baseline failed for ${packageName}: ${baseline.error?.code ?? baseline.status}`)
    }
  }
  for (const mutation of config.cases) {
    if (!mutation.id || !mutation.file || !mutation.package || !mutation.from || !mutation.to) {
      throw new Error('every mutation requires id, file, package, from, and to')
    }
    const sourcePath = join(workspace, mutation.file)
    const original = readFileSync(sourcePath, 'utf8')
    const occurrences = original.split(mutation.from).length - 1
    if (occurrences !== 1) {
      throw new Error(`${mutation.id}: expected one source match, found ${occurrences}`)
    }
    writeFileSync(sourcePath, original.replace(mutation.from, mutation.to))
    const startedAt = Date.now()
    const test = spawnSync('go', ['test', mutation.package, '-count=1'], {
      cwd: workspace,
      encoding: 'utf8',
      env: { ...process.env, KABOOM_TELEMETRY_DISABLED: '1' },
      timeout: config.timeout_ms ?? 120_000
    })
    writeFileSync(sourcePath, original)
    if (test.status === null && test.error?.code !== 'ETIMEDOUT') {
      throw new Error(`${mutation.id}: mutation test could not execute: ${test.error?.code ?? 'unknown'}`)
    }
    results.push({
      id: mutation.id,
      package: mutation.package,
      status: test.status === 0 ? 'survived' : 'killed',
      duration_ms: Date.now() - startedAt,
      timed_out: test.error?.code === 'ETIMEDOUT'
    })
  }
} finally {
  rmSync(workspaceParent, { recursive: true, force: true })
}

const killed = results.filter((result) => result.status === 'killed').length
const survived = results.length - killed
const score = Number(((killed / results.length) * 100).toFixed(1))
const report = {
  schema_version: 1,
  minimum_score: config.minimum_score,
  score,
  killed,
  survived,
  survivors: results.filter((result) => result.status === 'survived').map((result) => result.id),
  results
}
mkdirSync(dirname(outputPath), { recursive: true })
writeFileSync(outputPath, `${JSON.stringify(report, null, 2)}\n`, { encoding: 'utf8', flag: 'w' })
process.stdout.write(`Mutation score: ${score}% (${killed}/${results.length} killed)\n`)
if (score < config.minimum_score) {
  process.stderr.write(
    `Mutation score is below the ${config.minimum_score}% gate; survivors: ${report.survivors.join(', ')}\n`
  )
  process.exitCode = 1
}
