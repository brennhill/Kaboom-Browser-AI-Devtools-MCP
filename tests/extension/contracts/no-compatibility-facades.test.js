// no-compatibility-facades.test.js — Prevents deleted extension compatibility barrels from returning.

import assert from 'node:assert/strict'
import { existsSync, readFileSync, readdirSync } from 'node:fs'
import test from 'node:test'

test('obsolete extension type compatibility barrel is absent', () => {
  assert.equal(
    existsSync('src/types/messages.ts'),
    false,
    'src/types/messages.ts is a compatibility facade; import focused type modules directly'
  )
  for (const compiledPath of [
    'extension/types/messages.js',
    'extension/types/messages.js.map',
    'extension/types/messages.d.ts',
    'extension/types/messages.d.ts.map'
  ]) {
    assert.equal(existsSync(compiledPath), false, `${compiledPath} is a stale compiled compatibility facade`)
  }
})

test('npm package has no QA skill redirect', () => {
  assert.equal(
    existsSync('npm/kaboom-agentic-browser/skills/qa/SKILL.md'),
    false,
    'QA must use the canonical audit skill directly'
  )
})

test('npm config exports only canonical client APIs', () => {
  const source = readFileSync('npm/kaboom-agentic-browser/lib/config/config.js', 'utf8')
  assert.doesNotMatch(source, /function getConfigCandidates\(/)
  assert.doesNotMatch(source, /function getToolNameFromPath\(/)
})

test('background cache modules have no state-manager facade', () => {
  assert.equal(
    existsSync('src/background/caches/state-manager.ts'),
    false,
    'cache consumers must import error-groups, cache-limits, snapshots, or debug-log directly'
  )
})

test('background sync modules have no communication facade', () => {
  assert.equal(
    existsSync('src/background/sync/communication.ts'),
    false,
    'sync consumers must import circuit-breaker, batchers, server, log-processing, or screenshot directly'
  )
})

test('environment transactions expose no QA-specific private transport aliases', () => {
  assert.equal(
    existsSync('src/background/qa-fixture'),
    false,
    'private browser state transactions belong to the canonical environment-transaction module'
  )
  assert.equal(
    existsSync('extension/background/qa-fixture'),
    false,
    'compiled QA-specific private transport must not survive an atomic migration'
  )

  for (const path of [
    'src/types/runtime/queries.ts',
    'src/background/commands/helpers.ts',
    'src/background/pending-queries.ts',
    'cmd/browser-agent/internal/toolconfigure/qafixture/handler.go'
  ]) {
    const source = readFileSync(path, 'utf8')
    assert.doesNotMatch(source, /qa_fixture_(?:snapshot|apply|restore)/, `${path} retains an obsolete private command`)
  }
})

test('background runtime entrypoint is not an API compatibility facade', () => {
  const source = readFileSync('src/background.ts', 'utf8')
  assert.doesNotMatch(
    source,
    /export\s+(?:type\s+)?(?:\{|\*)[^;]*\s+from\s+['"]/s,
    'background.ts is a runtime entrypoint; consumers must import the focused owner module'
  )

  for (const testPath of collectTestFiles('tests/extension')) {
    const testSource = readFileSync(testPath, 'utf8')
    assert.doesNotMatch(
      testSource,
      /(?:from\s+|import\s*\()\s*['"][^'"]*extension\/background\.js['"]/,
      `${testPath} imports the background runtime entrypoint instead of an owner module`
    )
  }
})

test('inject runtime entrypoint is not an API compatibility facade', () => {
  const source = readFileSync('src/inject.ts', 'utf8')
  assert.doesNotMatch(
    source,
    /export\s+(?:type\s+)?(?:\{|\*)[^;]*\s+from\s+['"]/s,
    'inject.ts is a runtime entrypoint; consumers must import the focused owner module'
  )
  assert.equal(existsSync('src/inject/index.ts'), false, 'inject/index.ts is a redundant compatibility barrel')
  for (const compiledPath of [
    'extension/inject/index.js',
    'extension/inject/index.js.map',
    'extension/inject/index.d.ts',
    'extension/inject/index.d.ts.map'
  ]) {
    assert.equal(existsSync(compiledPath), false, `${compiledPath} is a stale compiled compatibility facade`)
  }

  for (const testPath of collectTestFiles('tests/extension')) {
    const testSource = readFileSync(testPath, 'utf8')
    assert.doesNotMatch(
      testSource,
      /(?:from\s+|import\s*\()\s*['"][^'"]*extension\/inject\.js['"]/,
      `${testPath} imports the inject runtime entrypoint instead of an owner module`
    )
  }
})

test('inject state exposes no performance snapshot compatibility wrapper', () => {
  for (const path of ['src/inject/state.ts', 'extension/inject/state.js', 'extension/inject/state.d.ts']) {
    const source = readFileSync(path, 'utf8')
    assert.doesNotMatch(source, /sendPerformanceSnapshotWrapper/, `${path} retains a compatibility-only export`)
  }
})

test('health and diagnostics expose only canonical OpenAPI surfaces', () => {
  const openapi = readFileSync('cmd/browser-agent/openapi.json', 'utf8')
  const generated = readFileSync('src/generated/openapi-types.ts', 'utf8')
  for (const source of [openapi, generated]) {
    assert.doesNotMatch(source, /diagnostics\.json/)
    assert.doesNotMatch(source, /service-name/)
  }
  assert.equal(existsSync('src/popup/update-button.ts'), false, 'popup update client has no backing HTTP routes')
  assert.equal(existsSync('extension/popup/update-button.js'), false, 'compiled popup update client is stale')
})

test('pending query dispatcher does not re-export APIs owned by command modules', () => {
  const source = readFileSync('src/background/pending-queries.ts', 'utf8')
  assert.doesNotMatch(source, /export\s+(?:type\s+)?\{/, 'dispatcher must not re-export command helper APIs')
})

function collectTestFiles(directory) {
  const entries = readdirSync(directory, { withFileTypes: true })
  return entries.flatMap((entry) => {
    const path = `${directory}/${entry.name}`
    if (entry.isDirectory()) return collectTestFiles(path)
    return entry.name.endsWith('.test.js') ? [path] : []
  })
}

test('event listener module does not re-export UI module APIs', () => {
  const source = readFileSync('src/background/event-listeners.ts', 'utf8')
  assert.doesNotMatch(source, /export\s+(?:type\s+)?\{/, 'event listeners must export only owned listener functions')
})

test('AI context consumers use focused parsing and enrichment modules', () => {
  assert.equal(
    existsSync('src/lib/ai-context/ai-context.ts'),
    false,
    'ai-context.ts is an alias-only compatibility barrel; import its focused modules directly'
  )
})

test('storage consumers use focused owner modules', () => {
  assert.equal(
    existsSync('src/lib/storage-utils.ts'),
    false,
    'storage-utils.ts combines unrelated storage responsibilities behind a facade'
  )
  for (const compiledPath of [
    'extension/lib/storage-utils.js',
    'extension/lib/storage-utils.js.map',
    'extension/lib/storage-utils.d.ts',
    'extension/lib/storage-utils.d.ts.map'
  ]) {
    assert.equal(existsSync(compiledPath), false, `${compiledPath} is a stale compiled facade`)
  }
})

test('cross-context messages require the canonical page nonce', () => {
  for (const path of [
    'src/content/message-handlers.ts',
    'src/content/script-injection.ts',
    'src/content/window-message-listener.ts',
    'src/inject/message-handlers.ts',
    'src/inject/state.ts'
  ]) {
    const source = readFileSync(path, 'utf8')
    assert.doesNotMatch(source, /backwards? compat/i, `${path} retains a nonce migration branch`)
    assert.doesNotMatch(source, /\b(?:nonce|pageNonce)\s*&&[^;\n]*_nonce/, `${path} accepts a missing nonce`)
  }
})

test('page bridge uses one canonical authenticated contract and dispatcher', () => {
  const runtimeTypes = readFileSync('src/types/runtime-messages.ts', 'utf8')
  const contentTypes = readFileSync('src/content/types.ts', 'utf8')
  const contentListener = readFileSync('src/content/window-message-listener.ts', 'utf8')
  const injectDispatcher = readFileSync('src/inject/message-handlers.ts', 'utf8')
  const injectState = readFileSync('src/inject/state.ts', 'utf8')

  for (const messageType of [
    'kaboom_computed_styles_query',
    'kaboom_computed_styles_response',
    'kaboom_form_discovery_query',
    'kaboom_form_discovery_response'
  ]) {
    assert.match(runtimeTypes, new RegExp(`['"]${messageType}['"]`), `${messageType} is missing from the contract`)
  }
  assert.match(runtimeTypes, /interface PageMessageEventData[\s\S]*readonly _nonce:\s*string/)
  assert.doesNotMatch(contentTypes, /interface PageMessageEventData/, 'content must not redeclare the canonical envelope')
  assert.match(contentListener, /from ['"]\.\.\/types\/runtime-messages\.js['"]/)
  assert.match(injectDispatcher, /kaboom_highlight_request:\s*\(data\)\s*=>/)
  assert.doesNotMatch(injectState, /addEventListener\(['"]message['"]/, 'state must not install a second dispatcher')
  assert.doesNotMatch(injectState, /data-kaboom-nonce/, 'state must not own a second nonce read')
})

test('WebSocket instrumentation does not re-export tracking APIs', () => {
  const source = readFileSync('src/lib/net/websocket.ts', 'utf8')
  assert.doesNotMatch(
    source,
    /export\s+(?:type\s+)?\{[^}]*\}\s+from\s+['"]\.\/websocket-tracking\.js['"]/s,
    'WebSocket tracking consumers must import websocket-tracking.ts directly'
  )
})
