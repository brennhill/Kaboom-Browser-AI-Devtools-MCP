// @ts-nocheck
/**
 * @fileoverview frame-provenance.test.js — Frame attribution for element and AX snapshots (kaboom-x0li.3).
 *
 * list_interactive merges every frame's elements into one flat list, and the accessibility tree
 * spans frames too. Without frame attribution an agent cannot tell the site's own checkout button
 * from a button drawn by an ad iframe, because both arrive in the same array.
 */

import { describe, test } from 'node:test'
import assert from 'node:assert'

const { mergeListInteractive } = await import('../../../extension/background/dom/dom-result-reconcile.js')
const { fetchAXNodes, fetchFrameOrigins, axProvenance } = await import(
  '../../../extension/background/dom/cdp/cdp-ax-tree.js'
)

/** Fake lease mirroring extension/background/__tests__/cdp-ax-tree.test.js. */
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

const injection = (frameId, elements) => ({ frameId, result: { success: true, elements } })

describe('mergeListInteractive — which frame each element came from', () => {
  const origins = new Map([
    [0, { origin: 'https://shop.example', is_top_level_document: true }],
    [7, { origin: 'https://ads.example', is_top_level_document: false }]
  ])

  test('stamps every element with its frame and that frame’s origin', () => {
    const merged = mergeListInteractive(
      [injection(0, [{ element_id: 'e1', text: 'Checkout' }]), injection(7, [{ element_id: 'e2', text: 'Claim prize' }])],
      origins
    )
    assert.strictEqual(merged.elements.length, 2)
    assert.strictEqual(merged.elements[0].frame_id, 0)
    assert.strictEqual(merged.elements[0].frame_origin, 'https://shop.example')
    assert.strictEqual(merged.elements[1].frame_id, 7)
    assert.strictEqual(merged.elements[1].frame_origin, 'https://ads.example')
  })

  test('classifies each contributing frame', () => {
    const merged = mergeListInteractive(
      [injection(0, [{ element_id: 'e1' }]), injection(7, [{ element_id: 'e2' }])],
      origins
    )
    const byOrigin = new Map(merged.provenance.regions.map((region) => [region.origin, region]))
    assert.strictEqual(byOrigin.get('https://shop.example').classification, 'first_party_document')
    assert.strictEqual(byOrigin.get('https://ads.example').classification, 'third_party_frame')
    assert.strictEqual(merged.provenance.region_counts.third_party_frame, 1)
  })

  // Frame timing is not observable from the background, and claiming it was is worse than a gap.
  test('reports frame delivery timing as unknown rather than assuming it', () => {
    const merged = mergeListInteractive([injection(0, [{ element_id: 'e1' }])], origins)
    assert.strictEqual(merged.provenance.regions[0].delivered_in_initial_document, null)
  })

  test('says attribution is unavailable when the origin probe returned nothing', () => {
    const merged = mergeListInteractive([injection(0, [{ element_id: 'e1' }])], new Map())
    assert.strictEqual(merged.provenance.attribution_available, false)
    assert.strictEqual(merged.elements[0].frame_origin, undefined)
  })

  test('merging without provenance still returns the elements', () => {
    const merged = mergeListInteractive([injection(0, [{ element_id: 'e1' }])])
    assert.strictEqual(merged.success, true)
    assert.strictEqual(merged.elements.length, 1)
    assert.strictEqual(merged.provenance.attribution_available, false)
  })
})

describe('fetchAXNodes — frame attribution', () => {
  const axNode = (over = {}) => ({
    nodeId: '1',
    ignored: false,
    role: { type: 'role', value: 'button' },
    name: { type: 'computedString', value: 'Submit' },
    properties: [],
    backendDOMNodeId: 100,
    ...over
  })

  test('a node inherits the frame of its nearest framed ancestor', async () => {
    const nodes = [
      axNode({ nodeId: 'root', frameId: 'FRAME_MAIN', childIds: ['adframe', 'own'], backendDOMNodeId: 1 }),
      axNode({ nodeId: 'own', backendDOMNodeId: 2, name: { value: 'Checkout' } }),
      axNode({ nodeId: 'adframe', frameId: 'FRAME_AD', childIds: ['adbutton'], backendDOMNodeId: 3 }),
      axNode({ nodeId: 'adbutton', backendDOMNodeId: 4, name: { value: 'Claim prize' } })
    ]
    const lease = makeLease({ 'Accessibility.getFullAXTree': { nodes } })
    const parsed = await fetchAXNodes(lease.lease)
    const byName = new Map(parsed.map((node) => [node.name, node.frame_id]))
    assert.strictEqual(byName.get('Checkout'), 'FRAME_MAIN')
    assert.strictEqual(byName.get('Claim prize'), 'FRAME_AD', 'a node under an iframe belongs to that frame')
  })

  test('a node with no framed ancestor reports no frame rather than guessing the main one', async () => {
    const lease = makeLease({ 'Accessibility.getFullAXTree': { nodes: [axNode()] } })
    const [node] = await fetchAXNodes(lease.lease)
    assert.strictEqual(node.frame_id, undefined)
  })
})

describe('fetchFrameOrigins', () => {
  const frameTree = {
    frameTree: {
      frame: { id: 'FRAME_MAIN', url: 'https://shop.example/cart?token=secret' },
      childFrames: [
        { frame: { id: 'FRAME_AD', url: 'https://ads.example/unit?uid=9f2c' } },
        {
          frame: { id: 'FRAME_REVIEWS', url: 'https://shop.example/reviews' },
          childFrames: [{ frame: { id: 'FRAME_DEEP', url: 'https://tracker.example/pixel' } }]
        }
      ]
    }
  }

  test('flattens the frame tree to origins, dropping paths and query strings', async () => {
    const lease = makeLease({ 'Page.getFrameTree': frameTree })
    const frames = await fetchFrameOrigins(lease.lease)
    assert.strictEqual(frames.top_frame_id, 'FRAME_MAIN')
    assert.strictEqual(frames.origins.get('FRAME_MAIN'), 'https://shop.example')
    assert.strictEqual(frames.origins.get('FRAME_AD'), 'https://ads.example')
    assert.strictEqual(frames.origins.get('FRAME_DEEP'), 'https://tracker.example', 'nested frames are reached')
    assert.ok(!JSON.stringify([...frames.origins]).includes('token=secret'))
  })

  test('enables the Page domain through the lease', async () => {
    const lease = makeLease({ 'Page.getFrameTree': frameTree })
    await fetchFrameOrigins(lease.lease)
    assert.deepStrictEqual(lease.domains, ['Page'])
  })

  test('returns null when the frame tree cannot be read, so nothing is invented', async () => {
    const lease = makeLease({
      'Page.getFrameTree': () => {
        throw new Error('Target closed')
      }
    })
    assert.strictEqual(await fetchFrameOrigins(lease.lease), null)
  })
})

describe('axProvenance', () => {
  const frames = {
    top_frame_id: 'FRAME_MAIN',
    origins: new Map([
      ['FRAME_MAIN', 'https://shop.example'],
      ['FRAME_AD', 'https://ads.example']
    ])
  }

  test('classifies the frames the candidates actually came from', () => {
    const provenance = axProvenance(
      [
        { ref: 'ax_1', frame_id: 'FRAME_MAIN' },
        { ref: 'ax_2', frame_id: 'FRAME_AD' }
      ],
      frames
    )
    const byOrigin = new Map(provenance.regions.map((region) => [region.origin, region.classification]))
    assert.strictEqual(byOrigin.get('https://shop.example'), 'first_party_document')
    assert.strictEqual(byOrigin.get('https://ads.example'), 'third_party_frame')
  })

  test('reports attribution as unavailable when the frame tree was not readable', () => {
    const provenance = axProvenance([{ ref: 'ax_1', frame_id: 'FRAME_MAIN' }], null)
    assert.strictEqual(provenance.attribution_available, false)
    assert.deepStrictEqual(provenance.regions, [])
  })
})
