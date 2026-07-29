#!/usr/bin/env node
// version-sync.mjs — Atomically synchronize canonical release-version targets.
// Docs: docs/core/release.md
import { existsSync, readFileSync, renameSync, rmSync, writeFileSync } from 'node:fs'
import { dirname, join, resolve } from 'node:path'

const strictSemver = /^\d+\.\d+\.\d+$/
const jsonTargets = [
  'package.json',
  'extension/manifest.json',
  'extension/package.json',
  'server/package.json',
  'npm/kaboom-agentic-browser/package.json',
  'npm/darwin-arm64/package.json',
  'npm/darwin-x64/package.json',
  'npm/linux-arm64/package.json',
  'npm/linux-x64/package.json',
  'npm/win32-x64/package.json',
  'packages/kaboom-ci/package.json',
  'packages/kaboom-playwright/package.json'
]
const allTargets = [
  'VERSION',
  ...jsonTargets,
  'package-lock.json',
  'cmd/browser-agent/main.go',
  'cmd/hooks/main.go',
  'README.md',
  'claude_skill/kaboom/SKILL.md'
]

function parseArgs(argv) {
  const args = [...argv]
  let root = resolve(new URL('../../..', import.meta.url).pathname)
  const rootIndex = args.indexOf('--root')
  if (rootIndex !== -1) {
    const value = args[rootIndex + 1]
    if (!value) throw new Error('--root requires a path')
    root = resolve(value)
    args.splice(rootIndex, 2)
  }
  if (args.length !== 1) {
    throw new Error('usage: version-sync.mjs [--root PATH] <X.Y.Z|--sync|--check>')
  }
  return { root, action: args[0] }
}

function parseVersion(value, label) {
  const version = value.trim()
  if (!strictSemver.test(version)) {
    throw new Error(`${label} must be strict semver X.Y.Z; got ${JSON.stringify(version)}`)
  }
  return version
}

function compareVersions(left, right) {
  const a = left.split('.').map(Number)
  const b = right.split('.').map(Number)
  for (let i = 0; i < 3; i += 1) {
    if (a[i] !== b[i]) return a[i] - b[i]
  }
  return 0
}

function readTargets(root) {
  const missing = allTargets.filter((target) => !existsSync(join(root, target)))
  if (missing.length > 0) throw new Error(`missing canonical version target(s): ${missing.join(', ')}`)
  return new Map(allTargets.map((target) => [target, readFileSync(join(root, target), 'utf8')]))
}

function formatJSON(value) {
  return `${JSON.stringify(value, null, 2)}\n`
}

function updateJSON(relativePath, content, version) {
  const value = JSON.parse(content)
  value.version = version
  if (relativePath === 'npm/kaboom-agentic-browser/package.json') {
    for (const dependency of Object.keys(value.optionalDependencies ?? {})) {
      if (dependency.startsWith('@brennhill/kaboom-agentic-browser-')) {
        value.optionalDependencies[dependency] = version
      }
    }
  }
  if (relativePath === 'packages/kaboom-playwright/package.json') {
    if (!value.dependencies?.['@anthropic/kaboom-ci']) {
      throw new Error(`${relativePath} is missing @anthropic/kaboom-ci`)
    }
    value.dependencies['@anthropic/kaboom-ci'] = version
  }
  return formatJSON(value)
}

function replaceOne(content, pattern, replacement, relativePath, label) {
  const matches = content.match(pattern)
  if (!matches || matches.length !== 1) {
    throw new Error(`${relativePath} must contain exactly one ${label}; found ${matches?.length ?? 0}`)
  }
  return content.replace(pattern, replacement)
}

function desiredTargets(originals, version) {
  const desired = new Map(originals)
  desired.set('VERSION', `${version}\n`)
  for (const target of jsonTargets) {
    desired.set(target, updateJSON(target, originals.get(target), version))
  }

  const lock = JSON.parse(originals.get('package-lock.json'))
  if (!lock.packages?.['']) throw new Error('package-lock.json is missing packages[""]')
  lock.version = version
  lock.packages[''].version = version
  desired.set('package-lock.json', formatJSON(lock))

  for (const target of ['cmd/browser-agent/main.go', 'cmd/hooks/main.go']) {
    desired.set(
      target,
      replaceOne(
        originals.get(target),
        /var version = "\d+\.\d+\.\d+"/g,
        `var version = "${version}"`,
        target,
        'Go version fallback'
      )
    )
  }

  let readme = replaceOne(
    originals.get('README.md'),
    /version-\d+\.\d+\.\d+-green/g,
    `version-${version}-green`,
    'README.md',
    'version badge'
  )
  readme = replaceOne(
    readme,
    /Current version: \*\*v\d+\.\d+\.\d+\*\*/g,
    `Current version: **v${version}**`,
    'README.md',
    'current-version declaration'
  )
  desired.set('README.md', readme)
  desired.set(
    'claude_skill/kaboom/SKILL.md',
    replaceOne(
      originals.get('claude_skill/kaboom/SKILL.md'),
      /^ {2}version: \d+\.\d+\.\d+$/gm,
      `  version: ${version}`,
      'claude_skill/kaboom/SKILL.md',
      'skill metadata version'
    )
  )
  return desired
}

function driftedTargets(originals, desired) {
  return allTargets.filter((target) => originals.get(target) !== desired.get(target))
}

function writeTransaction(root, originals, desired, changed) {
  const staged = new Map()
  const committed = []
  try {
    for (const target of changed) {
      const destination = join(root, target)
      const temporary = join(dirname(destination), `.${target.split('/').at(-1)}.version-${process.pid}`)
      writeFileSync(temporary, desired.get(target), 'utf8')
      staged.set(target, temporary)
    }
    for (const target of changed) {
      renameSync(staged.get(target), join(root, target))
      staged.delete(target)
      committed.push(target)
    }
  } catch (error) {
    for (const target of committed.reverse()) {
      writeFileSync(join(root, target), originals.get(target), 'utf8')
    }
    throw error
  } finally {
    for (const temporary of staged.values()) rmSync(temporary, { force: true })
  }
}

function main() {
  const { root, action } = parseArgs(process.argv.slice(2))
  const originals = readTargets(root)
  const current = parseVersion(originals.get('VERSION'), 'VERSION')
  const checking = action === '--check'
  const syncing = action === '--sync'
  const target = checking || syncing ? current : parseVersion(action, 'new version')
  if (!checking && !syncing && compareVersions(target, current) < 0) {
    throw new Error(`version regression is not allowed: ${current} -> ${target}`)
  }

  const desired = desiredTargets(originals, target)
  const changed = driftedTargets(originals, desired)
  if (checking) {
    if (changed.length > 0) throw new Error(`version drift from VERSION ${current}: ${changed.join(', ')}`)
    console.log(`Version targets match VERSION ${current}`)
    return
  }
  writeTransaction(root, originals, desired, changed)
  console.log(`Version ${syncing ? 'sync' : 'bump'} complete: ${current} -> ${target} (${changed.length} files)`)
}

try {
  main()
} catch (error) {
  console.error(`Version synchronization failed: ${error instanceof Error ? error.message : String(error)}`)
  process.exitCode = 1
}
