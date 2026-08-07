// check-go-test-targets.mjs — Verifies targeted workflow Go tests exist in the selected package.

import { spawnSync } from 'node:child_process'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

function unquote(token) {
  if ((token.startsWith("'") && token.endsWith("'")) || (token.startsWith('"') && token.endsWith('"'))) {
    return token.slice(1, -1)
  }
  return token
}

export function extractTargetedGoTests(source) {
  const targets = []
  for (const command of source.matchAll(/\bgo test\s+([^\r\n]+)/g)) {
    const tokens = command[1].match(/'[^']*'|"[^"]*"|\S+/g)?.map(unquote) ?? []
    const packagePath = tokens.find((token) => token.startsWith('./') && token !== './...')
    const runIndex = tokens.indexOf('-run')
    const runAssignment = tokens.find((token) => token.startsWith('-run='))
    const pattern = runIndex >= 0 ? tokens[runIndex + 1] : runAssignment?.slice('-run='.length)
    const tagsIndex = tokens.indexOf('-tags')
    const tagsAssignment = tokens.find((token) => token.startsWith('-tags='))
    const tags = tagsIndex >= 0 ? tokens[tagsIndex + 1] : (tagsAssignment?.slice('-tags='.length) ?? '')
    if (packagePath && pattern) targets.push({ packagePath, pattern, tags })
  }
  return targets
}

export function findMissingTargetedGoTests(repoRoot) {
  const workflowRoot = path.join(repoRoot, '.github', 'workflows')
  const missing = []
  const checked = new Map()
  for (const workflow of fs
    .readdirSync(workflowRoot)
    .filter((name) => /\.ya?ml$/.test(name))
    .sort()) {
    const source = fs.readFileSync(path.join(workflowRoot, workflow), 'utf8')
    for (const target of extractTargetedGoTests(source)) {
      const key = `${target.packagePath}\0${target.pattern}\0${target.tags}`
      let exists = checked.get(key)
      if (exists === undefined) {
        const tagArgs = target.tags ? [`-tags=${target.tags}`] : []
        const result = spawnSync('go', ['test', ...tagArgs, target.packagePath, '-list', target.pattern], {
          cwd: repoRoot,
          encoding: 'utf8',
          timeout: 120_000
        })
        exists = result.status === 0 && /^(?:Test|Benchmark|Fuzz)\S+/m.test(result.stdout)
        checked.set(key, exists)
      }
      if (!exists) missing.push(`${workflow}:${target.packagePath}:${target.pattern}`)
    }
  }
  return [...new Set(missing)].sort()
}

function main() {
  const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../..')
  const missing = findMissingTargetedGoTests(repoRoot)
  if (missing.length > 0) {
    process.stderr.write(`GitHub workflows select absent Go tests:\n${missing.join('\n')}\n`)
    process.exitCode = 1
    return
  }
  process.stdout.write('GitHub workflow targeted Go tests resolve\n')
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main()
