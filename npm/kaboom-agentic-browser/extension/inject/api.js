/**
 * Purpose: Exposes the window.__kaboom developer API for programmatic access to capture capabilities, context annotations, and event buffers.
 * Docs: docs/features/feature/custom-event-api/index.md
 */
import { setContextAnnotation, removeContextAnnotation, clearContextAnnotations, getContextAnnotations } from '../lib/context.js';
import { computeSelectors, recordEnhancedAction, getEnhancedActionBuffer, clearEnhancedActionBuffer, generatePlaywrightScript } from '../lib/reproduction.js';
import { getActionBuffer, clearActionBuffer, setActionCaptureEnabled } from '../lib/actions.js';
import { getNetworkWaterfall, setNetworkWaterfallEnabled } from '../lib/network.js';
import { getPerformanceMarks, getPerformanceMeasures, setPerformanceMarksEnabled } from '../lib/performance.js';
import { enrichErrorWithAiContext, setAiContextEnabled, setAiContextStateSnapshot } from '../lib/ai-context.js';
function setWithNativeSetter(element, proto, prop, val) {
    const setter = Object.getOwnPropertyDescriptor(proto.prototype, prop)?.set;
    if (setter)
        setter.call(element, val);
    else
        element[prop] = val;
}
/** Use native property setter to set value on form elements, bypassing framework interception */
function setNativeValue(element, value) {
    if (element instanceof HTMLInputElement) {
        if (element.type === 'checkbox' || element.type === 'radio') {
            setWithNativeSetter(element, HTMLInputElement, 'checked', Boolean(value));
        }
        else {
            setWithNativeSetter(element, HTMLInputElement, 'value', String(value));
        }
        return true;
    }
    if (element instanceof HTMLTextAreaElement) {
        setWithNativeSetter(element, HTMLTextAreaElement, 'value', String(value));
        return true;
    }
    if (element instanceof HTMLSelectElement) {
        setWithNativeSetter(element, HTMLSelectElement, 'value', String(value));
        return true;
    }
    return false;
}
/**
 * Install the window.__kaboom API for developers to interact with Kaboom
 */
// #lizard forgives
export function installKaboomAPI() {
    if (typeof window === 'undefined')
        return;
    window.__kaboom = {
        /**
         * Add a context annotation that will be included with errors
         * @param key - Annotation key (e.g., 'checkout-flow', 'user')
         * @param value - Annotation value
         * @example
         * window.__kaboom.annotate('checkout-flow', { step: 'payment', items: 3 })
         */
        annotate(key, value) {
            return setContextAnnotation(key, value);
        },
        /**
         * Remove a context annotation
         * @param key - Annotation key to remove
         */
        removeAnnotation(key) {
            return removeContextAnnotation(key);
        },
        /**
         * Clear all context annotations
         */
        clearAnnotations() {
            clearContextAnnotations();
        },
        /**
         * Get current context annotations
         * @returns Current annotations or null if none
         */
        getContext() {
            return getContextAnnotations();
        },
        /**
         * Get the user action replay buffer
         * @returns Recent user actions
         */
        getActions() {
            return getActionBuffer();
        },
        /**
         * Clear the user action replay buffer
         */
        clearActions() {
            clearActionBuffer();
        },
        /**
         * Enable or disable action capture
         * @param enabled - Whether to capture user actions
         */
        setActionCapture(enabled) {
            setActionCaptureEnabled(enabled);
        },
        /**
         * Enable or disable network waterfall capture
         * @param enabled - Whether to capture network waterfall
         */
        setNetworkWaterfall(enabled) {
            setNetworkWaterfallEnabled(enabled);
        },
        /**
         * Get current network waterfall
         * @param options - Filter options
         * @returns Network waterfall entries
         */
        getNetworkWaterfall(options) {
            return getNetworkWaterfall(options);
        },
        /**
         * Enable or disable performance marks capture
         * @param enabled - Whether to capture performance marks
         */
        setPerformanceMarks(enabled) {
            setPerformanceMarksEnabled(enabled);
        },
        /**
         * Get performance marks
         * @param options - Filter options
         * @returns Performance mark entries
         */
        getMarks(options) {
            return getPerformanceMarks(options);
        },
        /**
         * Get performance measures
         * @param options - Filter options
         * @returns Performance measure entries
         */
        getMeasures(options) {
            return getPerformanceMeasures(options);
        },
        // === AI Context ===
        /**
         * Enrich an error entry with AI context
         * @param error - Error entry to enrich
         * @returns Enriched error entry
         */
        enrichError(error) {
            // enrichErrorWithAiContext expects ErrorEntryForEnrichment which is compatible with LogEntry
            // The return type EnrichedErrorEntry extends LogEntry, so we can safely cast
            return enrichErrorWithAiContext(error);
        },
        /**
         * Enable or disable AI context enrichment
         * @param enabled
         */
        setAiContext(enabled) {
            setAiContextEnabled(enabled);
        },
        /**
         * Enable or disable state snapshot in AI context
         * @param enabled
         */
        setStateSnapshot(enabled) {
            setAiContextStateSnapshot(enabled);
        },
        // === Reproduction Scripts ===
        /**
         * Record an enhanced action (for testing)
         * @param type - Action type (click, input, keypress, navigate, select, scroll)
         * @param element - Target element
         * @param opts - Options
         */
        recordAction(type, element, opts) {
            recordEnhancedAction(type, element, opts);
        },
        /**
         * Get the enhanced action buffer
         * @returns
         */
        getEnhancedActions() {
            return getEnhancedActionBuffer();
        },
        /**
         * Clear the enhanced action buffer
         */
        clearEnhancedActions() {
            clearEnhancedActionBuffer();
        },
        /**
         * Generate a Playwright reproduction script
         * @param opts - Generation options
         * @returns Playwright test script
         */
        generateScript(opts) {
            // Uses the internal enhanced action buffer which is populated by recordEnhancedAction
            return generatePlaywrightScript(getEnhancedActionBuffer(), opts);
        },
        /**
         * Compute multi-strategy selectors for an element
         * @param element
         * @returns
         */
        getSelectors(element) {
            return computeSelectors(element);
        },
        /**
         * Set input value and trigger React/Vue/Svelte change events
         * Works with frameworks that track form state internally by dispatching
         * the events that frameworks listen for.
         *
         * @param selector - CSS selector for the input element
         * @param value - Value to set (string for text inputs, boolean for checkboxes)
         * @returns true if successful, false if element not found
         *
         * @example
         * // Text input
         * window.__kaboom.setInputValue('input[name="email"]', 'test@example.com')
         *
         * // Checkbox
         * window.__kaboom.setInputValue('input[type="checkbox"]', true)
         *
         * // Select dropdown
         * window.__kaboom.setInputValue('select[name="country"]', 'US')
         */
        setInputValue(selector, value) {
            const element = document.querySelector(selector);
            if (!element) {
                console.error('[KaBOOM!] Element not found:', selector);
                return false;
            }
            try {
                if (!setNativeValue(element, value)) {
                    console.error('[KaBOOM!] Element is not a form input:', selector);
                    return false;
                }
                // Dispatch events that React/Vue/Svelte listen for
                element.dispatchEvent(new Event('input', { bubbles: true }));
                element.dispatchEvent(new Event('change', { bubbles: true }));
                element.dispatchEvent(new Event('blur', { bubbles: true }));
                return true;
            }
            catch (err) {
                console.error('[KaBOOM!] Failed to set input value:', err);
                return false;
            }
        },
        /**
         * Version of the Kaboom API
         */
        version: __KABOOM_VERSION__
    };
}
/**
 * Uninstall the window.__kaboom API
 */
export function uninstallKaboomAPI() {
    if (typeof window !== 'undefined' && window.__kaboom) {
        delete window.__kaboom;
    }
}
//# sourceMappingURL=api.js.map