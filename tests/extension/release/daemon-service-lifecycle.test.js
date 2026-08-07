// daemon-service-lifecycle.test.js — Tests deterministic macOS LaunchAgent identity refresh.

import assert from 'node:assert/strict'
import { execFileSync } from 'node:child_process'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import test from 'node:test'
import { fileURLToPath } from 'node:url'

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../..')
const helper = path.join(root, 'scripts/setup/install.sh')

test('macOS service registration refreshes launchd identity before bootstrap', () => {
  const fixture = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-launch-agent-'))
  const commandLog = path.join(fixture, 'launchctl.log')
  const fakeLaunchctl = path.join(fixture, 'launchctl')
  const plist = path.join(fixture, 'com.kaboom.daemon.plist')
  fs.writeFileSync(
    fakeLaunchctl,
    `#!/bin/sh\nprintf '%s\\n' "$*" >> "$KABOOM_LAUNCHCTL_LOG"\nexit 0\n`,
    { mode: 0o755 }
  )

  execFileSync(
    'bash',
    ['-c', '. "$1"; register_macos_launch_agent "$2" "$3" "$4"', 'test', helper, '/opt/kaboom/bin/kaboom-agentic-browser', plist, '501'],
    { env: { ...process.env, KABOOM_INSTALL_LIBRARY_ONLY: '1', KABOOM_LAUNCHCTL: fakeLaunchctl, KABOOM_LAUNCHCTL_LOG: commandLog } }
  )

  assert.deepEqual(fs.readFileSync(commandLog, 'utf8').trim().split('\n'), [
    'bootout gui/501/com.kaboom.daemon',
    `bootstrap gui/501 ${plist}`,
    'kickstart gui/501/com.kaboom.daemon'
  ])
  assert.match(fs.readFileSync(plist, 'utf8'), /\/opt\/kaboom\/bin\/kaboom-agentic-browser/)
})

test('macOS service registration fails loudly when bootstrap and legacy load fail', () => {
  const fixture = fs.mkdtempSync(path.join(os.tmpdir(), 'kaboom-launch-agent-failure-'))
  assert.throws(
    () => execFileSync('bash', ['-c', '. "$1"; register_macos_launch_agent "$2" "$3" "$4"', 'test', helper, '/opt/kaboom/bin/kaboom-agentic-browser', path.join(fixture, 'com.kaboom.daemon.plist'), '501'], {
      env: { ...process.env, KABOOM_INSTALL_LIBRARY_ONLY: '1', KABOOM_LAUNCHCTL: '/usr/bin/false' },
      stdio: 'pipe'
    }),
    /Command failed/
  )
})

test('installer verifies health only after refreshing the service identity', () => {
  const script = fs.readFileSync(helper, 'utf8')
  const registration = script.lastIndexOf('\nregister_autostart\n')
  const health = script.indexOf('HEALTH_RESPONSE=$(')
  assert.ok(registration > 0, 'installer must invoke service registration')
  assert.ok(health > registration, 'health verification must run after launchd identity refresh')
})
