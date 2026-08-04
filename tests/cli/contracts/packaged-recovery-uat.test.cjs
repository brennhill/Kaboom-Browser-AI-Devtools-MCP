// packaged-recovery-uat.test.cjs — Release contract for isolated packaged-state recovery UAT.

const { readFileSync } = require('node:fs')
const { describe, test } = require('node:test')
const assert = require('node:assert/strict')

describe('packaged corruption recovery release UAT', () => {
  test('runs an npm-packed binary with isolated corrupt state and redaction checks', () => {
    const uat = readFileSync(
      'scripts/tests/release/cat-34-packaged-corruption-recovery.sh',
      'utf8',
    )

    assert.match(uat, /npm pack/)
    assert.match(uat, /KABOOM_STATE_DIR/)
    assert.match(uat, /KABOOM_TELEMETRY=off/)
    assert.match(uat, /state_recovery_failed/)
    assert.match(uat, /correlation_id.*install_identity/)
    assert.match(uat, /session_metadata_state/)
    assert.match(uat, /response_mode_state/)
    assert.match(uat, /saved_sequence_state/)
    assert.match(uat, /event_recording_state/)
    assert.match(uat, /restart_history_state/)
    assert.match(uat, /configure.*doctor/)
    assert.match(uat, /lifecycle.*active/)
    assert.match(uat, /lifecycle.*recovered/)
    assert.match(uat, /RAW_FIXTURE_SECRET/)
    assert.match(uat, /finish_category/)
  })

  test('is a blocking release CI check and comprehensive offline category', () => {
    const workflow = readFileSync('.github/workflows/ci.yml', 'utf8')
    const runner = readFileSync('scripts/test-all-tools-comprehensive.sh', 'utf8')

    assert.match(workflow, /Packaged corruption and recovery UAT/)
    assert.match(workflow, /cat-34-packaged-corruption-recovery\.sh/)
    assert.match(runner, /OFFLINE_CAT_IDS="[^"]*\b34\b/)
    assert.match(runner, /34\) echo "Packaged Corruption Recovery"/)
  })
})
