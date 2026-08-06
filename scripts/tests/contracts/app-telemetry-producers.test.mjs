// app-telemetry-producers.test.mjs — Enforces the canonical outbound producer boundary.

import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { resolve } from 'node:path'
import test from 'node:test'

const root = resolve(import.meta.dirname, '../../..')

async function source(relativePath) {
  return readFile(resolve(root, relativePath), 'utf8')
}

test('extension and shell lifecycle code cannot post product telemetry', async () => {
  const paths = [
    'src/background/init.ts',
    'src/background/sync/sync-client.ts',
    'scripts/setup/install.sh',
    'scripts/setup/uninstall.sh'
  ]
  for (const path of paths) {
    const content = await source(path)
    assert.doesNotMatch(content, /t\.gokaboom\.dev|telemetry-beacon|BeaconEvent|beacon\s*\(/, path)
  }
})

test('generic Go lifecycle beacon surface stays deleted', async () => {
  const content = await source('internal/telemetry/beacon.go')
  assert.doesNotMatch(content, /func BeaconEvent\b|func sendBeacon\b/)
})

test('runtime errors use the closed incident registry', async () => {
  const beacon = await source('internal/telemetry/beacon.go')
  const registry = await source('internal/incident/registry.go')
  assert.match(beacon, /func AppError\(code incident\.Code\)/)
  assert.doesNotMatch(beacon, /classifyAppError|normalizeAppErrorCode/)
  assert.match(registry, /PrivacyBoundedProductMetadata/)
  assert.match(registry, /CodeBridgeSpawnTimeout/)
  assert.match(beacon, /"outcome":\s+string\(event\.Outcome\)/)
  assert.match(beacon, /"attempt_bucket":\s+string\(event\.AttemptBucket\)/)
  assert.match(beacon, /"latency_bucket":\s+string\(event\.LatencyBucket\)/)
})

test('canonical contract names exactly match the structured-event allowlist', async () => {
  const content = await source('internal/telemetry/usage_counter.go')
  const match = content.match(
    /case "tool_call", "first_tool_call", "session_start", "session_end", "usage_summary", "app_error":/
  )
  assert.ok(match, 'structured telemetry allowlist drifted from docs/core/quality/app-metrics.md')
})
