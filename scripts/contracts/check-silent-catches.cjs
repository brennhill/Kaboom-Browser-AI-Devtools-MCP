#!/usr/bin/env node
// check-silent-catches.cjs — Enforce explicit rationale for intentionally suppressed JavaScript failures.
'use strict'

const fs = require('node:fs')
const path = require('node:path')

const root = path.resolve(process.argv[2] || process.cwd())
const violations = []
const generatedPrimitiveRoot = path.join(root, 'src', 'background', 'dom', 'primitives')

function sourceFiles(directory) {
  if (!fs.existsSync(directory)) return []
  return fs.readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const absolute = path.join(directory, entry.name)
    if (entry.isDirectory()) return sourceFiles(absolute)
    return entry.isFile() && /\.(?:ts|js|tpl)$/.test(entry.name) ? [absolute] : []
  })
}

function advanceLineComment(chars, index) {
  if (chars[index] === '\n') return { index, state: 'code' }
  chars[index] = ' '
  return { index, state: 'line-comment' }
}

function advanceBlockComment(chars, index) {
  if (chars[index] === '*' && chars[index + 1] === '/') {
    chars[index] = chars[index + 1] = ' '
    return { index: index + 1, state: 'code' }
  }
  if (chars[index] !== '\n') chars[index] = ' '
  return { index, state: 'block-comment' }
}

function advanceString(chars, index, quote) {
  const char = chars[index]
  if (char === '\\') {
    chars[index] = ' '
    if (index + 1 < chars.length) chars[index + 1] = ' '
    return { index: index + 1, state: 'string' }
  }
  if (char === quote) {
    chars[index] = ' '
    return { index, state: 'code' }
  }
  if (char !== '\n') chars[index] = ' '
  return { index, state: 'string' }
}

function advanceMasked(chars, index, state, quote) {
  if (state === 'line-comment') return advanceLineComment(chars, index)
  if (state === 'block-comment') return advanceBlockComment(chars, index)
  return advanceString(chars, index, quote)
}

function maskNonCode(source) {
  const chars = [...source]
  let state = 'code'
  let quote = ''
  for (let index = 0; index < chars.length; index += 1) {
    const char = chars[index]
    const next = chars[index + 1]
    if (state === 'line-comment' || state === 'block-comment' || state === 'string') {
      const step = advanceMasked(chars, index, state, quote)
      index = step.index
      state = step.state
      continue
    }
    if (char === '/' && next === '/') {
      chars[index] = chars[index + 1] = ' '
      index += 1
      state = 'line-comment'
    } else if (char === '/' && next === '*') {
      chars[index] = chars[index + 1] = ' '
      index += 1
      state = 'block-comment'
    } else if (char === "'" || char === '"' || char === '`') {
      quote = char
      chars[index] = ' '
      state = 'string'
    }
  }
  return chars.join('')
}

function matching(source, start, open, close) {
  let depth = 0
  for (let index = start; index < source.length; index += 1) {
    if (source[index] === open) depth += 1
    if (source[index] === close && --depth === 0) return index
  }
  return -1
}

function catchBodies(source) {
  const masked = maskNonCode(source)
  const bodies = []
  const pattern = /\bcatch\b/g
  let match
  while ((match = pattern.exec(masked)) !== null) {
    let before = match.index - 1
    while (before >= 0 && /\s/.test(masked[before])) before -= 1
    let cursor = pattern.lastIndex
    while (/\s/.test(masked[cursor])) cursor += 1
    if (masked[before] === '.') {
      const arrow = masked.indexOf('=>', cursor)
      const callEnd = matching(masked, cursor, '(', ')')
      if (arrow < 0 || callEnd < 0 || arrow > callEnd) continue
      cursor = arrow + 2
      while (/\s/.test(masked[cursor])) cursor += 1
      if (masked[cursor] !== '{') {
        bodies.push({ start: cursor, end: callEnd, expression: true })
        continue
      }
    } else if (masked[cursor] === '(') {
      cursor = matching(masked, cursor, '(', ')') + 1
    }
    while (/\s/.test(masked[cursor])) cursor += 1
    if (masked[cursor] !== '{') continue
    const end = matching(masked, cursor, '{', '}')
    if (end < 0) continue
    bodies.push({ start: cursor + 1, end, expression: false })
  }
  return bodies
}

function intentionallySilent(body, expression) {
  if (body.includes('EXPECTED_ABSENCE')) return false
  const code = maskNonCode(body).replace(/[\s;]/g, '')
  if (expression) return /^(?:false|true|null|undefined)$/.test(code)
  return code === '' || /^(?:return)?(?:false|true|null|undefined)?$/.test(code)
}

const authoredFiles = [
  ...sourceFiles(path.join(root, 'src')).filter((file) => !file.startsWith(`${generatedPrimitiveRoot}${path.sep}`)),
  ...sourceFiles(path.join(root, 'scripts', 'templates'))
]

for (const file of authoredFiles) {
  const source = fs.readFileSync(file, 'utf8')
  for (const body of catchBodies(source)) {
    const text = source.slice(body.start, body.end)
    if (!intentionallySilent(text, body.expression)) continue
    const line = source.slice(0, body.start).split('\n').length
    violations.push(`${path.relative(root, file).split(path.sep).join('/')}:${line}`)
  }
}

if (violations.length > 0) {
  process.stderr.write(
    `Silent catches require an EXPECTED_ABSENCE rationale or explicit diagnostic handling:\n${violations.map((item) => `- ${item}`).join('\n')}\n`
  )
  process.exit(1)
}

process.stdout.write('Every intentionally silent catch is explicitly classified\n')
