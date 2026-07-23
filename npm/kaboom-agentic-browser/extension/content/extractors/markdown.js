// markdown.ts — Markdown content extraction for get_markdown query type.
// Runs in the content script's ISOLATED world (CSP-safe, no eval).
// Issue #257: Replaces the IIFE string that was embedded in the Go handler.
import { findMainContentElement } from './shared.js';
/** Maximum output size in characters to prevent memory pressure on large pages. */
const MAX_OUTPUT_CHARS = 200_000;
/** Tags to skip entirely during markdown conversion. */
const SKIP_TAGS = ['nav', 'header', 'footer', 'aside', 'script', 'style', 'noscript', 'svg'];
function tableToMarkdown(table) {
    const rows = table.querySelectorAll('tr');
    if (rows.length === 0)
        return '';
    let md = '';
    for (let r = 0; r < rows.length; r++) {
        const rowEl = rows[r];
        if (!rowEl)
            continue;
        const cells = rowEl.querySelectorAll('th,td');
        let row = '|';
        for (let c = 0; c < cells.length; c++) {
            row += ' ' + (cells[c].innerText || '').trim().replace(/\|/g, '\\|').replace(/\n/g, ' ') + ' |';
        }
        md += row + '\n';
        if (r === 0 && rowEl.querySelector('th')) {
            md += '|';
            for (let c2 = 0; c2 < cells.length; c2++)
                md += ' --- |';
            md += '\n';
        }
    }
    return md;
}
function nodeToMarkdown(node, depth, budget) {
    if (!node || budget.remaining <= 0)
        return '';
    if (depth > 20)
        return '';
    if (node.nodeType === 3) {
        const text = node.textContent || '';
        budget.remaining -= text.length;
        return text;
    }
    if (node.nodeType !== 1)
        return '';
    const el = node;
    const tag = el.tagName.toLowerCase();
    // Skip unwanted elements
    if (SKIP_TAGS.includes(tag))
        return '';
    if (el.getAttribute('role') === 'navigation')
        return '';
    if (el.getAttribute('aria-hidden') === 'true')
        return '';
    let children = '';
    for (let i = 0; i < el.childNodes.length; i++) {
        if (budget.remaining <= 0)
            break;
        const child = el.childNodes[i];
        if (child)
            children += nodeToMarkdown(child, depth + 1, budget);
    }
    children = children.replace(/\n{3,}/g, '\n\n');
    switch (tag) {
        case 'h1':
            return '\n# ' + children.trim() + '\n\n';
        case 'h2':
            return '\n## ' + children.trim() + '\n\n';
        case 'h3':
            return '\n### ' + children.trim() + '\n\n';
        case 'h4':
            return '\n#### ' + children.trim() + '\n\n';
        case 'h5':
            return '\n##### ' + children.trim() + '\n\n';
        case 'h6':
            return '\n###### ' + children.trim() + '\n\n';
        case 'p':
            return '\n' + children.trim() + '\n\n';
        case 'br':
            return '\n';
        case 'hr':
            return '\n---\n\n';
        case 'strong':
        case 'b':
            return '**' + children.trim() + '**';
        case 'em':
        case 'i':
            return '*' + children.trim() + '*';
        case 'code':
            return '`' + children.trim() + '`';
        case 'pre':
            return '\n```\n' + (el.innerText || '').trim() + '\n```\n\n';
        case 'a': {
            let href = el.getAttribute('href') || '';
            if (href && href !== '#' && !href.startsWith('javascript:')) {
                try {
                    href = new URL(href, window.location.href).href;
                }
                catch {
                    /* keep original */
                }
                return '[' + children.trim() + '](' + href + ')';
            }
            return children;
        }
        case 'img': {
            let src = el.getAttribute('src') || '';
            const alt = el.getAttribute('alt') || '';
            if (src) {
                try {
                    src = new URL(src, window.location.href).href;
                }
                catch {
                    /* keep original */
                }
                return '![' + alt + '](' + src + ')';
            }
            return '';
        }
        case 'ul':
        case 'ol':
            return '\n' + children + '\n';
        case 'li': {
            const parent = el.parentElement;
            if (parent && parent.tagName.toLowerCase() === 'ol') {
                const idx = Array.from(parent.children).indexOf(el) + 1;
                return idx + '. ' + children.trim() + '\n';
            }
            return '- ' + children.trim() + '\n';
        }
        case 'blockquote':
            return '\n> ' + children.trim().replace(/\n/g, '\n> ') + '\n\n';
        case 'table':
            return '\n' + tableToMarkdown(el) + '\n\n';
        case 'div':
        case 'section':
        case 'article':
        case 'main':
            return children;
        default:
            return children;
    }
}
/**
 * Extract page content and convert to Markdown.
 * Returns structured data with title, markdown content, word count, and URL.
 * Output is capped at MAX_OUTPUT_CHARS to prevent memory pressure on large pages.
 */
export function extractMarkdown() {
    const main = findMainContentElement(100);
    const budget = { remaining: MAX_OUTPUT_CHARS };
    let markdown = nodeToMarkdown(main, 0, budget).trim();
    const truncated = budget.remaining <= 0;
    if (truncated) {
        markdown = markdown.slice(0, MAX_OUTPUT_CHARS) + '\n\n[...truncated]';
    }
    const words = markdown
        .replace(/[#*[\]()`|>-]/g, ' ')
        .split(/\s+/)
        .filter(Boolean);
    return {
        title: document.title || '',
        markdown,
        word_count: words.length,
        url: window.location.href,
        ...(truncated ? { truncated: true } : {})
    };
}
//# sourceMappingURL=markdown.js.map