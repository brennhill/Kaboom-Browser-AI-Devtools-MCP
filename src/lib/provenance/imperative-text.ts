/**
 * Purpose: Detect text shaped like instructions addressed to an agent, and report what matched.
 * Why: Imperative text arriving from anything other than the first-party document is the shape of
 *      an injection. Naming it is the asymmetric case the provenance layer exists to surface — and
 *      it is only ever named. Nothing here filters, blocks, or rewrites content.
 * Docs: docs/features/feature/content-provenance/index.md
 */

// imperative-text.ts — Named markers for agent-directed imperative text.

import type { ImperativeTextEvidence } from './provenance-types.js'

/** Characters of context kept on each side of the first match. */
const SAMPLE_CONTEXT = 80
/** Hard cap on the excerpt, so a hostile page cannot pad the response. */
const MAX_SAMPLE_CHARS = 200
/** Text beyond this is not scanned; a page that long is already over the extraction budget. */
const MAX_SCAN_CHARS = 200_000

interface MarkerPattern {
  name: string
  /** A strong marker is enough on its own. A weak one has to be corroborated. */
  strong: boolean
  pattern: RegExp
}

/**
 * Deliberately small and explicit.
 *
 * A fuzzy match that fires on ordinary page copy makes the signal worthless, and an agent that
 * learns to ignore the alert is worse off than one that never had it. Each pattern is bounded —
 * no unbounded repetition between anchors — so scanning cost stays linear in the text length.
 */
const MARKERS: readonly MarkerPattern[] = [
  {
    name: 'override_prior_instructions',
    strong: true,
    pattern:
      /\b(?:ignore|disregard|forget|override)\b[^.\n]{0,60}\b(?:previous|prior|earlier|above|all)\b[^.\n]{0,60}\b(?:instruction|prompt|rule|direction|command)s?\b/i
  },
  {
    name: 'system_prompt_shape',
    strong: true,
    pattern:
      /\bsystem\s*(?:prompt|message)\b|\bnew\s+instructions?\b|\bupdated\s+instructions?\b|\bdeveloper\s+message\b|<\s*\/?\s*(?:system|instructions?)\s*>/i
  },
  {
    name: 'credential_disclosure',
    strong: true,
    pattern:
      /\b(?:api[\s_-]?key|password|secret|access\s+token|credential|session\s+cookie)s?\b[^.\n]{0,60}\b(?:send|post|email|share|reveal|disclose|output|print|paste|upload)\b|\b(?:send|post|email|share|reveal|disclose|output|print|paste|upload)\b[^.\n]{0,60}\b(?:api[\s_-]?key|password|secret|access\s+token|credential|session\s+cookie)s?\b/i
  },
  {
    name: 'addresses_an_agent',
    strong: false,
    pattern:
      /\b(?:ai\s+(?:agent|assistant|model)|language\s+model|llm|chatbot|copilot|claude|chatgpt|gpt-?\d*|gemini|autonomous\s+agent|browser\s+agent)\b/i
  },
  {
    name: 'agent_directive',
    strong: false,
    pattern:
      /\byou\s+(?:must|should|shall|will|need\s+to|are\s+required\s+to)\b|\b(?:immediately|now)\s+(?:navigate|go|send|email|transfer|delete|execute|run|fetch|download|reveal|disclose|output|print)\b/i
  }
]

/** Collapse whitespace so a sample is one readable line regardless of the source markup. */
function collapse(text: string): string {
  return text.replace(/\s+/g, ' ').trim()
}

/**
 * Report imperative markers found in `text`, or `null` when none apply.
 *
 * A single strong marker is enough. Weak markers only count together: addressing an agent is
 * common in ordinary copy ("our assistant is available Monday to Friday") and a bare directive is
 * how every call-to-action on the web reads, but the two in the same region are not.
 */
export function detectImperativeText(text: string | null | undefined): ImperativeTextEvidence | null {
  // Whitespace is collapsed before matching: markup puts line breaks wherever it likes, and a
  // pattern that stopped at a newline would miss an injection merely because it wrapped.
  const source = collapse((text ?? '').slice(0, MAX_SCAN_CHARS))
  if (source === '') return null

  const markers: string[] = []
  let firstIndex = -1
  let hasStrong = false
  for (const marker of MARKERS) {
    const match = marker.pattern.exec(source)
    if (!match) continue
    markers.push(marker.name)
    if (marker.strong) hasStrong = true
    if (firstIndex < 0 || match.index < firstIndex) firstIndex = match.index
  }

  const corroborated = markers.includes('addresses_an_agent') && markers.includes('agent_directive')
  if (!hasStrong && !corroborated) return null

  const start = Math.max(0, firstIndex - SAMPLE_CONTEXT)
  return { markers, sample: source.slice(start, firstIndex + SAMPLE_CONTEXT * 2).slice(0, MAX_SAMPLE_CHARS) }
}
