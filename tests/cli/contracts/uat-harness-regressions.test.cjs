// uat-harness-regressions.test.cjs — Regression contracts for deterministic comprehensive UAT accounting.
'use strict'

const assert = require('node:assert/strict')
const { execFileSync } = require('node:child_process')
const { existsSync, globSync, readFileSync } = require('node:fs')
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

  test('command failures retain the structured extension error instead of truncating the envelope', () => {
    const response = JSON.stringify({
      result: {
        content: [{
          type: 'text',
          text: 'Command trace: complete\n{"result":{"error":"performance_trace_failed","message":"Runtime.evaluate failed: target rejected","resolved_tab_id":41,"resolved_url":"chrome-extension://other/panel.html","target_context":{"source":"tracked_tab"}}}'
        }]
      }
    })
    assert.equal(
      frameworkCall(`command_failure_message '${response}'`),
      'performance_trace_failed: Runtime.evaluate failed: target rejected [tab_id=41 source=tracked_tab url=chrome-extension://other/panel.html]'
    )
  })

  test('unknown command failure envelopes remain complete for local diagnosis', () => {
    const detail = `unclassified:${'x'.repeat(400)}`
    const response = JSON.stringify({ result: { content: [{ type: 'text', text: detail }] } })
    assert.equal(frameworkCall(`command_failure_message '${response}'`), detail)
  })

  // The /sync wire field is `ext_session_id`. A payload using `session_id` leaves
  // ExtSessionID empty, so the daemon's apply phase rejects the whole request with
  // 409 stale_connection_generation — after it has already overwritten settings. The
  // 409 body is still valid JSON, so tests that only assert "a JSON response came
  // back" pass while exercising nothing. cat-11.7 asserted harder and failed for it.
  test('UAT sync payloads use the real wire session field', () => {
    for (const script of globSync('scripts/tests/*/cat-*.sh')) {
      // Comments are stripped: prose explaining the bug legitimately names the
      // wrong field, and only the payloads sent on the wire are in scope here.
      const payloads = readFileSync(script, 'utf8')
        .split('\n')
        .filter((line) => !/^\s*#/.test(line))
        .join('\n')
      assert.ok(
        !payloads.includes('"session_id"'),
        `${script} posts "session_id"; the SyncRequest wire field is "ext_session_id"`
      )
    }
  })

  // A connected category runs beside a real extension. Posting /sync as the extension
  // client makes the daemon apply the mock's settings over the real ones: TrackingEnabled
  // and TrackedTabID are plain bool/int, so a payload carrying only pilot_enabled erases
  // the tracked tab the extension reported. The extension restores it on its next full
  // heartbeat, so this is a race — the source of the intermittent "no tracked browser
  // tab" failures in categories scheduled after the /sync-posting ones.
  //
  // Categories 14 and 16 verify that settings are applied, which a probe cannot observe,
  // so they claim the extension identity and run offline where no browser is attached.
  test('connected categories never impersonate the extension session on /sync', () => {
    const connectedScripts = readFileSync('scripts/uat/runners/test-all-tools-comprehensive.sh', 'utf8')
      .match(/^CONNECTED_CAT_IDS="([^"]+)"$/m)[1]
      .split(' ')
      .flatMap((id) => globSync(`scripts/tests/*/cat-${id}-*.sh`))

    assert.ok(connectedScripts.length > 0, 'connected category scripts must be discoverable')
    for (const script of connectedScripts) {
      const source = readFileSync(script, 'utf8')
      const lines = source.split('\n')
      lines.forEach((line, index) => {
        if (!line.includes('X-Kaboom-Client: kaboom-extension')) return
        const target = lines.slice(index + 1, index + 15).find((next) => /localhost:\$\{PORT\}\//.test(next))
        assert.ok(
          !target || !target.includes('/sync'),
          `${script}:${index + 1} posts /sync as the extension; use kaboom-probe so the real session is untouched`
        )
      })

      // post_extension/post_logs are framework helpers that send the extension
      // identity, so a connected category can claim the session without the
      // header literal ever appearing in its own source.
      for (const helper of ['post_extension', 'post_logs']) {
        assert.ok(
          !new RegExp(`(^|[\\s;&|(){}])${helper}\\s`, 'm').test(source),
          `${script} calls ${helper}, which posts as the extension and would overwrite the live session`
        )
      }
    }
  })

  // Every connected category restarts the daemon, and the suite toggles pilot state,
  // so a tracked tab established once by the preflight cannot be assumed to survive to
  // the last category. A category that finds the extension up but nothing tracked must
  // re-establish its own disposable fixture instead of failing the whole category.
  test('connected categories re-establish a lost fixture tab before failing', () => {
    const output = frameworkCall(`
      KABOOM_UAT_REQUIRE_CONNECTED=1
      PORT=7890
      WRAPPER=/test/wrapper
      UAT_CONNECTED_READY_ATTEMPTS=1
      uat_readiness_sleep() { :; }
      TRACKED=""
      uat_connected_health_payload() { printf '%s\\n' '{"capture":{"extension_connected":true}}'; }
      uat_connected_tracked_tab() { printf '%s\\n' "$TRACKED"; }
      uat_create_disposable_tab() { printf 'created\\n'; TRACKED='73\thttps://test.example/'; }
      wait_for_required_connected_browser && printf 'ready\\n'
    `)
    assert.match(output, /created/, 'a missing tracked tab must trigger disposable-tab recovery')
    assert.match(output, /ready/, 'recovery must let the category proceed')
  })

  test('connected categories do not attempt tab recovery while the extension is down', () => {
    const output = frameworkCall(`
      KABOOM_UAT_REQUIRE_CONNECTED=1
      PORT=7890
      WRAPPER=/test/wrapper
      UAT_CONNECTED_READY_ATTEMPTS=1
      uat_readiness_sleep() { :; }
      uat_connected_health_payload() { printf '%s\\n' '{"capture":{"extension_connected":false,"extension_last_seen":"never"}}'; }
      uat_connected_tracked_tab() { printf '\\n'; }
      uat_create_disposable_tab() { printf 'created\\n'; }
      wait_for_required_connected_browser || printf 'failed\\n'
    `)
    assert.doesNotMatch(output, /created/, 'a disconnected extension cannot open a tab; recovery must not be attempted')
    assert.match(output, /failed/)
  })

  test('offline and connected categories have explicit, disjoint suite boundaries', () => {
    const runner = readFileSync('scripts/uat/runners/test-all-tools-comprehensive.sh', 'utf8')
    const categoryIds = (name) => {
      const match = runner.match(new RegExp(`^${name}="([^"]+)"$`, 'm'))
      assert.ok(match, `${name} must be declared`)
      return match[1].split(' ')
    }
    const offline = categoryIds('OFFLINE_CAT_IDS')
    const connected = categoryIds('CONNECTED_CAT_IDS')

    // Derived from disk, not a magic count: a category script that exists but is
    // in neither list looks like coverage while never running. Seven were in that
    // state, and one of them (32) reported 8/8 green while every call it made
    // failed to parse. Category 27 is the sole permitted exclusion — it blocks on
    // `read -r` for human visual verification and would hang the suite.
    const HUMAN_INTERACTIVE = ['27']
    const onDisk = globSync('scripts/tests/*/cat-*.sh')
      .map((file) => file.match(/cat-(\d+)-/)[1])
      .sort()
    const scheduled = new Set([...offline, ...connected])
    const unscheduled = onDisk.filter((id) => !scheduled.has(id) && !HUMAN_INTERACTIVE.includes(id))
    assert.deepEqual(
      unscheduled,
      [],
      `these category scripts exist but no suite runs them: ${unscheduled.join(', ')}`
    )
    for (const id of HUMAN_INTERACTIVE) {
      assert.ok(
        !scheduled.has(id),
        `category ${id} pauses for human input and cannot be scheduled in an automated suite`
      )
    }
    assert.deepEqual(offline.filter((id) => connected.includes(id)), [])
    assert.ok(offline.includes('05'), 'Pilot-unavailable contract belongs offline')
    assert.ok(connected.includes('15'), 'Pilot success path belongs connected')
    assert.ok(connected.includes('24'), 'Upload success path dispatches through the extension')
    assert.ok(connected.includes('35'), 'QA fixture transactions require a real attached browser')
    assert.match(runner, /--suite offline\|connected\|all/)
  })

  test('comprehensive UAT never emits production telemetry', () => {
    const runner = readFileSync('scripts/uat/runners/test-all-tools-comprehensive.sh', 'utf8')
    assert.match(
      runner,
      /export KABOOM_TELEMETRY=off/,
      'isolated UAT daemon states must not inflate production install analytics'
    )
    const smokeRunner = readFileSync('scripts/uat/runners/smoke-test.sh', 'utf8')
    assert.match(
      smokeRunner,
      /export KABOOM_TELEMETRY=off/,
      'smoke tests must not create production analytics activity'
    )
    const framework = readFileSync('scripts/tests/framework/framework.sh', 'utf8')
    assert.match(
      framework,
      /export KABOOM_TELEMETRY=off/,
      'standalone category tests must not create production analytics activity'
    )
  })

  test('offline suite is isolated from the extension port and connected suite is preflighted', () => {
    const runner = readFileSync('scripts/uat/runners/test-all-tools-comprehensive.sh', 'utf8')

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
    assert.match(runner, /"\$WRAPPER" --daemon --parallel --port "\$CONNECTED_UAT_PORT"/)
    assert.match(runner, /KABOOM_UAT_REQUIRE_CONNECTED=1/)
    assert.match(runner, /uat_wait_for_extension/)
    assert.match(runner, /source "\$TESTS_DIR\/framework\/uat-user-state\.sh"/)
    assert.match(runner, /uat_snapshot_user_state "\$CONNECTED_UAT_PORT" "\$WRAPPER"/)
    assert.match(runner, /trap 'uat_exit_for_signal TERM' TERM/)
    assert.match(runner, /offline\) _cleanup_ports="\$OFFLINE_UAT_PORT"/)
    assert.doesNotMatch(runner, /lsof -ti :/)
    assert.match(runner, /Running .* categories sequentially/)
    assert.match(runner, /KABOOM_UAT_CATEGORY/)
    assert.doesNotMatch(runner, /Running \d+ parallel groups/)
    assert.doesNotMatch(runner, /PORT_GROUP\d+=/)
  })

  test('connected readiness distinguishes daemon, extension, and tracked-tab prerequisites', () => {
    const output = userStateCall(`
      uat_connected_health_payload() { printf '%s\\n' "$UAT_TEST_HEALTH"; }
      uat_connected_tracked_tab() { printf '%s\\n' "$UAT_TEST_TAB"; }

      UAT_TEST_HEALTH=''
      UAT_TEST_TAB=''
      uat_check_connected_readiness 7890 /test/wrapper || true
      printf 'daemon=%s\\n' "$UAT_CONNECTED_READINESS_REASON"

      UAT_TEST_HEALTH='{"capture":{"extension_connected":false,"extension_last_seen":"never"}}'
      uat_check_connected_readiness 7890 /test/wrapper || true
      printf 'extension=%s\\n' "$UAT_CONNECTED_READINESS_REASON"

      UAT_TEST_HEALTH='{"capture":{"extension_connected":true}}'
      uat_check_connected_readiness 7890 /test/wrapper || true
      printf 'tab=%s\\n' "$UAT_CONNECTED_READINESS_REASON"

      UAT_TEST_TAB='73\thttps://test.example/'
      uat_check_connected_readiness 7890 /test/wrapper
      printf 'ready=%s\\n' "$UAT_CONNECTED_READINESS_REASON"
    `)

    assert.equal(
      output,
      [
        'daemon=daemon health unavailable on port 7890',
        'extension=extension not connected on port 7890 (last seen: never)',
        'tab=no tracked browser tab on port 7890',
        'ready=ready'
      ].join('\n')
    )
  })

  test('disposable-tab preflight accepts a connected extension without requiring user tab state', () => {
    const output = userStateCall(`
      uat_connected_health_payload() {
        printf '%s\\n' '{"capture":{"extension_connected":true}}'
      }
      uat_readiness_sleep() { :; }
      UAT_CONNECTED_READY_ATTEMPTS=1
      uat_wait_for_extension 7890
      printf '%s\\n' "$UAT_CONNECTED_READINESS_REASON"
    `)

    assert.equal(output, 'extension ready')
  })

  test('connected readiness retries boundedly until all prerequisites become available', () => {
    const output = userStateCall(`
      readiness_dir="$(mktemp -d)"
      uat_connected_health_payload() {
        count="$(cat "$readiness_dir/health" 2>/dev/null || echo 0)"
        count=$((count + 1))
        printf '%s' "$count" > "$readiness_dir/health"
        if [ "$count" -lt 2 ]; then
          return
        fi
        printf '%s\\n' '{"capture":{"extension_connected":true}}'
      }
      uat_connected_tracked_tab() {
        count="$(cat "$readiness_dir/tab" 2>/dev/null || echo 0)"
        count=$((count + 1))
        printf '%s' "$count" > "$readiness_dir/tab"
        if [ "$count" -ge 2 ]; then
          printf '%s\\n' '81\thttps://ready.example/'
        fi
      }
      uat_readiness_sleep() { :; }
      UAT_CONNECTED_READY_ATTEMPTS=4
      uat_wait_for_connected_browser 7890 /test/wrapper
      printf 'health=%s tab=%s\\n' "$(cat "$readiness_dir/health")" "$(cat "$readiness_dir/tab")"
      rm -rf "$readiness_dir"
    `)

    assert.equal(output, 'health=3 tab=2')
  })

  test('connected readiness budget covers the extension maximum reconnect backoff', () => {
    const userState = readFileSync('scripts/tests/framework/uat-user-state.sh', 'utf8')
    const attempts = Number(userState.match(/UAT_CONNECTED_READY_ATTEMPTS="\$\{UAT_CONNECTED_READY_ATTEMPTS:-([0-9]+)\}"/)?.[1])

    assert.ok(attempts >= 400, `connected readiness budget is only ${attempts * 100}ms`)
    assert.match(userState, /uat_readiness_sleep\(\) \{[\s\S]*sleep 0\.1/)
  })

  test('connected UAT creates, tracks, and closes a dedicated browser tab', () => {
    const output = userStateCall(`
      uat_call_tool() {
        case "$3" in
          interact)
            if printf '%s' "$4" | grep -q '"what":"new_tab"'; then
              printf '%s\\n' '{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"Command completed\\n{\\"action\\":\\"new_tab\\",\\"tab_id\\":222}"}]}}'
            else
              printf 'call:%s\\n' "$4" >> "$close_log"
            fi
            ;;
        esac
      }
      uat_ensure_cleanup_daemon() { return 0; }
      uat_wait_for_extension() { return 0; }
      uat_wait_for_disposable_tracking() { return 0; }
      close_log="$(mktemp)"

      uat_create_disposable_tab 7890 /test/wrapper
      printf 'created=%s url=%s\\n' "$UAT_DISPOSABLE_TAB_ID" "$UAT_DISPOSABLE_TAB_URL"
      uat_close_disposable_tab
      cat "$close_log"
      rm -f "$close_log"
      printf 'closed=%s\\n' "$UAT_DISPOSABLE_TAB_CLOSED"
    `)

    assert.match(output, /^created=222 url=http:\/\/127\.0\.0\.1:7890\/tests\/interact\.html$/m)
    assert.match(output, /"what":"switch_tab","tab_id":222,"set_tracked":true/)
    assert.match(output, /call:\{"what":"close_tab","tab_id":222\}/)
    assert.match(output, /closed=1/)
  })

  test('user restoration closes the disposable tab before replacing the suite daemon', () => {
    const output = userStateCall(`
      UAT_USER_STATE_SNAPSHOTTED=1
      UAT_DISPOSABLE_TAB_ID=222
      uat_close_disposable_tab() { echo close-test-tab; UAT_DISPOSABLE_TAB_CLOSED=1; }
      uat_stop_port() { echo stop-suite-daemon; }
      uat_restore_user_state
    `)

    assert.equal(output, ['close-test-tab', 'stop-suite-daemon'].join('\n'))
  })

  test('category discovery follows feature-family directories', () => {
    const runner = readFileSync('scripts/uat/runners/test-all-tools-comprehensive.sh', 'utf8')
    const framework = readFileSync('scripts/tests/framework/framework.sh', 'utf8')

    assert.match(runner, /find "\$TESTS_DIR" -type f -name "cat-\$\{cat_id\}-\*\.sh"/)
    assert.doesNotMatch(runner, /"\$TESTS_DIR\/cat-\$\{cat_id\}-"\*\.sh/)
    assert.match(framework, /local project_root="\$script_dir\/\.\.\/\.\.\/\.\."/)
    assert.match(framework, /TEST_DAEMON_CLEANER="\$FRAMEWORK_DIR\/\.\.\/\.\.\/cleanup-test-daemons\.sh"/)
  })

  test('connected UAT derives action coverage from the live five-tool schema', () => {
    const runner = readFileSync('scripts/uat/runners/test-all-tools-comprehensive.sh', 'utf8')
    const actionCoverage = readFileSync('scripts/tests/browser/cat-33-connected-action-coverage.sh', 'utf8')

    assert.match(runner, /CONNECTED_CAT_IDS="[^"]*\b33\b/)
    assert.match(actionCoverage, /"method":"tools\/list"/)
    assert.match(actionCoverage, /observe generate configure interact analyze/)
    assert.match(actionCoverage, /\.inputSchema\.properties\.what\.enum/)
    assert.match(actionCoverage, /schema_count/)
    assert.match(actionCoverage, /executed_count/)
    assert.match(actionCoverage, /action_expectation/)
    assert.match(actionCoverage, /unclassified/)
    assert.match(actionCoverage, /expected_error:/)
    assert.match(actionCoverage, /user_mediated/)
    assert.match(actionCoverage, /classified_count/)
    for (const mode of ['configure/qa_fixture', 'analyze/performance_trace', 'analyze/react_profile', 'analyze/verification']) {
      assert.match(actionCoverage, new RegExp(mode.replace('/', '\\/')), `${mode} must have a connected contract`)
    }
    assert.match(actionCoverage, /configure\/qa_fixture\) echo '\{"what":"qa_fixture","fixture_action":"validate"/)
    assert.match(actionCoverage, /analyze\/performance_trace\|analyze\/react_profile\)/)
    assert.match(actionCoverage, /"action":"stop"/)
    assert.match(actionCoverage, /analyze\/performance_trace\|analyze\/react_profile\)[\s\S]*ensure_fixture_page/)
    // A profile names one tab, so start and stop must both target the fixture
    // explicitly rather than whatever happens to be tracked when each call lands.
    assert.match(actionCoverage, /tracked_tab_id\(\)/)
    assert.match(
      actionCoverage,
      /analyze\/performance_trace\|analyze\/react_profile\)[\s\S]*"action":"start","tab_id":/,
      'the trace lifecycle must start against an explicit tab'
    )
    assert.match(
      actionCoverage,
      /analyze\/performance_trace\|analyze\/react_profile\) echo '\{"what":"'"\$mode"'","action":"stop","tab_id":/,
      'the trace lifecycle must stop against the same explicit tab it started'
    )
    // Chrome refuses the same target every time, so the refusal must never enter
    // the retry set — retrying only wastes the budget. It may, and should, be
    // classified: refusing an extension debugger access to another extension's
    // page is a browser security boundary working as intended, so an action
    // blocked by it is reported as skipped rather than as a product failure.
    const retryBody = actionCoverage.slice(actionCoverage.indexOf('call_action_with_retry() {'))
    const retryPattern = retryBody.match(/grep -qE '([^']*)'/)
    assert.ok(retryPattern, 'call_action_with_retry must declare a retry pattern')
    assert.doesNotMatch(
      retryPattern[1],
      /performance_trace_target_not_debuggable|chrome-extension:\/\/ URL of different extension/,
      'a permanent debugger refusal must not be retried'
    )
    assert.match(
      actionCoverage,
      /is_debugger_refusal\(\)[\s\S]*performance_trace_target_not_debuggable/,
      'a debugger refusal must be classified rather than reported as a product failure'
    )
    assert.match(
      actionCoverage,
      /is_debugger_refusal[^\n]*\n\s*skip /,
      'a classified debugger refusal must skip, not fail'
    )
    // HEALTH_TRACKED_TAB_ID carries "id<TAB>url"; interpolating it raw into a
    // tab_id field emits malformed JSON that silently stops testing the action.
    assert.doesNotMatch(actionCoverage, /"tab_id":'"\$\{HEALTH_TRACKED_TAB_ID/)
    assert.match(actionCoverage, /KABOOM_UAT_ACTION/)
    assert.match(actionCoverage, /selected_count/)
    assert.match(actionCoverage, /No live schema action matched KABOOM_UAT_ACTION/)
    assert.match(actionCoverage, /ensure_event_recording/)
    assert.match(actionCoverage, /Event recording start returned no recording_id/)
    assert.match(actionCoverage, /call_action_with_retry\(\)/)
    assert.match(actionCoverage, /context deadline exceeded\|extension_timeout\|no_result\|dismiss_loop_detected\|extension_lost_command\|screenshot_failed/)
    assert.match(actionCoverage, /attempt" -le 3/)
    assert.match(actionCoverage, /uat_wait_for_connected_browser "\$PORT" "\$WRAPPER"/)
    assert.match(actionCoverage, /prepare_action "\$tool" "\$mode"/)
    assert.match(actionCoverage, /report_target_drift "\$tool" "\$mode"/)
    // Clipboard read depends on a browser permission no script can grant, so the
    // contract is a bounded classification — never a generic execution_timeout.
    assert.match(actionCoverage, /interact\/clipboard_read\) echo "permission_gated"/)
    assert.match(actionCoverage, /evaluate_permission_gated\(\)[\s\S]*execution_timeout/)
    assert.match(actionCoverage, /expectation" = "permission_gated"[\s\S]*evaluate_permission_gated "\$tool" "\$mode"/)
    for (const outcome of [
      'clipboard_permission_denied',
      'clipboard_permission_prompt_required',
      'clipboard_read_navigation_cancelled',
      'clipboard_read_context_destroyed',
      'clipboard_document_not_focused',
      'clipboard_read_timeout'
    ]) {
      assert.match(actionCoverage, new RegExp(outcome), `${outcome} must be an accepted bounded clipboard outcome`)
    }
    assert.match(actionCoverage, /ensure_fixture_page \|\| finish_category/)
    // Establishing the fixture must not require the tracking it is about to create:
    // navigate_and_wait_for re-tracks the tab, so gating on a tracked tab first turns
    // any transient tracking loss into an unrecoverable category failure.
    const fixtureBody = actionCoverage.slice(
      actionCoverage.indexOf('ensure_fixture_page() {'),
      actionCoverage.indexOf('report_target_drift() {')
    )
    assert.match(fixtureBody, /uat_wait_for_extension "\$PORT"/)
    const fixtureNavigation = fixtureBody.indexOf('\'{"what":"navigate_and_wait_for"')
    assert.ok(fixtureNavigation > 0, 'fixture setup must navigate to the fixture page')
    assert.ok(
      fixtureBody.indexOf('uat_wait_for_extension "$PORT"') < fixtureNavigation,
      'fixture setup must wait for the extension before navigating'
    )
    assert.ok(
      fixtureNavigation < fixtureBody.indexOf('uat_wait_for_connected_browser "$PORT" "$WRAPPER"'),
      'the tracked-tab gate must come after the navigation that establishes tracking'
    )
    // A tracked tab lost mid-sequence must re-establish the fixture and retry, not
    // fail every remaining action that needs a target.
    assert.match(actionCoverage, /No tab is being tracked/)
    assert.match(actionCoverage, /call_action_with_retry\(\)[\s\S]*ensure_fixture_page/)
    assert.ok(
      actionCoverage.indexOf('ensure_fixture_page || finish_category') < actionCoverage.indexOf('for tool in $TOOLS'),
      'connected action coverage must establish its tracked fixture before invoking schema actions'
    )
    assert.doesNotMatch(actionCoverage, /call_action_with_retry\(\)[\s\S]*sleep /)
    assert.match(actionCoverage, /if ! prepare_action[\s\S]*args="\$\(action_args/)
    assert.doesNotMatch(actionCoverage, /history\.pushState/)
    assert.match(actionCoverage, /interact\/back\)[\s\S]*"what":"navigate"/)
    assert.match(
      actionCoverage,
      /interact\/forward\)[\s\S]*"what":"navigate"[\s\S]*"what":"back"/,
    )
    assert.match(actionCoverage, /HEALTH_TRACKED_TAB_ID/)
    assert.match(actionCoverage, /interact\/switch_tab.*tracked_tab_id/)
    assert.match(actionCoverage, /"fields":\[\{/)
    assert.match(actionCoverage, /ensure_fixture_page\(\)/)
    assert.match(actionCoverage, /fixture_attempt=1/)
    assert.match(actionCoverage, /fixture_attempt" -le 2/)
    assert.match(actionCoverage, /uat_wait_for_connected_browser "\$PORT" "\$WRAPPER"/)
    assert.match(
      actionCoverage,
      /ensure_fixture_page\(\)[\s\S]*uat_wait_for_connected_browser[\s\S]*response="\$\(call_tool "interact"/,
    )
    assert.match(
      actionCoverage,
      /ensure_fixture_page\(\)[\s\S]*"what":"navigate_and_wait_for"[\s\S]*"wait_for":"#sf-btn"/,
    )
    assert.match(
      actionCoverage,
      /interact\/open_composer\).*"scope_selector":"#uat-composer-scope"/,
    )
    assert.match(
      actionCoverage,
      /interact\/submit_active_composer\).*"scope_selector":"#uat-composer-active-scope"/,
    )
    assert.match(
      actionCoverage,
      /interact\/navigate_and_document\)[\s\S]*ensure_fixture_page[\s\S]*documented=1/,
    )
    assert.match(
      actionCoverage,
      /interact\/navigate_and_document\).*"wait_for_url_change":true/,
    )
    assert.match(
      actionCoverage,
      /interact\/hardware_click\)[\s\S]*ensure_fixture_page/,
    )
    assert.match(actionCoverage, /interact\/close_tab\)[\s\S]*HEALTH_EXTRA_TAB_ID/)
    assert.match(
      actionCoverage,
      /interact\/hardware_click\).*"tab_id":'"\$\(tracked_tab_id\)"'/,
    )
    assert.match(
      actionCoverage,
      /analyze\/visual_diff\)[\s\S]*ensure_fixture_page[\s\S]*"what":"visual_baseline"/,
    )
    assert.match(actionCoverage, /visual_baseline_attempt=1/)
    assert.match(actionCoverage, /visual_baseline_attempt" -le 2/)
    assert.match(actionCoverage, /fail "Action coverage mismatch/)
  })

  test('upload UAT invokes the canonical change-coupled upload server fixture', () => {
    const uploadCategory = readFileSync('scripts/tests/browser/cat-24-upload.sh', 'utf8')
    const canonicalFixture = 'scripts/smoke-tests/upload/upload-server.py'

    assert.equal(existsSync(canonicalFixture), true, 'canonical upload server fixture must exist')
    assert.match(uploadCategory, /\.\.\/\.\.\/smoke-tests\/upload\/upload-server\.py/)
    assert.doesNotMatch(uploadCategory, /\.\.\/\.\.\/smoke-tests\/upload-server\.py/)
  })

  test('dynamic-upgrade UAT launches an isolated daemon lifecycle', () => {
    const upgradeCategory = readFileSync('scripts/tests/runtime/cat-26-dynamic-upgrade.sh', 'utf8')

    assert.match(upgradeCategory, /UPGRADE_STATE_DIR="\$UPGRADE_DIR\/state"/)
    assert.match(upgradeCategory, /"\$bin" --daemon --parallel --state-dir "\$UPGRADE_STATE_DIR" --port "\$UPGRADE_PORT"/)
    assert.match(upgradeCategory, /marker_path="\$UPGRADE_STATE_DIR\/run\/last-upgrade\.json"/)
    assert.doesNotMatch(upgradeCategory, /marker_path="\$HOME\//)
  })

  test('link-health UAT requests background mode for queued-response contracts', () => {
    const linkCategory = readFileSync('scripts/tests/browser/cat-19-link-health.sh', 'utf8')
    const queuedContractCalls = linkCategory.match(/call_tool "analyze" '\{"what":"link_health","sync":false\}'/g) || []

    assert.equal(queuedContractCalls.length, 2, 'status and polling-hint contracts must inspect the queued response')
    assert.ok(linkCategory.includes(`check_matches "$text" '"status":\\s*"queued"'`))
  })

  test('long-running categories retain complete result accounting', () => {
    const runner = readFileSync('scripts/uat/runners/test-all-tools-comprehensive.sh', 'utf8')
    const dynamicUpgrade = readFileSync('scripts/tests/runtime/cat-26-dynamic-upgrade.sh', 'utf8')

    assert.match(runner, /19\) echo 600/)
    assert.match(dynamicUpgrade.trimEnd(), /finish_category$/)
  })

  test('connected UAT includes deterministic QA fixture mutation and rollback coverage', () => {
    const runner = readFileSync('scripts/uat/runners/test-all-tools-comprehensive.sh', 'utf8')
    const fixtureUAT = readFileSync('scripts/tests/browser/cat-35-qa-fixtures.sh', 'utf8')

    assert.match(runner, /CONNECTED_CAT_IDS=.*35/)
    assert.match(runner, /35\) echo "QA Fixture Transactions"/)
    assert.match(fixtureUAT, /fixture_action":"apply"/)
    assert.match(fixtureUAT, /restore_fixture_transaction/)
    assert.match(fixtureUAT, /fixture_action:"restore"/)
    assert.match(fixtureUAT, /transaction_id/)
    assert.match(fixtureUAT, /snapshot_failed/)
    assert.match(fixtureUAT, /apply_failed_rolled_back/)
    assert.match(fixtureUAT, /private-fixture-secret/)
    assert.match(fixtureUAT, /finish_category/)
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
        `source scripts/uat/orchestration/uat-result-lib.sh; parse_uat_category_result "$1"; printf '%s|%s|%s|%s\\n' "$UAT_RESULT_PASS" "$UAT_RESULT_FAIL" "$UAT_RESULT_SKIP" "$UAT_RESULT_ELAPSED"`,
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
        ['-c', 'source scripts/uat/orchestration/uat-result-lib.sh; parse_uat_category_result "$1"', 'bash', path],
        { cwd: process.cwd(), encoding: 'utf8' }
      )
      assert.equal(result.status, status)
    }

    const runner = readFileSync('scripts/uat/runners/test-all-tools-comprehensive.sh', 'utf8')
    assert.match(runner, /AGGREGATION_ERRORS/)
    assert.match(runner, /UAT_RESULT_SKIP/)
    assert.match(runner, /missing result file/)
    assert.match(runner, /malformed result file/)
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

  test('exit cleanup cannot kill a daemon that explicit restoration already replaced', () => {
    const runner = readFileSync('scripts/uat/runners/test-all-tools-comprehensive.sh', 'utf8')
    const cleanup = runner.match(/_uat_cleanup\(\) \{([\s\S]*?)\n\}/)?.[1] ?? ''
    const restoredGuard = cleanup.indexOf('UAT_USER_STATE_RESTORED')
    const firstPortKill = cleanup.indexOf('lsof -tiTCP')

    assert.ok(restoredGuard >= 0, 'EXIT cleanup must detect completed restoration')
    assert.ok(firstPortKill > restoredGuard, 'completed-restoration guard must run before any port kill')
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
