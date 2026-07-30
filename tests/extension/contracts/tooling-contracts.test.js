// @ts-nocheck
import { test, describe } from 'node:test'
import assert from 'node:assert'
import { readFileSync } from 'node:fs'

import eslintConfig from '../../../eslint.config.js'

function findConfigBlock(glob) {
  return eslintConfig.find((entry) => Array.isArray(entry.files) && entry.files.includes(glob))
}

describe('Tooling contracts', () => {
  test('eslint scripts block should load security plugin', () => {
    const scriptsBlock = findConfigBlock('scripts/**/*.js')
    assert.ok(scriptsBlock, 'expected scripts block in eslint.config.js')
    assert.ok(
      scriptsBlock.plugins && scriptsBlock.plugins.security,
      'scripts/**/*.js block must load eslint-plugin-security'
    )
  })

  test('eslint extension test block should define chrome global', () => {
    const testsBlock = findConfigBlock('tests/extension/**/*.js')
    assert.ok(testsBlock, 'expected tests/extension block in eslint.config.js')
    assert.strictEqual(
      testsBlock.languageOptions?.globals?.chrome,
      'readonly',
      'tests/extension block must define chrome as readonly'
    )
  })

  test('version tooling has one explicit, transactional source-of-truth implementation', () => {
    const script = readFileSync('scripts/release/version/version-sync.mjs', 'utf8')
    const makefile = readFileSync('Makefile', 'utf8')
    assert.match(script, /'VERSION'/, 'the synchronizer must inventory VERSION')
    assert.match(script, /writeTransaction/, 'version writes must use the transactional path')
    assert.match(makefile, /version-sync\.mjs "\$\(NEW_VERSION\)"/)
    assert.match(makefile, /version-sync\.mjs --sync/)
    assert.match(makefile, /version-sync\.mjs --check/)
    assert.match(makefile, /^compile-ts: validate-versions /m)
    assert.match(makefile, /^\$\(PLATFORMS\): validate-versions$/m)
    assert.match(
      makefile,
      /^VERSION = \$\(shell cat VERSION\)$/m,
      'Make must read VERSION lazily so bump-version + build cannot embed the old value'
    )
    assert.doesNotMatch(makefile, /perl -pi.*version/, 'Make must not contain a second version rewriter')
  })

  test('validate-architecture should enforce /sync handler instead of removed legacy handlers', () => {
    const script = readFileSync('scripts/validate-architecture.sh', 'utf8')
    assert.match(script, /HandleSync/, 'validate-architecture should require HandleSync')
    assert.doesNotMatch(
      script,
      /HandlePendingQueries|HandleDOMResult|HandleExecuteResult|HandlePilotStatus/,
      'validate-architecture should not require removed legacy handlers'
    )
  })

  test('validate-architecture stub check should not depend on fixed grep context windows', () => {
    const script = readFileSync('scripts/validate-architecture.sh', 'utf8')
    assert.doesNotMatch(
      script,
      /grep\s+-r?A\s+20/,
      'stub detection must not use grep -A 20 windows (brittle false negatives)'
    )
  })

  test('validate-architecture should not hardcode AsyncCommandTimeout to 30s', () => {
    const script = readFileSync('scripts/validate-architecture.sh', 'utf8')
    assert.doesNotMatch(
      script,
      /AsyncCommandTimeout\.\*30\.\*time\.Second/,
      'AsyncCommandTimeout check should not be hardcoded to exactly 30s'
    )
    assert.match(
      script,
      /AsyncCommandTimeout too low/,
      'AsyncCommandTimeout check should enforce a minimum threshold'
    )
  })

  test('canonical version inventory includes shipped packages, binaries, README, and skill metadata', () => {
    const script = readFileSync('scripts/release/version/version-sync.mjs', 'utf8')
    for (const target of [
      'extension/manifest.json',
      'npm/kaboom-agentic-browser/package.json',
      'packages/kaboom-playwright/package.json',
      'cmd/browser-agent/main.go',
      'cmd/hooks/main.go',
      'README.md',
      'claude_skill/kaboom/SKILL.md'
    ]) {
      assert.ok(script.includes(target), `missing version target ${target}`)
    }
  })

  test('ts runtime contracts should use kaboom headers and storage keys', () => {
    const daemonHttp = readFileSync('src/lib/daemon-http.ts', 'utf8')
    const constants = readFileSync('src/lib/constants.ts', 'utf8')
    const options = readFileSync('src/options.ts', 'utf8')
    const terminalWorkspace = readFileSync('src/background/ui/terminal-workspace.ts', 'utf8')
    const storageSession = readFileSync('src/lib/storage/session.ts', 'utf8')

    assert.match(daemonHttp, /const DEFAULT_CLIENT_NAME = 'kaboom-extension'/)
    assert.match(daemonHttp, /'X-Kaboom-Client'/)
    assert.match(daemonHttp, /'X-Kaboom-Extension-Version'/)
    assert.doesNotMatch(daemonHttp, /X-Gasoline|X-STRUM/)

    assert.match(constants, /SHOW_TRACKED_HOVER_LAUNCHER: 'kaboom_show_tracked_hover_launcher'/)
    assert.match(constants, /TERMINAL_AI_COMMAND: 'kaboom_terminal_ai_command'/)
    assert.match(constants, /TERMINAL_WORKSPACE_GROUP_ID: 'kaboom_terminal_workspace_group_id'/)
    assert.doesNotMatch(constants, /gasoline_show_tracked_hover_launcher/)
    assert.doesNotMatch(constants, /gasoline_terminal_ai_command/)

    assert.match(options, /kaboom_terminal_ai_command\?: string/)
    assert.match(options, /kaboom_terminal_dev_root\?: string/)
    assert.match(options, /kaboom-debug-\$\{timestamp\}\.json/)
    assert.doesNotMatch(options, /gasoline_terminal_ai_command/)
    assert.doesNotMatch(options, /gasoline_terminal_dev_root/)

    assert.match(terminalWorkspace, /kaboom_terminal_workspace_group_id\?: number/)
    assert.match(terminalWorkspace, /kaboom_terminal_workspace_main_tab_id\?: number/)
    assert.doesNotMatch(terminalWorkspace, /gasoline_terminal_workspace_group_id/)

    assert.match(storageSession, /const STATE_VERSION_KEY = 'kaboom_state_version'/)
    assert.doesNotMatch(storageSession, /gasoline_state_version/)
  })
})
