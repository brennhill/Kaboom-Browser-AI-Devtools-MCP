// element-analysis.js — Accessibility, selector, CSS-rule, and component-source analysis.
/* eslint-disable no-unused-vars, no-undef */
function runA11yChecks(el, computed) {
  const flags = []
  if (!el || !el.tagName) return flags
  const tag = el.tagName.toLowerCase()
  const getAttribute = (name) => (typeof el.getAttribute === 'function' ? el.getAttribute(name) : null)

  // 1. Image without alt text
  if (tag === 'img' && !getAttribute('alt')) {
    flags.push('missing_alt_text')
  }

  // 2. Interactive element without accessible name
  const interactiveTags = ['button', 'a', 'input', 'select', 'textarea']
  if (interactiveTags.includes(tag)) {
    const hasLabel = getAttribute('aria-label') || getAttribute('aria-labelledby') || getAttribute('title')
    const hasText = (el.textContent || '').trim()
    const hasPlaceholder = getAttribute('placeholder')
    if (!hasLabel && !hasText && !hasPlaceholder) {
      flags.push('missing_accessible_name')
    }
  }

  // 3. Div/span with click handler but no role
  if ((tag === 'div' || tag === 'span') && !getAttribute('role')) {
    if (getAttribute('onclick') || getAttribute('tabindex')) {
      flags.push('interactive_without_role')
    }
  }

  // 4. Contrast ratio check (foreground vs background)
  try {
    if (computed && typeof computed.getPropertyValue === 'function') {
      const fg = parseRGBColor(computed.getPropertyValue('color'))
      const bg = parseRGBColor(computed.getPropertyValue('background-color'))
      if (fg && bg && bg.a > 0) {
        const ratio = contrastRatio(fg, bg)
        const fontSize = parseFloat(computed.getPropertyValue('font-size'))
        const isBold = parseInt(computed.getPropertyValue('font-weight'), 10) >= 700
        const isLargeText = fontSize >= 24 || (fontSize >= 18.66 && isBold)
        const minRatio = isLargeText ? 3 : 4.5
        if (ratio < minRatio) {
          flags.push(`low_contrast:${ratio.toFixed(1)}:1`)
        }
      }
    }
  } catch {
    // Ignore contrast parse errors
  }

  // 5. Focus indicator removed
  try {
    if (interactiveTags.includes(tag) && computed && typeof computed.getPropertyValue === 'function') {
      const outline = computed.getPropertyValue('outline')
      const outlineStyle = computed.getPropertyValue('outline-style')
      if (outlineStyle === 'none' || outline === '0' || outline === 'none') {
        const boxShadow = computed.getPropertyValue('box-shadow')
        if (!boxShadow || boxShadow === 'none') {
          flags.push('no_focus_indicator')
        }
      }
    }
  } catch {
    // Ignore focus indicator check errors
  }

  // 6. Missing form label
  try {
    if ((tag === 'input' || tag === 'select' || tag === 'textarea') && !getAttribute('aria-label')) {
      const id = el.id
      const hasLabelFor =
        id &&
        typeof document !== 'undefined' &&
        typeof document.querySelector === 'function' &&
        document.querySelector(`label[for="${CSS.escape(id)}"]`)
      if (!hasLabelFor) {
        const parent = typeof el.closest === 'function' ? el.closest('label') : null
        if (!parent) {
          flags.push('missing_form_label')
        }
      }
    }
  } catch {
    // Ignore form label check errors
  }

  // 7. Small touch target (< 44x44 CSS pixels per WCAG 2.5.8)
  try {
    if (interactiveTags.includes(tag) && typeof el.getBoundingClientRect === 'function') {
      const rect = el.getBoundingClientRect()
      if (rect.width > 0 && rect.height > 0 && (rect.width < 44 || rect.height < 44)) {
        flags.push(`small_touch_target:${Math.round(rect.width)}x${Math.round(rect.height)}`)
      }
    }
  } catch {
    // Ignore touch target check errors
  }

  return flags
}

/**
 * Parse an RGB/RGBA color string into {r, g, b, a}.
 * @param {string} str - e.g. "rgb(255, 0, 0)" or "rgba(0, 0, 0, 0.5)"
 * @returns {{r:number, g:number, b:number, a:number}|null}
 */
function parseRGBColor(str) {
  if (!str) return null
  const m = str.match(/rgba?\((\d+),\s*(\d+),\s*(\d+)(?:,\s*([\d.]+))?\)/)
  if (!m) return null
  return {
    r: parseInt(m[1], 10),
    g: parseInt(m[2], 10),
    b: parseInt(m[3], 10),
    a: m[4] !== undefined ? parseFloat(m[4]) : 1
  }
}

/**
 * Calculate relative luminance of an sRGB color per WCAG 2.x.
 * @param {{r:number, g:number, b:number}} c
 * @returns {number}
 */
function luminance(c) {
  const [rs, gs, bs] = [c.r / 255, c.g / 255, c.b / 255].map((v) =>
    v <= 0.04045 ? v / 12.92 : Math.pow((v + 0.055) / 1.055, 2.4)
  )
  return 0.2126 * rs + 0.7152 * gs + 0.0722 * bs
}

/**
 * Calculate WCAG contrast ratio between two colors.
 * @param {{r:number, g:number, b:number}} fg
 * @param {{r:number, g:number, b:number}} bg
 * @returns {number}
 */
function contrastRatio(fg, bg) {
  const l1 = luminance(fg)
  const l2 = luminance(bg)
  const lighter = Math.max(l1, l2)
  const darker = Math.min(l1, l2)
  return (lighter + 0.05) / (darker + 0.05)
}

/**
 * Build a CSS selector for the element.
 */
function buildCSSSelector(el) {
  const tag = el.tagName.toLowerCase()
  if (el.id) return `${tag}#${CSS.escape(el.id)}`
  const classes = Array.from(el.classList).slice(0, 3)
  if (classes.length > 0) return `${tag}.${classes.map((c) => CSS.escape(c)).join('.')}`
  return tag
}

const MAX_SELECTOR_CANDIDATES = 8

function collectSelectorCandidates(el) {
  const candidates = []
  if (!el || !el.tagName) {
    return candidates
  }

  const safeAdd = (candidate) => {
    if (!candidate || typeof candidate !== 'string') return
    const normalized = candidate.trim()
    if (!normalized || candidates.includes(normalized)) return
    if (candidates.length >= MAX_SELECTOR_CANDIDATES) return
    candidates.push(normalized)
  }
  const getAttribute = (name) => (typeof el.getAttribute === 'function' ? el.getAttribute(name) : null)
  const normalizeText = (value, max) =>
    String(value || '')
      .replace(/\s+/g, ' ')
      .replace(/\|/g, '/')
      .trim()
      .slice(0, max)
  const escapeAttr = (value) => {
    const raw = String(value || '')
    if (typeof CSS !== 'undefined' && typeof CSS.escape === 'function') return CSS.escape(raw)
    return raw.replace(/\\/g, '\\\\').replace(/"/g, '\\"')
  }
  const tag = el.tagName.toLowerCase()
  const text = normalizeText(el.textContent || '', 80)

  if (el.id) {
    safeAdd(`css=#${escapeAttr(el.id)}`)
  }

  const testID = getAttribute('data-testid') || getAttribute('data-test-id') || getAttribute('data-cy')
  if (testID) {
    safeAdd(`testid=${normalizeText(testID, 120)}`)
  }

  const ariaLabel = getAttribute('aria-label')
  if (ariaLabel) {
    safeAdd(`label=${normalizeText(ariaLabel, 120)}`)
  }

  const placeholder = getAttribute('placeholder')
  if (placeholder) {
    safeAdd(`placeholder=${normalizeText(placeholder, 120)}`)
  }

  const explicitRole = getAttribute('role')
  const implicitRole = inferImplicitRole(el)
  const role = explicitRole || implicitRole
  if (role && text) {
    safeAdd(`role=${normalizeText(role, 60)}|${text}`)
  } else if (role) {
    safeAdd(`role=${normalizeText(role, 60)}`)
  }

  if (text) {
    safeAdd(`text=${text}`)
  }

  const nameAttr = getAttribute('name')
  if (nameAttr) {
    safeAdd(`css=${tag}[name="${escapeAttr(nameAttr)}"]`)
  }

  safeAdd(`css=${buildCSSSelector(el)}`)
  return candidates
}

function inferImplicitRole(el) {
  if (!el || !el.tagName) return ''
  const tag = el.tagName.toLowerCase()
  if (tag === 'button') return 'button'
  if (tag === 'a' && typeof el.getAttribute === 'function' && el.getAttribute('href')) return 'link'
  if (tag === 'select') return 'combobox'
  if (tag === 'textarea') return 'textbox'
  if (tag === 'input') {
    const inputType = (typeof el.getAttribute === 'function' ? el.getAttribute('type') : '') || 'text'
    switch (inputType.toLowerCase()) {
      case 'button':
      case 'submit':
      case 'reset':
        return 'button'
      case 'checkbox':
        return 'checkbox'
      case 'radio':
        return 'radio'
      default:
        return 'textbox'
    }
  }
  return ''
}

/**
 * Trace CSS rules that match an element using document.styleSheets.
 * Returns matched rules with selector, properties, and source stylesheet.
 * Capped at MAX_MATCHED_RULES to avoid huge payloads.
 */
const MAX_MATCHED_RULES = 20
const MAX_RULES_EXAMINED = 5000 // Safety cap to prevent excessive work on huge stylesheets

function traceMatchedCSSRules(el) {
  const rules = []
  let totalExamined = 0
  try {
    for (const sheet of document.styleSheets) {
      if (totalExamined >= MAX_RULES_EXAMINED) break
      let cssRules
      try {
        cssRules = sheet.cssRules || sheet.rules
      } catch {
        // CORS blocks access to cross-origin stylesheets
        rules.push({
          stylesheet: sheet.href || '(inline)',
          access: 'blocked',
          note: 'Cross-origin stylesheet — rules not accessible'
        })
        continue
      }
      if (!cssRules) continue

      const sheetHref = sheet.href || '(inline)'
      for (let i = 0; i < cssRules.length; i++) {
        if (rules.length >= MAX_MATCHED_RULES) break
        if (++totalExamined > MAX_RULES_EXAMINED) break
        const rule = cssRules[i]
        if (rule.type !== CSSRule.STYLE_RULE) continue
        try {
          if (!el.matches(rule.selectorText)) continue
        } catch {
          continue // Invalid selector
        }
        // Extract only properties that differ from defaults (non-empty)
        const properties = {}
        for (let j = 0; j < rule.style.length; j++) {
          const prop = rule.style[j]
          properties[prop] = rule.style.getPropertyValue(prop)
          const priority = rule.style.getPropertyPriority(prop)
          if (priority) properties[prop] += ' !' + priority
        }
        rules.push({
          selector: rule.selectorText,
          properties,
          stylesheet: sheetHref,
          rule_index: i
        })
      }
      if (rules.length >= MAX_MATCHED_RULES) break
    }
  } catch {
    // Stylesheet enumeration may fail in rare cases
  }
  return rules
}

/**
 * Detect framework component information for an element.
 * Supports React, Vue, Angular, and common data attributes.
 */
function detectComponentSource(el) {
  const info = {}

  // React: __reactFiber$ or __reactInternalInstance$
  try {
    for (const key of Object.keys(el)) {
      if (key.startsWith('__reactFiber$') || key.startsWith('__reactInternalInstance$')) {
        const fiber = el[key]
        if (fiber) {
          info.framework = 'react'
          // Walk up to find the named component
          let node = fiber
          for (let depth = 0; depth < 10 && node; depth++) {
            if (typeof node.type === 'function' || typeof node.type === 'object') {
              const name = node.type?.displayName || node.type?.name || node.type?.render?.name
              if (name) {
                info.component = name
                // Try to get source file from _source (dev mode only)
                if (node._debugSource) {
                  info.source_file = node._debugSource.fileName
                  info.source_line = node._debugSource.lineNumber
                }
                break
              }
            }
            node = node.return
          }
        }
        break
      }
    }
  } catch {
    // React internals may throw
  }

  // Vue 2: __vue__ / Vue 3: __vue_app__ or __vueParentComponent
  if (!info.framework) {
    try {
      const vue = el.__vue__ || el.__vueParentComponent
      if (vue) {
        info.framework = 'vue'
        info.component = vue.$options?.name || vue.type?.name || vue.type?.__name || ''
        if (vue.$options?.__file) info.source_file = vue.$options.__file
        if (vue.type?.__file) info.source_file = vue.type.__file
      }
    } catch {
      // Vue internals may throw
    }
  }

  // Angular: ng-* attributes
  if (!info.framework) {
    try {
      for (const attr of el.attributes) {
        if (attr.name.startsWith('_ngcontent') || attr.name.startsWith('_nghost')) {
          info.framework = 'angular'
          // Try to get component name from ng-reflect-* or constructor
          const ngComponent = el.__ngContext__
          if (ngComponent) {
            info.component = el.constructor?.name || ''
          }
          break
        }
      }
    } catch {
      // Angular detection may fail
    }
  }

  // Common data attributes
  try {
    const testId = el.getAttribute('data-testid') || el.getAttribute('data-test-id') || el.getAttribute('data-cy')
    if (testId) info.test_id = testId
    const component = el.getAttribute('data-component') || el.getAttribute('data-source')
    if (component) info.data_component = component
  } catch {
    // Attribute access may fail
  }

  return Object.keys(info).length > 0 ? info : null
}

// ============================================================================
// PERSISTENCE (chrome.storage.session)
// ============================================================================

const MAX_PERSISTED_ANNOTATIONS = 50

// Guard: detect if chrome.storage.session is accessible in this execution context.
// In web_accessible_resource contexts the API object exists but every call throws
// "Access to storage is not allowed from this context". We disable persistence
// permanently on the first failure to avoid noisy console errors.
let storageAvailable = typeof chrome !== 'undefined' && !!chrome.storage?.session
