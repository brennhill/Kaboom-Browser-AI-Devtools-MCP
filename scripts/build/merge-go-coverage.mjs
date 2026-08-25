#!/usr/bin/env node
// merge-go-coverage.mjs — Merge Go coverage profiles without double-counting blocks.
//
// Streams input line-by-line: a full-tree -coverpkg=./... package profile
// exceeds V8's maximum string length (~512MB), so readFile-as-string crashed
// the coverage gate wholesale. Output is written incrementally for the same
// reason.

import { createReadStream, createWriteStream } from 'node:fs'
import { once } from 'node:events'
import { createInterface } from 'node:readline'
import { finished } from 'node:stream/promises'

const [outputPath, ...inputPaths] = process.argv.slice(2)
if (!outputPath || inputPaths.length === 0) {
  console.error('usage: merge-go-coverage.mjs <output> <profile>...')
  process.exit(2)
}

let mode = ''
const blocks = new Map()
const canonicalByStart = new Map()

for (const [inputIndex, inputPath] of inputPaths.entries()) {
  const reader = createInterface({ input: createReadStream(inputPath, { encoding: 'utf-8' }), crlfDelay: Infinity })
  let first = true
  for await (const line of reader) {
    if (first) {
      first = false
      const inputMode = line.replace(/^mode:\s*/, '')
      if (!inputMode) {
        throw new Error(`missing coverage mode in ${inputPath}`)
      }
      if (mode && inputMode !== mode) {
        throw new Error(`profile mode mismatch: ${mode} != ${inputMode}`)
      }
      mode = inputMode
      continue
    }
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
  if (first) {
    throw new Error(`missing coverage mode in ${inputPath}`)
  }
}

const keys = [...blocks.keys()].sort((left, right) => left.localeCompare(right))
const output = createWriteStream(outputPath, { encoding: 'utf-8' })
output.write(`mode: ${mode}\n`)
for (const key of keys) {
  if (!output.write(`${key} ${blocks.get(key)}\n`)) {
    await once(output, 'drain')
  }
}
output.end()
await finished(output)
