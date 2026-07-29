#!/usr/bin/env node
// check-architecture-boundaries.cjs — Enforce dependency direction and public-surface budgets.
'use strict'

const fs = require('node:fs')
const path = require('node:path')

const root = path.resolve(process.argv[2] || process.cwd())
const config = JSON.parse(fs.readFileSync(path.join(root, '.architecture-boundaries.json'), 'utf8'))
const sourceRoot = path.join(root, 'src')
const violations = []

function sourceFiles(directory) {
  return fs.readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const absolute = path.join(directory, entry.name)
    if (entry.isDirectory()) return sourceFiles(absolute)
    return entry.isFile() && entry.name.endsWith('.ts') ? [absolute] : []
  })
}

function exportCount(source) {
  return source.split('\n').filter((line) =>
    /^export\s+(?:declare\s+)?(?:async\s+)?(?:function|class|const|let|var|type|interface|enum)\b/.test(line) ||
    /^export\s*\{/.test(line)
  ).length
}

function importedFeature(specifier, file) {
  if (!specifier.startsWith('.')) return null
  const resolved = path.resolve(path.dirname(file), specifier)
  const relative = path.relative(sourceRoot, resolved)
  if (relative.startsWith('..')) return null
  return relative.split(path.sep)[0]
}

for (const file of sourceFiles(sourceRoot)) {
  const relative = path.relative(root, file).split(path.sep).join('/')
  const source = fs.readFileSync(file, 'utf8')
  const owner = path.relative(sourceRoot, file).split(path.sep)[0]
  const forbidden = config.forbidden_imports[owner] || []
  const importPattern = /(?:from\s+|import\s*\()\s*['"]([^'"]+)['"]/g
  let match
  while ((match = importPattern.exec(source)) !== null) {
    const target = importedFeature(match[1], file)
    if (target && forbidden.includes(target)) {
      violations.push(`${relative}: ${owner} must not import ${target} (${match[1]})`)
    }
  }

  const count = exportCount(source)
  const exception = config.export_exceptions[relative]
  const maximum = exception?.max ?? config.max_exports_per_file
  if (count > maximum) {
    violations.push(`${relative}: ${count} exports exceeds public-surface budget ${maximum}`)
  }
}

if (violations.length > 0) {
  process.stderr.write(`Architectural boundary violations:\n${violations.map((item) => `- ${item}`).join('\n')}\n`)
  process.exit(1)
}

process.stdout.write('Architectural dependency and public-surface boundaries respected\n')
