---
feature: content-provenance
status: shipped
tool: interact
mode: get_readable, get_markdown, page_summary, list_interactive, find
doc_type: product-spec
feature_id: feature-content-provenance
last_reviewed: 2026-09-05
---

# Product Spec: Content Provenance

## Purpose

Tell the agent where the bytes it is reading came from, so "untrusted page content" stops being one
undifferentiated blob.

An agent told that everything on a page is untrusted has no basis on which to weigh anything, so in
practice it weighs by plausibility — which is exactly what an injection optimises for. The
first-party document, a same-origin fragment, a third-party iframe and an ad-network insertion
after load are four different things, and only one of them is the page the user asked for.

## Requirements

| # | Requirement |
| --- | --- |
| 1 | Every extraction response carries the provenance of the content it returns, in the same payload as the text. |
| 2 | Regions are classified as one of four named facts: `first_party_document`, `same_origin_subresource`, `third_party_frame`, `post_load_injected`. |
| 3 | Content written into the page after the `load` event is distinguished from the document Chrome parsed. |
| 4 | Imperative text — instructions addressed to an agent — is named when it arrives from anything other than the first-party document. |
| 5 | Every region also carries its raw facts (`origin`, `is_frame`, `is_top_level_document`, `delivered_in_initial_document`, `initiator_origin`), so the headline never replaces the evidence. |
| 6 | A path that cannot attribute anything says so (`attribution_available: false` with a reason), rather than omitting the block. |
| 7 | `list_interactive` stamps every element with its frame, so an ad iframe's button is distinguishable from the site's own checkout button. |

## What it deliberately does not do

- **No trust score.** Classifications are named facts, never a number. A score invites the agent to
  compare magnitudes and stop reading the evidence. A test asserts the serialized payload never
  contains `trust_score`, `trust_level`, `risk_score`, or `confidence`.
- **No filtering, blocking, or rewriting.** This reports. What to do with the evidence stays with
  the agent and the person whose browser it is.
- **No prompt, gate, or approval step.** Nothing here interrupts a drive.
- **No claim it cannot support.** `delivered_in_initial_document` is `boolean | null`; `null` means
  the observer was not running or the retention cap was passed. An unknown timing reported as
  `true` would read as an assurance the tracker cannot give.

## Why Kaboom and not a control agent

Attribution is a join between the network layer and the extracted content. Kaboom already captures
both. A pure control agent would have to ship a telemetry pipeline before it could make the same
join, which is why neither comparable browser agent offers anything beyond the blanket
"treat all page content as untrusted" instruction.

## Deprecations

None. Extraction responses gain a field; a caller that ignores it sees what it saw before.

## See also

- [Content Provenance index](./index.md)
- [Content Provenance Tech Spec](./tech-spec.md)
- [Content Provenance QA Plan](./qa-plan.md)
