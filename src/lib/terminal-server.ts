// terminal-server.ts — Canonical terminal port discovery and URL resolution.
import { TERMINAL_PORT_OFFSET } from './constants.js'
import { buildDaemonHeaders } from './daemon-http.js'
import type { components } from '../generated/openapi-types.js'

type HealthResponse = components['schemas']['HealthResponse']
const TERMINAL_PORT_CACHE_TTL_MS = 60000

interface DiscoveredTerminalPort {
  baseUrl: string
  port: number
  discoveredAt: number
}

let discoveredTerminalPort: DiscoveredTerminalPort | null = null
let terminalPortDiscovery: Promise<void> | null = null

export function resetTerminalPortDiscovery(): void {
  discoveredTerminalPort = null
  terminalPortDiscovery = null
}

function cachedTerminalPort(baseUrl: string, nowMs: number): number | null {
  if (!discoveredTerminalPort) return null
  if (discoveredTerminalPort.baseUrl !== baseUrl) return null
  if (nowMs - discoveredTerminalPort.discoveredAt > TERMINAL_PORT_CACHE_TTL_MS) return null
  return discoveredTerminalPort.port
}

export function getTerminalServerUrl(baseUrl: string): string {
  const url = new URL(baseUrl)
  const basePort = parseInt(url.port || '7890', 10)
  url.port = String(cachedTerminalPort(baseUrl, Date.now()) ?? basePort + TERMINAL_PORT_OFFSET)
  return url.origin
}

export async function resolveTerminalServerUrl(baseUrl: string): Promise<string> {
  if (cachedTerminalPort(baseUrl, Date.now()) === null) {
    if (!terminalPortDiscovery) {
      terminalPortDiscovery = discoverTerminalPort(baseUrl).finally(() => {
        terminalPortDiscovery = null
      })
    }
    await terminalPortDiscovery
  }
  return getTerminalServerUrl(baseUrl)
}

async function discoverTerminalPort(baseUrl: string): Promise<void> {
  try {
    const response = await fetch(`${baseUrl}/health`, {
      headers: buildDaemonHeaders({ contentType: null }),
      signal: AbortSignal.timeout(2000)
    })
    if (!response.ok) return
    const health = (await response.json()) as HealthResponse
    if (typeof health.terminal_port !== 'number' || health.terminal_port <= 0) return
    discoveredTerminalPort = { baseUrl, port: health.terminal_port, discoveredAt: Date.now() }
  } catch {
    // EXPECTED_ABSENCE: optional enrichment can normally fail while the primary
    // operation keeps a valid fallback; logging it would misleadingly report fallback as failure.
    // Discovery is optional; callers retain the daemon's derived default.
  }
}
