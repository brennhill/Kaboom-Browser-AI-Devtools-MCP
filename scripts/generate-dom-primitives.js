#!/usr/bin/env node
// Purpose: Automate generate-dom-primitives.js workflow behavior for repository tooling.
// Why: Keeps repetitive maintenance and verification steps deterministic.
// Docs: docs/DEVELOPMENT.md

/**
 * generate-dom-primitives.js
 *
 * Generates src/background/dom/dom-primitives.ts from the main template plus partials:
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
 *   node scripts/generate-dom-primitives.js         # write/update generated file
 *   node scripts/generate-dom-primitives.js --check # exit non-zero if out of date
 */

import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const ROOT = path.join(__dirname, '..')

const TEMPLATE_PATH = path.join(ROOT, 'scripts', 'templates', 'dom-primitives.ts.tpl')
const PARTIALS_DIR = path.join(ROOT, 'scripts', 'templates', 'partials')
const OUTPUT_PATH = path.join(ROOT, 'src', 'background', 'dom', 'dom-primitives.ts')
const CHECK_ONLY = process.argv.includes('--check')

const GENERATED_BANNER = `// AUTO-GENERATED FILE. DO NOT EDIT DIRECTLY.
// Source: scripts/templates/dom-primitives.ts.tpl + partials/
//   _dom-selectors.tpl, _dom-semantic-resolvers.tpl, _dom-overlay-helpers.tpl,
//   _dom-intent.tpl, _dom-intent-actions.tpl, _dom-ranking.tpl,
//   _dom-action-helpers.tpl, _dom-action-handlers-core.tpl,
//   _dom-action-handlers-input.tpl, _dom-action-handlers-overlay.tpl
// Generator: scripts/generate-dom-primitives.js

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

function buildOutput(templateContent) {
  const resolved = resolveIncludes(templateContent)
  return GENERATED_BANNER + normalize(resolved)
}

function main() {
  if (!fs.existsSync(TEMPLATE_PATH)) {
    console.error(`Template not found: ${TEMPLATE_PATH}`)
    process.exit(1)
  }

  const templateContent = fs.readFileSync(TEMPLATE_PATH, 'utf8')
  const generatedContent = buildOutput(templateContent)
  const existingContent = fs.existsSync(OUTPUT_PATH) ? fs.readFileSync(OUTPUT_PATH, 'utf8') : ''
  const isDrifted = normalize(existingContent) !== normalize(generatedContent)

  if (CHECK_ONLY) {
    if (isDrifted) {
      console.error('dom-primitives.ts is out of date.')
      console.error('Run: node scripts/generate-dom-primitives.js')
      process.exit(1)
    }
    console.log('dom-primitives.ts is up to date.')
    return
  }

  fs.writeFileSync(OUTPUT_PATH, generatedContent, 'utf8')
  if (isDrifted) {
    console.log('Generated src/background/dom/dom-primitives.ts from template.')
  } else {
    console.log('dom-primitives.ts already current (rewritten for normalized line endings).')
  }
}

main()
