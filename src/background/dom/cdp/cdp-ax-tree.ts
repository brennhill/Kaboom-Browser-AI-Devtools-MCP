/**
 * Purpose: Targets elements by their ACCESSIBILITY semantics — role, accessible name, state —
 *          rather than by DOM shape, and resolves a natural-language query to candidates.
 * Why: list_interactive is a hand-rolled DOM scan that infers roles from tag names. On a
 *      canvas-drawn control, a custom grid, or any widget whose meaning lives in ARIA rather
 *      than markup, kaboom had no way to name the target at all. Accessibility.getFullAXTree
 *      appeared 0 times in this repo.
 * Docs: docs/features/feature/interact-explore/index.md
 */

// cdp-ax-tree.ts — Accessibility snapshot over CDP, and natural-language candidate ranking.

import type { Lease } from './cdp-session.js'
import { errorMessage } from '../../../lib/error-utils.js'

/** One actionable node from the accessibility tree. */
export interface AXNode {
  ref: string
  role: string
  name: string
  value?: string
  states: string[]
  /**
   * Viewport centre — the point CDP input would target. Undefined until resolveAXGeometry
   * runs: the accessibility tree carries no coordinates, and a box-model round trip for
   * every node on the page would cost one CDP call per element.
   */
  x?: number
  y?: number
  width?: number
  height?: number
  backend_node_id?: number
}

export interface AXCandidate {
  node: AXNode
  confidence: number
  why: string
}

/** Below this a match is noise, and reporting it would invite a blind click. */
export const AX_MIN_CONFIDENCE = 0.3

/** States that make an element unactionable regardless of how well its name matches. */
const HIDDEN_STATES = new Set(['hidden', 'ignored', 'invisible'])

/**
 * Phrasings people actually use, mapped to ARIA roles.
 *
 * Deliberately small and explicit: a fuzzy role guess that silently retargets the agent is
 * worse than no role signal, because the resulting click looks successful.
 */
const ROLE_SYNONYMS: Record<string, readonly string[]> = {
  searchbox: ['search', 'searchbar', 'searchbox', 'searchfield'],
  textbox: ['input', 'field', 'textbox', 'textfield', 'textarea', 'box'],
  button: ['button', 'btn', 'submit'],
  link: ['link', 'anchor', 'hyperlink'],
  checkbox: ['checkbox', 'check', 'tickbox'],
  radio: ['radio', 'radiobutton'],
  combobox: ['dropdown', 'select', 'combobox', 'picker'],
  tab: ['tab'],
  menuitem: ['menu', 'menuitem'],
  slider: ['slider', 'range'],
  switch: ['switch', 'toggle']
}

/** Split a query into comparable tokens, discarding case, punctuation and spacing. */
export function normalizeQuery(query: string | null | undefined): string[] {
  if (typeof query !== 'string') return []
  return query
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, ' ')
    .trim()
    .split(/\s+/)
    .filter(Boolean)
}

/** True when the query names the role, e.g. "search bar" for role searchbox. */
export function roleMatchesQuery(role: string, query: string): boolean {
  const tokens = normalizeQuery(query)
  if (tokens.length === 0) return false
  const synonyms = ROLE_SYNONYMS[role.toLowerCase()]
  if (!synonyms) return tokens.includes(role.toLowerCase())
  return tokens.some((token) => synonyms.includes(token))
}

function isActionable(node: AXNode): boolean {
  if (node.states.some((state) => HIDDEN_STATES.has(state.toLowerCase()))) return false
  // Unknown geometry is not a reason to exclude — it is simply not resolved yet. A KNOWN
  // zero area is: there is no point to click, however well the name reads.
  if (node.width === undefined || node.height === undefined) return true
  return node.width > 0 && node.height > 0
}

/** Score the accessible name against the query. 1 is an exact match. */
function scoreName(name: string, queryTokens: string[]): { score: number; why: string } {
  const nameTokens = normalizeQuery(name)
  if (nameTokens.length === 0) return { score: 0, why: '' }

  const nameJoined = nameTokens.join(' ')
  const queryJoined = queryTokens.join(' ')
  if (nameJoined === queryJoined) return { score: 1, why: 'exact accessible name' }
  if (nameJoined.includes(queryJoined)) {
    // Prefer the shortest containing name: "Add to cart" beats "Add to cart and checkout".
    const excess = nameTokens.length - queryTokens.length
    return { score: 0.9 - Math.min(excess, 8) * 0.05, why: 'accessible name contains the query' }
  }

  const matched = queryTokens.filter((token) => nameTokens.includes(token)).length
  if (matched === 0) return { score: 0, why: '' }
  return {
    score: 0.55 * (matched / queryTokens.length),
    why: `${matched} of ${queryTokens.length} query words in the accessible name`
  }
}

/**
 * Rank accessibility nodes against a natural-language query.
 *
 * Pure, so it is testable with no browser. Returns EVERY candidate above the confidence
 * floor rather than one answer: an ambiguous query must stay ambiguous in the response so
 * the caller can disambiguate instead of blind-clicking the first hit.
 */
export function rankAXCandidates(
  nodes: readonly AXNode[] | null | undefined,
  query: string | null | undefined
): AXCandidate[] {
  const queryTokens = normalizeQuery(query)
  if (!nodes || queryTokens.length === 0) return []

  const candidates: AXCandidate[] = []
  for (const node of nodes) {
    if (!isActionable(node)) continue

    const named = scoreName(node.name, queryTokens)
    const roleHit = roleMatchesQuery(node.role, queryTokens.join(' '))

    // A role word alone is a weak signal; it refines a name match rather than standing in
    // for one, or "button" would match every button on the page.
    let score = named.score
    let why = named.why
    if (roleHit && score > 0) {
      score = Math.min(1, score + 0.1)
      why = why ? `${why}; role ${node.role} matches` : `role ${node.role} matches`
    } else if (roleHit && queryTokens.length === 1) {
      score = 0.35
      why = `role ${node.role} matches`
    }

    // Disabled controls are reported but demoted: clicking one does nothing, and an agent
    // that does so then reasons about a page that never changed.
    if (node.states.some((state) => state.toLowerCase() === 'disabled')) {
      score *= 0.5
      why = why ? `${why} (disabled)` : 'disabled'
    }

    if (score >= AX_MIN_CONFIDENCE) {
      candidates.push({ node, confidence: Math.round(score * 100) / 100, why })
    }
  }

  return candidates.sort((a, b) => b.confidence - a.confidence)
}

// =============================================================================
// CDP SNAPSHOT
// =============================================================================

/** Chrome's AXValue wrapper: every field arrives as { type, value }. */
interface AXValue {
  type?: string
  value?: unknown
}

interface RawAXNode {
  nodeId?: string
  ignored?: boolean
  role?: AXValue
  name?: AXValue
  value?: AXValue
  backendDOMNodeId?: number
  properties?: Array<{ name?: string; value?: AXValue }>
}

function axString(value: AXValue | undefined): string {
  return typeof value?.value === 'string' ? value.value : ''
}

/**
 * Which AX properties count as "states".
 *
 * A property that is present but false is not a state — `focusable: false` says the element
 * is NOT focusable, and recording it as a state would invert its meaning.
 */
function statesFrom(properties: RawAXNode['properties']): string[] {
  const states: string[] = []
  for (const property of properties ?? []) {
    const name = property.name
    if (!name) continue
    const raw = property.value?.value
    const truthy = raw === true || raw === 'true' || raw === 'mixed'
    if (truthy) states.push(name.toLowerCase())
  }
  return states
}

/**
 * Read the page's accessibility tree.
 *
 * This is the semantic view assistive technology sees, so it names controls the DOM cannot:
 * a canvas-drawn widget with ARIA attributes, an aria-label that differs from visible text,
 * a role overridden on a plain div.
 */
export async function fetchAXNodes(lease: Lease): Promise<AXNode[]> {
  await lease.ensureDomain('Accessibility')
  const reply = (await lease.send('Accessibility.getFullAXTree', {})) as { nodes?: RawAXNode[] } | undefined
  const raw = Array.isArray(reply?.nodes) ? reply.nodes : []

  const nodes: AXNode[] = []
  for (const item of raw) {
    // Chrome marks nodes it excludes from the accessibility tree. They are unreachable by
    // assistive technology, so they are not legitimate targets for us either.
    if (item.ignored === true) continue
    // Without a backend DOM id there is nothing to resolve geometry against, so the node
    // could be named but never acted upon.
    if (typeof item.backendDOMNodeId !== 'number') continue

    nodes.push({
      ref: `ax_${item.backendDOMNodeId}`,
      role: axString(item.role),
      name: axString(item.name),
      value: axString(item.value) || undefined,
      states: statesFrom(item.properties),
      backend_node_id: item.backendDOMNodeId
    })
  }
  return nodes
}

/**
 * Fill in viewport geometry for the nodes given, dropping any whose box cannot be read.
 *
 * Called for ranked candidates only. A node scrolled out of layout, or removed between the
 * snapshot and this call, has no box model — dropping it is correct, because inventing 0,0
 * would send a click to the top-left corner of the page.
 */
export async function resolveAXGeometry(lease: Lease, nodes: readonly AXNode[]): Promise<AXNode[]> {
  const resolved: AXNode[] = []
  for (const node of nodes) {
    if (typeof node.backend_node_id !== 'number') continue
    try {
      const reply = (await lease.send('DOM.getBoxModel', { backendNodeId: node.backend_node_id })) as {
        model?: { content?: number[] }
      }
      const quad = reply?.model?.content
      if (!Array.isArray(quad) || quad.length < 8) continue
      // A content quad is four x,y corners, flattened. Bail on any non-finite value rather
      // than deriving a centre from NaN, which would place a click at an arbitrary point.
      const points = quad.slice(0, 8)
      if (!points.every((n) => typeof n === 'number' && Number.isFinite(n))) continue
      const xs = points.filter((_, index) => index % 2 === 0)
      const ys = points.filter((_, index) => index % 2 === 1)
      const left = Math.min(...xs)
      const right = Math.max(...xs)
      const top = Math.min(...ys)
      const bottom = Math.max(...ys)
      resolved.push({
        ...node,
        x: Math.round((left + right) / 2),
        y: Math.round((top + bottom) / 2),
        width: Math.round(right - left),
        height: Math.round(bottom - top)
      })
    } catch (err) {
      // EXPECTED_ABSENCE: an element scrolled out of layout or removed since the snapshot
      // normally has no box model. Dropping it is the correct answer, and logging would
      // report routine page churn as a failure on every call.
      void errorMessage(err, 'no_box_model')
    }
  }
  return resolved
}
