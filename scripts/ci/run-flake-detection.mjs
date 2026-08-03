#!/usr/bin/env node
// run-flake-detection.mjs — Scheduled, replayable order/load-sensitivity campaigns.

import { readdirSync, writeFileSync } from 'node:fs'
import { spawnSync } from 'node:child_process'
import { relative, resolve } from 'node:path'
import { pathToFileURL } from 'node:url'

const schemaVersion = '1'
const defaultReproductions = 2
const maxRuns = 10
const outputLimit = 32 * 1024

export function seededShuffle(values, seed) {
  const shuffled = [...values]
  const random = mulberry32(seed >>> 0)
  for (let index = shuffled.length - 1; index > 0; index -= 1) {
    const selected = Math.floor(random() * (index + 1))
    ;[shuffled[index], shuffled[selected]] = [shuffled[selected], shuffled[index]]
  }
  return shuffled
}

export function buildPlan({ seed, runs, concurrency, goPackages, jsFiles }) {
  if (!Number.isInteger(seed) || !Number.isInteger(runs) || runs < 1 || runs > maxRuns) {
    throw new Error(`seed must be an integer and runs must be between 1 and ${maxRuns}`)
  }
  if (!Number.isInteger(concurrency) || concurrency < 1 || concurrency > 32) {
    throw new Error('concurrency must be between 1 and 32')
  }
  const plannedRuns = []
  for (let runIndex = 0; runIndex < runs; runIndex += 1) {
    const runSeed = (seed + runIndex) >>> 0
    const packageOrder = seededShuffle(goPackages, runSeed)
    const fileOrder = seededShuffle(jsFiles, runSeed ^ 0x9e3779b9)
    plannedRuns.push({
      index: runIndex + 1,
      seed: runSeed,
      commands: [
        {
          id: `go-race-${runIndex + 1}`,
          executable: 'go',
          args: [
            'test',
            '-race',
            `-shuffle=${runSeed}`,
            `-p=${concurrency}`,
            '-count=1',
            '-timeout=25m',
            ...packageOrder
          ],
          test_order: packageOrder,
          environment: { GOMAXPROCS: String(concurrency), KABOOM_TELEMETRY_DISABLED: '1' }
        },
        {
          id: `javascript-${runIndex + 1}`,
          executable: process.execPath,
          args: ['--test', `--test-concurrency=${concurrency}`, ...fileOrder],
          test_order: fileOrder,
          environment: { KABOOM_TELEMETRY_DISABLED: '1' }
        }
      ]
    })
  }
  return {
    schema_version: schemaVersion,
    seed,
    resource_pressure: {
      gomaxprocs: concurrency,
      go_package_parallelism: concurrency,
      node_test_concurrency: concurrency
    },
    replay_command: `KABOOM_FLAKE_SEED=${seed} node scripts/ci/run-flake-detection.mjs`,
    runs: plannedRuns
  }
}

export async function executePlan(plan, runCommand, reproductionCount = defaultReproductions) {
  const startedAt = new Date().toISOString()
  const commandResults = []
  const originalFailures = []
  const reproductions = []
  for (const run of plan.runs) {
    for (const command of run.commands) {
      const result = sanitizeResult(await runCommand(command))
      const recorded = { run_index: run.index, seed: run.seed, command, ...result }
      commandResults.push(recorded)
      if (result.exit_code === 0) continue

      originalFailures.push(structuredClone(recorded))
      const attempts = []
      for (let retry = 0; retry < reproductionCount; retry += 1) {
        attempts.push(sanitizeResult(await runCommand(command)))
      }
      const failedRetries = attempts.filter((attempt) => attempt.exit_code !== 0).length
      const classification =
        failedRetries === attempts.length ? 'reproduced' : failedRetries === 0 ? 'flaky' : 'intermittent'
      reproductions.push({ command_id: command.id, run_index: run.index, classification, attempts })
    }
  }
  return {
    ...plan,
    started_at: startedAt,
    completed_at: new Date().toISOString(),
    exit_code: originalFailures.length > 0 ? 1 : 0,
    command_results: commandResults,
    original_failures: originalFailures,
    reproductions
  }
}

function sanitizeResult(result) {
  return {
    exit_code: Number.isInteger(result.exit_code) ? result.exit_code : 1,
    duration_ms: Math.max(0, Number(result.duration_ms) || 0),
    stdout: redactAndClamp(result.stdout),
    stderr: redactAndClamp(result.stderr)
  }
}

function redactAndClamp(value) {
  const redacted = String(value ?? '')
    .replace(/(authorization\s*[:=]\s*bearer\s+)[^\s"']+/gi, '$1[REDACTED]')
    .replace(/((?:api[_-]?key|token|password|secret)\s*[:=]\s*)[^\s,"']+/gi, '$1[REDACTED]')
  if (redacted.length <= outputLimit) return redacted
  return `${redacted.slice(0, outputLimit)}\n[TRUNCATED]`
}

function mulberry32(initial) {
  let state = initial
  return () => {
    state = (state + 0x6d2b79f5) | 0
    let value = Math.imul(state ^ (state >>> 15), 1 | state)
    value = (value + Math.imul(value ^ (value >>> 7), 61 | value)) ^ value
    return ((value ^ (value >>> 14)) >>> 0) / 4294967296
  }
}

function discoverJSFiles(root) {
  const matches = []
  const visit = (directory) => {
    for (const entry of readdirSync(directory, { withFileTypes: true })) {
      if (entry.name === 'node_modules' || entry.name === '.git') continue
      const path = resolve(directory, entry.name)
      if (entry.isDirectory()) {
        visit(path)
      } else if (/\.test\.(?:js|mjs|cjs)$/.test(entry.name)) {
        matches.push(relative(root, path))
      }
    }
  }
  for (const directory of ['tests', 'scripts']) visit(resolve(root, directory))
  return matches.sort()
}

function discoverGoPackages(root) {
  const result = spawnSync('go', ['list', './cmd/browser-agent/...', './internal/...'], { cwd: root, encoding: 'utf8' })
  if (result.status !== 0) throw new Error(`go list failed: ${result.stderr}`)
  const modulePrefix = 'github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/'
  return result.stdout
    .trim()
    .split('\n')
    .filter(Boolean)
    .map((name) => `./${name.replace(modulePrefix, '')}`)
}

function runProcess(root, command) {
  const started = performance.now()
  const result = spawnSync(command.executable, command.args, {
    cwd: root,
    encoding: 'utf8',
    env: { ...process.env, ...command.environment },
    maxBuffer: 16 * 1024 * 1024,
    timeout: 30 * 60 * 1000
  })
  return {
    exit_code: result.status ?? 1,
    stdout: result.stdout,
    stderr: result.error ? `${result.stderr ?? ''}\n${result.error.message}` : result.stderr,
    duration_ms: Math.round(performance.now() - started)
  }
}

async function main() {
  const root = process.cwd()
  const seed = Number.parseInt(process.env.KABOOM_FLAKE_SEED ?? String(Date.now() >>> 0), 10)
  const runs = Number.parseInt(process.env.KABOOM_FLAKE_RUNS ?? '3', 10)
  const concurrency = Number.parseInt(process.env.KABOOM_FLAKE_CONCURRENCY ?? '2', 10)
  const reproductionCount = Number.parseInt(process.env.KABOOM_FLAKE_REPRODUCTIONS ?? String(defaultReproductions), 10)
  const outputPath = resolve(root, process.env.KABOOM_FLAKE_EVIDENCE ?? 'flake-evidence.json')
  const plan = buildPlan({
    seed,
    runs,
    concurrency,
    goPackages: discoverGoPackages(root),
    jsFiles: discoverJSFiles(root)
  })
  const evidence = await executePlan(plan, (command) => runProcess(root, command), reproductionCount)
  evidence.local_diagnostics = {
    platform: process.platform,
    architecture: process.arch,
    node_version: process.version,
    ci: process.env.CI === 'true'
  }
  writeFileSync(outputPath, `${JSON.stringify(evidence, null, 2)}\n`, { mode: 0o600 })
  process.stderr.write(
    `Flake campaign seed ${seed}: ${evidence.original_failures.length} original failure(s). Replay: ${plan.replay_command}\n`
  )
  process.exitCode = evidence.exit_code
}

const invokedPath = process.argv[1] ? pathToFileURL(resolve(process.argv[1])).href : ''
if (import.meta.url === invokedPath) {
  try {
    await main()
  } catch (error) {
    const outputPath = resolve(process.cwd(), process.env.KABOOM_FLAKE_EVIDENCE ?? 'flake-evidence.json')
    const failure = {
      schema_version: schemaVersion,
      exit_code: 1,
      fatal_error: redactAndClamp(error instanceof Error ? error.message : String(error)),
      replay_command: `KABOOM_FLAKE_SEED=${process.env.KABOOM_FLAKE_SEED ?? '<seed>'} node scripts/ci/run-flake-detection.mjs`,
      completed_at: new Date().toISOString()
    }
    writeFileSync(outputPath, `${JSON.stringify(failure, null, 2)}\n`, { mode: 0o600 })
    process.stderr.write(`Flake campaign could not start: ${failure.fatal_error}\n`)
    process.exitCode = 1
  }
}
