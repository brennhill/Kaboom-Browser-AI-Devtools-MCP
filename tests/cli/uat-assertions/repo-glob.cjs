// repo-glob.cjs — Node 20-compatible single-segment glob for repo-relative test fixtures.
//
// fs.globSync only exists on Node >= 22, but package.json declares
// engines.node >= 20 and CI's JavaScript Checks job runs Node 20, where
// `require('node:fs').globSync` is undefined. This helper supports `*` inside a
// path segment (no `**`, no braces), which is all the UAT-script contracts need.
'use strict'

const fs = require('node:fs')
const path = require('node:path')

function segmentRegex(segment) {
  const escaped = segment.replace(/[.+^${}()|[\]\\?]/g, '\\$&').replace(/\*/g, '[^/]*')
  return new RegExp(`^${escaped}$`)
}

/**
 * Expand a repo-relative pattern such as 'scripts/tests/*\/cat-*.sh'.
 * Returns '/'-joined relative paths, sorted, matching fs.globSync's output shape.
 */
function repoGlobSync(pattern, cwd = process.cwd()) {
  let matches = ['']
  for (const segment of pattern.split('/')) {
    const next = []
    for (const base of matches) {
      if (!segment.includes('*')) {
        const candidate = base ? `${base}/${segment}` : segment
        if (fs.existsSync(path.join(cwd, candidate))) next.push(candidate)
        continue
      }
      const dir = path.join(cwd, base || '.')
      if (!fs.existsSync(dir) || !fs.statSync(dir).isDirectory()) continue
      const re = segmentRegex(segment)
      for (const entry of fs.readdirSync(dir)) {
        if (re.test(entry)) next.push(base ? `${base}/${entry}` : entry)
      }
    }
    matches = next
  }
  return matches.sort()
}

module.exports = { repoGlobSync }
