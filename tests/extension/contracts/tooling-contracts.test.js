// @ts-nocheck
import { test, describe } from 'node:test'
import assert from 'node:assert'
import { readFileSync, readdirSync } from 'node:fs'

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

  test('JavaScript CI rebuilds TypeScript and rejects generated drift', () => {
    const workflow = readFileSync('.github/workflows/ci.yml', 'utf8')
    assert.match(workflow, /name: Compile TypeScript and reject generated drift[\s\S]*make compile-ts/)
    assert.match(
      workflow,
      /git diff --exit-code -- extension/,
      'compiled extension drift must fail deterministically instead of comparing checkout mtimes'
    )
  })

  test('reliability CI invokes the canonical Doctor entry point', () => {
    const workflow = readFileSync('.github/workflows/ci.yml', 'utf8')
    const reliabilityGate = workflow.match(
      /- name: Reliability soak gate \(bridge fast-start\)([\s\S]*?)(?=\n {6}- name:)/
    )
    assert.ok(reliabilityGate, 'expected the bridge fast-start reliability gate')
    assert.match(reliabilityGate[1], /go run \.\/cmd\/browser-agent --doctor\b/)
    assert.doesNotMatch(
      reliabilityGate[1],
      /go run \.\/cmd\/browser-agent --check\b/,
      'CI must not invoke the removed --check compatibility facade'
    )
  })

  test('subprocess lifecycle tests run in the explicit Go integration job', () => {
    const workflow = readFileSync('.github/workflows/ci.yml', 'utf8')
    const coverageRunner = readFileSync('scripts/build/run-go-coverage.sh', 'utf8')
    const transportSmoke = readFileSync('scripts/smoke-mcp-transport.sh', 'utf8')
    assert.match(workflow, /name: Go Integration Checks/)
    assert.match(workflow, /run-go-integration\.sh -race -count=1/)
    assert.match(
      coverageRunner,
      /run-go-integration\.sh -count=1/,
      'honest aggregate coverage must include the tagged real-binary suite'
    )
    assert.match(
      workflow,
      /TestFastStart_ResourceWorkflowSoak[\s\S]*-tags=integration|go test -race -tags=integration[\s\S]*TestFastStart_ResourceWorkflowSoak/
    )
    assert.match(
      transportSmoke,
      /go test[\s\S]*-tags=integration[\s\S]*TestStdioIsolation_/,
      'the focused transport smoke gate must compile and execute its integration-tagged tests'
    )

    for (const path of [
      'cmd/browser-agent/bridge_faststart_extended_test.go',
      'cmd/browser-agent/bridge_faststart_test.go',
      'cmd/browser-agent/bridge_startup_contention_test.go',
      'cmd/browser-agent/cli_modes_subprocess_test.go',
      'cmd/browser-agent/integration_test.go',
      'cmd/browser-agent/mcp_initialize_test.go',
      'cmd/browser-agent/mcp_protocol_test.go',
      'cmd/browser-agent/server_persistence_test.go',
      'cmd/browser-agent/server_reliability_integration_test.go',
      'cmd/browser-agent/server_reliability_test.go',
      'cmd/browser-agent/stdio_silence_test.go'
    ]) {
      assert.match(readFileSync(path, 'utf8'), /^\/\/go:build integration$/m)
    }
  })

  test('hosted and local CI share honest aggregate Go coverage', () => {
    const workflow = readFileSync('.github/workflows/ci.yml', 'utf8')
    const makefile = readFileSync('Makefile', 'utf8')
    const coverageRunner = readFileSync('scripts/build/run-go-coverage.sh', 'utf8')

    assert.match(workflow, /name: Honest aggregate Go coverage[\s\S]*run: make test-cover/)
    assert.match(workflow, /name: Retain aggregate Go coverage[\s\S]*path:[\s\S]*coverage\.out/)
    assert.match(makefile, /^ci-go:[\s\S]*\n\t\$\(MAKE\) test-cover/m)
    assert.match(coverageRunner, /MINIMUM="\$\{GO_COVERAGE_MINIMUM:-89\}"/)
    assert.doesNotMatch(workflow, /70% minimum|COVERAGE < 70|coverprofile=coverage\.out/)
  })

  test('security CI scans all Go code and enforces bounded dependency policy', () => {
    const workflow = readFileSync('.github/workflows/ci.yml', 'utf8')
    const makefile = readFileSync('Makefile', 'utf8')

    assert.match(workflow, /scripts\/security\/install-go-tools\.sh/)
    assert.doesNotMatch(
      workflow,
      /go install .*\/(?:gosec|govulncheck)@/,
      'hosted CI must consume the canonical pinned installer instead of duplicating scanner versions'
    )
    assert.match(workflow, /name: Canonical security gate[\s\S]*run: make security-check/)
    assert.match(makefile, /"\$\(GOSEC_BIN\)"[^\n]*\.\/cmd\/browser-agent\/\.\.\. \.\/internal\/\.\.\./)
    assert.match(
      makefile,
      /GOTOOLCHAIN=\$\(SUPPORTED_GO_TOOLCHAIN\) "\$\(GOVULNCHECK_BIN\)" \.\/cmd\/browser-agent\/\.\.\. \.\/internal\/\.\.\./
    )
    assert.match(makefile, /node scripts\/security\/check-npm-audit\.mjs/)
  })

  test('hosted workflows invoke canonical invariant owners without copied implementations', () => {
    const makefile = readFileSync('Makefile', 'utf8')
    const ci = readFileSync('.github/workflows/ci.yml', 'utf8')
    const architecture = readFileSync('.github/workflows/architecture-validation.yml', 'utf8')
    const versions = readFileSync('.github/workflows/validate-versions.yml', 'utf8')
    const release = readFileSync('.github/workflows/release.yml', 'utf8')
    const cutRelease = readFileSync('.github/workflows/cut-release.yml', 'utf8')

    assert.match(makefile, /^check-schema:/m)
    assert.match(makefile, /^validate-architecture:/m)
    assert.match(makefile, /^verify-llm: check-invariants check-schema$/m)
    assert.match(ci, /name: Wire drift gate[^\n]*\n\s*run: make check-invariants/)
    assert.match(ci, /name: Deterministic performance SLO gate\s*\n\s*run: make test-performance/)
    assert.match(architecture, /name: Run architecture validation\s*\n\s*run: make validate-architecture/)
    assert.match(versions, /name: Validate all versions match VERSION file\s*\n\s*run: make validate-versions/)
    assert.match(release, /name: Wire drift gate[^\n]*\n\s*run: make check-invariants/)
    assert.match(cutRelease, /name: Validate version consistency[^\n]*\n\s*run: make validate-versions/)
    assert.match(cutRelease, /name: Wire drift gate[^\n]*\n\s*run: make check-invariants/)

    for (const [name, workflow] of [
      ['ci.yml', ci],
      ['architecture-validation.yml', architecture],
      ['validate-versions.yml', versions],
      ['release.yml', release],
      ['cut-release.yml', cutRelease]
    ]) {
      assert.doesNotMatch(workflow, /generate-wire-types\.js --check/, `${name} duplicates wire generation checks`)
      assert.doesNotMatch(workflow, /check-sync-wire-drift\.js/, `${name} bypasses make check-invariants`)
      assert.doesNotMatch(workflow, /version-sync\.mjs --check/, `${name} bypasses make validate-versions`)
      assert.doesNotMatch(workflow, /scripts\/validate-architecture\.sh/, `${name} bypasses make validate-architecture`)
      assert.doesNotMatch(workflow, /GO_COVERAGE_MINIMUM|COVERAGE\s*[<=>]|89%/, `${name} copies the coverage threshold`)
    }
  })

  test('scheduled and pull-request fuzzing share the canonical bounded campaign owner', () => {
    const makefile = readFileSync('Makefile', 'utf8')
    const ci = readFileSync('.github/workflows/ci.yml', 'utf8')
    const fuzz = readFileSync('.github/workflows/fuzz.yml', 'utf8')
    const runner = readFileSync('scripts/ci/run-fuzz-campaigns.sh', 'utf8')

    assert.match(makefile, /^fuzz-smoke:\s*\n\s*@?scripts\/ci\/run-fuzz-campaigns\.sh --smoke$/m)
    assert.match(makefile, /^fuzz-nightly:\s*\n\s*@?scripts\/ci\/run-fuzz-campaigns\.sh --nightly$/m)
    assert.doesNotMatch(makefile, /go test[^\n]*-fuzz=/, 'Make recipes must not bypass the canonical campaign owner')
    assert.match(ci, /name: Deterministic fuzz seed smoke\s*\n\s*run: make fuzz-smoke/)
    assert.match(fuzz, /schedule:\s*\n\s*- cron:/)
    assert.match(fuzz, /run: make fuzz-nightly/)
    assert.match(fuzz, /if: always\(\)[\s\S]*path:[\s\S]*artifacts\/fuzz[\s\S]*testdata\/fuzz/)
    for (const target of [
      'FuzzParseFixture',
      'FuzzRegistryGenerationTransitions',
      'FuzzCollectorLifecycleTransitions',
      'FuzzSyncRequestCanonicalRoundTrip',
      'FuzzRedactJSON'
    ]) {
      assert.ok(runner.includes(target), `canonical fuzz campaign missing ${target}`)
    }
    assert.doesNotMatch(ci, /go test[^\n]*-fuzz=/, 'pull requests must replay seeds rather than run random campaigns')
  })

  test('scheduled mutation analysis uses the pinned canonical runner and retains survivors', () => {
    const makefile = readFileSync('Makefile', 'utf8')
    const fuzz = readFileSync('.github/workflows/fuzz.yml', 'utf8')
    const config = JSON.parse(readFileSync('scripts/ci/mutation-cases.json', 'utf8'))

    assert.equal(config.minimum_score, 100)
    assert.match(makefile, /^mutation-test:\s*\n\s*@?node scripts\/ci\/run-targeted-mutations\.mjs$/m)
    assert.match(fuzz, /name: Critical State Mutation Gate[\s\S]*run: make mutation-test/)
    assert.match(fuzz, /if: always\(\)[\s\S]*path: artifacts\/mutation/)
  })

  test('active workflows pin the patched Go version declared by go.mod', () => {
    const goVersion = readFileSync('go.mod', 'utf8').match(/^go (\S+)$/m)?.[1]
    assert.equal(goVersion, '1.25.12')
    for (const name of readdirSync('.github/workflows').filter((file) => file.endsWith('.yml'))) {
      const workflow = readFileSync(`.github/workflows/${name}`, 'utf8')
      for (const match of workflow.matchAll(/go-version:\s*["']?([^\s"']+)/g)) {
        assert.equal(match[1], goVersion, `${name} uses Go ${match[1]} instead of ${goVersion}`)
      }
    }
  })

  test('hardening lint is a named CI gate rather than a subprocess unit test', () => {
    const workflow = readFileSync('.github/workflows/ci.yml', 'utf8')
    const hardeningTests = readFileSync('cmd/browser-agent/lint_hardening_test.go', 'utf8')
    assert.match(workflow, /name: Hardening lint[\s\S]*run: make lint-hardening/)
    assert.doesNotMatch(hardeningTests, /exec\.Command\("bash", scriptPath\)/)
  })

  test('JavaScript shard reporting identifies only the shards that failed', () => {
    const runner = readFileSync('scripts/test-js-sharded.sh', 'utf8')
    assert.match(runner, /SHARD_FAILED/)
    assert.doesNotMatch(
      runner,
      /elif \[\[ \$FAILED -ne 0 \]\]/,
      'one failing shard must not label every successful shard as nonzero'
    )
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

  test('validate-architecture follows canonical post-refactor owners', () => {
    const script = readFileSync('scripts/validate-architecture.sh', 'utf8')
    for (const currentOwner of [
      'internal/queries/dispatcher_results.go',
      'internal/capture/syncruntime/handler.go',
      'internal/toolobserve/dispatcher.go',
      'internal/toolinteract/interact_browser.go',
      'internal/bridge/bridge.go'
    ]) {
      assert.ok(script.includes(currentOwner), `missing canonical owner ${currentOwner}`)
    }
    for (const deletedSurface of [
      'internal/capture/query_dispatcher.go',
      'internal/capture/types.go',
      'tools_observe.go',
      'bridge_adapter.go',
      'toolObserveCommandResult'
    ]) {
      assert.ok(!script.includes(deletedSurface), `obsolete architecture surface ${deletedSurface}`)
    }
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
