// run-tests.test.mjs — Verifies recursive npm wrapper release-test discovery.

import assert from 'node:assert/strict'
import { mkdtempSync, mkdirSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import path from 'node:path'
import test from 'node:test'
import { discoverTestFiles } from './run-tests.mjs'

test('discovers change-coupled wrapper tests recursively in stable order', () => {
  const fixtureRoot = mkdtempSync(path.join(tmpdir(), 'kaboom-npm-wrapper-tests-'))
  try {
    mkdirSync(path.join(fixtureRoot, 'daemon'), { recursive: true })
    mkdirSync(path.join(fixtureRoot, 'browser'), { recursive: true })
    writeFileSync(path.join(fixtureRoot, 'daemon', 'health.test.js'), '')
    writeFileSync(path.join(fixtureRoot, 'browser', 'browser.test.js'), '')
    writeFileSync(path.join(fixtureRoot, 'browser', 'browser.js'), '')

    assert.deepEqual(discoverTestFiles(fixtureRoot), [
      path.join(fixtureRoot, 'browser', 'browser.test.js'),
      path.join(fixtureRoot, 'daemon', 'health.test.js')
    ])
  } finally {
    rmSync(fixtureRoot, { recursive: true, force: true })
  }
})

test('returns no tests for an empty package tree', () => {
  const fixtureRoot = mkdtempSync(path.join(tmpdir(), 'kaboom-npm-wrapper-empty-'))
  try {
    assert.deepEqual(discoverTestFiles(fixtureRoot), [])
  } finally {
    rmSync(fixtureRoot, { recursive: true, force: true })
  }
})
