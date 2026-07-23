/**
 * Purpose: Queries elements by CSS selector and returns computed CSS properties, box model dimensions, and contrast ratios for the analyze tool.
 * Docs: docs/features/feature/analyze-tool/index.md
 */
// computed-styles.ts — Computed styles query handler for inject context.
// Queries elements matching a CSS selector and returns computed CSS properties,
// box model dimensions, and contrast ratio for text elements.
/**
 * Default CSS properties to return when no filter is specified.
 */
const DEFAULT_PROPERTIES = [
    'color',
    'background-color',
    'font-size',
    'font-family',
    'font-weight',
    'line-height',
    'display',
    'position',
    'width',
    'height',
    'margin',
    'padding',
    'border',
    'opacity',
    'visibility',
    'z-index',
    'overflow',
    'text-align',
    'text-decoration',
    'box-sizing'
];
/**
 * Compute the relative luminance of an RGB color per WCAG 2.0.
 */
function relativeLuminance(r, g, b) {
    const [rs, gs, bs] = [r / 255, g / 255, b / 255].map((c) => c <= 0.03928 ? c / 12.92 : Math.pow((c + 0.055) / 1.055, 2.4));
    return 0.2126 * rs + 0.7152 * gs + 0.0722 * bs;
}
/**
 * Parse a CSS color string (rgb/rgba) into [r, g, b] components.
 */
function parseRGBColor(colorStr) {
    const match = colorStr.match(/rgba?\((\d+),\s*(\d+),\s*(\d+)/);
    if (!match)
        return null;
    return [parseInt(match[1], 10), parseInt(match[2], 10), parseInt(match[3], 10)];
}
/**
 * Compute contrast ratio between foreground and background colors.
 */
function computeContrastRatio(fgColor, bgColor) {
    const fg = parseRGBColor(fgColor);
    const bg = parseRGBColor(bgColor);
    if (!fg || !bg)
        return undefined;
    const fgLum = relativeLuminance(fg[0], fg[1], fg[2]);
    const bgLum = relativeLuminance(bg[0], bg[1], bg[2]);
    const lighter = Math.max(fgLum, bgLum);
    const darker = Math.min(fgLum, bgLum);
    return Math.round(((lighter + 0.05) / (darker + 0.05)) * 100) / 100;
}
/**
 * Build a minimal CSS selector for an element.
 */
function buildSelector(el) {
    if (el.id)
        return `#${el.id}`;
    const tag = el.tagName.toLowerCase();
    const classes = Array.from(el.classList)
        .slice(0, 3)
        .map((c) => `.${c}`)
        .join('');
    return tag + classes;
}
/**
 * Query computed styles for all elements matching a CSS selector.
 */
export function queryComputedStyles(params) {
    const elements = document.querySelectorAll(params.selector);
    const propList = params.properties && params.properties.length > 0 ? params.properties : DEFAULT_PROPERTIES;
    const results = [];
    const MAX_ELEMENTS = 50;
    for (let i = 0; i < elements.length && results.length < MAX_ELEMENTS; i++) {
        const el = elements[i];
        const style = window.getComputedStyle(el);
        const rect = el.getBoundingClientRect();
        const computedStyles = {};
        for (const prop of propList) {
            computedStyles[prop] = style.getPropertyValue(prop);
        }
        const result = {
            selector: buildSelector(el),
            tag: el.tagName.toLowerCase(),
            computed_styles: computedStyles,
            box_model: {
                x: Math.round(rect.x),
                y: Math.round(rect.y),
                width: Math.round(rect.width),
                height: Math.round(rect.height),
                top: Math.round(rect.top),
                right: Math.round(rect.right),
                bottom: Math.round(rect.bottom),
                left: Math.round(rect.left)
            }
        };
        // Compute contrast ratio for elements that likely contain text
        const color = style.getPropertyValue('color');
        const bgColor = style.getPropertyValue('background-color');
        if (color && bgColor && bgColor !== 'rgba(0, 0, 0, 0)') {
            const ratio = computeContrastRatio(color, bgColor);
            if (ratio !== undefined) {
                result.contrast_ratio = ratio;
            }
        }
        results.push(result);
    }
    return results;
}
//# sourceMappingURL=computed-styles.js.map