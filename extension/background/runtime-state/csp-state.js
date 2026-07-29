let lastCSPStatus = { csp_restricted: false, csp_level: 'none' };
export function getLastCSPStatus() {
    return lastCSPStatus;
}
export function setLastCSPStatus(status) {
    lastCSPStatus = status;
}
//# sourceMappingURL=csp-state.js.map