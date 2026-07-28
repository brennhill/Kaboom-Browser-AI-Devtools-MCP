#!/usr/bin/env node
// Purpose: Automate generate-dom-primitives.js workflow behavior for repository tooling.
// Why: Keeps repetitive maintenance and verification steps deterministic.
// Docs: docs/DEVELOPMENT.md

/**
 * generate-dom-primitives.js
 *
 * Generates pointer, form, and read primitive modules from the canonical template and partials.
 *   scripts/templates/dom-primitives.ts.tpl
 *   scripts/templates/partials/_dom-selectors.tpl
 *   scripts/templates/partials/_dom-semantic-resolvers.tpl
 *   scripts/templates/partials/_dom-overlay-helpers.tpl
 *   scripts/templates/partials/_dom-intent.tpl
 *   scripts/templates/partials/_dom-intent-actions.tpl
 *   scripts/templates/partials/_dom-ranking.tpl
 *   scripts/templates/partials/_dom-action-helpers.tpl
 *   scripts/templates/partials/_dom-action-handlers-core.tpl
 *   scripts/templates/partials/_dom-action-handlers-input.tpl
 *   scripts/templates/partials/_dom-action-handlers-overlay.tpl
 *
 * The main template may contain `// @include <filename>` directives.
 * Each directive is replaced with the contents of scripts/templates/partials/<filename>.
 *
 * Usage:
 *   node scripts/build/generate-dom-primitives.js         # write/update generated file
 *   node scripts/build/generate-dom-primitives.js --check # exit non-zero if out of date
 */

import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { transformSync } from 'esbuild'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const ROOT = path.join(__dirname, '..', '..')

const TEMPLATE_PATH = path.join(ROOT, 'scripts', 'templates', 'dom-primitives.ts.tpl')
const PARTIALS_DIR = path.join(ROOT, 'scripts', 'templates', 'partials')
const OUTPUT_DIR = path.join(ROOT, 'src', 'background', 'dom', 'primitives')
const CHECK_ONLY = process.argv.includes('--check')

const ACTION_FAMILIES = {
  pointer: ['click', 'hover', 'focus', 'scroll_to'],
  form: ['type', 'paste', 'select', 'check', 'key_press', 'set_attribute'],
  read: ['get_text', 'get_value', 'get_attribute', 'wait_for', 'wait_for_text', 'wait_for_absent']
}

const GENERATED_BANNER = (family) => `// @ts-nocheck -- generated JavaScript is type-checked before transformation.
// AUTO-GENERATED FILE. DO NOT EDIT DIRECTLY.
// Source: scripts/templates/dom-primitives.ts.tpl + partials/
// Action family: ${family}
// Generator: scripts/build/generate-dom-primitives.js

`

function normalize(content) {
  return content.replace(/\r\n/g, '\n').trimEnd() + '\n'
}

function resolveIncludes(templateContent) {
  const includePattern = /^[^\S\n]*\/\/ @include (\S+)[^\S\n]*$/gm
  return templateContent.replace(includePattern, (_match, filename) => {
    const partialPath = path.join(PARTIALS_DIR, filename)
    if (!fs.existsSync(partialPath)) {
      console.error(`Partial not found: ${partialPath}`)
      process.exit(1)
    }
    return fs.readFileSync(partialPath, 'utf8').trimEnd()
  })
}

function retainActionHandlers(source, allowedActions) {
  const startMarker = '    return {\n'
  const endMarker = '\n    }\n  }\n\n  const handlers'
  const start = source.indexOf(startMarker, source.indexOf('function buildActionHandlers'))
  const end = source.indexOf(endMarker, start)
  if (start < 0 || end < 0) {
    throw new Error('Could not locate generated DOM action handler map')
  }

  const bodyStart = start + startMarker.length
  const body = source.slice(bodyStart, end)
  const matches = [...body.matchAll(/^ {6}([a-z_]+):/gm)]
  const retained = []
  for (let index = 0; index < matches.length; index += 1) {
    const action = matches[index][1]
    if (!allowedActions.has(action)) continue
    const entryStart = matches[index].index
    const entryEnd = index + 1 < matches.length ? matches[index + 1].index : body.length
    retained.push(body.slice(entryStart, entryEnd).trimEnd())
  }
  return source.slice(0, bodyStart) + retained.join('\n\n') + source.slice(end)
}

function buildOutput(templateContent, family, actions) {
  let resolved = resolveIncludes(templateContent)
  resolved = retainActionHandlers(resolved, new Set(actions))
  const exportName = `domPrimitive${family[0].toUpperCase()}${family.slice(1)}`
  resolved = resolved
    .replace(/export \{ domPrimitiveListInteractive \}[^\n]*\n/, '')
    .replace('export function domPrimitive(', `export function ${exportName}(`)
  const transformed = transformSync(resolved, {
    loader: 'ts',
    minifyWhitespace: true,
    minifyIdentifiers: false,
    minifySyntax: false,
    legalComments: 'none',
    lineLimit: 160
  }).code
  const typedExport = transformed.replace(
    `export function ${exportName}(action,selector,options)`,
    `export function ${exportName}(action: string, selector: string, options: DOMPrimitiveOptions): DOMResult | Promise<DOMResult>`
  )
  const typeImport = "import type { DOMPrimitiveOptions, DOMResult } from '../dom-types.js'\n\n"
  return GENERATED_BANNER(family) + typeImport + normalize(typedExport)
}

function main() {
  if (!fs.existsSync(TEMPLATE_PATH)) {
    console.error(`Template not found: ${TEMPLATE_PATH}`)
    process.exit(1)
  }

  const templateContent = fs.readFileSync(TEMPLATE_PATH, 'utf8')
  const outputs = Object.entries(ACTION_FAMILIES).map(([family, actions]) => {
    const outputPath = path.join(OUTPUT_DIR, `dom-primitives-${family}.ts`)
    const generatedContent = buildOutput(templateContent, family, actions)
    const existingContent = fs.existsSync(outputPath) ? fs.readFileSync(outputPath, 'utf8') : ''
    return { outputPath, generatedContent, isDrifted: normalize(existingContent) !== normalize(generatedContent) }
  })
  const isDrifted = outputs.some((output) => output.isDrifted)

  if (CHECK_ONLY) {
    if (isDrifted) {
      console.error('DOM action-family primitives are out of date.')
      console.error('Run: node scripts/build/generate-dom-primitives.js')
      process.exit(1)
    }
    console.log('DOM action-family primitives are up to date.')
    return
  }

  for (const { outputPath, generatedContent } of outputs) {
    fs.writeFileSync(outputPath, generatedContent, 'utf8')
  }
  if (isDrifted) {
    console.log('Generated action-family DOM primitives from template.')
  } else {
    console.log('DOM action-family primitives already current.')
  }
}

main()
