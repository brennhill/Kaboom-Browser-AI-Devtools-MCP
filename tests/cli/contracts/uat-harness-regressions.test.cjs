// uat-harness-regressions.test.cjs — Regression contracts for deterministic comprehensive UAT accounting.
'use strict'

const assert = require('node:assert/strict')
const { execFileSync } = require('node:child_process')
const { readFileSync } = require('node:fs')
const { describe, test } = require('node:test')
const { chmodSync, copyFileSync, mkdirSync, mkdtempSync, writeFileSync } = require('node:fs')
const { tmpdir } = require('node:os')
const { join } = require('node:path')

function frameworkCall(command) {
  return execFileSync('/bin/bash', ['-c', `source scripts/tests/framework/framework.sh; ${command}`], {
    cwd: process.cwd(),
    encoding: 'utf8'
  }).trim()
}

function userStateCall(command) {
  return execFileSync('/bin/bash', ['-c', `source scripts/tests/framework/uat-user-state.sh; ${command}`], {
    cwd: process.cwd(),
    encoding: 'utf8'
  }).trim()
}

describe('comprehensive UAT harness regressions', () => {
  test('elapsed time uses numeric arithmetic without quoted operands', () => {
    assert.equal(frameworkCall('elapsed_seconds 100 106'), '6')
  })

  test('JSON boolean parsing preserves false instead of treating it as missing', () => {
    assert.equal(frameworkCall(`json_boolean '{"valid":true}' valid`), 'true')
    assert.equal(frameworkCall(`json_boolean '{"valid":false}' valid`), 'false')
    assert.equal(frameworkCall(`json_boolean '{}' valid`), '')
  })

  test('offline and connected categories have explicit, disjoint suite boundaries', () => {
    const runner = readFileSync('scripts/test-all-tools-comprehensive.sh', 'utf8')
    const categoryIds = (name) => {
      const match = runner.match(new RegExp(`^${name}="([^"]+)"$`, 'm'))
      assert.ok(match, `${name} must be declared`)
      return match[1].split(' ')
    }
    const offline = categoryIds('OFFLINE_CAT_IDS')
    const connected = categoryIds('CONNECTED_CAT_IDS')

    assert.equal(new Set([...offline, ...connected]).size, 24)
    assert.deepEqual(offline.filter((id) => connected.includes(id)), [])
    assert.ok(offline.includes('05'), 'Pilot-unavailable contract belongs offline')
    assert.ok(connected.includes('15'), 'Pilot success path belongs connected')
    assert.ok(connected.includes('24'), 'Upload success path dispatches through the extension')
    assert.match(runner, /--suite offline\|connected\|all/)
  })

  test('offline suite is isolated from the extension port and connected suite is preflighted', () => {
    const runner = readFileSync('scripts/test-all-tools-comprehensive.sh', 'utf8')

    assert.ok(
      runner.indexOf('KABOOM_UAT_WRAPPER') < runner.indexOf('command -v kaboom-agentic-browser'),
      'explicit UAT binary override must take precedence'
    )
    assert.ok(
      runner.indexOf('command -v kaboom-agentic-browser') < runner.indexOf('$PROJECT_ROOT/dist/kaboom-agentic-browser'),
      'UAT must prefer the installed package from PATH'
    )
    assert.match(runner, /OFFLINE_UAT_PORT=.*17890/)
    assert.match(runner, /CONNECTED_UAT_PORT=.*7890/)
    assert.match(runner, /preflight_connected_extension/)
    assert.match(runner, /source "\$TESTS_DIR\/framework\/uat-user-state\.sh"/)
    assert.match(runner, /uat_snapshot_user_state "\$CONNECTED_UAT_PORT" "\$WRAPPER"/)
    assert.match(runner, /trap 'uat_exit_for_signal TERM' TERM/)
    assert.match(runner, /offline\) _cleanup_ports="\$OFFLINE_UAT_PORT"/)
    assert.doesNotMatch(runner, /lsof -ti :/)
    assert.match(runner, /Running .* categories sequentially/)
    assert.doesNotMatch(runner, /Running \d+ parallel groups/)
    assert.doesNotMatch(runner, /PORT_GROUP\d+=/)
  })

  test('category discovery follows feature-family directories', () => {
    const runner = readFileSync('scripts/test-all-tools-comprehensive.sh', 'utf8')
    const framework = readFileSync('scripts/tests/framework/framework.sh', 'utf8')

    assert.match(runner, /find "\$TESTS_DIR" -type f -name "cat-\$\{cat_id\}-\*\.sh"/)
    assert.doesNotMatch(runner, /"\$TESTS_DIR\/cat-\$\{cat_id\}-"\*\.sh/)
    assert.match(framework, /local project_root="\$script_dir\/\.\.\/\.\.\/\.\."/)
    assert.match(framework, /TEST_DAEMON_CLEANER="\$FRAMEWORK_DIR\/\.\.\/\.\.\/cleanup-test-daemons\.sh"/)
  })

  test('long-running categories retain complete result accounting', () => {
    const runner = readFileSync('scripts/test-all-tools-comprehensive.sh', 'utf8')
    const dynamicUpgrade = readFileSync('scripts/tests/runtime/cat-26-dynamic-upgrade.sh', 'utf8')

    assert.match(runner, /19\) echo 600/)
    assert.match(dynamicUpgrade.trimEnd(), /finish_category$/)
  })

  test('result parsing preserves skips and rejects missing or malformed files', () => {
    const dir = mkdtempSync(join(tmpdir(), 'kaboom-uat-result-'))
    const valid = join(dir, 'valid.results')
    const malformed = join(dir, 'malformed.results')
    writeFileSync(
      valid,
      'PASS_COUNT=8\nFAIL_COUNT=0\nSKIP_COUNT=1\nELAPSED=18\nCATEGORY_ID=23\nCATEGORY_NAME="Draw Mode"\n'
    )
    writeFileSync(malformed, 'PASS_COUNT=oops\nFAIL_COUNT=0\nSKIP_COUNT=1\n')

    const parsed = execFileSync(
      '/bin/bash',
      [
        '-c',
        `source scripts/uat-result-lib.sh; parse_uat_category_result "$1"; printf '%s|%s|%s|%s\\n' "$UAT_RESULT_PASS" "$UAT_RESULT_FAIL" "$UAT_RESULT_SKIP" "$UAT_RESULT_ELAPSED"`,
        'bash',
        valid
      ],
      { cwd: process.cwd(), encoding: 'utf8' }
    ).trim()
    assert.equal(parsed, '8|0|1|18')

    for (const [path, status] of [
      [join(dir, 'missing.results'), 1],
      [malformed, 3]
    ]) {
      const result = require('node:child_process').spawnSync(
        '/bin/bash',
        ['-c', 'source scripts/uat-result-lib.sh; parse_uat_category_result "$1"', 'bash', path],
        { cwd: process.cwd(), encoding: 'utf8' }
      )
      assert.equal(result.status, status)
    }

    const runner = readFileSync('scripts/test-all-tools-comprehensive.sh', 'utf8')
    assert.match(runner, /AGGREGATION_ERRORS/)
    assert.match(runner, /UAT_RESULT_SKIP/)
    assert.match(runner, /missing result file/)
    assert.match(runner, /malformed result file/)
  })

  test('runner reports category skips and exits nonzero when a selected result is missing', () => {
    const makeProject = (withResults) => {
      const root = mkdtempSync(join(tmpdir(), 'kaboom-uat-runner-'))
      const testsDir = join(root, 'scripts', 'tests')
      mkdirSync(join(testsDir, 'framework'), { recursive: true })
      copyFileSync(
        'scripts/tests/framework/uat-user-state.sh',
        join(testsDir, 'framework', 'uat-user-state.sh')
      )
      const wrapper = join(root, 'kaboom-agentic-browser')
      writeFileSync(wrapper, '#!/bin/sh\nexit 0\n')
      chmodSync(wrapper, 0o755)
      if (withResults) {
        for (const id of [
          '01',
          '02',
          '03',
          '04',
          '05',
          '06',
          '07',
          '08',
          '09',
          '10',
          '11',
          '12',
          '13',
          '20',
          '25',
          '26',
          '28'
        ]) {
          const script = join(testsDir, `cat-${id}-fake.sh`)
          writeFileSync(
            script,
            `#!/bin/sh\ncat > "$2" <<EOF\nPASS_COUNT=1\nFAIL_COUNT=0\nSKIP_COUNT=1\nELAPSED=0\nCATEGORY_ID=${id}\nCATEGORY_NAME="Fake ${id}"\nEOF\n`
          )
          chmodSync(script, 0o755)
        }
      }
      return { root, wrapper }
    }
    const run = ({ root, wrapper }) =>
      require('node:child_process').spawnSync(
        '/bin/bash',
        ['scripts/test-all-tools-comprehensive.sh', '--suite', 'offline'],
        {
          cwd: process.cwd(),
          encoding: 'utf8',
          env: { ...process.env, KABOOM_PROJECT_ROOT: root, KABOOM_UAT_WRAPPER: wrapper }
        }
      )

    const missing = run(makeProject(false))
    assert.equal(missing.status, 1)
    assert.match(missing.stderr, /missing result file for category 01/)

    const complete = run(makeProject(true))
    assert.equal(complete.status, 0)
    assert.match(complete.stdout, /TOTAL\s+\|\s+17\s+\|\s+0\s+\|\s+17\s+\|\s+34/)
    assert.match(complete.stdout, /ALL 17 TESTS PASSED \(17 skipped\)/)
  })

  test('user daemon and tracked-tab state restore exactly once on normal completion', () => {
    const stateGuard = readFileSync('scripts/tests/framework/uat-user-state.sh', 'utf8')
    const framework = readFileSync('scripts/tests/framework/framework.sh', 'utf8')
    assert.doesNotMatch(stateGuard, /lsof -ti :/)
    assert.doesNotMatch(framework, /lsof -ti :/)
    assert.match(stateGuard, /-sTCP:LISTEN/)

    const output = userStateCall(`
      uat_find_port_pid() { echo 41; }
      uat_process_executable() { echo /installed/kaboom; }
      uat_daemon_version() { echo 0.9.0; }
      uat_launchagent_registered() { return 0; }
      uat_launchagent_running() { return 0; }
      uat_capture_tracked_tab() { printf '73\\thttps://user.test/work\\n'; }
      uat_snapshot_user_state 7890 /test/wrapper
      uat_stop_port() { echo stop; }
      uat_restore_launchagent() { echo launchagent; }
      uat_wait_for_daemon() { return 0; }
      uat_restore_tracked_tab() { echo "tab:$1:$2"; }
      uat_restore_user_state
      uat_restore_user_state
    `)

    assert.equal(output, ['stop', 'launchagent', 'tab:73:https://user.test/work'].join('\n'))
  })

  test('signal exit runs the same state restoration before preserving its exit code', () => {
    const result = require('node:child_process').spawnSync(
      '/bin/bash',
      [
        '-c',
        `
          source scripts/tests/framework/uat-user-state.sh
          UAT_USER_STATE_SNAPSHOTTED=1
          uat_stop_port() { echo restored; }
          uat_restore_user_state
          uat_exit_for_signal TERM
        `
      ],
      { cwd: process.cwd(), encoding: 'utf8' }
    )

    assert.equal(result.stdout.trim(), 'restored')
    assert.equal(result.status, 143)
  })
})
