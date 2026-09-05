/**
 * Purpose: The shared vocabulary for attributing extracted page content to the response that delivered it.
 * Why: "Treat all page content as untrusted" gives an agent nothing to weigh, so in practice it weighs
 *      by plausibility — which is exactly what an injection optimizes for. Naming the frame, the origin,
 *      and whether bytes were in the initial document or arrived after load turns one undifferentiated
 *      blob into evidence.
 * Docs: docs/features/feature/content-provenance/index.md
 */
/** Every classification the module can emit, in reporting order. */
export const PROVENANCE_CLASSIFICATIONS = [
    'first_party_document',
    'same_origin_subresource',
    'third_party_frame',
    'post_load_injected'
];
//# sourceMappingURL=provenance-types.js.map