// markdown.ts — Markdown content extraction for get_markdown query type.
// Runs in the content script's ISOLATED world (CSP-safe, no eval).
// Issue #257: Replaces the IIFE string that was embedded in the Go handler.

import { findMainContentElement } from './shared.js'

/** Maximum output size in characters to prevent memory pressure on large pages. */
const MAX_OUTPUT_CHARS = 200_000

/**
 * Result shape returned by extractMarkdown.
 */
export interface MarkdownResult {
  title: string
  markdown: string
  word_count: number
  url: string
  truncated?: boolean
}

/** Tags to skip entirely during markdown conversion. */
const SKIP_TAGS = ['nav', 'header', 'footer', 'aside', 'script', 'style', 'noscript', 'svg']

function tableToMarkdown(table: Element): string {
  const rows = table.querySelectorAll('tr')
  if (rows.length === 0) return ''
  let md = ''
  for (let r = 0; r < rows.length; r++) {
    const rowEl = rows[r]
    if (!rowEl) continue
    const cells = rowEl.querySelectorAll('th,td')
    let row = '|'
    for (let c = 0; c < cells.length; c++) {
      row += ' ' + ((cells[c] as HTMLElement).innerText || '').trim().replace(/\|/g, '\\|').replace(/\n/g, ' ') + ' |'
    }
    md += row + '\n'
    if (r === 0 && rowEl.querySelector('th')) {
      md += '|'
      for (let c2 = 0; c2 < cells.length; c2++) md += ' --- |'
      md += '\n'
    }
  }
  return md
}

const HEADING_MARKS: Record<string, string> = {
  h1: '#',
  h2: '##',
  h3: '###',
  h4: '####',
  h5: '#####',
  h6: '######'
}

const INLINE_WRAPS: Record<string, string> = {
  strong: '**',
  b: '**',
  em: '*',
  i: '*',
  code: '`'
}

/** Resolves a tag's own property only, so prototype names hit the default render path. */
function ownMark(marks: Record<string, string>, tag: string): string | undefined {
  return Object.prototype.hasOwnProperty.call(marks, tag) ? marks[tag] : undefined
}

function collectChildren(el: HTMLElement, depth: number, budget: { remaining: number }): string {
  let children = ''
  for (let i = 0; i < el.childNodes.length; i++) {
    if (budget.remaining <= 0) break
    const child = el.childNodes[i]
    if (child) children += nodeToMarkdown(child, depth + 1, budget)
  }
  return children.replace(/\n{3,}/g, '\n\n')
}

function resolvePageUrl(value: string): string {
  try {
    return new URL(value, window.location.href).href
  } catch {
    // EXPECTED_ABSENCE: optional enrichment can normally fail while the primary
    // operation keeps a valid fallback; logging it would misleadingly report fallback as failure.
    return value
  }
}

function renderLinkMarkdown(el: HTMLElement, children: string): string {
  const href = el.getAttribute('href') || ''
  if (href && href !== '#' && !href.startsWith('javascript:')) {
    return '[' + children.trim() + '](' + resolvePageUrl(href) + ')'
  }
  return children
}

function renderImageMarkdown(el: HTMLElement): string {
  const src = el.getAttribute('src') || ''
  const alt = el.getAttribute('alt') || ''
  if (src) {
    return '![' + alt + '](' + resolvePageUrl(src) + ')'
  }
  return ''
}

function renderListItemMarkdown(el: HTMLElement, children: string): string {
  const parent = el.parentElement
  if (parent && parent.tagName.toLowerCase() === 'ol') {
    const idx = Array.from(parent.children).indexOf(el) + 1
    return idx + '. ' + children.trim() + '\n'
  }
  return '- ' + children.trim() + '\n'
}

function renderElementMarkdown(tag: string, el: HTMLElement, children: string): string {
  const headingMark = ownMark(HEADING_MARKS, tag)
  if (headingMark) return '\n' + headingMark + ' ' + children.trim() + '\n\n'
  const wrap = ownMark(INLINE_WRAPS, tag)
  if (wrap) return wrap + children.trim() + wrap
  return renderBlockMarkdown(tag, el, children)
}

function renderBlockMarkdown(tag: string, el: HTMLElement, children: string): string {
  switch (tag) {
    case 'p':
      return '\n' + children.trim() + '\n\n'
    case 'br':
      return '\n'
    case 'hr':
      return '\n---\n\n'
    case 'pre':
      return '\n```\n' + (el.innerText || '').trim() + '\n```\n\n'
    case 'a':
      return renderLinkMarkdown(el, children)
    case 'img':
      return renderImageMarkdown(el)
    case 'ul':
    case 'ol':
      return '\n' + children + '\n'
    case 'li':
      return renderListItemMarkdown(el, children)
    case 'blockquote':
      return '\n> ' + children.trim().replace(/\n/g, '\n> ') + '\n\n'
    case 'table':
      return '\n' + tableToMarkdown(el) + '\n\n'
    default:
      return children
  }
}

function nodeToMarkdown(node: Node, depth: number, budget: { remaining: number }): string {
  if (!node || budget.remaining <= 0) return ''
  if (depth > 20) return ''
  if (node.nodeType === 3) {
    const text = node.textContent || ''
    budget.remaining -= text.length
    return text
  }
  if (node.nodeType !== 1) return ''
  const el = node as HTMLElement
  const tag = el.tagName.toLowerCase()

  // Skip unwanted elements
  if (SKIP_TAGS.includes(tag)) return ''
  if (el.getAttribute('role') === 'navigation') return ''
  if (el.getAttribute('aria-hidden') === 'true') return ''

  return renderElementMarkdown(tag, el, collectChildren(el, depth, budget))
}

/**
 * Extract page content and convert to Markdown.
 * Returns structured data with title, markdown content, word count, and URL.
 * Output is capped at MAX_OUTPUT_CHARS to prevent memory pressure on large pages.
 */
export function extractMarkdown(): MarkdownResult {
  const main = findMainContentElement(100)
  const budget = { remaining: MAX_OUTPUT_CHARS }
  let markdown = nodeToMarkdown(main, 0, budget).trim()
  const truncated = budget.remaining <= 0
  if (truncated) {
    markdown = markdown.slice(0, MAX_OUTPUT_CHARS) + '\n\n[...truncated]'
  }
  const words = markdown
    .replace(/[#*[\]()`|>-]/g, ' ')
    .split(/\s+/)
    .filter(Boolean)

  return {
    title: document.title || '',
    markdown,
    word_count: words.length,
    url: window.location.href,
    ...(truncated ? { truncated: true } : {})
  }
}
