#!/usr/bin/env node
// check-architecture-boundaries.cjs — Enforce dependency direction and public-surface budgets.
'use strict'

const fs = require('node:fs')
const path = require('node:path')

const root = path.resolve(process.argv[2] || process.cwd())
const config = JSON.parse(fs.readFileSync(path.join(root, '.architecture-boundaries.json'), 'utf8'))
const sourceRoot = path.join(root, 'src')
const violations = []
const dependencyGraph = new Map()

function sourceFiles(directory) {
  return fs.readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const absolute = path.join(directory, entry.name)
    if (entry.isDirectory()) return sourceFiles(absolute)
    return entry.isFile() && entry.name.endsWith('.ts') ? [absolute] : []
  })
}

function exportCount(source) {
  return source
    .split('\n')
    .filter(
      (line) =>
        /^export\s+(?:declare\s+)?(?:async\s+)?(?:function|class|const|let|var|type|interface|enum)\b/.test(line) ||
        /^export\s*\{/.test(line)
    ).length
}

function escapePattern(value) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

function importedFeature(specifier, file) {
  if (!specifier.startsWith('.')) return null
  const resolved = path.resolve(path.dirname(file), specifier)
  const relative = path.relative(sourceRoot, resolved)
  if (relative.startsWith('..')) return null
  return relative.split(path.sep)[0]
}

function resolvedSourceImport(specifier, file, knownFiles) {
  if (!specifier.startsWith('.')) return null
  const resolved = path.resolve(path.dirname(file), specifier)
  const candidate = resolved.endsWith('.js') ? `${resolved.slice(0, -3)}.ts` : `${resolved}.ts`
  return knownFiles.has(candidate) ? candidate : null
}

function circularComponents(graph) {
  let nextIndex = 0
  const indexes = new Map()
  const lowLinks = new Map()
  const stack = []
  const onStack = new Set()
  const components = []

  function visit(file) {
    indexes.set(file, nextIndex)
    lowLinks.set(file, nextIndex)
    nextIndex += 1
    stack.push(file)
    onStack.add(file)

    for (const dependency of graph.get(file) || []) {
      if (!indexes.has(dependency)) {
        visit(dependency)
        lowLinks.set(file, Math.min(lowLinks.get(file), lowLinks.get(dependency)))
      } else if (onStack.has(dependency)) {
        lowLinks.set(file, Math.min(lowLinks.get(file), indexes.get(dependency)))
      }
    }

    if (lowLinks.get(file) !== indexes.get(file)) return
    const component = []
    let member
    do {
      member = stack.pop()
      onStack.delete(member)
      component.push(member)
    } while (member !== file)
    if (component.length > 1 || (graph.get(file) || []).includes(file)) components.push(component)
  }

  for (const file of graph.keys()) {
    if (!indexes.has(file)) visit(file)
  }
  return components
}

const files = sourceFiles(sourceRoot)
const knownFiles = new Set(files)
for (const file of files) {
  const relative = path.relative(root, file).split(path.sep).join('/')
  if (config.forbidden_source_files?.includes(relative)) {
    violations.push(`${relative}: prohibited aggregate or compatibility surface exists`)
  }
  const source = fs.readFileSync(file, 'utf8')
  const owner = path.relative(sourceRoot, file).split(path.sep)[0]
  const forbidden = config.forbidden_imports[owner] || []
  const importPattern = /(?:from\s+|import\s*\()\s*['"]([^'"]+)['"]/g
  const dependencies = []
  let match
  while ((match = importPattern.exec(source)) !== null) {
    if (config.forbidden_import_suffixes?.some((suffix) => match[1].endsWith(suffix))) {
      violations.push(`${relative}: import must target the canonical owner (${match[1]})`)
    }
    const dependency = resolvedSourceImport(match[1], file, knownFiles)
    if (dependency) dependencies.push(dependency)
    const target = importedFeature(match[1], file)
    if (target && forbidden.includes(target)) {
      violations.push(`${relative}: ${owner} must not import ${target} (${match[1]})`)
    }
  }
  dependencyGraph.set(file, dependencies)

  for (const [contract, canonicalOwner] of Object.entries(config.canonical_type_owners || {})) {
    const name = escapePattern(contract)
    const declaration = new RegExp(
      `(?:^|\\n)\\s*(?:export\\s+)?(?:(?:interface|class|enum)\\s+${name}\\b|type\\s+${name}\\s*=)`
    )
    if (declaration.test(source) && relative !== canonicalOwner) {
      violations.push(`${relative}: ${contract} must be declared only by ${canonicalOwner}`)
    }
  }

  const reexportPattern = /export\s+(?:type\s+)?(?:\{[^}]*\}|\*)\s+from\s+['"]([^'"]+)['"]/g
  while ((match = reexportPattern.exec(source)) !== null) {
    if (config.forbid_reexports) {
      violations.push(`${relative}: internal re-export is prohibited (${match[1]})`)
    }
  }

  const count = exportCount(source)
  const exception = config.export_exceptions[relative]
  const maximum = exception?.max ?? config.max_exports_per_file
  if (count > maximum) {
    violations.push(`${relative}: ${count} exports exceeds public-surface budget ${maximum}`)
  }
}

if (config.enforce_zero_cycles) {
  for (const component of circularComponents(dependencyGraph)) {
    const members = component
      .map((file) => path.relative(root, file).split(path.sep).join('/'))
      .sort()
      .join(' -> ')
    violations.push(`circular dependency: ${members}`)
  }
}

if (violations.length > 0) {
  process.stderr.write(`Architectural boundary violations:\n${violations.map((item) => `- ${item}`).join('\n')}\n`)
  process.exit(1)
}

process.stdout.write('Architectural dependency and public-surface boundaries respected\n')
