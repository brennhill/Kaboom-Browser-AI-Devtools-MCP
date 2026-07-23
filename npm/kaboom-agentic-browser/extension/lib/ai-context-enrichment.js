/**
 * Purpose: Detects UI frameworks (React/Vue/Svelte), captures state snapshots, and generates AI-friendly error summaries with timeout guards.
 * Docs: docs/features/feature/error-bundling/index.md
 */
import { AI_CONTEXT_MAX_ANCESTRY_DEPTH, AI_CONTEXT_MAX_PROP_KEYS, AI_CONTEXT_MAX_STATE_KEYS, AI_CONTEXT_MAX_RELEVANT_SLICE, AI_CONTEXT_MAX_VALUE_LENGTH, AI_CONTEXT_PIPELINE_TIMEOUT_MS } from './constants.js';
import { parseStackFrames, extractSourceSnippets, getSourceMapCache } from './ai-context-parsing.js';
// =============================================================================
// MODULE STATE
// =============================================================================
let aiContextEnabled = true;
let aiContextStateSnapshotEnabled = false;
// =============================================================================
// FRAMEWORK DETECTION
// =============================================================================
/**
 * Detect which UI framework an element belongs to
 * @param element - The DOM element (or element-like object)
 * @returns { framework, key? } or null
 */
export function detectFramework(element) {
    if (!element || typeof element !== 'object')
        return null;
    // React: __reactFiber$ or __reactInternalInstance$
    const keys = Object.keys(element);
    const reactKey = keys.find((k) => k.startsWith('__reactFiber$') || k.startsWith('__reactInternalInstance$'));
    if (reactKey)
        return { framework: 'react', key: reactKey };
    // Vue 3: __vueParentComponent or __vue_app__
    if (element.__vueParentComponent || element.__vue_app__) {
        return { framework: 'vue' };
    }
    // Svelte: __svelte_meta
    if (element.__svelte_meta) {
        return { framework: 'svelte' };
    }
    return null;
}
// =============================================================================
// REACT COMPONENT ANCESTRY
// =============================================================================
/**
 * Walk a React fiber tree to extract component ancestry
 * @param fiber - The React fiber node
 * @returns Array of { name, propKeys?, hasState?, stateKeys? } in root-first order
 */
// #lizard forgives
export function getReactComponentAncestry(fiber) {
    if (!fiber)
        return null;
    const ancestry = [];
    let current = fiber;
    let depth = 0;
    while (current && depth < AI_CONTEXT_MAX_ANCESTRY_DEPTH) {
        depth++;
        // Only include component fibers (type is function/object), skip host elements (type is string)
        if (current.type && typeof current.type !== 'string') {
            const typeObj = current.type;
            const name = typeObj.displayName || typeObj.name || 'Anonymous';
            const entry = { name };
            // Extract prop keys (excluding children)
            if (current.memoizedProps && typeof current.memoizedProps === 'object') {
                entry.propKeys = Object.keys(current.memoizedProps)
                    .filter((k) => k !== 'children')
                    .slice(0, AI_CONTEXT_MAX_PROP_KEYS);
            }
            // Extract state keys
            if (current.memoizedState && typeof current.memoizedState === 'object' && !Array.isArray(current.memoizedState)) {
                entry.hasState = true;
                entry.stateKeys = Object.keys(current.memoizedState).slice(0, AI_CONTEXT_MAX_STATE_KEYS);
            }
            ancestry.push(entry);
        }
        current = current.return;
    }
    return ancestry.reverse(); // Root-first order
}
// =============================================================================
// STATE SNAPSHOT
// =============================================================================
function classifyValueType(value) {
    if (Array.isArray(value))
        return 'array';
    if (value === null)
        return 'null';
    return typeof value;
}
const RELEVANT_STATE_KEYS = ['error', 'loading', 'status', 'failed'];
// #lizard forgives
function buildRelevantSlice(state, errorWords) {
    const relevantSlice = {};
    let sliceCount = 0;
    for (const [key, value] of Object.entries(state)) {
        if (sliceCount >= AI_CONTEXT_MAX_RELEVANT_SLICE)
            break;
        if (typeof value !== 'object' || value === null || Array.isArray(value))
            continue;
        for (const [subKey, subValue] of Object.entries(value)) {
            if (sliceCount >= AI_CONTEXT_MAX_RELEVANT_SLICE)
                break;
            const isRelevantKey = RELEVANT_STATE_KEYS.some((k) => subKey.toLowerCase().includes(k));
            const isKeywordMatch = errorWords.some((w) => key.toLowerCase().includes(w));
            if (!isRelevantKey && !isKeywordMatch)
                continue;
            let val = subValue;
            if (typeof val === 'string' && val.length > AI_CONTEXT_MAX_VALUE_LENGTH) {
                val = val.slice(0, AI_CONTEXT_MAX_VALUE_LENGTH);
            }
            relevantSlice[`${key}.${subKey}`] = val;
            sliceCount++;
        }
    }
    return relevantSlice;
}
/**
 * Capture application state snapshot from known store patterns.
 *
 * STATE RELEVANCE MATCHING STRATEGY:
 * 1. Extract error keywords from the error message (words > 2 chars).
 * 2. Build a "relevant slice" by matching nested state keys against common error state
 *    keys ('error', 'loading', 'status', 'failed') and error message keywords.
 * 3. Caps at AI_CONTEXT_MAX_RELEVANT_SLICE entries; values truncated at MAX_VALUE_LENGTH.
 *
 * NOTE: Only supports Redux. Other state management would need additional window.__* patterns.
 */
export function captureStateSnapshot(errorMessage) {
    if (typeof window === 'undefined')
        return null;
    try {
        const store = window.__REDUX_STORE__;
        if (!store || typeof store.getState !== 'function')
            return null;
        const state = store.getState();
        if (!state || typeof state !== 'object')
            return null;
        const keys = {};
        for (const [key, value] of Object.entries(state)) {
            keys[key] = { type: classifyValueType(value) };
        }
        const errorWords = (errorMessage || '')
            .toLowerCase()
            .split(/\W+/)
            .filter((w) => w.length > 2);
        const relevantSlice = buildRelevantSlice(state, errorWords);
        return { source: 'redux', keys, relevantSlice };
    }
    catch {
        return null;
    }
}
// =============================================================================
// AI SUMMARY GENERATION
// =============================================================================
/**
 * Generate a template-based AI summary from enrichment data
 * @param data - { errorType, message, file, line, componentAncestry, stateSnapshot }
 * @returns Summary string
 */
export function generateAiSummary(data) {
    const parts = [];
    // Error type and location
    if (data.file && data.line) {
        parts.push(`${data.errorType} in ${data.file}:${data.line} — ${data.message}`);
    }
    else {
        parts.push(`${data.errorType}: ${data.message}`);
    }
    // Component context
    if (data.componentAncestry && data.componentAncestry.components) {
        const path = data.componentAncestry.components.map((c) => c.name).join(' > ');
        parts.push(`Component tree: ${path}.`);
    }
    // State context
    if (data.stateSnapshot && data.stateSnapshot.relevantSlice) {
        const sliceKeys = Object.keys(data.stateSnapshot.relevantSlice);
        if (sliceKeys.length > 0) {
            const stateInfo = sliceKeys.map((k) => `${k}=${JSON.stringify(data.stateSnapshot.relevantSlice[k])}`).join(', '); // nosemgrep: no-stringify-keys
            parts.push(`State: ${stateInfo}.`);
        }
    }
    return parts.join(' ');
}
// =============================================================================
// ERROR ENRICHMENT PIPELINE
// =============================================================================
/**
 * Full error enrichment pipeline
 * @param error - The error entry to enrich
 * @returns The enriched error entry
 */
// #lizard forgives
async function buildAiContext(error) {
    const result = {};
    const frames = parseStackFrames(error.stack);
    if (frames.length === 0)
        return { summary: error.message || 'Unknown error' };
    const topFrame = frames[0];
    // Source snippets (from cache)
    if (topFrame) {
        const cached = getSourceMapCache(topFrame.filename);
        if (cached) {
            const snippets = await extractSourceSnippets(frames, { [topFrame.filename]: cached });
            if (snippets.length > 0)
                result.sourceSnippets = snippets;
        }
    }
    // Component ancestry from activeElement
    result.componentAncestry = extractComponentAncestry() || undefined;
    // State snapshot (if enabled)
    if (aiContextStateSnapshotEnabled) {
        const snapshot = captureStateSnapshot(error.message || '');
        if (snapshot)
            result.stateSnapshot = snapshot;
    }
    result.summary = generateAiSummary({
        errorType: error.message?.split(':')[0] || 'Error',
        message: error.message || '',
        file: topFrame?.filename || null,
        line: topFrame?.lineno || null,
        componentAncestry: result.componentAncestry || null,
        stateSnapshot: result.stateSnapshot || null
    });
    return result;
}
function extractComponentAncestry() {
    if (typeof document === 'undefined' || !document.activeElement)
        return null;
    const framework = detectFramework(document.activeElement);
    if (!framework || framework.framework !== 'react' || !framework.key)
        return null;
    const fiber = document.activeElement[framework.key];
    const components = getReactComponentAncestry(fiber);
    if (!components || components.length === 0)
        return null;
    return { framework: 'react', components };
}
function applyAiContext(enriched, context) {
    enriched._aiContext = context;
    if (!enriched._enrichments)
        enriched._enrichments = [];
    enriched._enrichments.push('aiContext');
}
export async function enrichErrorWithAiContext(error) {
    if (!aiContextEnabled)
        return error;
    const enriched = { ...error };
    try {
        const context = await Promise.race([
            buildAiContext(error),
            new Promise((resolve) => {
                setTimeout(() => resolve({ summary: `${error.message || 'Error'}` }), AI_CONTEXT_PIPELINE_TIMEOUT_MS);
            })
        ]);
        applyAiContext(enriched, context);
    }
    catch {
        applyAiContext(enriched, { summary: error.message || 'Unknown error' });
    }
    return enriched;
}
// =============================================================================
// CONFIGURATION
// =============================================================================
/**
 * Enable or disable AI context enrichment
 * @param enabled
 */
export function setAiContextEnabled(enabled) {
    aiContextEnabled = enabled;
}
/**
 * Enable or disable state snapshot in AI context
 * @param enabled
 */
export function setAiContextStateSnapshot(enabled) {
    aiContextStateSnapshotEnabled = enabled;
}
/**
 * Reset enrichment module state for testing purposes.
 * Call this in beforeEach/afterEach test hooks to prevent test pollution.
 */
export function resetEnrichmentForTesting() {
    aiContextEnabled = true;
    aiContextStateSnapshotEnabled = false;
}
//# sourceMappingURL=ai-context-enrichment.js.map