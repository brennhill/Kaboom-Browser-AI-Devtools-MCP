// check-local-paths.mjs — Verifies first-party file paths referenced by GitHub workflows.

import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const localPathPattern =
  /(?:^|[\s"'=(])((?:\.\/)?(?:scripts|npm|server|tests|\.github)\/[A-Za-z0-9_./-]+\.(?:sh|js|mjs|cjs|py|yml|yaml))(?=$|[\s"')\\])/gm

export function extractLocalPaths(source) {
  return [...source.matchAll(localPathPattern)].map((match) => match[1].replace(/^\.\//, ''))
}

export function findMissingWorkflowPaths(repoRoot) {
  const workflowRoot = path.join(repoRoot, '.github/workflows')
  const workflows = fs
    .readdirSync(workflowRoot)
    .filter((name) => name.endsWith('.yml') || name.endsWith('.yaml'))
    .sort()
  const missing = []
  for (const workflow of workflows) {
    const source = fs.readFileSync(path.join(workflowRoot, workflow), 'utf8')
    for (const relativePath of extractLocalPaths(source)) {
      if (!fs.existsSync(path.join(repoRoot, relativePath))) missing.push(`${workflow}:${relativePath}`)
    }
  }
  return [...new Set(missing)].sort()
}

function main() {
  const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../..')
  const missing = findMissingWorkflowPaths(repoRoot)
  if (missing.length > 0) {
    process.stderr.write(`GitHub workflows reference missing first-party paths:\n${missing.join('\n')}\n`)
    process.exitCode = 1
    return
  }
  process.stdout.write('GitHub workflow local paths resolve\n')
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main()
