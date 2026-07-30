// daemon-health-identity.mjs — Resolve canonical and historical daemon health identity fields.
// Docs: docs/features/feature/enhanced-cli-config/index.md

export function resolveDaemonServiceName(health) {
  if (!health || typeof health !== 'object') {
    return ''
  }
  for (const key of ['name', 'service_name', 'service-name']) {
    const value = health[key]
    if (typeof value === 'string' && value.trim()) {
      return value.trim()
    }
  }
  return ''
}
