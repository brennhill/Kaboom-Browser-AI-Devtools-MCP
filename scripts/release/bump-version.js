#!/usr/bin/env node
// Purpose: Automate bump-version.js workflow behavior for repository tooling.
// Why: Keeps repetitive maintenance and verification steps deterministic.
// Docs: docs/DEVELOPMENT.md

/**
 * Comprehensive version bump script for Kaboom Agentic Browser
 *
 * Usage: node scripts/release/bump-version.js <new-version>
 * Example: node scripts/release/bump-version.js 6.1.0
 *
 * This script:
 * 1. Reads current version from VERSION file
 * 2. Validates new version is valid semver
 * 3. Finds ALL files referencing the old version or earlier versions
 * 4. Updates them to the new version
 * 5. Validates all package.json dependencies are consistent
 */

import fs from 'fs'
import path from 'path'
// Node built-in import, not hiding a core module
import { fileURLToPath } from 'url'
import { normalizeMainPyprojectFile, validateMainPyprojectContent } from './normalize-pypi-main-pyproject.js'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const ROOT = path.join(__dirname, '..', '..')
const VERSION_FILE = path.join(ROOT, 'VERSION')

// Color codes for terminal output
const colors = {
  reset: '\x1b[0m',
  green: '\x1b[32m',
  red: '\x1b[31m',
  yellow: '\x1b[33m',
  blue: '\x1b[34m',
  cyan: '\x1b[36m'
}

function log(color, prefix, message) {
  console.log(`${colors[color]}${prefix}${colors.reset} ${message}`)
  return
}

function getCurrentVersion() {
  const version = fs.readFileSync(VERSION_FILE, 'utf8').trim()
  return version
}

function isValidSemver(version) {
  return /^\d+\.\d+\.\d+$/.test(version)
}

function parseSemver(version) {
  const [major, minor, patch] = version.split('.').map(Number)
  return { major, minor, patch }
}

// Find all files that contain version references
function findVersionReferences(oldVersion, searchDir = ROOT) {
  const files = []
  const ignore = new Set([
    'node_modules',
    '.git',
    'dist',
    '.next',
    'build',
    'coverage',
    '.claude',
    '.venv-build',
    '.kaboom',
    '.worktrees'
  ])
  const ext = new Set(['.js', '.json', '.go', '.ts', '.md', '.py', '.toml', '.sh', '.yaml', '.yml'])

  function walk(dir) {
    try {
      const entries = fs.readdirSync(dir, { withFileTypes: true })
      for (const entry of entries) {
        if (ignore.has(entry.name)) continue

        const fullPath = path.join(dir, entry.name)
        if (entry.isDirectory()) {
          walk(fullPath)
        } else if (ext.has(path.extname(entry.name))) {
          try {
            const content = fs.readFileSync(fullPath, 'utf8')
            if (content.includes(oldVersion)) {
              files.push(fullPath)
            }
          } catch (_e) {
            // Skip unreadable files
          }
        }
      }
    } catch (_e) {
      // Skip unreadable directories
    }
    return
  }

  walk(searchDir)
  return files
}

// Key files that MUST be updated (validation purposes)
const CRITICAL_FILES = [
  'VERSION',
  'cmd/browser-agent/main.go',
  'npm/kaboom-agentic-browser/package.json',
  'extension/inject.bundled.js',
  'server/package.json',
  'cmd/browser-agent/openapi.json',
  'docs/core/async-command-api.yaml',
  'packages/kaboom-playwright/package.json',
  'packages/kaboom-ci/package.json',
  'pypi/create-platform-packages.sh'
]

function updateVersionInFile(filePath, oldVersion, newVersion) {
  const content = fs.readFileSync(filePath, 'utf8')
  let updated = content

  // Handle different formats
  if (filePath.endsWith('.json')) {
    // JSON files: "version": "6.0.1"
    updated = updated.replace(
      new RegExp(`"version":\\s*"${oldVersion.replace(/\./g, '\\.')}"`, 'g'), // nosemgrep: javascript.lang.security.audit.detect-non-literal-regexp.detect-non-literal-regexp, javascript_dos_rule-non-literal-regexp -- RegExp from trusted local version string
      `"version": "${newVersion}"`
    )
    // optionalDependencies: "@package": "6.0.1"
    updated = updated.replace(
      new RegExp(`"(@\\w+/[^"]+)":\\s*"${oldVersion.replace(/\./g, '\\.')}"`, 'g'), // nosemgrep: javascript.lang.security.audit.detect-non-literal-regexp.detect-non-literal-regexp, javascript_dos_rule-non-literal-regexp -- RegExp from trusted local version string
      `"$1": "${newVersion}"`
    )
    // JSON example values: "example": "6.0.1"
    updated = updated.replace(
      new RegExp(`"example":\\s*"${oldVersion.replace(/\./g, '\\.')}"`, 'g'), // nosemgrep: javascript.lang.security.audit.detect-non-literal-regexp.detect-non-literal-regexp, javascript_dos_rule-non-literal-regexp -- RegExp from trusted local version string
      `"example": "${newVersion}"`
    )
  } else if (filePath.endsWith('.go')) {
    // Go: var version = "6.0.1"
    updated = updated.replace(
      new RegExp(`version = "${oldVersion.replace(/\./g, '\\.')}"`, 'g'), // nosemgrep: javascript.lang.security.audit.detect-non-literal-regexp.detect-non-literal-regexp, javascript_dos_rule-non-literal-regexp -- RegExp from trusted local version string
      `version = "${newVersion}"`
    )
    // Go const
    updated = updated.replace(
      new RegExp(`const version = "${oldVersion.replace(/\./g, '\\.')}"`, 'g'), // nosemgrep: javascript.lang.security.audit.detect-non-literal-regexp.detect-non-literal-regexp, javascript_dos_rule-non-literal-regexp -- RegExp from trusted local version string
      `const version = "${newVersion}"`
    )
    // Go User-Agent: "Kaboom/6.0.3 ..."

    updated = updated.replace(new RegExp(`Kaboom/${oldVersion.replace(/\./g, '\\.')}`, 'g'), `Kaboom/${newVersion}`) // nosemgrep: javascript.lang.security.audit.detect-non-literal-regexp.detect-non-literal-regexp, javascript_dos_rule-non-literal-regexp -- RegExp from trusted local version string
  } else if (filePath.endsWith('.js') || filePath.endsWith('.ts')) {
    // JavaScript/TypeScript: version: '6.0.1' or version: "6.0.1"
    updated = updated.replace(
      new RegExp(`version:\\s*'${oldVersion.replace(/\./g, '\\.')}'`, 'g'), // nosemgrep: javascript.lang.security.audit.detect-non-literal-regexp.detect-non-literal-regexp, javascript_dos_rule-non-literal-regexp -- RegExp from trusted local version string
      `version: '${newVersion}'`
    )
    updated = updated.replace(
      new RegExp(`version:\\s*"${oldVersion.replace(/\./g, '\\.')}"`, 'g'), // nosemgrep: javascript.lang.security.audit.detect-non-literal-regexp.detect-non-literal-regexp, javascript_dos_rule-non-literal-regexp -- RegExp from trusted local version string
      `version: "${newVersion}"`
    )
    // __version__ = "7.8.0"
    updated = updated.replace(
      new RegExp(`__version__ = "${oldVersion.replace(/\./g, '\\.')}"`, 'g'), // nosemgrep: javascript.lang.security.audit.detect-non-literal-regexp.detect-non-literal-regexp, javascript_dos_rule-non-literal-regexp -- RegExp from trusted local version string
      `__version__ = "${newVersion}"`
    )
    updated = updated.replace(
      new RegExp(`__version__ = '${oldVersion.replace(/\./g, '\\.')}'`, 'g'), // nosemgrep: javascript.lang.security.audit.detect-non-literal-regexp.detect-non-literal-regexp, javascript_dos_rule-non-literal-regexp -- RegExp from trusted local version string
      `__version__ = '${newVersion}'`
    )
    // const VERSION = '7.8.0' or const VERSION = "7.8.0"
    updated = updated.replace(
      new RegExp(`const VERSION = '${oldVersion.replace(/\./g, '\\.')}'`, 'g'), // nosemgrep: javascript.lang.security.audit.detect-non-literal-regexp.detect-non-literal-regexp, javascript_dos_rule-non-literal-regexp -- RegExp from trusted local version string
      `const VERSION = '${newVersion}'`
    )
    updated = updated.replace(
      new RegExp(`const VERSION = "${oldVersion.replace(/\./g, '\\.')}"`, 'g'), // nosemgrep: javascript.lang.security.audit.detect-non-literal-regexp.detect-non-literal-regexp, javascript_dos_rule-non-literal-regexp -- RegExp from trusted local version string
      `const VERSION = "${newVersion}"`
    )
  } else if (filePath.endsWith('.py')) {
    // Python: __version__ = "7.8.0"
    updated = updated.replace(
      new RegExp(`__version__ = "${oldVersion.replace(/\./g, '\\.')}"`, 'g'), // nosemgrep: javascript.lang.security.audit.detect-non-literal-regexp.detect-non-literal-regexp, javascript_dos_rule-non-literal-regexp -- RegExp from trusted local version string
      `__version__ = "${newVersion}"`
    )
  } else if (filePath.endsWith('.toml')) {
    // TOML: version = "6.0.1"
    updated = updated.replace(
      new RegExp(`version = "${oldVersion.replace(/\./g, '\\.')}"`, 'g'), // nosemgrep: javascript.lang.security.audit.detect-non-literal-regexp.detect-non-literal-regexp, javascript_dos_rule-non-literal-regexp -- RegExp from trusted local version string
      `version = "${newVersion}"`
    )
    // Dependencies in TOML (PEP 440: ==6.0.3)

    updated = updated.replace(new RegExp(`==${oldVersion.replace(/\./g, '\\.')}`, 'g'), `==${newVersion}`) // nosemgrep: javascript.lang.security.audit.detect-non-literal-regexp.detect-non-literal-regexp, javascript_dos_rule-non-literal-regexp -- RegExp from trusted local version string
  } else if (filePath.endsWith('.md')) {
    // Markdown: version-6.0.1-green or just 6.0.1

    updated = updated.replace(new RegExp(oldVersion.replace(/\./g, '\\.'), 'g'), newVersion) // nosemgrep: javascript.lang.security.audit.detect-non-literal-regexp.detect-non-literal-regexp, javascript_dos_rule-non-literal-regexp -- RegExp from trusted local version string
  } else if (filePath.endsWith('.yaml') || filePath.endsWith('.yml')) {
    // YAML: version: 6.0.1 or version: "6.0.1"

    updated = updated.replace(new RegExp(`version: ${oldVersion.replace(/\./g, '\\.')}`, 'g'), `version: ${newVersion}`) // nosemgrep: javascript.lang.security.audit.detect-non-literal-regexp.detect-non-literal-regexp, javascript_dos_rule-non-literal-regexp -- RegExp from trusted local version string
    updated = updated.replace(
      new RegExp(`version: "${oldVersion.replace(/\./g, '\\.')}"`, 'g'), // nosemgrep: javascript.lang.security.audit.detect-non-literal-regexp.detect-non-literal-regexp, javascript_dos_rule-non-literal-regexp -- RegExp from trusted local version string
      `version: "${newVersion}"`
    )
    updated = updated.replace(
      new RegExp(`version: '${oldVersion.replace(/\./g, '\\.')}'`, 'g'), // nosemgrep: javascript.lang.security.audit.detect-non-literal-regexp.detect-non-literal-regexp, javascript_dos_rule-non-literal-regexp -- RegExp from trusted local version string
      `version: '${newVersion}'`
    )
  } else if (filePath.endsWith('.sh')) {
    // Shell: VERSION="6.0.1"
    updated = updated.replace(
      new RegExp(`VERSION="${oldVersion.replace(/\./g, '\\.')}"`, 'g'), // nosemgrep: javascript.lang.security.audit.detect-non-literal-regexp.detect-non-literal-regexp, javascript_dos_rule-non-literal-regexp -- RegExp from trusted local version string
      `VERSION="${newVersion}"`
    )
    // Shell: VERSION='6.0.1'
    updated = updated.replace(
      new RegExp(`VERSION='${oldVersion.replace(/\./g, '\\.')}'`, 'g'), // nosemgrep: javascript.lang.security.audit.detect-non-literal-regexp.detect-non-literal-regexp, javascript_dos_rule-non-literal-regexp -- RegExp from trusted local version string
      `VERSION='${newVersion}'`
    )
  }

  if (updated !== content) {
    fs.writeFileSync(filePath, updated, 'utf8')
    return true
  }
  return false
}

async function main() {
  const newVersion = process.argv[2]

  log('cyan', '=>', 'Kaboom Agentic Browser Version Bump Script')
  log('cyan', '=>', '')

  // Step 1: Get current version
  const currentVersion = getCurrentVersion()
  log('blue', 'Current version:', currentVersion)

  // Step 2: Validate new version
  if (!newVersion) {
    // HTML-like content in CLI log output, not rendered in browser
    log('red', 'ERROR:', 'No version provided. Usage: node scripts/release/bump-version.js <version>')
    // CLI script exits with error status on validation failure
    process.exit(1)
  }

  if (!isValidSemver(newVersion)) {
    log('red', 'ERROR:', `Invalid semver format: ${newVersion}. Expected format: X.Y.Z`)
    // CLI script exits with error status
    process.exit(1)
  }

  if (newVersion === currentVersion) {
    log('yellow', 'WARN:', 'New version is same as current version. Skipping.')
    // CLI script exits with error status on validation failure
    process.exit(0)
  }

  const current = parseSemver(currentVersion)
  const next = parseSemver(newVersion)

  // Validate version progression
  if (
    next.major < current.major ||
    (next.major === current.major && next.minor < current.minor) ||
    (next.major === current.major && next.minor === current.minor && next.patch < current.patch)
  ) {
    log('red', 'ERROR:', `Version regression detected: ${currentVersion} -> ${newVersion}`)
    // CLI script exits with error status on validation failure
    process.exit(1)
  }

  log('green', '✓', `Valid semver: ${newVersion}`)
  log('cyan', '=>', '')

  // Step 3: Find all files with old version
  log('cyan', 'Scanning for version references...')
  const filesWithVersion = findVersionReferences(currentVersion)

  if (filesWithVersion.length === 0) {
    log('red', 'ERROR:', `No files found containing version ${currentVersion}`)
    // CLI script exits with error status on validation failure
    process.exit(1)
  }

  log('green', '✓', `Found ${filesWithVersion.length} files with version ${currentVersion}`)
  log('cyan', '=>', '')

  // Step 4: Update VERSION file first
  log('cyan', 'Updating VERSION file...')
  fs.writeFileSync(VERSION_FILE, newVersion, 'utf8')
  log('green', '✓', `Updated VERSION`)

  // Step 5: Update all files
  log('cyan', 'Updating version in all files...')
  const updated = []
  const failed = []

  for (const filePath of filesWithVersion) {
    const relPath = path.relative(ROOT, filePath)
    try {
      if (updateVersionInFile(filePath, currentVersion, newVersion)) {
        updated.push(relPath)
        log('green', '✓', relPath)
      }
    } catch (error) {
      log('red', '✗', `${relPath}: ${error.message}`)
      failed.push(relPath)
    }
  }

  log('cyan', '=>', '')
  log('green', '✓', `Updated ${updated.length} files`)

  if (failed.length > 0) {
    log('red', '✗', `Failed to update ${failed.length} files:`)
    for (const f of failed) log('red', '  -', f)
    // CLI script exits with error status
    process.exit(1)
  }

  // Step 5b: Normalize the main PyPI pyproject structure defensively.
  const normalizeResult = normalizeMainPyprojectFile(
    path.join(ROOT, 'pypi', 'kaboom-agentic-browser', 'pyproject.toml'),
    {
      write: true
    }
  )
  if (normalizeResult.changed) {
    const normalizedRelPath = path.relative(ROOT, normalizeResult.filePath)
    if (!updated.includes(normalizedRelPath)) {
      updated.push(normalizedRelPath)
    }
    log('yellow', '⚠', `Normalized ${normalizedRelPath} (moved dependencies under [project])`)
  }

  const mainPyprojectPath = path.join(ROOT, 'pypi', 'kaboom-agentic-browser', 'pyproject.toml')
  const mainPyprojectContent = fs.readFileSync(mainPyprojectPath, 'utf8')
  const metadataValidation = validateMainPyprojectContent(mainPyprojectContent, { expectedVersion: newVersion })
  if (!metadataValidation.valid) {
    log('red', 'ERROR:', 'PyPI main package metadata validation failed:')
    for (const validationError of metadataValidation.errors) {
      log('red', '  -', validationError)
    }
    process.exit(1)
  }
  log('green', '✓', 'PyPI main package metadata validated')

  // Step 6: Validate package.json dependencies
  log('cyan', '=>', '')
  log('cyan', 'Validating package.json dependencies...')

  const mainPackageJson = JSON.parse(fs.readFileSync(path.join(ROOT, 'npm/kaboom-agentic-browser/package.json')))
  const optionalDeps = mainPackageJson.optionalDependencies || {}

  let depsMatch = true
  for (const [pkg, version] of Object.entries(optionalDeps)) {
    if (version !== newVersion) {
      log('red', '✗', `${pkg}: ${version} (expected ${newVersion})`)
      depsMatch = false
    }
  }

  if (!depsMatch) {
    log('red', 'ERROR:', 'optionalDependencies version mismatch')
    // CLI script exits with error status
    process.exit(1)
  }

  log('green', '✓', 'All optionalDependencies match')

  // Step 7: Check critical files were updated
  log('cyan', '=>', '')
  log('cyan', 'Checking critical files...')

  for (const file of CRITICAL_FILES) {
    const filePath = path.join(ROOT, file)

    const content = fs.readFileSync(filePath, 'utf8')
    if (content.includes(newVersion)) {
      log('green', '✓', file)
    } else {
      log('red', '✗', `${file} does NOT contain ${newVersion}`)
      // CLI script exits with error status
      process.exit(1)
    }
  }

  // Step 8: Summary
  log('cyan', '=>', '')
  log('green', '✓✓✓', `Version bump complete: ${currentVersion} → ${newVersion}`)
  log('cyan', 'Summary:')
  log('cyan', '  -', `Updated ${updated.length} files`)
  log('cyan', '  -', 'All critical files verified')
  log('cyan', '  -', 'All dependencies synchronized')
  log('yellow', '', '')
  log('yellow', 'Next steps:')
  log('yellow', '  1', 'Run tests: make test && npm run test:ext')
  log('yellow', '  2', 'Review changes: git diff')
  log('yellow', '  3', 'Commit: git add . && git commit -m "chore: Bump version to ' + newVersion + '"')
  log('cyan', '=>', '')
  return
}

main().catch((error) => {
  log('red', 'ERROR:', error.message)
  // CLI script exits with error status
  process.exit(1)
})
