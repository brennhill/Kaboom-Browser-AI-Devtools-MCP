// process-census.test.cjs — The leak guard must actually catch leaks.
//
// An assertion that cannot fail is worse than no assertion: it reports green and
// buys confidence it did not earn. Both prior leaks here were invisible for
// exactly that reason — the sweeper's pattern silently stopped matching, and
// nothing ever counted the launcher processes. So every case below is checked in
// both directions: it fires on the real leak, and it stays quiet on the thing
// that only looks like one.
'use strict'

const assert = require('node:assert/strict')
const { execFileSync, spawnSync } = require('node:child_process')
const { readFileSync } = require('node:fs')
const { describe, test } = require('node:test')
const { join, resolve } = require('node:path')

const REPO = resolve(__dirname, '..', '..', '..')
const CENSUS = join(REPO, 'scripts', 'tests', 'framework', 'process-census.sh')
const UAT_RUNNER = join(REPO, 'scripts', 'uat', 'runners', 'test-all-tools-comprehensive.sh')
const SMOKE_RUNNER = join(REPO, 'scripts', 'uat', 'runners', 'smoke-test.sh')
const RESULT_LIB = join(REPO, 'scripts', 'uat', 'orchestration', 'uat-result-lib.sh')

/** Ask the runner's own exit rule whether a suite with these counters passed. */
function suitePassed(pass, fail, aggregationErrors, leaks) {
  return (
    spawnSync(
      '/bin/bash',
      [
        '-c',
        'source "$1"; uat_suite_passed "$2" "$3" "$4" "$5" ""',
        'test',
        RESULT_LIB,
        pass,
        fail,
        aggregationErrors,
        leaks,
      ],
      { encoding: 'utf8' }
    ).status === 0
  )
}

/** Run a bash snippet with the census sourced. Returns {status, stdout, stderr}. */
function withCensus(snippet, { timeout = 60000 } = {}) {
  return spawnSync('/bin/bash', ['-c', `source ${JSON.stringify(CENSUS)}\n${snippet}`], {
    encoding: 'utf8',
    timeout,
    env: { ...process.env, KABOOM_CENSUS_SETTLE_SECONDS: '2' },
  })
}

/** Whether `grep -E <pattern>` selects the given command line. */
function patternSelects(pattern, commandLine) {
  return spawnSync(
    '/bin/bash',
    ['-c', `printf '%s' ${JSON.stringify(commandLine)} | grep -qE ${JSON.stringify(pattern)}`],
    { encoding: 'utf8' }
  ).status === 0
}

function censusPatterns() {
  const out = execFileSync('/bin/bash', ['-c', `source ${JSON.stringify(CENSUS)}; census_patterns`], { encoding: 'utf8' })
  return out.split('\n').filter(Boolean)
}

function launcherPattern() {
  return execFileSync(
    '/bin/bash',
    ['-c', `source ${JSON.stringify(CENSUS)}; printf '%s' "$LAUNCHER_PATTERN"`],
    { encoding: 'utf8' }
  )
}

describe('census patterns', () => {
  test('match a daemon that rewrote its own title with a version tag', () => {
    // procctl.Argv0ForVersion turns the title into "kaboom-test-binary-090 ...".
    // A pattern assuming a space after the base name matched nothing, and twelve
    // daemons survived every sweep for twenty hours while runs reported green.
    const versioned = 'kaboom-test-binary-090 --daemon --port 57566 --state-dir /tmp/x'
    assert.ok(
      censusPatterns().some((p) => patternSelects(p, versioned)),
      'no census pattern matches a versioned daemon title'
    )
  })

  test('match an unversioned daemon title too', () => {
    const plain = 'kaboom-test-binary --daemon --port 57566'
    assert.ok(censusPatterns().some((p) => patternSelects(p, plain)))
  })

  test('never match the developer\'s production daemon', () => {
    // Counting it would fail every run on a machine with kaboom installed.
    const production = '/Users/someone/.kaboom/bin/kaboom-agentic-browser --daemon --port 7890'
    for (const p of censusPatterns()) {
      assert.ok(!patternSelects(p, production), `pattern ${p} would count the production daemon`)
    }
    assert.ok(!patternSelects(launcherPattern(), production), 'launcher pattern counts the production daemon')
  })
})

describe('launcher pattern', () => {
  test('matches a launcher invoked WITH arguments', () => {
    // Regression: an end-of-line anchor missed every real launcher, because a
    // real one is always invoked with arguments.
    assert.ok(patternSelects(launcherPattern(), 'node /proj/node_modules/kaboom-agentic-browser/bin/kaboom-agentic-browser --port 7890'))
    assert.ok(patternSelects(launcherPattern(), 'node /proj/node_modules/kaboom-agentic-browser/bin/kaboom-hooks blast-radius'))
  })

  test('matches a launcher invoked with no arguments', () => {
    assert.ok(patternSelects(launcherPattern(), 'node /usr/local/lib/node_modules/kaboom-agentic-browser/bin/kaboom-agentic-browser'))
  })

  test('does not match node running a file inside the package', () => {
    // The shim's own CLI branch (`exec node lib/cli/cli.js --install`) is short
    // lived and legitimate; so is the test runner.
    for (const legit of [
      'node /proj/node_modules/kaboom-agentic-browser/lib/cli/cli.js --install',
      'node /proj/node_modules/kaboom-agentic-browser/lib/runtime/postinstall-shims.js',
      'node --test /repo/npm/kaboom-agentic-browser/lib/config/config.test.js',
    ]) {
      assert.ok(!patternSelects(launcherPattern(), legit), `launcher pattern flags a legitimate process: ${legit}`)
    }
  })
})

describe('assertions fire on real processes', () => {
  test('a leaked daemon fails the growth assertion and is named', () => {
    const r = withCensus(`
      record_census_baseline
      exec -a "kaboom-test-binary-090 --daemon --port 57566" sleep 30 &
      LEAK=$!
      sleep 0.5
      assert_no_process_growth "decoy" && echo UNEXPECTED_PASS
      kill -9 $LEAK 2>/dev/null
    `)
    assert.doesNotMatch(r.stdout, /UNEXPECTED_PASS/, 'growth assertion passed despite a leak')
    assert.match(r.stderr, /PROCESS LEAK after decoy/)
    assert.match(r.stderr, /kaboom-test-binary-090 --daemon --port 57566/, 'failure must name the offending process')
  })

  test('a clean run passes the growth assertion', () => {
    const r = withCensus(`
      record_census_baseline
      assert_no_process_growth "clean" && echo CLEAN_PASS
    `)
    assert.match(r.stdout, /CLEAN_PASS/, 'growth assertion failed on a clean run')
  })

  test('two daemons on one port fail; one does not', () => {
    const r = withCensus(`
      exec -a "kaboom-test-binary-090 --daemon --port 7999" sleep 30 & A=$!
      sleep 0.4
      assert_no_duplicate_daemons "single" && echo SINGLE_OK
      exec -a "kaboom-test-binary-090 --daemon --port 7999" sleep 30 & B=$!
      sleep 0.4
      assert_no_duplicate_daemons "double" && echo UNEXPECTED_PASS
      kill -9 $A $B 2>/dev/null
    `)
    assert.match(r.stdout, /SINGLE_OK/, 'one daemon on a port must pass')
    assert.doesNotMatch(r.stdout, /UNEXPECTED_PASS/, 'duplicate daemons passed')
    assert.match(r.stderr, /DUPLICATE DAEMONS after double/)
  })

  test('a returning Node launcher fails even when nothing else leaked', () => {
    const r = withCensus(`
      exec -a "node /proj/node_modules/kaboom-agentic-browser/bin/kaboom-agentic-browser --port 7890" sleep 30 &
      L=$!
      sleep 0.5
      assert_no_launcher_processes "decoy" && echo UNEXPECTED_PASS
      kill -9 $L 2>/dev/null
    `)
    assert.doesNotMatch(r.stdout, /UNEXPECTED_PASS/, 'launcher assertion passed despite a launcher')
    assert.match(r.stderr, /LAUNCHER REGRESSION after decoy/)
  })

  test('the production daemon does not trip any assertion', () => {
    const r = withCensus(`
      exec -a "/Users/someone/.kaboom/bin/kaboom-agentic-browser --daemon --port 7890" sleep 30 &
      P=$!
      sleep 0.5
      record_census_baseline
      echo "baseline=$KABOOM_CENSUS_BASELINE"
      assert_no_process_growth "with production daemon" && \
        assert_no_duplicate_daemons "with production daemon" && \
        assert_no_launcher_processes "with production daemon" && echo PROD_SAFE
      kill -9 $P 2>/dev/null
    `)
    assert.match(r.stdout, /baseline=0/, 'production daemon was counted into the baseline')
    assert.match(r.stdout, /PROD_SAFE/, 'production daemon tripped a leak assertion')
  })
})

describe('the runners actually enforce it', () => {
  test('the UAT runner asserts after every category and gates its exit code', () => {
    const src = readFileSync(UAT_RUNNER, 'utf8')
    assert.match(src, /source "\$TESTS_DIR\/framework\/process-census\.sh"/, 'census not sourced')
    assert.match(src, /record_census_baseline/, 'no baseline recorded')
    for (const fn of ['assert_no_process_growth', 'assert_no_duplicate_daemons', 'assert_no_launcher_processes']) {
      assert.match(src, new RegExp(`${fn} "category `), `${fn} not called per category`)
    }
    // The exit rule moved out of the runner into uat_suite_passed so the printed
    // verdict and the exit status could stop disagreeing. Matching the old inline
    // `[ -n "$PROCESS_LEAK_CATEGORIES" ] ... exit 1` would now prove nothing, and
    // matching the new call site would only prove a literal exists — so run the
    // rule and check what it answers.
    assert.match(
      src,
      /uat_suite_passed "\$TOTAL_PASS" "\$TOTAL_FAIL" "\$AGGREGATION_ERRORS" \\\n\s*"\$PROCESS_LEAK_CATEGORIES" "\$TIMED_OUT_CATEGORIES"/,
      'the runner no longer hands its leak list to the exit rule'
    )
    assert.match(src, /^exit "\$SUITE_EXIT"$/m, 'the runner does not exit with the rule’s verdict')
    assert.equal(suitePassed('1', '0', '0', ' 21'), false, 'a process leak does not fail the run')
    assert.equal(suitePassed('1', '0', '0', ''), true, 'control: a clean run cannot pass either')
  })

  test('the smoke runner asserts after every module and gates its exit code', () => {
    const src = readFileSync(SMOKE_RUNNER, 'utf8')
    assert.match(src, /process-census\.sh"/, 'census not sourced')
    for (const fn of ['assert_no_duplicate_daemons', 'assert_no_launcher_processes']) {
      assert.match(src, new RegExp(`${fn} "module `), `${fn} not called per module`)
    }
    assert.match(src, /\[ -n "\$SMOKE_PROCESS_LEAK" \]; then\n {4}exit 1/, 'a process leak does not fail smoke')
  })

  test('the census counts before cleanup runs, not after', () => {
    // Counting after a sweep measures what cleanup hid, which is how both prior
    // leaks stayed invisible.
    const src = readFileSync(UAT_RUNNER, 'utf8')
    const assertAt = src.indexOf('assert_no_process_growth "category ')
    const sweepAt = src.indexOf('cleanup-test-daemons.sh', assertAt)
    assert.ok(assertAt > 0, 'per-category assertion missing')
    assert.ok(sweepAt > assertAt, 'the sweep runs before the census inside run_category')
  })
})
