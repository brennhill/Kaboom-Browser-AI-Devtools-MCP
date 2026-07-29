export const SEARCH_SYNONYMS: Record<string, string[]> = {
  'bug-triage': ['bug repro', 'bug reproduction', 'repro', 'triage', 'incident triage'],
  debugging: ['debug', 'troubleshoot', 'investigate error'],
  automation: ['workflow automation', 'auto-run', 'automate flow'],
  accessibility: ['a11y', 'wcag', 'screen reader checks'],
  'api-validation': ['contract testing', 'schema validation', 'response validation'],
  websocket: ['real-time', 'socket debugging', 'ws debugging'],
  performance: ['core web vitals', 'slow page', 'latency regression'],
  security: ['security audit', 'headers check', 'privacy checks'],
  annotations: ['ui notes', 'visual feedback', 'design feedback'],
  demos: ['demo recording', 'click-through demo', 'sales demo']
}

const aliasToCanonical = new Map<string, string>()
for (const [canonical, aliases] of Object.entries(SEARCH_SYNONYMS)) {
  aliasToCanonical.set(canonical, canonical)
  for (const alias of aliases) {
    aliasToCanonical.set(alias, canonical)
  }
}

export function normalizeTag(input: string): string {
  const raw = input
    .trim()
    .toLowerCase()
    .replace(/[_\s]+/g, '-')
  return aliasToCanonical.get(raw) ?? raw
}

export function normalizeTags(inputs: string[]): string[] {
  const out = new Set<string>()
  for (const input of inputs) {
    if (!input || !input.trim()) continue
    out.add(normalizeTag(input))
  }
  return [...out]
}
