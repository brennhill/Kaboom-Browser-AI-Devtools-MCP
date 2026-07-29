// terminal-server.ts — Canonical terminal port discovery and URL resolution.
import { TERMINAL_PORT_OFFSET } from './constants.js';
import { buildDaemonHeaders } from './daemon-http.js';
const TERMINAL_PORT_CACHE_TTL_MS = 60000;
let discoveredTerminalPort = null;
let terminalPortDiscovery = null;
export function resetTerminalPortDiscovery() {
    discoveredTerminalPort = null;
    terminalPortDiscovery = null;
}
function cachedTerminalPort(baseUrl, nowMs) {
    if (!discoveredTerminalPort)
        return null;
    if (discoveredTerminalPort.baseUrl !== baseUrl)
        return null;
    if (nowMs - discoveredTerminalPort.discoveredAt > TERMINAL_PORT_CACHE_TTL_MS)
        return null;
    return discoveredTerminalPort.port;
}
export function getTerminalServerUrl(baseUrl) {
    const url = new URL(baseUrl);
    const basePort = parseInt(url.port || '7890', 10);
    url.port = String(cachedTerminalPort(baseUrl, Date.now()) ?? basePort + TERMINAL_PORT_OFFSET);
    return url.origin;
}
export async function resolveTerminalServerUrl(baseUrl) {
    if (cachedTerminalPort(baseUrl, Date.now()) === null) {
        if (!terminalPortDiscovery) {
            terminalPortDiscovery = discoverTerminalPort(baseUrl).finally(() => {
                terminalPortDiscovery = null;
            });
        }
        await terminalPortDiscovery;
    }
    return getTerminalServerUrl(baseUrl);
}
async function discoverTerminalPort(baseUrl) {
    try {
        const response = await fetch(`${baseUrl}/health`, {
            headers: buildDaemonHeaders({ contentType: null }),
            signal: AbortSignal.timeout(2000)
        });
        if (!response.ok)
            return;
        const health = (await response.json());
        if (typeof health.terminal_port !== 'number' || health.terminal_port <= 0)
            return;
        discoveredTerminalPort = { baseUrl, port: health.terminal_port, discoveredAt: Date.now() };
    }
    catch {
        // Discovery is optional; callers retain the daemon's derived default.
    }
}
//# sourceMappingURL=terminal-server.js.map