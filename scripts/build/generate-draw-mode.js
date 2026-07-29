#!/usr/bin/env node
// generate-draw-mode.js — Builds the MV3 draw-mode module from canonical feature partials.

import { readFileSync, writeFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = join(dirname(fileURLToPath(import.meta.url)), '..', '..')
const sourceDir = join(root, 'src', 'content', 'draw-mode')
const outputPath = join(root, 'extension', 'content', 'draw-mode.js')
const checkOnly = process.argv.includes('--check')
const partials = [
  'lifecycle-overlay.js',
  'input-rendering.js',
  'element-capture.js',
  'element-analysis.js',
  'persistence-submission.js',
  'geometry-context.js'
]
const banner = `// GENERATED FILE — DO NOT EDIT.
// Canonical sources: src/content/draw-mode/*.js
// Generator: scripts/build/generate-draw-mode.js

`

function normalize(value) {
  return value.replace(/\r\n/g, '\n').trimEnd() + '\n'
}

const generated =
  banner +
  partials
    .map((name) => normalize(readFileSync(join(sourceDir, name), 'utf8')))
    .join('\n')

if (checkOnly) {
  let current = ''
  try {
    current = readFileSync(outputPath, 'utf8')
  } catch {
    // Missing output is reported by the same drift error below.
  }
  if (current !== generated) {
    console.error('draw-mode.js is stale; run make compile-ts')
    process.exit(1)
  }
  console.log('Draw mode artifact is current.')
} else {
  writeFileSync(outputPath, generated)
  console.log('Generated extension/content/draw-mode.js')
}
