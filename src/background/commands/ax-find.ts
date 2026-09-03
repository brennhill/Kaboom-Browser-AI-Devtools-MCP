/**
 * Purpose: Resolves a natural-language description to page elements using the accessibility
 *          tree, so an agent can name a target the DOM cannot express.
 * Why: Selectors fail on canvas-drawn controls, custom grids, and any widget whose semantics
 *      live in ARIA rather than markup. This is the targeting layer both comparable browser
 *      agents have and kaboom did not.
 * Docs: docs/features/feature/interact-explore/index.md
 */

// ax-find.ts — The `find` command: natural language -> accessibility candidates.

import { registerCommand } from './registry.js'
import { cdpSessions } from '../dom/cdp/cdp-session.js'
import { fetchAXNodes, rankAXCandidates, resolveAXGeometry } from '../dom/cdp/cdp-ax-tree.js'
import { errorMessage } from '../../lib/error-utils.js'

/**
 * How many ranked candidates get geometry resolved.
 *
 * Each costs one DOM.getBoxModel round trip, so this bounds the cost of a vague query
 * against a large page. Ambiguity still reaches the caller — it is just bounded.
 */
const MAX_RESOLVED_CANDIDATES = 10

registerCommand('ax_find', async (ctx) => {
  const query = typeof ctx.params.query === 'string' ? ctx.params.query : ''
  if (!query.trim()) {
    ctx.sendResult({
      error: 'missing_query',
      message: "find requires a 'query', for example query='add to cart button'."
    })
    return
  }

  const sessions = cdpSessions()
  if (!sessions) {
    ctx.sendResult({
      error: 'cdp_unavailable',
      message:
        'The accessibility tree needs the Chrome debugger, which is unavailable here. Use list_interactive instead.'
    })
    return
  }

  let lease
  try {
    lease = await sessions.acquire(ctx.tabId)
  } catch (err) {
    ctx.sendResult({ error: 'cdp_attach_failed', message: errorMessage(err, 'Could not attach the debugger') })
    return
  }

  try {
    const nodes = await fetchAXNodes(lease)
    // Rank before resolving geometry: ranking needs only role and name, and resolving every
    // node's box would be one CDP round trip per element on the page.
    const ranked = rankAXCandidates(nodes, query).slice(0, MAX_RESOLVED_CANDIDATES)
    const withGeometry = await resolveAXGeometry(
      lease,
      ranked.map((candidate) => candidate.node)
    )
    const byRef = new Map(withGeometry.map((node) => [node.ref, node]))

    // A candidate whose box could not be read is dropped rather than returned without
    // coordinates: a ref nothing can act on is worse than one fewer answer.
    const candidates = ranked
      .filter((candidate) => byRef.has(candidate.node.ref))
      .map((candidate) => {
        const node = byRef.get(candidate.node.ref)
        return {
          ref: candidate.node.ref,
          role: candidate.node.role,
          name: candidate.node.name,
          value: candidate.node.value,
          states: candidate.node.states,
          confidence: candidate.confidence,
          why: candidate.why,
          x: node?.x,
          y: node?.y,
          width: node?.width,
          height: node?.height
        }
      })

    ctx.sendResult({
      success: true,
      action: 'find',
      query,
      match_count: candidates.length,
      // Reported so the caller can tell "no such element" from "the page has no
      // accessibility tree", which are different problems with different fixes.
      ax_node_count: nodes.length,
      candidates,
      ambiguous: candidates.length > 1
    })
  } catch (err) {
    ctx.sendResult({
      error: 'ax_query_failed',
      message: errorMessage(err, 'Failed to read the accessibility tree')
    })
  } finally {
    lease.release()
  }
})
