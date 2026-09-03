// cdp-ax-tree.test.js — Accessibility-tree targeting (kaboom-05ue.4).
//
// Why this layer exists: list_interactive is a hand-rolled DOM heuristic that infers roles
// from tag names. On a canvas-drawn control, a custom grid, or any widget whose semantics
// live in ARIA rather than DOM shape, kaboom had no way to name the target — getFullAXTree
// appeared 0 times in the repo. Ranking is a pure function so it is testable with no browser.

import { describe, test } from 'node:test'
import assert from 'node:assert'

const { rankAXCandidates, normalizeQuery, roleMatchesQuery, AX_MIN_CONFIDENCE } = await import(
  '../dom/cdp/cdp-ax-tree.js'
)

const node = (over = {}) => ({
  ref: 'ax_1',
  role: 'button',
  name: '',
  value: '',
  states: [],
  x: 10,
  y: 20,
  width: 80,
  height: 30,
  ...over
})

describe('rankAXCandidates — accessible name', () => {
  test('an exact accessible-name match outranks a partial one', () => {
    const nodes = [
      node({ ref: 'a', name: 'Add to cart and checkout' }),
      node({ ref: 'b', name: 'Add to cart' })
    ]
    const [top] = rankAXCandidates(nodes, 'add to cart')
    assert.strictEqual(top.node.ref, 'b', 'the exact name must win over the longer partial')
  })

  test('matching is case and whitespace insensitive', () => {
    const nodes = [node({ ref: 'a', name: '  ADD  TO   CART ' })]
    const [top] = rankAXCandidates(nodes, 'add to cart')
    assert.strictEqual(top.node.ref, 'a')
    assert.ok(top.confidence >= AX_MIN_CONFIDENCE)
  })

  // The whole point of the AX layer: aria-label wins where the DOM text says something else.
  test('finds an element whose accessible name differs from its text', () => {
    const nodes = [
      node({ ref: 'icon', role: 'button', name: 'Close dialog' }),
      node({ ref: 'other', role: 'button', name: 'Submit' })
    ]
    const [top] = rankAXCandidates(nodes, 'close dialog')
    assert.strictEqual(top.node.ref, 'icon')
  })

  test('returns nothing rather than a bad guess when nothing matches', () => {
    const nodes = [node({ ref: 'a', name: 'Submit' }), node({ ref: 'b', name: 'Cancel' })]
    assert.deepStrictEqual(rankAXCandidates(nodes, 'nonexistent widget'), [])
  })
})

describe('rankAXCandidates — role', () => {
  test('a role word in the query steers toward that role', () => {
    const nodes = [
      node({ ref: 'link', role: 'link', name: 'Search' }),
      node({ ref: 'field', role: 'searchbox', name: 'Search' })
    ]
    const [top] = rankAXCandidates(nodes, 'search box')
    assert.strictEqual(top.node.ref, 'field', 'the role word must break the tie between equal names')
  })

  test('common phrasings map to roles', () => {
    assert.ok(roleMatchesQuery('searchbox', 'search bar'))
    assert.ok(roleMatchesQuery('textbox', 'input field'))
    assert.ok(roleMatchesQuery('button', 'submit button'))
    assert.ok(roleMatchesQuery('link', 'link to pricing'))
    assert.ok(!roleMatchesQuery('button', 'search bar'))
  })
})

describe('rankAXCandidates — actionability', () => {
  // A disabled control that matches perfectly is still the wrong thing to click, and an agent
  // that clicks it silently does nothing and then reasons about a page that never changed.
  test('a disabled element ranks below an enabled one with the same name', () => {
    const nodes = [
      node({ ref: 'off', name: 'Submit', states: ['disabled'] }),
      node({ ref: 'on', name: 'Submit' })
    ]
    const [top] = rankAXCandidates(nodes, 'submit')
    assert.strictEqual(top.node.ref, 'on')
  })

  test('a hidden element is excluded outright', () => {
    const nodes = [node({ ref: 'ghost', name: 'Submit', states: ['hidden'] })]
    assert.deepStrictEqual(rankAXCandidates(nodes, 'submit'), [])
  })

  test('a zero-area element is excluded — nothing can be clicked there', () => {
    const nodes = [node({ ref: 'collapsed', name: 'Submit', width: 0, height: 0 })]
    assert.deepStrictEqual(rankAXCandidates(nodes, 'submit'), [])
  })
})

describe('rankAXCandidates — result shape', () => {
  test('candidates are ordered by confidence and carry a reason', () => {
    const nodes = [
      node({ ref: 'a', name: 'Add to cart' }),
      node({ ref: 'b', name: 'Add to cart later' }),
      node({ ref: 'c', name: 'Cart' })
    ]
    const ranked = rankAXCandidates(nodes, 'add to cart')
    for (let i = 1; i < ranked.length; i++) {
      assert.ok(ranked[i - 1].confidence >= ranked[i].confidence, 'must be sorted by confidence')
    }
    for (const candidate of ranked) {
      assert.ok(typeof candidate.why === 'string' && candidate.why.length > 0, 'each hit explains itself')
      assert.ok(candidate.confidence > 0 && candidate.confidence <= 1)
    }
  })

  // Returning every weak match would let an agent blind-click the first hit. Ambiguity has
  // to survive into the response so it can be disambiguated instead of guessed.
  test('reports more than one candidate when the query is genuinely ambiguous', () => {
    const nodes = [
      node({ ref: 'a', name: 'Delete account' }),
      node({ ref: 'b', name: 'Delete post' })
    ]
    const ranked = rankAXCandidates(nodes, 'delete')
    assert.ok(ranked.length >= 2, 'an ambiguous query must not silently collapse to one answer')
  })

  test('an empty query matches nothing rather than everything', () => {
    const nodes = [node({ ref: 'a', name: 'Submit' })]
    for (const q of ['', '   ', null, undefined]) {
      assert.deepStrictEqual(rankAXCandidates(nodes, q), [], `query ${JSON.stringify(q)} must not match all`)
    }
  })

  test('handles an empty tree without throwing', () => {
    assert.deepStrictEqual(rankAXCandidates([], 'submit'), [])
    assert.deepStrictEqual(rankAXCandidates(null, 'submit'), [])
  })
})

describe('normalizeQuery', () => {
  test('collapses case, punctuation and whitespace so phrasing does not matter', () => {
    assert.deepStrictEqual(normalizeQuery('  Add   To-Cart!  '), ['add', 'to', 'cart'])
    assert.deepStrictEqual(normalizeQuery(''), [])
  })
})

// ---------------------------------------------------------------------------
// CDP snapshot
// ---------------------------------------------------------------------------

const { fetchAXNodes, resolveAXGeometry } = await import('../dom/cdp/cdp-ax-tree.js')

/** Fake lease. Records every CDP call so the domain contract can be asserted. */
function makeLease(responses = {}) {
  const calls = []
  const domains = []
  return {
    calls,
    domains,
    lease: {
      tabId: 1,
      valid: true,
      async ensureDomain(domain) {
        domains.push(domain)
      },
      async send(method, params) {
        calls.push({ method, params })
        const responder = responses[method]
        if (typeof responder === 'function') return responder(params)
        return responder ?? {}
      }
    }
  }
}

/** Chrome's AXNode wire shape: every value is wrapped in {type, value}. */
const axNode = (over = {}) => ({
  nodeId: '1',
  ignored: false,
  role: { type: 'role', value: 'button' },
  name: { type: 'computedString', value: 'Submit' },
  properties: [],
  backendDOMNodeId: 100,
  ...over
})

describe('fetchAXNodes', () => {
  test('enables the Accessibility domain through the lease, not per call', async () => {
    const f = makeLease({ 'Accessibility.getFullAXTree': { nodes: [axNode()] } })
    await fetchAXNodes(f.lease)
    assert.deepStrictEqual(f.domains, ['Accessibility'], 'domain enablement is the session manager’s job')
  })

  test('reads the real accessibility tree', async () => {
    const f = makeLease({ 'Accessibility.getFullAXTree': { nodes: [axNode()] } })
    const nodes = await fetchAXNodes(f.lease)
    assert.ok(
      f.calls.some((c) => c.method === 'Accessibility.getFullAXTree'),
      'must query Chrome’s accessibility tree rather than re-deriving it from the DOM'
    )
    assert.strictEqual(nodes.length, 1)
    assert.strictEqual(nodes[0].role, 'button')
    assert.strictEqual(nodes[0].name, 'Submit')
    assert.strictEqual(nodes[0].backend_node_id, 100)
  })

  test('drops nodes Chrome itself marks ignored', async () => {
    const f = makeLease({
      'Accessibility.getFullAXTree': {
        nodes: [axNode({ nodeId: '1' }), axNode({ nodeId: '2', ignored: true })]
      }
    })
    const nodes = await fetchAXNodes(f.lease)
    assert.strictEqual(nodes.length, 1, 'an ignored node is not reachable by assistive tech or by us')
  })

  test('carries disabled and checked state through as states', async () => {
    const f = makeLease({
      'Accessibility.getFullAXTree': {
        nodes: [
          axNode({
            properties: [
              { name: 'disabled', value: { type: 'boolean', value: true } },
              { name: 'checked', value: { type: 'tristate', value: 'true' } },
              { name: 'focusable', value: { type: 'boolean', value: false } }
            ]
          })
        ]
      }
    })
    const [node] = await fetchAXNodes(f.lease)
    assert.ok(node.states.includes('disabled'))
    assert.ok(node.states.includes('checked'))
    assert.ok(!node.states.includes('focusable'), 'a false-valued property is not a state')
  })

  test('a node with no backend DOM id is dropped — nothing can be acted on', async () => {
    const f = makeLease({
      'Accessibility.getFullAXTree': { nodes: [axNode({ backendDOMNodeId: undefined })] }
    })
    assert.deepStrictEqual(await fetchAXNodes(f.lease), [])
  })

  test('an empty or malformed reply yields no nodes rather than throwing', async () => {
    for (const reply of [{}, { nodes: null }, undefined]) {
      const f = makeLease({ 'Accessibility.getFullAXTree': reply })
      assert.deepStrictEqual(await fetchAXNodes(f.lease), [])
    }
  })
})

describe('resolveAXGeometry', () => {
  // Geometry is resolved only for ranked candidates. The AX tree carries no coordinates, and
  // a box-model round trip per node would be one CDP call per element on the page.
  test('resolves box models only for the nodes asked about', async () => {
    const f = makeLease({
      'DOM.getBoxModel': () => ({ model: { content: [10, 20, 110, 20, 110, 60, 10, 60] } })
    })
    const nodes = [
      { ref: 'a', role: 'button', name: 'Submit', states: [], backend_node_id: 100 },
      { ref: 'b', role: 'button', name: 'Cancel', states: [], backend_node_id: 200 }
    ]
    const resolved = await resolveAXGeometry(f.lease, [nodes[0]])
    assert.strictEqual(f.calls.filter((c) => c.method === 'DOM.getBoxModel').length, 1)
    assert.strictEqual(resolved[0].x, 60, 'x is the centre of the content box')
    assert.strictEqual(resolved[0].y, 40, 'y is the centre of the content box')
    assert.strictEqual(resolved[0].width, 100)
    assert.strictEqual(resolved[0].height, 40)
  })

  // A node scrolled out of the layout, or removed between snapshot and resolve, has no box.
  // Dropping it is correct; inventing 0,0 would send a click to the top-left of the page.
  test('drops a node whose box model cannot be read', async () => {
    const f = makeLease({
      'DOM.getBoxModel': () => {
        throw new Error('Could not compute box model')
      }
    })
    const resolved = await resolveAXGeometry(f.lease, [
      { ref: 'a', role: 'button', name: 'Submit', states: [], backend_node_id: 100 }
    ])
    assert.deepStrictEqual(resolved, [], 'never fabricate coordinates for an element with no box')
  })
})
