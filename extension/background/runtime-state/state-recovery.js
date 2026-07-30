import { pushExtensionLog } from './log-queue.js';
export function reportStateRecovery(diagnostic, lifecycle = 'active') {
    pushExtensionLog({
        timestamp: new Date().toISOString(),
        level: lifecycle === 'active' ? 'warn' : 'info',
        message: lifecycle === 'active'
            ? 'Persisted extension state recovered with fallback'
            : 'Persisted extension state verified',
        source: 'background',
        category: 'state_recovery',
        data: { ...diagnostic, lifecycle }
    });
}
export function resolveStateRecovery(name) {
    reportStateRecovery({ name, detail: '', fix: '' }, 'recovered');
}
//# sourceMappingURL=state-recovery.js.map