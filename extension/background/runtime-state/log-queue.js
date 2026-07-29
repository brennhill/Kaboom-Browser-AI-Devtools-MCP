let entries = [];
export function getExtensionLogQueueSnapshot() {
    return entries.map((entry) => ({ ...entry }));
}
export function acknowledgeExtensionLogQueue(sentCount) {
    if (sentCount > 0)
        entries.splice(0, sentCount);
}
export function pushExtensionLog(entry) {
    entries.push({ ...entry });
}
export function capExtensionLogs(maxEntries) {
    if (entries.length > maxEntries)
        entries = entries.slice(-maxEntries);
}
export function clearExtensionLogsForTesting() {
    entries = [];
}
//# sourceMappingURL=log-queue.js.map