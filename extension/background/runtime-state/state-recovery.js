import { pushExtensionLog } from './log-queue.js';
export function reportStateRecovery(diagnostic) {
    pushExtensionLog({
        timestamp: new Date().toISOString(),
        level: 'warn',
        message: 'Persisted extension state recovered',
        source: 'background',
        category: 'state_recovery',
        data: { ...diagnostic }
    });
}
//# sourceMappingURL=state-recovery.js.map