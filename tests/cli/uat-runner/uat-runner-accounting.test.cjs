// uat-runner-accounting.test.cjs — Result accounting and artifact contracts for the comprehensive runner.
'use strict'

const assert = require('node:assert/strict')
const { execFileSync } = require('node:child_process')
const { chmodSync, copyFileSync, mkdirSync, mkdtempSync, readFileSync, writeFileSync } = require('node:fs')
const { tmpdir } = require('node:os')
const { join } = require('node:path')
const { describe, test } = require('node:test')

describe('comprehensive UAT runner accounting', () => {
  test('runner reports category skips and exits nonzero when a selected result is missing', () => {
    const makeProject = (withResults) => {
      const root = mkdtempSync(join(tmpdir(), 'kaboom-uat-runner-'))
      const testsDir = join(root, 'scripts', 'tests')
      mkdirSync(join(testsDir, 'framework'), { recursive: true })
      copyFileSync(
        'scripts/tests/framework/uat-user-state.sh',
        join(testsDir, 'framework', 'uat-user-state.sh')
      )
      copyFileSync(
        'scripts/tests/framework/uat-artifacts.sh',
        join(testsDir, 'framework', 'uat-artifacts.sh')
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
          '14',
          '16',
          '17',
          '20',
          '21',
          '25',
          '26',
          '28',
          '29',
          '34'
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
    const run = ({ root, wrapper }) => {
      const artifactDir = join(root, 'artifacts')
      const result =
      require('node:child_process').spawnSync(
        '/bin/bash',
        ['scripts/uat/runners/test-all-tools-comprehensive.sh', '--suite', 'offline'],
        {
          cwd: process.cwd(),
          encoding: 'utf8',
          env: {
            ...process.env,
            KABOOM_PROJECT_ROOT: root,
            KABOOM_UAT_WRAPPER: wrapper,
            KABOOM_UAT_ARTIFACT_DIR: artifactDir,
            // Without this the spawned runner defaults to 17890 — the port a real
            // UAT run uses — so `npm test` during a UAT restarts daemons underneath
            // it and empties the capture buffers. That corrupted cat-11 and cat-29
            // in a full run and looked like two unrelated product failures.
            KABOOM_UAT_OFFLINE_PORT: '17801'
          }
        }
      )
      return { ...result, artifactDir }
    }

    const missing = run(makeProject(false))
    assert.equal(missing.status, 1)
    assert.match(missing.stderr, /missing result file for category 01/)

    const complete = run(makeProject(true))
    assert.equal(complete.status, 0)
    assert.match(complete.stdout, /TOTAL\s+\|\s+23\s+\|\s+0\s+\|\s+23\s+\|\s+46/)
    assert.match(complete.stdout, /ALL 23 TESTS PASSED \(23 skipped\)/)
    const report = JSON.parse(readFileSync(join(complete.artifactDir, 'uat-results.json'), 'utf8'))
    assert.equal(report.categories.length, 23)
    assert.equal(report.totals.skip, 23)
    assert.equal(report.restoration.status, 'not_required')
    assert.match(readFileSync(join(complete.artifactDir, 'uat-results.xml'), 'utf8'), /<testsuites/)
  })

  test('machine-readable artifacts preserve skip reasons and incomplete categories', () => {
    const dir = mkdtempSync(join(tmpdir(), 'kaboom-uat-artifacts-'))
    const records = join(dir, 'categories.ndjson')
    const json = join(dir, 'uat.json')
    const junit = join(dir, 'uat.xml')
    writeFileSync(records, [
      '{"id":"01","name":"Protocol & <XML>","pass":2,"fail":0,"skip":1,"total":3,"elapsed_seconds":4,"result_status":"complete","skip_reasons":["Needs connected browser"]}',
      '{"id":"02","name":"Observe","pass":0,"fail":1,"skip":0,"total":1,"elapsed_seconds":1,"result_status":"missing_result","skip_reasons":[]}'
    ].join('\n'))

    execFileSync('/bin/bash', [
      '-c',
      'source scripts/tests/framework/uat-artifacts.sh; uat_emit_artifacts "$1" "$2" "$3" all 5 failed ready',
      'bash',
      records,
      json,
      junit
    ], { cwd: process.cwd() })

    const report = JSON.parse(readFileSync(json, 'utf8'))
    assert.deepEqual(report.categories[0].skip_reasons, ['Needs connected browser'])
    assert.equal(report.totals.aggregation_errors, 1)
    assert.equal(report.restoration.status, 'failed')
    const xml = readFileSync(junit, 'utf8')
    assert.match(xml, /Protocol &amp; &lt;XML&gt;/)
    assert.match(xml, /<skipped message="Needs connected browser"/)
    assert.match(xml, /<failure message="missing_result"/)
  })
})
