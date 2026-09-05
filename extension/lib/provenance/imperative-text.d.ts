/**
 * Purpose: Detect text shaped like instructions addressed to an agent, and report what matched.
 * Why: Imperative text arriving from anything other than the first-party document is the shape of
 *      an injection. Naming it is the asymmetric case the provenance layer exists to surface — and
 *      it is only ever named. Nothing here filters, blocks, or rewrites content.
 * Docs: docs/features/feature/content-provenance/index.md
 */
import type { ImperativeTextEvidence } from './provenance-types.js';
/**
 * Report imperative markers found in `text`, or `null` when none apply.
 *
 * A single strong marker is enough. Weak markers only count together: addressing an agent is
 * common in ordinary copy ("our assistant is available Monday to Friday") and a bare directive is
 * how every call-to-action on the web reads, but the two in the same region are not.
 */
export declare function detectImperativeText(text: string | null | undefined): ImperativeTextEvidence | null;
//# sourceMappingURL=imperative-text.d.ts.map