const defaultStatus = {
    connected: false,
    entries: 0,
    maxEntries: 1000,
    errorCount: 0,
    logFile: '',
    securityMode: 'normal',
    productionParity: true,
    insecureRewritesApplied: []
};
let connectionStatus = { ...defaultStatus };
let connectionCheckRunning = false;
export function getConnectionStatus() {
    return Object.freeze({ ...connectionStatus });
}
export function setConnectionStatus(patch) {
    connectionStatus = { ...connectionStatus, ...patch };
}
export function isConnectionCheckRunning() {
    return connectionCheckRunning;
}
export function setConnectionCheckRunning(running) {
    connectionCheckRunning = running;
}
export function applyConnectionOverrides(overrides) {
    const rewrites = (overrides.insecure_rewrites_applied || '')
        .split(',')
        .map((value) => value.trim())
        .filter(Boolean);
    setConnectionStatus({
        securityMode: overrides.security_mode === 'insecure_proxy' ? 'insecure_proxy' : 'normal',
        productionParity: overrides.production_parity !== 'false',
        insecureRewritesApplied: rewrites
    });
}
//# sourceMappingURL=connection-state.js.map