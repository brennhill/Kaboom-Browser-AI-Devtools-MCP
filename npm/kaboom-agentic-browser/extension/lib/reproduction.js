/**
 * Purpose: Records user interactions with multi-strategy selectors (testId, role, aria, text, CSS path) and generates Playwright reproduction scripts.
 * Docs: docs/features/feature/reproduction-scripts/index.md
 */
/**
 * @fileoverview Reproduction script generation and enhanced action recording.
 * Captures user interactions with multi-strategy selectors (testId, role, aria,
 * text, CSS path) and generates Playwright test scripts for reproducing issues.
 */
import { ENHANCED_ACTION_BUFFER_SIZE, CSS_PATH_MAX_DEPTH, SELECTOR_TEXT_MAX_LENGTH, SCRIPT_MAX_SIZE, CLICKABLE_TAGS } from './constants.js';
import { isSensitiveInput } from './serialize.js';
// Enhanced action buffer (separate from v3 action buffer)
let enhancedActionBuffer = [];
/**
 * Get the implicit ARIA role for an element
 */
const TAG_TO_ROLE = {
    button: 'button',
    textarea: 'textbox',
    select: 'combobox',
    nav: 'navigation',
    main: 'main',
    header: 'banner',
    footer: 'contentinfo'
};
const INPUT_TYPE_TO_ROLE = {
    text: 'textbox',
    email: 'textbox',
    password: 'textbox',
    tel: 'textbox',
    url: 'textbox',
    checkbox: 'checkbox',
    radio: 'radio',
    search: 'searchbox',
    number: 'spinbutton',
    range: 'slider'
};
export function getImplicitRole(element) {
    if (!element || !element.tagName)
        return null;
    const tag = element.tagName.toLowerCase();
    const el = element;
    if (tag === 'a') {
        return el.getAttribute && el.getAttribute('href') !== null ? 'link' : null;
    }
    if (tag === 'input') {
        const type = el.getAttribute ? el.getAttribute('type') : null;
        return INPUT_TYPE_TO_ROLE[type || 'text'] ?? 'textbox';
    }
    return TAG_TO_ROLE[tag] ?? null;
}
/**
 * Detect if a CSS class name is dynamically generated (CSS-in-JS)
 */
export function isDynamicClass(className) {
    if (!className)
        return false;
    // Known CSS-in-JS prefixes
    if (/^(css|sc|emotion|styled|chakra)-/.test(className))
        return true;
    // Random hash classes: 5-8 lowercase-only chars
    if (/^[a-z]{5,8}$/.test(className))
        return true;
    return false;
}
/**
 * Compute a CSS path for an element
 */
export function computeCssPath(element) {
    if (!element)
        return '';
    const parts = [];
    let current = element;
    while (current && parts.length < CSS_PATH_MAX_DEPTH) {
        let selector = current.tagName ? current.tagName.toLowerCase() : '';
        // Stop at element with ID
        if (current.id) {
            selector = `#${current.id}`;
            parts.unshift(selector);
            break;
        }
        // Add non-dynamic classes (max 2)
        const classNameValue = current.className;
        const classList = classNameValue && typeof classNameValue === 'string'
            ? classNameValue
                .trim()
                .split(/\s+/)
                .filter((c) => c && !isDynamicClass(c))
            : [];
        if (classList.length > 0) {
            selector += '.' + classList.slice(0, 2).join('.');
        }
        parts.unshift(selector);
        current = current.parentElement;
    }
    return parts.join(' > ');
}
/**
 * Compute multi-strategy selectors for an element
 */
// #lizard forgives
export function computeSelectors(element) {
    if (!element)
        return { cssPath: '' };
    const selectors = {};
    const el = element;
    // MULTI-STRATEGY SELECTOR FALLBACK ORDER & RATIONALE:
    //
    // Playwright test generation requires reliable selectors to reproduce user interactions.
    // This function implements a priority-based fallback strategy to handle diverse DOM
    // patterns. Each selector type has different reliability characteristics:
    //
    // PRIORITY 1: TEST ID (data-testid, data-test-id, data-cy)
    //   Why first: Explicitly designed for testing, guaranteed unique, stable across refactors.
    //   Reliability: Highest. Used by developers as test hooks. Never changes in production.
    //   Fallback trigger: Element has no test attribute.
    //
    // PRIORITY 2: ARIA LABEL (aria-label)
    //   Why second: Accessibility-first, explicitly describes element, human-readable.
    //   Reliability: High. Well-maintained in modern apps. Semantic meaning stable.
    //   Fallback trigger: Element has no aria-label or it's empty.
    //   Edge case: Ignored if empty or whitespace-only.
    //
    // PRIORITY 3: ROLE + ACCESSIBLE NAME (role + implicit/explicit name)
    //   Why third: Combines semantic role (button, link, textbox) with accessible name
    //   (either aria-label or text content). Playwright's getByRole() is powerful for
    //   interactive elements but requires a name to disambiguate siblings.
    //   Reliability: Medium-high. Role is stable; text content can change in i18n apps.
    //   Edge cases:
    //     - Elements without roles (divs, spans) fall through
    //     - Multiple elements with same role+name require additional strategies
    //   Optimization: Only considers implicit roles from HTML semantics or explicit @role
    //
    // PRIORITY 4: ID (element.id)
    //   Why fourth: Simple, unique within page, but often dynamically generated or missing.
    //   Reliability: Medium. Some frameworks auto-generate IDs; some don't use IDs at all.
    //   Risk: If ID is dynamic (e.g., "mui-123"), test becomes fragile.
    //   Advantage: Playwright's locator('#id') is efficient (native DOM API).
    //
    // PRIORITY 5: TEXT CONTENT (innerText/textContent for clickables)
    //   Why fifth: Accessible fallback for buttons, links, list items. Users click text.
    //   Reliability: Low-medium. Changes with UX copy; vulnerable to localization.
    //   Constraint: Only used for elements in CLICKABLE_TAGS (button, a, li, etc.)
    //   or elements with explicit role="button". Prevents false matches on labels, headers.
    //   Truncation: Limited to SELECTOR_TEXT_MAX_LENGTH (128 chars) to avoid long predicates.
    //
    // PRIORITY 6: CSS PATH (always computed as fallback)
    //   Why last: Brittle but guaranteed to exist. DOM tree structure often changes during
    //   refactoring or with dynamic content. Used only when all else fails.
    //   Computation: Via computeCssPath() which builds CSS selectors up the tree, filtering
    //   dynamic classes and stopping at elements with IDs.
    //   Risk: Highly sensitive to DOM changes. Test breaks if any parent node is removed.
    //
    // EDGE CASES HANDLED:
    //   - No attributes: Falls through to CSS path (always safe).
    //   - Dynamic classes (css-*, sc-*, emotion-*): Filtered by isDynamicClass().
    //   - iframes: CSS path may not work cross-frame; role/text often more reliable.
    //   - Shadow DOM: Element attributes visible but CSS path doesn't cross boundaries.
    //   - Hidden elements: All strategies still apply (Playwright can interact with hidden).
    //   - Dynamically created elements: All text/ID strategies remain valid; CSS path may shift.
    // Priority 1: Test ID
    const testId = (el.getAttribute &&
        (el.getAttribute('data-testid') || el.getAttribute('data-test-id') || el.getAttribute('data-cy'))) ||
        undefined;
    if (testId)
        selectors.testId = testId;
    // Priority 2: ARIA label
    const ariaLabel = el.getAttribute && el.getAttribute('aria-label');
    if (ariaLabel)
        selectors.ariaLabel = ariaLabel;
    // Priority 3: Role + accessible name
    const explicitRole = el.getAttribute && el.getAttribute('role');
    const role = explicitRole || getImplicitRole(element);
    const name = ariaLabel || (el.textContent && el.textContent.trim().slice(0, SELECTOR_TEXT_MAX_LENGTH));
    if (role && name) {
        selectors.role = { role, name: ariaLabel || name };
    }
    // Priority 4: ID
    if (element.id)
        selectors.id = element.id;
    // Priority 5: Text content (for clickable elements or role="button")
    const isClickable = (element.tagName && CLICKABLE_TAGS.has(element.tagName.toUpperCase())) ||
        (el.getAttribute && el.getAttribute('role') === 'button');
    if (isClickable) {
        const text = (el.textContent || el.innerText || '').trim();
        if (text)
            selectors.text = text.slice(0, SELECTOR_TEXT_MAX_LENGTH);
    }
    // Priority 6: CSS path (always computed as fallback)
    selectors.cssPath = computeCssPath(element);
    return selectors;
}
const ACTION_DATA_ENRICHERS = {
    input: (a, el, o) => {
        const typedEl = el;
        const inputType = typedEl && typedEl.getAttribute ? typedEl.getAttribute('type') : 'text';
        a.input_type = inputType || 'text';
        a.value = inputType === 'password' || (el && isSensitiveInput(el)) ? '[redacted]' : o.value || '';
    },
    keypress: (a, _el, o) => {
        a.key = o.key || '';
    },
    navigate: (a, _el, o) => {
        a.from_url = o.from_url || '';
        a.to_url = o.to_url || '';
    },
    select: (a, _el, o) => {
        a.selected_value = o.selected_value || '';
        a.selected_text = o.selected_text || '';
    },
    scroll: (a, _el, o) => {
        a.scroll_y = o.scroll_y || 0;
    },
    transient: (a, _el, o) => {
        a.classification = o.classification || 'unknown';
        if (o.duration_ms !== undefined)
            a.duration_ms = o.duration_ms;
        if (o.role)
            a.role = o.role;
        if (o.value)
            a.value = o.value;
    }
};
/**
 * Record an enhanced action with multi-strategy selectors
 */
export function recordEnhancedAction(type, element, opts = {}) {
    const action = {
        type,
        timestamp: Date.now(),
        url: typeof window !== 'undefined' && window.location ? window.location.href : ''
    };
    if (element) {
        action.selectors = computeSelectors(element);
    }
    const enricher = ACTION_DATA_ENRICHERS[type];
    if (enricher)
        enricher(action, element, opts);
    // Add to buffer
    enhancedActionBuffer.push(action);
    if (enhancedActionBuffer.length > ENHANCED_ACTION_BUFFER_SIZE) {
        enhancedActionBuffer.shift();
    }
    // Emit to content script for server relay
    if (typeof window !== 'undefined' && window.postMessage) {
        window.postMessage({ type: 'kaboom_enhanced_action', payload: action }, window.location.origin);
    }
    return action;
}
/**
 * Get the enhanced action buffer
 */
export function getEnhancedActionBuffer() {
    return [...enhancedActionBuffer];
}
/**
 * Clear the enhanced action buffer
 */
export function clearEnhancedActionBuffer() {
    enhancedActionBuffer = [];
}
function rebaseUrl(url, baseUrl) {
    if (!baseUrl || !url)
        return url;
    try {
        return baseUrl + new URL(url).pathname;
    }
    catch {
        return url;
    }
}
const ACTION_STEP_GENERATORS = {
    click: (_action, locator) => locator ? `  await page.${locator}.click();` : `  // click action - no selector available`,
    input: (action, locator) => {
        if (!locator)
            return null;
        const value = action.value === '[redacted]' ? '[user-provided]' : action.value || '';
        return `  await page.${locator}.fill('${escapeString(value)}');`;
    },
    keypress: (action) => `  await page.keyboard.press('${escapeString(action.key || '')}');`,
    navigate: (action, _locator, baseUrl) => `  await page.waitForURL('${escapeString(rebaseUrl(action.to_url || '', baseUrl))}');`,
    select: (action, locator) => locator ? `  await page.${locator}.selectOption('${escapeString(action.selected_value || '')}');` : null,
    scroll: (action) => `  // User scrolled to y=${action.scroll_y || 0}`,
    transient: (action) => `  // [${action.classification || 'transient'}] "${(action.value || '').slice(0, 80)}"`
};
// #lizard forgives
function actionToPlaywrightStep(action, baseUrl) {
    const locator = getPlaywrightLocator(action.selectors || { cssPath: '' });
    const generator = ACTION_STEP_GENERATORS[action.type];
    return generator ? generator(action, locator, baseUrl) : null;
}
/**
 * Generate a Playwright test script from captured actions
 */
export function generatePlaywrightScript(actions, opts = {}) {
    const { errorMessage, baseUrl, lastNActions } = opts;
    // Apply lastNActions filter
    let filteredActions = actions;
    if (lastNActions && lastNActions > 0 && actions.length > lastNActions) {
        filteredActions = actions.slice(-lastNActions);
    }
    // Determine start URL
    let startUrl = '';
    if (filteredActions.length > 0) {
        const firstAction = filteredActions[0];
        if (firstAction) {
            startUrl = firstAction.url || '';
        }
    }
    if (baseUrl && startUrl) {
        try {
            const parsed = new URL(startUrl);
            startUrl = baseUrl + parsed.pathname;
        }
        catch {
            startUrl = baseUrl;
        }
    }
    // Build test name
    const testName = errorMessage ? `reproduction: ${errorMessage.slice(0, 80)}` : 'reproduction: captured user actions';
    // Generate step code
    const steps = [];
    let prevTimestamp = null;
    for (const action of filteredActions) {
        if (prevTimestamp && action.timestamp - prevTimestamp > 2000) {
            const gap = Math.round((action.timestamp - prevTimestamp) / 1000);
            steps.push(`  // [${gap}s pause]`);
        }
        prevTimestamp = action.timestamp;
        const step = actionToPlaywrightStep(action, baseUrl);
        if (step)
            steps.push(step);
    }
    // Assemble script
    let script = `import { test, expect } from '@playwright/test';\n\n`; // nosemgrep: missing-template-string-indicator
    script += `test('${escapeString(testName)}', async ({ page }) => {\n`; // nosemgrep: missing-template-string-indicator
    if (startUrl) {
        script += `  await page.goto('${escapeString(startUrl)}');\n\n`;
    }
    script += steps.join('\n');
    if (steps.length > 0)
        script += '\n';
    if (errorMessage) {
        script += `\n  // Error occurred here: ${errorMessage}\n`;
    }
    script += `});\n`;
    // Cap output size
    if (script.length > SCRIPT_MAX_SIZE) {
        script = script.slice(0, SCRIPT_MAX_SIZE);
    }
    return script;
}
/**
 * Get the best Playwright locator for a set of selectors
 * Priority: testId > role > ariaLabel > text > id > cssPath
 */
function getPlaywrightLocator(selectors) {
    if (selectors.testId)
        return `getByTestId('${escapeString(selectors.testId)}')`;
    if (selectors.role && selectors.role.role) {
        const escaped = escapeString(selectors.role.role);
        return selectors.role.name
            ? `getByRole('${escaped}', { name: '${escapeString(selectors.role.name)}' })`
            : `getByRole('${escaped}')`;
    }
    if (selectors.ariaLabel)
        return `getByLabel('${escapeString(selectors.ariaLabel)}')`;
    if (selectors.text)
        return `getByText('${escapeString(selectors.text)}')`;
    if (selectors.id)
        return `locator('#${escapeString(selectors.id)}')`;
    if (selectors.cssPath)
        return `locator('${escapeString(selectors.cssPath)}')`;
    return null;
}
/**
 * Escape a string for use in JavaScript string literals
 */
function escapeString(str) {
    if (!str)
        return '';
    return str
        .replace(/\\/g, '\\\\')
        .replace(/'/g, "\\'")
        .replace(/\n/g, '\\n')
        .replace(/\r/g, '\\r')
        .replace(/\t/g, '\\t')
        .replace(/`/g, '\\`');
}
//# sourceMappingURL=reproduction.js.map