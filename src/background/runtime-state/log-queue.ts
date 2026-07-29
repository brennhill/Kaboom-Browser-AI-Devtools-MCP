/**
 * Purpose: Own the bounded extension diagnostic log queue.
 * Why: Callers receive snapshots so they cannot mutate or retain the live queue.
 */
export interface ExtensionLogQueueEntry {
  timestamp: string
  level: string
  message: string
  source: string
  category: string
  data?: unknown
}

let entries: ExtensionLogQueueEntry[] = []
export function getExtensionLogQueueSnapshot(): ExtensionLogQueueEntry[] {
  return entries.map((entry) => ({ ...entry }))
}
export function acknowledgeExtensionLogQueue(sentCount: number): void {
  if (sentCount > 0) entries.splice(0, sentCount)
}
export function pushExtensionLog(entry: ExtensionLogQueueEntry): void {
  entries.push({ ...entry })
}
export function capExtensionLogs(maxEntries: number): void {
  if (entries.length > maxEntries) entries = entries.slice(-maxEntries)
}
export function clearExtensionLogsForTesting(): void {
  entries = []
}
