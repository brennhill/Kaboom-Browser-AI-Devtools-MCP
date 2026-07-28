// Purpose: Enforce canonical identity boundaries in the server installer.
// Docs: docs/features/feature/enhanced-cli-config/index.md

const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const test = require('node:test')

test('server installer contains no old-brand cleanup paths', () => {
  const source = fs.readFileSync(path.join(__dirname, 'install.js'), 'utf8')
  assert.doesNotMatch(source, /\b(?:gasoline|strum)\b/i)
  assert.doesNotMatch(source, /\blegacy\b/i)
})
