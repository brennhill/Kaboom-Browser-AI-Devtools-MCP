// check-destructive-git.mjs — Any script that discards working-tree changes must check the tree first.
//
// PURPOSE: `git checkout -- <file>` and `git restore` silently destroy
// uncommitted work. A mutation harness that reverts each mutated file this way
// is correct on a clean tree and catastrophic on a dirty one — it deleted an
// afternoon of uncommitted work here twice, because nothing in the script
// distinguished "restore my mutation" from "delete the author's edits".
//
// CONTRACT: a script that runs a discarding git command must also refuse to run
// on a dirty tree. The guard is cheap, and the alternative is unrecoverable:
// discarded working-tree changes are not in the reflog.

import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join, relative } from 'node:path'
import { fileURLToPath } from 'node:url'

const REPO_ROOT = fileURLToPath(new URL('../../..', import.meta.url))
const SCAN_ROOT = join(REPO_ROOT, 'scripts')

// Commands that overwrite or delete working-tree state without a recovery path.
const DESTRUCTIVE = [
  { pattern: /\bgit\s+checkout\s+(--\s|-f\b|--force\b)/, name: 'git checkout --' },
  { pattern: /\bgit\s+restore\b/, name: 'git restore' },
  { pattern: /\bgit\s+clean\s+.*-[a-z]*f/, name: 'git clean -f' },
  { pattern: /\bgit\s+reset\s+--hard\b/, name: 'git reset --hard' },
  { pattern: /\bgit\s+stash\s+(drop|clear)\b/, name: 'git stash drop/clear' }
]

// Any of these reads as "this script checks the tree before discarding".
const GUARDS = [/git\s+status\s+--porcelain/, /git\s+diff\s+--quiet/, /require_clean_tree/]

export function shellFiles(root) {
  const found = []
  const walk = (dir) => {
    for (const entry of readdirSync(dir)) {
      if (entry.startsWith('.') || entry === 'node_modules') continue
      const path = join(dir, entry)
      if (statSync(path).isDirectory()) {
        walk(path)
      } else if (/\.(sh|mjs|cjs|js)$/.test(entry)) {
        found.push(path)
      }
    }
  }
  walk(root)
  return found.sort()
}

function isComment(line) {
  return /^\s*(#|\/\/)/.test(line)
}

// destructiveUses returns the discarding commands a script actually runs.
export function destructiveUses(source) {
  const uses = []
  source.split('\n').forEach((line, index) => {
    if (isComment(line)) return
    for (const { pattern, name } of DESTRUCTIVE) {
      if (pattern.test(line)) uses.push({ line: index + 1, name })
    }
  })
  return uses
}

// hasCleanTreeGuard reports whether the script checks the working tree. The
// check is file-scoped rather than order-sensitive: a guard in a sourced setup
// block or an early-exit function still protects the later command, and
// demanding textual precedence would reward moving the call rather than adding
// the guard.
export function hasCleanTreeGuard(source) {
  return source
    .split('\n')
    .filter((line) => !isComment(line))
    .some((line) => GUARDS.some((guard) => guard.test(line)))
}

export function evaluate(files) {
  const violations = []
  for (const { path, source } of files) {
    const uses = destructiveUses(source)
    if (uses.length === 0 || hasCleanTreeGuard(source)) continue
    const first = uses[0]
    violations.push(
      `${path}:${first.line} runs \`${first.name}\` with no working-tree check. Discarded uncommitted changes are not recoverable — refuse to run when \`git status --porcelain\` is non-empty.`
    )
  }
  return violations
}

function main() {
  const files = shellFiles(SCAN_ROOT).map((path) => ({
    path: relative(REPO_ROOT, path).split('\\').join('/'),
    source: readFileSync(path, 'utf8')
  }))
  const violations = evaluate(files)
  if (violations.length > 0) {
    console.error(`FAIL: ${violations.length} unguarded destructive git command(s)`)
    for (const violation of violations) console.error(`  - ${violation}`)
    process.exit(1)
  }
  const guarded = files.filter(({ source }) => destructiveUses(source).length > 0).length
  console.log(`OK: ${guarded} script(s) discard working-tree state, all guarded by a clean-tree check.`)
}

if (process.argv[1] && process.argv[1].endsWith('check-destructive-git.mjs')) {
  main()
}
