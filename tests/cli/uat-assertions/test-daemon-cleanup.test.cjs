// test-daemon-cleanup.test.cjs — The daemon sweeper must match versioned process titles.
'use strict'

const assert = require('node:assert/strict')
const { execFileSync } = require('node:child_process')
const { readFileSync } = require('node:fs')
const { describe, test } = require('node:test')

const SCRIPT = 'scripts/maintenance/cleanup-test-daemons.sh'

/** Returns whether `pgrep -f <pattern>` would select the given command line. */
function patternSelects(pattern, commandLine) {
  // pgrep -f matches an extended regex against the full command line, so the
  // same semantics can be checked with grep -E without spawning processes.
  const result = require('node:child_process').spawnSync(
    '/bin/bash',
    ['-c', `printf '%s' ${JSON.stringify(commandLine)} | grep -qE ${JSON.stringify(pattern)}`],
    { encoding: 'utf8' }
  )
  return result.status === 0
}

describe('test daemon cleanup', () => {
  // Integration daemons rewrite their own process title to include a compact
  // version tag (procctl.Argv0ForVersion), so the running command line is
  // "kaboom-test-binary-090 --daemon --port 57566". A sweep pattern that
  // assumes a space after the base name silently matches nothing: the cleaner
  // ran after every UAT category and still left twelve daemons alive for
  // twenty hours, each holding a port and a state directory.
  const VERSIONED = 'kaboom-test-binary-090 --daemon --port 57566 --state-dir /tmp/x'
  const UNVERSIONED = 'kaboom-test-binary --daemon --port 57566'

  test('the sweeper matches a versioned daemon process title', () => {
    const patterns = [...readFileSync(SCRIPT, 'utf8').matchAll(/kill_pattern\s+"([^"]+)"/g)].map((m) => m[1])
    assert.ok(patterns.length > 0, 'cleanup script must declare kill patterns')

    const daemonPattern = patterns.find((p) => p.includes('--daemon'))
    assert.ok(daemonPattern, 'cleanup script must sweep daemons')
    assert.ok(
      patternSelects(daemonPattern, VERSIONED),
      `pattern ${daemonPattern} does not match a versioned title: ${VERSIONED}`
    )
    assert.ok(
      patternSelects(daemonPattern, UNVERSIONED),
      `pattern ${daemonPattern} must still match an unversioned title`
    )
  })

  test('the sweeper never matches the production daemon', () => {
    const patterns = [...readFileSync(SCRIPT, 'utf8').matchAll(/kill_pattern\s+"([^"]+)"/g)].map((m) => m[1])
    // The user's installed daemon must survive every sweep.
    const production = '/Users/someone/.kaboom/bin/kaboom-agentic-browser --daemon --port 7890'
    for (const pattern of patterns) {
      assert.ok(
        !patternSelects(pattern, production),
        `pattern ${pattern} would kill the production daemon: ${production}`
      )
    }
  })

  test('the sweeper script stays syntactically valid', () => {
    execFileSync('/bin/bash', ['-n', SCRIPT])
  })
})

// The sweeper classified 7890-7910 as "test ports". 7890 is the port a real
// user's daemon serves and 7891 is its terminal server, so cleanup_pid_files —
// which has no binary-name guard, unlike the process killer — deleted the
// production daemon's own pid file. Running maintenance broke the thing it was
// maintaining.
describe('test-port classification excludes the user daemon', () => {
  const sweeper = 'scripts/maintenance/cleanup-test-daemons.sh'

  // The sweeper runs main() on load, so extract just the predicate rather than
  // sourcing the whole script and triggering a real sweep from a test.
  const predicate = (() => {
    const src = readFileSync(sweeper, 'utf8')
    const start = src.indexOf('is_test_port() {')
    assert.notEqual(start, -1, 'is_test_port must exist')
    return src.slice(start, src.indexOf('\n}', start) + 2)
  })()

  function isTestPort(port) {
    const out = execFileSync('/bin/bash', ['-c', `${predicate}\nis_test_port ${port} && echo yes || echo no`], {
      encoding: 'utf8'
    })
    return out.trim().endsWith('yes')
  }

  test('the user daemon port and its terminal companion are never test ports', () => {
    assert.equal(isTestPort(7890), false, '7890 is the port a real extension connects to')
    assert.equal(isTestPort(7891), false, '7891 is the production terminal server (daemon port + 1)')
  })

  test('the reserved integration band is still swept', () => {
    for (const port of [7899, 7905, 7910, 17890, 17999]) {
      assert.equal(isTestPort(port), true, `${port} is a reserved test port and must still be swept`)
    }
  })
})
