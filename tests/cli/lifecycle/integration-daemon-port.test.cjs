// integration-daemon-port.test.cjs — Tests must never target the user's daemon port.
'use strict'

const assert = require('node:assert/strict')
const { execFileSync } = require('node:child_process')
const { readFileSync } = require('node:fs')
const { describe, test } = require('node:test')

// A leaked integration-test daemon was found holding 7890 — the port a real
// user's extension connects to. It answered health checks, so a freshly built
// daemon could not bind and exited silently, and every query went to the stale
// test binary instead. Isolating the port makes a leak harmless to the
// developer's own daemon, the way an isolated --state-dir already does for
// their data.
const { DEFAULT_PORT, INTEGRATION_TEST_PORT } = require('../../../npm/kaboom-agentic-browser/lib/daemon/health')

describe('integration daemon port isolation', () => {
  test('the launcher honours KABOOM_PORT so a test can be pointed elsewhere', () => {
    const script =
      "process.env.KABOOM_PORT='7899';" +
      "console.log(require('./npm/kaboom-agentic-browser/lib/daemon/health').DEFAULT_PORT)"
    const out = execFileSync('node', ['-e', script], { encoding: 'utf8', cwd: process.cwd() }).trim()
    assert.equal(out, '7899', 'DEFAULT_PORT must follow KABOOM_PORT; hardcoding it forces every test onto the user port')
  })

  test('the integration port is fixed, and is not the user port', () => {
    assert.equal(typeof INTEGRATION_TEST_PORT, 'number')
    assert.notEqual(INTEGRATION_TEST_PORT, 7890, 'integration tests must not target the default user port')
    // Fixed rather than random: a known port can be inspected and cleaned up
    // after a crash, which an ephemeral one cannot.
    assert.equal(INTEGRATION_TEST_PORT, 7899)
  })

  test('the default stays 7890 when nothing overrides it', () => {
    const out = execFileSync('node', ['-e',
      "delete process.env.KABOOM_PORT;console.log(require('./npm/kaboom-agentic-browser/lib/daemon/health').DEFAULT_PORT)"
    ], { encoding: 'utf8', cwd: process.cwd() }).trim()
    assert.equal(out, '7890', 'real users must still get 7890 by default')
    assert.equal(DEFAULT_PORT, 7890)
  })

  test('the CLI integration suite pins itself to the integration port', () => {
    const suite = readFileSync('tests/cli/runtime/cli-integration.test.cjs', 'utf8')
    assert.match(suite, /KABOOM_PORT/,
      'the CLI integration suite must pin KABOOM_PORT so a daemon it spawns cannot land on the user port')
    assert.match(suite, /INTEGRATION_TEST_PORT/,
      'pin to the shared constant rather than a literal, so there is one place to change it')
  })
})
