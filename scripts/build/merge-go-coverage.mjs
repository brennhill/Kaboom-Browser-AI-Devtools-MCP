#!/usr/bin/env node
// merge-go-coverage.mjs — Merge Go coverage profiles without double-counting blocks.

import { readFile, writeFile } from 'node:fs/promises'

const [outputPath, ...inputPaths] = process.argv.slice(2)
if (!outputPath || inputPaths.length === 0) {
  console.error('usage: merge-go-coverage.mjs <output> <profile>...')
  process.exit(2)
}

let mode = ''
const blocks = new Map()
const canonicalByStart = new Map()

for (const [inputIndex, inputPath] of inputPaths.entries()) {
  const lines = (await readFile(inputPath, 'utf8')).trim().split('\n')
  const inputMode = lines.shift()?.replace(/^mode:\s*/, '')
  if (!inputMode) {
    throw new Error(`missing coverage mode in ${inputPath}`)
  }
  if (mode && inputMode !== mode) {
    throw new Error(`profile mode mismatch: ${mode} != ${inputMode}`)
  }
  mode = inputMode

  for (const line of lines) {
    if (!line) continue
    const match = line.match(/^(\S+)\s+(\d+)\s+(\d+)$/)
    if (!match) {
      throw new Error(`invalid coverage block in ${inputPath}: ${line}`)
    }
    const [, span, statements, countText] = match
    const key = `${span} ${statements}`
    const count = Number(countText)
    if (inputIndex === 0) {
      const start = span.replace(/,[^,]+$/, '')
      const startKey = `${start} ${statements}`
      const canonicalKey = canonicalByStart.get(startKey) ?? key
      const previous = blocks.get(canonicalKey)
      if (previous === undefined || count > previous) {
        blocks.set(canonicalKey, count)
      }
      canonicalByStart.set(startKey, canonicalKey)
      continue
    }

    const canonicalKey = blocks.has(key) ? key : canonicalByStart.get(`${span.replace(/,[^,]+$/, '')} ${statements}`)
    if (canonicalKey !== undefined) {
      const previous = blocks.get(canonicalKey)
      if (previous === undefined || count > previous) {
        blocks.set(canonicalKey, count)
      }
    }
  }
}

const lines = [...blocks.entries()]
  .sort(([left], [right]) => left.localeCompare(right))
  .map(([key, count]) => `${key} ${count}`)
await writeFile(outputPath, `mode: ${mode}\n${lines.join('\n')}\n`)
