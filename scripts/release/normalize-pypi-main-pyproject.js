#!/usr/bin/env node
// Purpose: Automate normalize-pypi-main-pyproject.js workflow behavior for repository tooling.
// Why: Keeps repetitive maintenance and verification steps deterministic.
// Docs: docs/features/npm-preinstall-fix/tech-spec.md

/**
 * Normalize pypi/kaboom-agentic-browser/pyproject.toml so dependency metadata is under
 * [project] (PEP 621) and never nested under [project.scripts].
 */

import fs from 'fs'
import path from 'path'
import { fileURLToPath } from 'url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const ROOT = path.join(__dirname, '..', '..')
const MAIN_PYPROJECT = path.join(ROOT, 'pypi', 'kaboom-agentic-browser', 'pyproject.toml')
const VERSION_FILE = path.join(ROOT, 'VERSION')
const MAIN_SCRIPT_ENTRYPOINT = 'kaboom_agentic_browser.__main__:main'

function getExpectedDependencyEntries(version) {
  return [
    `kaboom-agentic-browser-darwin-arm64==${version}; sys_platform == 'darwin' and platform_machine == 'arm64'`,
    `kaboom-agentic-browser-darwin-x64==${version}; sys_platform == 'darwin' and platform_machine == 'x86_64'`,
    `kaboom-agentic-browser-linux-arm64==${version}; sys_platform == 'linux' and platform_machine == 'aarch64'`,
    `kaboom-agentic-browser-linux-x64==${version}; sys_platform == 'linux' and platform_machine == 'x86_64'`,
    `kaboom-agentic-browser-win32-x64==${version}; sys_platform == 'win32'`
  ]
}

function splitSections(content) {
  const sections = []
  const lines = content.split(/\r?\n/)
  let current = { header: '', start: 0, end: lines.length - 1 }
  let inSection = false

  for (let i = 0; i < lines.length; i += 1) {
    const line = lines[i].trim()
    if (/^\[[^\]]+\]$/.test(line)) {
      if (inSection) {
        current.end = i - 1
        sections.push(current)
      }
      current = { header: line, start: i, end: lines.length - 1 }
      inSection = true
    }
  }

  if (inSection) {
    sections.push(current)
  }

  return { sections, lines }
}

function findDependencyBlock(lines, start, end) {
  for (let i = start; i <= end; i += 1) {
    if (lines[i].trim().startsWith('dependencies = [')) {
      for (let j = i + 1; j <= end; j += 1) {
        if (lines[j].trim() === ']') {
          return { start: i, end: j, text: lines.slice(i, j + 1).join('\n') }
        }
      }
      throw new Error('Unterminated dependencies block in pyproject.toml')
    }
  }
  return null
}

export function normalizeMainPyprojectContent(content) {
  const { sections, lines } = splitSections(content)
  const projectSection = sections.find((s) => s.header === '[project]')
  const scriptsSection = sections.find((s) => s.header === '[project.scripts]')

  if (!projectSection || !scriptsSection) {
    return { changed: false, content }
  }

  const scriptsDeps = findDependencyBlock(lines, scriptsSection.start + 1, scriptsSection.end)
  const projectDeps = findDependencyBlock(lines, projectSection.start + 1, projectSection.end)
  if (!scriptsDeps) {
    return { changed: false, content }
  }

  const nextLines = [...lines]

  // Remove misplaced dependencies from [project.scripts]
  nextLines.splice(scriptsDeps.start, scriptsDeps.end - scriptsDeps.start + 1)
  if (nextLines[scriptsDeps.start] !== undefined && nextLines[scriptsDeps.start].trim() === '') {
    nextLines.splice(scriptsDeps.start, 1)
  }

  if (!projectDeps) {
    // Recompute section boundaries after removing from scripts.
    const reparsed = splitSections(nextLines.join('\n'))
    const reparsedProject = reparsed.sections.find((s) => s.header === '[project]')
    if (!reparsedProject) {
      throw new Error('Missing [project] section after normalization')
    }

    let insertAt = reparsedProject.end + 1
    // Insert before the next section header directly after [project] payload.
    for (let i = reparsedProject.start + 1; i < reparsed.lines.length; i += 1) {
      if (/^\[[^\]]+\]$/.test(reparsed.lines[i].trim())) {
        insertAt = i
        break
      }
    }

    const dependencyLines = scriptsDeps.text.split('\n')
    if (insertAt > 0 && reparsed.lines[insertAt - 1].trim() !== '') {
      dependencyLines.unshift('')
    }
    dependencyLines.push('')
    reparsed.lines.splice(insertAt, 0, ...dependencyLines)
    return { changed: true, content: reparsed.lines.join('\n') }
  }

  return { changed: true, content: nextLines.join('\n') }
}

function extractScriptEntry(lines, scriptsSection, key) {
  const entryPattern = new RegExp(`^${key}\\s*=\\s*"([^"]+)"\\s*$`) // nosemgrep: javascript.lang.security.audit.detect-non-literal-regexp.detect-non-literal-regexp -- regex uses static key
  for (let i = scriptsSection.start + 1; i <= scriptsSection.end; i += 1) {
    const line = lines[i].trim()
    const match = line.match(entryPattern)
    if (match) {
      return match[1]
    }
  }
  return null
}

function parseDependencyEntries(blockText) {
  if (!blockText) {
    return []
  }

  const entries = []
  const lines = blockText.split('\n')
  for (const rawLine of lines) {
    const line = rawLine.trim()
    if (line === '' || line === 'dependencies = [' || line === ']') {
      continue
    }
    const match = line.match(/^"([^"]+)",?\s*$/)
    if (!match) {
      throw new Error(`Unable to parse dependency line: ${line}`)
    }
    entries.push(match[1])
  }
  return entries
}

export function validateMainPyprojectContent(content, { expectedVersion } = {}) {
  const errors = []
  const { sections, lines } = splitSections(content)
  const projectSection = sections.find((s) => s.header === '[project]')
  const scriptsSection = sections.find((s) => s.header === '[project.scripts]')

  if (!projectSection) {
    errors.push('Missing [project] section')
  }
  if (!scriptsSection) {
    errors.push('Missing [project.scripts] section')
  }

  if (!projectSection || !scriptsSection) {
    return { valid: false, errors }
  }

  const projectDeps = findDependencyBlock(lines, projectSection.start + 1, projectSection.end)
  const scriptsDeps = findDependencyBlock(lines, scriptsSection.start + 1, scriptsSection.end)

  if (!projectDeps) {
    errors.push('Missing [project].dependencies block')
  }
  if (scriptsDeps) {
    errors.push('[project.scripts] must not define dependencies')
  }

  const scriptEntrypoint = extractScriptEntry(lines, scriptsSection, 'kaboom-agentic-browser')
  if (!scriptEntrypoint) {
    errors.push('Missing kaboom-agentic-browser entry under [project.scripts]')
  } else if (scriptEntrypoint !== MAIN_SCRIPT_ENTRYPOINT) {
    errors.push(`Unexpected kaboom-agentic-browser entrypoint: ${scriptEntrypoint}`)
  }

  if (projectDeps) {
    let dependencyEntries
    try {
      dependencyEntries = parseDependencyEntries(projectDeps.text)
    } catch (error) {
      errors.push(error.message)
    }

    if (expectedVersion && Array.isArray(dependencyEntries)) {
      const expected = new Set(getExpectedDependencyEntries(expectedVersion))
      const actual = new Set(dependencyEntries)

      for (const dep of expected) {
        if (!actual.has(dep)) {
          errors.push(`Missing expected dependency: ${dep}`)
        }
      }
      for (const dep of actual) {
        if (!expected.has(dep)) {
          errors.push(`Unexpected dependency: ${dep}`)
        }
      }
    }
  }

  return { valid: errors.length === 0, errors }
}

export function normalizeMainPyprojectFile(filePath = MAIN_PYPROJECT, { write = true } = {}) {
  const original = fs.readFileSync(filePath, 'utf8')
  const normalized = normalizeMainPyprojectContent(original)
  if (!normalized.changed) {
    return { changed: false, filePath }
  }

  if (write) {
    fs.writeFileSync(filePath, normalized.content, 'utf8')
  }
  return { changed: true, filePath }
}

function main() {
  const validateOnly = process.argv.includes('--validate')
  const checkOnly = process.argv.includes('--check')
  const version = fs.readFileSync(VERSION_FILE, 'utf8').trim()

  if (checkOnly) {
    const checkResult = normalizeMainPyprojectFile(MAIN_PYPROJECT, { write: false })
    if (checkResult.changed) {
      console.error('pypi/kaboom-agentic-browser/pyproject.toml needs normalization')
      process.exit(1)
    }
  }

  if (validateOnly) {
    const content = fs.readFileSync(MAIN_PYPROJECT, 'utf8')
    const validation = validateMainPyprojectContent(content, { expectedVersion: version })
    if (!validation.valid) {
      for (const error of validation.errors) {
        console.error(`- ${error}`)
      }
      process.exit(1)
    }
    console.log('pypi/kaboom-agentic-browser/pyproject.toml metadata is valid')
    return
  }

  const result = normalizeMainPyprojectFile(MAIN_PYPROJECT, { write: true })

  if (result.changed) {
    console.log('Normalized pypi/kaboom-agentic-browser/pyproject.toml')
  } else {
    console.log('pypi/kaboom-agentic-browser/pyproject.toml is already normalized')
  }
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  main()
}
