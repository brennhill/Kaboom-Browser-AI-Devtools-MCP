---
feature: content-provenance
status: shipped
tool: interact
doc_type: qa-plan
feature_id: feature-content-provenance
last_reviewed: 2026-09-05
---

# QA Plan: Content Provenance

57 tests across three files, all against `tests/extension/provenance/provenance-fixture.js`. No real
browser is involved.

## Vocabulary — `tests/extension/provenance/provenance-vocabulary.test.js` (25 tests)

### Origin reduction (`toOrigin`)

| Behaviour | Why it matters |
| --- | --- |
| Drops path, query and fragment | Rule 13. A tracking `?uid=` in an iframe `src` must never reach the response. |
| Keeps a non-default port | A different port is a different origin. |
| Resolves a relative `src` against the embedding document | Otherwise a relative frame src has no origin at all. |
| Reports an opaque origin for `data:`, `about:blank`, `javascript:` | These are same-origin with nothing. |
| Unwraps a `blob:` URL to its creating origin | A blob would otherwise be unattributable. |
| Returns the empty string when the origin cannot be determined | An unknown origin is stated, not guessed. |

### Origin comparison (`sameOrigin`)

Identical origins match; a different scheme, host or port does not; **an opaque origin is
same-origin with nothing, including another opaque origin**; an unknown origin never matches.

### Classification (`classifyRegion`)

Each of the four classifications has a test, plus: a `data:` frame is `third_party_frame` (an
opaque origin is not the first party); anything absent from the initial document is
`post_load_injected` whoever served it; **unknown delivery timing falls back to origin evidence
rather than claiming initial delivery**; every classification the module names is producible; the
counter counts all four including empty ones; unavailable attribution says so rather than implying
first-party content.

### Imperative text

Each strong marker has a test (`override_prior_instructions`, `system_prompt_shape`,
`credential_disclosure`), the weak pair is tested corroborated, and **ordinary page copy is asserted
not to fire** — the false-positive case that would teach an agent to ignore the alert. The sample is
asserted to be a bounded, whitespace-collapsed excerpt.

## Delivery timing and collection — `tests/extension/provenance/content-provenance.test.js` (20 tests)

| Behaviour | Consequence if wrong |
| --- | --- |
| Reports **unknown, not false**, when it never observed the document | A `false` would be a false assurance that content was in the parsed document. |
| Markup added while still loading is initial-document content | Deferred/async script output would otherwise classify most of the web as injected. |
| Markup added after `load` is injected, and so is everything under it | A subtree under an injected root is injected. |
| An injected text node is attributed to the element that now holds it | Otherwise text-only injections are invisible. |
| Records the origins of scripts and frames added after load | These are the candidate initiators. |
| The `load` event flips the boundary; `readyState: complete` starts past it | A tracker installed late must not claim to have watched the load. |
| Observes the whole subtree | Injection happens anywhere. |
| Bounds retention and **says so** rather than silently forgetting | The 200-root cap sets `overflowed`. |
| Reports **unknown, not false**, for unrecorded nodes once it stopped recording | Past the cap, `false` would tell a churning page every later injection was original content. |
| `disconnect` stops the observer and clears the active flag | A leaked observer would keep reporting on a dead document. |
| Names each region by how it reached the page | The core contract. |
| Separates post-load content from the initial document | The distinction the feature exists for. |
| Calls out imperative text that did **not** come from the first party | The asymmetric case. |
| Does **not** raise the alert for first-party imperative text | The same sentence is not the same event. |
| Names post-load injection that landed outside the extracted content | An injection just outside the extraction root is still worth reporting. |
| An inactive tracker yields unknown timing, never an assumed initial delivery | — |
| A subframe document is not reported as the first-party document | — |
| An empty extraction root still reports the document it came from | An absent block would read as a clean page. |
| **Emits no trust score** — asserts the serialized payload contains no `trust_score`, `trust_level`, `risk_score`, or `confidence` | A number would replace reading the evidence. |

## Frame attribution — `tests/extension/provenance/frame-provenance.test.js` (12 tests)

| Behaviour | Consequence if wrong |
| --- | --- |
| `list_interactive` stamps every element with its frame and that frame's origin | An ad iframe's button would be indistinguishable from the site's checkout button. |
| Each contributing frame is classified | — |
| Frame delivery timing is reported as unknown rather than assumed | — |
| Attribution is reported unavailable when the origin probe returned nothing | An absent block would read as clean. |
| Merging without provenance still returns the elements | Provenance must never cost the caller the answer it asked for. |
| An AX node inherits the frame of its nearest framed ancestor | — |
| A node with no framed ancestor reports **no frame** rather than guessing the main one | A guess would misattribute. |
| The frame tree is flattened to origins, dropping paths and query strings | Rule 13. |
| The `Page` domain is enabled through the lease | Otherwise the CDP session leaks an enabled domain. |
| `null` is returned when the frame tree cannot be read | Nothing is invented. |
| `find` classifies the frames its candidates actually came from | — |
| `find` reports attribution unavailable when the frame tree was not readable | — |

## Fallback path — `tests/extension/pilot/interact-content-fallback.test.js`

Covers the `executeScript` fallback used when the content script is unreachable: it emits
`attribution_available: false` with a reason rather than omitting the block.

## Verified manually, not automatically

| Behaviour | How to check |
| --- | --- |
| A real ad-network tag inserted after load is classified `post_load_injected` | Drive a page carrying real third-party tags and read `provenance.regions`. |
| A real cross-origin iframe's text is reported unreadable rather than silently empty | — |
| The `list_interactive` frame probe's added latency on a frame-heavy page | One extra `executeScript` per call; measure on a page with 20+ frames. |
| The imperative-text markers against real page copy at scale | The false-positive rate is the whole value of the signal, and no corpus test exists. |

## Not covered today

| Gap | Consequence if wrong |
| --- | --- |
| No test runs against real Chrome. The fixture models `MutationObserver`, frames, and the CDP frame tree. | A behaviour Chrome has that the fixture does not — same-origin frame timing, `srcdoc` frames, nested browsing contexts — would pass here and fail in the browser. |
| No corpus measures the imperative-text false-positive rate on ordinary web copy. | Markers that fire on normal pages make the alert worthless; only a single hand-written negative case guards this. |
| The Go side is untouched: extraction results ride `SyncCommandResult.result` typed `unknown`, so nothing on the server asserts the shape of what arrives. | A field renamed in TypeScript reaches the agent silently changed, with no Go-side contract test to fail. |
| The asymmetric-case alert appears inside the JSON, not as its own MCP text block. | An agent that reads only the summary line will not see it. |

## See also

- [Content Provenance index](./index.md)
- [Content Provenance Product Spec](./product-spec.md)
- [Content Provenance Tech Spec](./tech-spec.md)
