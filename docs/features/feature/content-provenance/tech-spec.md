---
feature: content-provenance
status: shipped
tool: interact
doc_type: tech-spec
feature_id: feature-content-provenance
last_reviewed: 2026-09-05
---

# Tech Spec: Content Provenance

## Components

| Component | Responsibility |
| --- | --- |
| `src/lib/provenance/provenance-types.ts` | The shared contract between content and background. |
| `src/lib/provenance/origins.ts` | `toOrigin` — the single point at which a URL is reduced to scheme, host and port. |
| `src/lib/provenance/classify.ts` | `classifyRegion` — maps frame and timing facts to one of four classifications. |
| `src/lib/provenance/imperative-text.ts` | Five named markers for agent-directed text, with bounded patterns. |
| `src/content/provenance/post-load-tracker.ts` | `PostLoadInjectionTracker` — a `MutationObserver` from `document_start`, with the `load` event as its boundary. |
| `src/content/provenance/collect.ts` | Walks an extraction root and produces one region per document, embedded frame, and post-load insertion. |
| `src/content/extractors/{readable,markdown,page-summary}.ts` | Attach the region list to their own responses. |
| `src/background/dom/dom-result-reconcile.ts`, `dom/primitives/dom-frame-probe.ts` | Stamp `list_interactive` elements with frame identity. |
| `src/background/dom/cdp/cdp-ax-tree.ts` | Joins `Accessibility.getFullAXTree` to `Page.getFrameTree` so `find` candidates carry a frame origin. |

`src/lib/provenance/` is pure — no DOM, no `chrome` APIs — so both `src/content` and
`src/background` can import it without crossing the `background → content` ban in
`.architecture-boundaries.json`.

## Classification precedence

`classifyRegion` applies four rules, most specific first:

```
1. delivered_in_initial_document === false     → post_load_injected
2. !sameOrigin(origin, document_origin)        → third_party_frame
3. is_frame || !is_top_level_document          → same_origin_subresource
4. otherwise                                   → first_party_document
```

Timing wins over origin because it is the fact an agent cannot recover from the payload it is
handed. A same-origin script writing an attacker's text into the page after load is still an
injection, and rule 1 says so.

`sameOrigin('null', 'null')` returns **false**. Treating two opaque origins as equal would let a
`data:` or sandboxed iframe classify as `first_party_document`.

The headline never replaces the facts: every region carries `origin`, `is_frame`,
`is_top_level_document`, `delivered_in_initial_document` and `initiator_origin` alongside its
classification, so a cross-origin frame injected after load still reports both halves.

## Delivery timing

`PostLoadInjectionTracker` observes from `document_start` and treats `load` as the boundary.
Deferred and async scripts are part of getting the page up, so counting their output as an
injection would classify most of the web as injected.

| Answer | Means |
| --- | --- |
| `false` | Present in the document Chrome parsed. |
| `true` | Written in after `load`. |
| `null` | The observer was not running for this document, or the retention cap was passed. |

`MAX_TRACKED_ROOTS` is 200. Passing it sets `capped` and is reported through `overflowed`; from
that point the tracker answers `null` rather than `false`. Answering `false` past the cap would
tell a page that churns enough to blow it that every later injection was initial-document content —
a false assurance in exactly the case the feature exists for. Detached roots are pruned once before
the cap is declared.

## Imperative-text markers

Five, deliberately explicit. A fuzzy match that fires on ordinary page copy teaches an agent to
ignore the alert, which is worse than never having had it.

| Marker | Strength |
| --- | --- |
| `override_prior_instructions` | strong — fires alone |
| `system_prompt_shape` | strong |
| `credential_disclosure` | strong |
| `addresses_an_agent` | weak — needs corroboration |
| `agent_directive` | weak — needs corroboration |

Each pattern is bounded (no unbounded repetition between anchors), so scanning stays linear in text
length. Scanning stops at 200,000 characters; the excerpt keeps 80 characters of context per side
and is capped at 200 characters, so a hostile page cannot pad the response.

`provenance.imperative_text_from_non_first_party` fires only for regions that are **not**
`first_party_document`. The same sentence in the page the user asked for is page copy.

## Where it is surfaced

| Surface | Carries |
| --- | --- |
| `get_readable`, `get_markdown`, `page_summary` | Full region list with per-region imperative-text evidence |
| `list_interactive` | `frame_id` and `frame_origin` on every element, plus one region per contributing frame |
| `find` | `frame_origin` per candidate (AX nodes inherit their nearest framed ancestor's frame), plus one region per frame |

Provenance the agent must make a second call to fetch is provenance it will not fetch, which is why
it rides in the extraction payload rather than behind its own mode.

## Failure modes

| Failure | Behaviour |
| --- | --- |
| Content script unreachable — the `executeScript` fallback runs | `attribution_available: false`, empty `regions`, reason in `notes`. The injected function is self-contained with no observer behind it. |
| Frame-origin probe or `Page.getFrameTree` fails | Same: `attribution_available: false` with a reason. |
| Cross-origin frame | Reported as a region and marked unreadable; its text is not read. |
| Retention cap passed | `delivered_in_initial_document: null`, `overflowed` set. |

An absent provenance block would read as a clean first-party page, so no path omits it.

## Privacy

`toOrigin` is the single reduction point, and it keeps only scheme, host and port. No path, query
string or fragment is recorded (rule 13). Origins and frame identity are captured data and stay
local; nothing here is transmitted (rule 7).

## Costs this added

| Surface | Cost |
| --- | --- |
| `list_interactive` | One extra `chrome.scripting.executeScript` (allFrames, MAIN world, two fields) per call to probe frame origins |
| `find` | One `Page.getFrameTree` per call, and enabling the `Page` CDP domain |

Folding the `list_interactive` probe into the existing call would first require splitting
`domPrimitiveListInteractive`, which is baselined at 750 lines in
`.function-length-baseline-ts.json`.

## See also

- [Content Provenance index](./index.md)
- [Content Provenance Product Spec](./product-spec.md)
- [Content Provenance QA Plan](./qa-plan.md)
