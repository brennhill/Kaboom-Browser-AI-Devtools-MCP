// geometry-context.js — Coordinate transforms, action trails, and surrounding UI context.
/* eslint-disable no-unused-vars, no-undef, no-useless-assignment */
function normalizeRect(x1, y1, x2, y2) {
  return {
    x: Math.min(x1, x2),
    y: Math.min(y1, y2),
    width: Math.abs(x2 - x1),
    height: Math.abs(y2 - y1)
  }
}

function scrollOffsets() {
  return {
    x: window.scrollX || window.pageXOffset || 0,
    y: window.scrollY || window.pageYOffset || 0
  }
}

function toDocumentRect(rect) {
  const scroll = scrollOffsets()
  return {
    x: rect.x + scroll.x,
    y: rect.y + scroll.y,
    width: rect.width,
    height: rect.height
  }
}

function toViewportRect(rect, coordSpace) {
  if (!rect) {
    return { x: 0, y: 0, width: 0, height: 0 }
  }
  if (coordSpace === COORD_SPACE_DOCUMENT || coordSpace === undefined || coordSpace === null || coordSpace === '') {
    const scroll = scrollOffsets()
    return {
      x: rect.x - scroll.x,
      y: rect.y - scroll.y,
      width: rect.width,
      height: rect.height
    }
  }
  return {
    x: rect.x,
    y: rect.y,
    width: rect.width,
    height: rect.height
  }
}

function normalizeLoadedAnnotation(annotation) {
  if (!annotation || !annotation.rect) return annotation
  if (annotation.coord_space === COORD_SPACE_DOCUMENT) {
    if (!Array.isArray(annotation.action_trail)) annotation.action_trail = []
    if (!annotation.ui_context) annotation.ui_context = collectUIContextMetadata()
    return annotation
  }
  return {
    ...annotation,
    rect: toDocumentRect(annotation.rect),
    coord_space: COORD_SPACE_DOCUMENT,
    action_trail: Array.isArray(annotation.action_trail) ? annotation.action_trail : [],
    ui_context: annotation.ui_context || collectUIContextMetadata()
  }
}

function recordRecentAction(type, target, extra = {}) {
  const entry = {
    type,
    target_summary: summarizeActionTarget(target),
    timestamp: Date.now(),
    ...extra
  }
  recentActions.push(entry)
  if (recentActions.length > ACTION_BUFFER_LIMIT) {
    recentActions = recentActions.slice(recentActions.length - ACTION_BUFFER_LIMIT)
  }
}

function snapshotActionTrail(limit) {
  const max = Number.isFinite(limit) && limit > 0 ? Math.floor(limit) : ACTION_TRAIL_LIMIT
  const selected = recentActions.slice(-max)
  const now = Date.now()
  return selected.map((entry, index) => ({
    type: entry.type,
    target_summary: entry.target_summary,
    timestamp: entry.timestamp,
    delta_ms: Math.max(0, now - entry.timestamp),
    order: index + 1
  }))
}

function summarizeActionTarget(target) {
  if (!target || !target.tagName) return 'unknown'
  const tag = target.tagName.toLowerCase()
  const selector = safeBuildSelector(target)
  const role = typeof target.getAttribute === 'function' ? target.getAttribute('role') || '' : ''
  const text = (target.textContent || '').trim().replace(/\s+/g, ' ').slice(0, 60)
  const parts = [selector || tag]
  if (role) parts.push(`role=${role}`)
  if (text) parts.push(`text="${text}"`)
  return parts.join(' ')
}

function collectUIContextMetadata() {
  return {
    theme: detectTheme(),
    viewport: {
      width: window.innerWidth,
      height: window.innerHeight
    },
    sidebars: {
      left_open: isSidebarOpen([
        '[data-sidebar="left"]',
        '#left-sidebar',
        '.left-sidebar',
        '.sidebar-left',
        'aside.left'
      ]),
      right_open: isSidebarOpen([
        '[data-sidebar="right"]',
        '#right-sidebar',
        '.right-sidebar',
        '.sidebar-right',
        'aside.right'
      ])
    },
    focused_element: summarizeFocusedElement()
  }
}

function detectTheme() {
  try {
    const html = document.documentElement
    const dataTheme = html?.dataset?.theme
    if (dataTheme === 'dark' || dataTheme === 'light') return dataTheme
    if (html?.classList?.contains('dark')) return 'dark'
    if (html?.classList?.contains('light')) return 'light'
    if (typeof window.matchMedia === 'function' && window.matchMedia('(prefers-color-scheme: dark)').matches) {
      return 'dark'
    }
  } catch {
    // fallback below
  }
  return 'light'
}

function isSidebarOpen(selectors) {
  for (const selector of selectors) {
    let el = null
    try {
      el = document.querySelector(selector)
    } catch {
      el = null
    }
    if (!el) continue
    const rect = el.getBoundingClientRect ? el.getBoundingClientRect() : null
    const width = rect?.width || 0
    const height = rect?.height || 0
    if (width <= 0 || height <= 0) continue
    const computed = window.getComputedStyle?.(el)
    if (computed?.display === 'none' || computed?.visibility === 'hidden') continue
    return true
  }
  return false
}

function summarizeFocusedElement() {
  const el = document.activeElement
  if (!el || el === document.body || el === document.documentElement) return null
  return {
    selector: safeBuildSelector(el),
    tag: el.tagName?.toLowerCase?.() || '',
    role: typeof el.getAttribute === 'function' ? el.getAttribute('role') || '' : '',
    text: (el.textContent || '').trim().replace(/\s+/g, ' ').slice(0, 80)
  }
}

function safeBuildSelector(el) {
  try {
    return buildCSSSelector(el)
  } catch {
    return el?.tagName?.toLowerCase?.() || 'unknown'
  }
}
