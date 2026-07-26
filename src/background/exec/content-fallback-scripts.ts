/**
 * Purpose: Self-contained extraction fallbacks used when content scripts are unavailable.
 * Why: Keep fallback script implementations centralized and reusable across command handlers.
 * Docs: docs/features/feature/interact-explore/index.md
 */

const READABLE_MAIN_SELECTORS = [
  'main',
  'article',
  '[role="main"]',
  '#main',
  '.main',
  '.post-content',
  '.entry-content',
  '.article-body',
  '.article-content',
  '.story-body',
  '.article',
  '.post',
  '#content',
  '.content',
  '.results'
]

const MARKDOWN_MAIN_SELECTORS = [
  'main',
  'article',
  '[role="main"]',
  '#main',
  '.main',
  '.post-content',
  '.entry-content',
  '.article-body',
  '.article-content'
]

const PAGE_SUMMARY_MAIN_SELECTORS = ['main', 'article', '[role="main"]', '#main', '.main', '.post-content', '.entry-content']

const COMMON_REMOVE_SELECTORS = [
  'nav',
  'header',
  'footer',
  'aside',
  'script',
  'style',
  'noscript',
  'svg',
  '[role="navigation"]',
  '[role="banner"]',
  '[role="contentinfo"]',
  '[aria-hidden="true"]'
]

const READABLE_EXTRA_REMOVE_SELECTORS = [
  '.ad',
  '.ads',
  '.advertisement',
  '.social-share',
  '.comments',
  '.sidebar',
  '.related-posts',
  '.newsletter'
]

function pickMainElement(mainSelectors: string[], minTextLength: number): Element {
  const fallback = document.body || document.documentElement
  for (const selector of mainSelectors) {
    const el = document.querySelector(selector)
    if (!el) continue
    const text = ((el as HTMLElement).innerText || el.textContent || '').trim()
    if (text.length > minTextLength) {
      return el
    }
  }
  return fallback
}

function extractCleanMainText(mainEl: Element, removeSelectors: string[]): string {
  const clone = mainEl.cloneNode(true) as Element
  for (const sel of removeSelectors) {
    for (const child of Array.from(clone.querySelectorAll(sel))) child.remove()
  }
  return ((clone as HTMLElement).innerText || clone.textContent || '').replace(/\s+/g, ' ').trim()
}

export function readableFallbackScript(): Record<string, unknown> {
  const mainEl = pickMainElement(READABLE_MAIN_SELECTORS, 100)
  const content = extractCleanMainText(mainEl, [...COMMON_REMOVE_SELECTORS, ...READABLE_EXTRA_REMOVE_SELECTORS])

  let byline = ''
  for (const sel of ['.author', '[rel="author"]', '.byline', '.post-author', 'meta[name="author"]']) {
    const el = document.querySelector(sel)
    if (el) {
      const text = (el.getAttribute('content') || (el as HTMLElement).innerText || '').trim()
      if (text.length > 0 && text.length < 200) {
        byline = text
        break
      }
    }
  }

  return {
    title: document.title || '',
    content,
    excerpt: content.slice(0, 300),
    byline,
    word_count: content.split(/\s+/).filter(Boolean).length,
    url: window.location.href,
    fallback: true
  }
}

function markdownFallbackScript(): Record<string, unknown> {
  const MAX_OUTPUT = 200000
  const mainEl = pickMainElement(MARKDOWN_MAIN_SELECTORS, 100)
  let markdown = extractCleanMainText(mainEl, COMMON_REMOVE_SELECTORS)
  if (markdown.length > MAX_OUTPUT) {
    markdown = markdown.slice(0, MAX_OUTPUT)
  }

  return {
    title: document.title || '',
    markdown,
    url: window.location.href,
    word_count: markdown.split(/\s+/).filter(Boolean).length,
    fallback: true
  }
}

function pageSummaryFallbackScript(): Record<string, unknown> {
  const headings: string[] = []
  for (const heading of Array.from(document.querySelectorAll('h1, h2, h3'))) {
    if (headings.length >= 30) break
    const text = ((heading as HTMLElement).innerText || heading.textContent || '')
      .replace(/\s+/g, ' ')
      .trim()
      .slice(0, 200)
    if (text) headings.push(heading.tagName.toLowerCase() + ': ' + text)
  }

  const navCandidates = document.querySelectorAll('nav a[href], header a[href], [role="navigation"] a[href]')
  const navLinks: Array<{ text: string; href: string }> = []
  const seenNav: Record<string, boolean> = {}
  for (const link of Array.from(navCandidates)) {
    if (navLinks.length >= 25) break
    const linkText = ((link as HTMLElement).innerText || link.textContent || '')
      .replace(/\s+/g, ' ')
      .trim()
      .slice(0, 80)
    let href = link.getAttribute('href') || ''
    try {
      href = new URL(href, window.location.href).href
    } catch {
      /* keep as-is */
    }
    if (!href) continue
    const key = linkText + '|' + href
    if (seenNav[key]) continue
    seenNav[key] = true
    navLinks.push({ text: linkText, href })
  }

  const forms: Array<{ action: string; method: string; fields: string[] }> = []
  for (const form of Array.from(document.querySelectorAll('form'))) {
    if (forms.length >= 10) break
    const fields: string[] = []
    const seenFields: Record<string, boolean> = {}
    for (const field of Array.from(form.querySelectorAll('input, select, textarea'))) {
      if (fields.length >= 25) break
      const name =
        field.getAttribute('name') ||
        field.getAttribute('id') ||
        field.getAttribute('aria-label') ||
        field.getAttribute('type') ||
        field.tagName.toLowerCase()
      const cleaned = (name || '').replace(/\s+/g, ' ').trim().slice(0, 60)
      if (!cleaned || seenFields[cleaned]) continue
      seenFields[cleaned] = true
      fields.push(cleaned)
    }
    let action = form.getAttribute('action') || window.location.href
    try {
      action = new URL(action, window.location.href).href
    } catch {
      /* keep as-is */
    }
    forms.push({ action, method: (form.getAttribute('method') || 'GET').toUpperCase(), fields })
  }

  const mainEl = pickMainElement(PAGE_SUMMARY_MAIN_SELECTORS, 120)
  const mainText = ((mainEl as HTMLElement).innerText || mainEl.textContent || '')
    .replace(/\s+/g, ' ')
    .trim()
    .slice(0, 20000)
  const preview = mainText.slice(0, 500)
  const wordCount = mainText ? mainText.split(/\s+/).filter(Boolean).length : 0

  const interactiveCount = document.querySelectorAll(
    'a[href],button,input:not([type="hidden"]),select,textarea,[role="button"],[role="link"]'
  ).length

  const linkCount = document.querySelectorAll('a[href]').length
  const paragraphCount = document.querySelectorAll('p').length
  const hasSearchInput = !!document.querySelector(
    'input[type="search"], input[name*="search" i], input[placeholder*="search" i]'
  )
  const likelySearchURL = /[?&](q|query|search)=/i.test(window.location.search)
  const hasArticle = document.querySelectorAll('article').length > 0
  const hasTable = document.querySelectorAll('table').length > 0
  let totalFormFields = 0
  for (const f of forms) totalFormFields += f.fields.length

  let type = 'generic'
  if (hasSearchInput && (likelySearchURL || linkCount > 10)) type = 'search_results'
  else if (forms.length > 0 && totalFormFields >= 3 && paragraphCount < 8) type = 'form'
  else if (hasArticle || (paragraphCount >= 8 && linkCount < paragraphCount * 2)) type = 'article'
  else if (hasTable || (interactiveCount > 25 && headings.length >= 2)) type = 'dashboard'
  else if (linkCount > 30 && paragraphCount < 10) type = 'link_list'
  else if (preview.length < 80 && interactiveCount > 10) type = 'app'

  return {
    url: window.location.href,
    title: document.title || '',
    type,
    headings,
    nav_links: navLinks,
    forms,
    interactive_element_count: interactiveCount,
    main_content_preview: preview,
    word_count: wordCount,
    fallback: true
  }
}

export type FallbackScript = () => Record<string, unknown>

// Keys MUST be the runtime message types dispatched by interact-content.ts and
// handled in content/runtime-message-listener.ts. They were previously UPPERCASE,
// which made every FALLBACK_SCRIPTS[messageType] lookup miss — silently disabling
// the content-script-unreachable fallback for all three extractors.
export const FALLBACK_SCRIPTS: Record<string, FallbackScript> = {
  kaboom_get_readable: readableFallbackScript,
  kaboom_get_markdown: markdownFallbackScript,
  kaboom_page_summary: pageSummaryFallbackScript
}
