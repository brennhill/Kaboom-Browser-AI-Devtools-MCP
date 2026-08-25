// element-capture.js — DOM capture, element summaries, style detail, and framework detection.
/* eslint-disable no-unused-vars, no-undef */
function captureElementsUnderRect(rect) {
  if (!overlay) return { summary: '', detail: {} }

  // Temporarily hide overlay
  overlay.style.pointerEvents = 'none'
  overlay.style.visibility = 'hidden'

  try {
    const seenElements = new Set()
    const elements = collectSampledElements(sampleRectPoints(rect), seenElements)
    if (elements.length === 0) probeCenterElement(rect, seenElements, elements)
    if (elements.length === 0) collectOverlappingElements(rect, seenElements, elements)

    // Also probe inside same-origin iframes
    const iframeElements = captureIframeElements(rect, seenElements)

    return buildCaptureResult(elements, iframeElements)
  } finally {
    // Always restore overlay, even if an exception occurs
    if (overlay) {
      overlay.style.pointerEvents = ''
      overlay.style.visibility = ''
    }
  }
}

function sampleRectPoints(rect) {
  // Sample points: corners + center + edge midpoints for better coverage
  return [
    { x: rect.x + rect.width / 2, y: rect.y + rect.height / 2 }, // center
    { x: rect.x + 2, y: rect.y + 2 }, // top-left
    { x: rect.x + rect.width - 2, y: rect.y + 2 }, // top-right
    { x: rect.x + 2, y: rect.y + rect.height - 2 }, // bottom-left
    { x: rect.x + rect.width - 2, y: rect.y + rect.height - 2 }, // bottom-right
    { x: rect.x + rect.width / 2, y: rect.y + 2 }, // top-center
    { x: rect.x + rect.width / 2, y: rect.y + rect.height - 2 }, // bottom-center
    { x: rect.x + 2, y: rect.y + rect.height / 2 }, // left-center
    { x: rect.x + rect.width - 2, y: rect.y + rect.height / 2 } // right-center
  ]
}

function collectSampledElements(points, seenElements) {
  const elements = []
  for (const pt of points) {
    try {
      const els = document.elementsFromPoint(pt.x, pt.y)
      for (const el of els) {
        if (seenElements.has(el)) continue
        if (el === document.body || el === document.documentElement) continue
        seenElements.add(el)
        elements.push(el)
        if (elements.length >= MAX_CAPTURED_ELEMENTS) break
      }
    } catch {
      // EXPECTED_ABSENCE: page-owned access can normally throw for detached,
      // cross-origin, or hostile objects; logging it would misleadingly blame Kaboom for page behavior.
      // elementsFromPoint may fail on some edge cases
    }
    if (elements.length >= MAX_CAPTURED_ELEMENTS) break
  }
  return elements
}

function probeCenterElement(rect, seenElements, elements) {
  // Fallback: if grid sampling found nothing, try single-point probe at rect center
  try {
    const cx = rect.x + rect.width / 2
    const cy = rect.y + rect.height / 2
    const el = document.elementFromPoint(cx, cy)
    if (el && el !== document.body && el !== document.documentElement && !seenElements.has(el)) {
      seenElements.add(el)
      elements.push(el)
    }
  } catch {
    // EXPECTED_ABSENCE: page-owned access can normally throw for detached,
    // cross-origin, or hostile objects; logging it would misleadingly blame Kaboom for page behavior.
    // elementFromPoint fallback failed
  }
}

function collectOverlappingElements(rect, seenElements, elements) {
  // Fallback: walk DOM for elements whose bounding rect overlaps the drawn rectangle
  try {
    const candidates = document.querySelectorAll('*')
    for (const el of candidates) {
      if (el === document.body || el === document.documentElement) continue
      if (seenElements.has(el)) continue
      const br = el.getBoundingClientRect()
      if (br.width === 0 && br.height === 0) continue
      const overlaps =
        br.left < rect.x + rect.width && br.right > rect.x && br.top < rect.y + rect.height && br.bottom > rect.y
      if (overlaps) {
        seenElements.add(el)
        elements.push(el)
        if (elements.length >= MAX_CAPTURED_ELEMENTS) break
      }
    }
  } catch {
    // EXPECTED_ABSENCE: page-owned access can normally throw for detached,
    // cross-origin, or hostile objects; logging it would misleadingly blame Kaboom for page behavior.
    // DOM walk fallback failed
  }
}

function buildCaptureResult(elements, iframeElements) {
  // Pick the most relevant element for the summary
  const target = pickBestElement(elements) || elements[0]

  if (!target && elements.length === 0 && iframeElements.length === 0) {
    return { summary: '', detail: {} }
  }

  const summary = target ? buildElementSummary(target) : ''

  // Build comprehensive detail: primary element + all elements + iframes
  const primaryDetail = target ? buildElementDetail(target) : {}
  const allElementDetails = elements.slice(0, MAX_CAPTURED_ELEMENTS).map((el) => ({
    tag: el.tagName.toLowerCase(),
    selector: buildCSSSelector(el),
    text: (el.textContent || '').trim().slice(0, 100),
    classes: Array.from(el.classList).slice(0, 10)
  }))

  const detail = {
    ...primaryDetail,
    all_elements: allElementDetails,
    element_count: elements.length
  }

  if (iframeElements.length > 0) {
    detail.iframe_content = iframeElements
  }

  return { summary, detail }
}

/**
 * Capture elements inside same-origin iframes that overlap the drawn rectangle.
 * Cross-origin iframes are noted but their DOM is inaccessible.
 */
function iframeOverlapsRect(iframeRect, rect) {
  return (
    iframeRect.right >= rect.x &&
    iframeRect.left <= rect.x + rect.width &&
    iframeRect.bottom >= rect.y &&
    iframeRect.top <= rect.y + rect.height
  )
}

function captureIframeElements(rect, seenElements) {
  const results = []
  try {
    const iframes = document.querySelectorAll('iframe')
    for (const iframe of iframes) {
      const iframeRect = iframe.getBoundingClientRect()
      // Check if iframe overlaps with drawn rectangle
      if (!iframeOverlapsRect(iframeRect, rect)) {
        continue
      }
      try {
        const iframeDoc = iframe.contentDocument
        if (!iframeDoc) {
          results.push({ src: iframe.src, access: 'cross_origin', note: 'Cannot access cross-origin iframe DOM' })
          continue
        }
        // Adjust coordinates relative to iframe
        const adjustedX = rect.x - iframeRect.left + rect.width / 2
        const adjustedY = rect.y - iframeRect.top + rect.height / 2
        const iframeEls = captureIframeInnerElements(iframeDoc, adjustedX, adjustedY, seenElements)
        if (iframeEls.length > 0) {
          results.push({ src: iframe.src, access: 'same_origin', elements: iframeEls })
        }
      } catch {
        results.push({ src: iframe.src, access: 'blocked', note: 'SecurityError accessing iframe' })
      }
    }
  } catch {
    // EXPECTED_ABSENCE: page-owned access can normally throw for detached,
    // cross-origin, or hostile objects; logging it would misleadingly blame Kaboom for page behavior.
    // Ignore iframe enumeration errors
  }
  return results
}

function captureIframeInnerElements(iframeDoc, x, y, seenElements) {
  const els = iframeDoc.elementsFromPoint(x, y)
  const iframeEls = []
  for (const el of els) {
    if (seenElements.has(el)) continue
    if (el === iframeDoc.body || el === iframeDoc.documentElement) continue
    seenElements.add(el)
    iframeEls.push({
      tag: el.tagName.toLowerCase(),
      selector: buildCSSSelector(el),
      text: (el.textContent || '').trim().slice(0, 100),
      outer_html: el.outerHTML.slice(0, 500)
    })
    if (iframeEls.length >= 5) break
  }
  return iframeEls
}

/**
 * Re-capture DOM element details for all existing annotations.
 * Called right before screenshot to ensure DOM data matches the visual state.
 */
function refreshElementDetails() {
  if (!overlay) return
  for (const ann of annotations) {
    if (!ann.rect || !ann.correlation_id) continue
    try {
      const freshData = captureElementsUnderRect(toViewportRect(ann.rect, ann.coord_space))
      if (freshData.detail && Object.keys(freshData.detail).length > 0) {
        const existing = elementDetails.get(ann.correlation_id) || {}
        elementDetails.set(ann.correlation_id, {
          ...freshData.detail,
          action_trail: existing.action_trail || ann.action_trail || [],
          ui_context: existing.ui_context || ann.ui_context || collectUIContextMetadata()
        })
        ann.element_summary = freshData.summary || ann.element_summary
      }
    } catch {
      // EXPECTED_ABSENCE: page-owned access can normally throw for detached,
      // cross-origin, or hostile objects; logging it would misleadingly blame Kaboom for page behavior.
      // Keep existing data if re-capture fails
    }
  }
}

/**
 * Pick the most semantically relevant element from candidates.
 * Prefers interactive elements (button, a, input) over containers (div, span).
 */
function pickBestElement(elements) {
  const interactiveTags = new Set(['BUTTON', 'A', 'INPUT', 'SELECT', 'TEXTAREA', 'LABEL'])
  for (const el of elements) {
    if (interactiveTags.has(el.tagName)) return el
  }
  // Fall back to first element with meaningful text content
  for (const el of elements) {
    const text = el.textContent?.trim()
    if (text && text.length < 200 && text.length > 0) return el
  }
  return null
}

/**
 * Build compact element summary: "tag.class1.class2 'text'"
 */
function buildElementSummary(el) {
  const tag = el.tagName.toLowerCase()
  const classes = Array.from(el.classList).slice(0, 3).join('.')
  const text = (el.textContent || '').trim().slice(0, 40)
  let summary = tag
  if (classes) summary += '.' + classes
  if (text) summary += ` '${text}'`
  return summary
}

const DETAIL_STYLE_PROPS = [
  'background-color',
  'color',
  'font-size',
  'font-weight',
  'font-family',
  'padding',
  'margin',
  'border',
  'border-radius',
  'display',
  'position',
  'z-index',
  'width',
  'height',
  'opacity',
  'flex-direction',
  'flex-wrap',
  'align-items',
  'justify-content',
  'gap',
  'grid-template-columns',
  'grid-template-rows',
  'overflow',
  'text-align',
  'text-decoration',
  'line-height',
  'letter-spacing',
  'box-shadow',
  'transform',
  'transition',
  'cursor',
  'visibility',
  'white-space',
  'max-width',
  'min-width',
  'max-height',
  'min-height'
]

function collectComputedStyles(computed) {
  const computedStyles = {}
  for (const prop of DETAIL_STYLE_PROPS) {
    computedStyles[prop] = computed.getPropertyValue(prop)
  }
  return computedStyles
}

function isSignificantAncestor(node) {
  return !!node && node !== document.body && node !== document.documentElement
}

/**
 * Build full element detail for lazy retrieval.
 */
function buildElementDetail(el) {
  const computed = window.getComputedStyle(el)
  const computedStyles = collectComputedStyles(computed)
  const boundingRect = el.getBoundingClientRect()
  const parentSelector = buildParentSelector(el)
  const outerHtml = captureOuterHtml(el)
  const shadowInfo = detectShadowInfo(el)
  const parentContext = buildParentContext(el)
  const siblings = captureSiblings(el)

  const detail = {
    selector: buildCSSSelector(el),
    tag: el.tagName.toLowerCase(),
    text_content: (el.textContent || '').trim().slice(0, 200),
    outer_html: outerHtml,
    classes: Array.from(el.classList).slice(0, 20),
    id: el.id || '',
    computed_styles: computedStyles,
    parent_selector: parentSelector,
    bounding_rect: {
      x: Math.round(boundingRect.x),
      y: Math.round(boundingRect.y),
      width: Math.round(boundingRect.width),
      height: Math.round(boundingRect.height)
    },
    a11y_flags: runA11yChecks(el, computed)
  }

  attachElementDetailExtras(detail, el, { parentContext, siblings, shadowInfo })
  return detail
}

function attachElementDetailExtras(detail, el, captured) {
  const selectorCandidates = collectSelectorCandidates(el)
  if (selectorCandidates.length > 0) {
    detail.selector_candidates = selectorCandidates
  }
  if (captured.parentContext) {
    detail.parent_context = captured.parentContext
  }
  if (captured.siblings.length > 0) {
    detail.siblings = captured.siblings
  }
  const cssFramework = detectCSSFramework(el)
  if (cssFramework) {
    detail.css_framework = cssFramework
  }
  if (captured.shadowInfo) {
    detail.shadow_dom = captured.shadowInfo
  }

  // CSS rule tracing — find source stylesheets and selectors
  const matchedRules = traceMatchedCSSRules(el)
  if (matchedRules.length > 0) {
    detail.matched_css_rules = matchedRules
  }

  // Framework component detection
  const componentInfo = detectComponentSource(el)
  if (componentInfo) {
    if (componentInfo.framework) {
      detail.js_framework = componentInfo.framework
    }
    detail.component = componentInfo
  }
}

function buildParentSelector(el) {
  // Build parent selector
  let parentSelector = ''
  try {
    const parent = el.parentElement
    if (isSignificantAncestor(parent)) {
      const pTag = parent.tagName.toLowerCase()
      const pClasses = Array.from(parent.classList).slice(0, 2).join('.')
      parentSelector = pTag
      if (parent.id) parentSelector += '#' + parent.id
      else if (pClasses) parentSelector += '.' + pClasses
      parentSelector += ' > '

      const childTag = el.tagName.toLowerCase()
      const childClasses = Array.from(el.classList).slice(0, 2).join('.')
      parentSelector += childTag
      if (el.id) parentSelector += '#' + el.id
      else if (childClasses) parentSelector += '.' + childClasses
    }
  } catch {
    // EXPECTED_ABSENCE: page-owned access can normally throw for detached,
    // cross-origin, or hostile objects; logging it would misleadingly blame Kaboom for page behavior.
    // Ignore selector build errors
  }
  return parentSelector
}

function captureOuterHtml(el) {
  // Capture outer HTML (truncated for large elements)
  let outerHtml = ''
  try {
    outerHtml = el.outerHTML.slice(0, 2000)
  } catch {
    // EXPECTED_ABSENCE: page-owned access can normally throw for detached,
    // cross-origin, or hostile objects; logging it would misleadingly blame Kaboom for page behavior.
    // outerHTML may fail on some special elements
  }
  return outerHtml
}

function detectShadowInfo(el) {
  // Shadow DOM detection
  try {
    if (el.shadowRoot) {
      // Open shadow DOM — capture inner HTML
      return {
        mode: 'open',
        html: el.shadowRoot.innerHTML.slice(0, 2000),
        child_count: el.shadowRoot.childElementCount
      }
    }
    if (el.attachShadow) {
      // Element supports shadow DOM but may have closed shadow root
      // We can detect this heuristically: if the element has no children but renders content
      const hasVisibleContent = el.getBoundingClientRect().height > 0
      const hasLightDOMChildren = el.childElementCount > 0
      if (hasVisibleContent && !hasLightDOMChildren && el.tagName.includes('-')) {
        return { mode: 'closed', note: 'Element likely has closed shadow DOM (content not accessible)' }
      }
    }
  } catch {
    // EXPECTED_ABSENCE: page-owned access can normally throw for detached,
    // cross-origin, or hostile objects; logging it would misleadingly blame Kaboom for page behavior.
    // Shadow DOM access may fail
  }
  return null
}

function describeAncestor(node) {
  return {
    tag: node.tagName.toLowerCase(),
    classes: Array.from(node.classList).slice(0, 5),
    id: node.id || '',
    role: (node.getAttribute && node.getAttribute('role')) || ''
  }
}

function buildParentContext(el) {
  // Build parent_context: structured 2-level ancestry
  try {
    const parent = el.parentElement
    if (isSignificantAncestor(parent)) {
      const parentInfo = describeAncestor(parent)
      let grandparentInfo = null
      if (isSignificantAncestor(parent.parentElement)) {
        grandparentInfo = describeAncestor(parent.parentElement)
      }
      return { parent: parentInfo, grandparent: grandparentInfo }
    }
  } catch {
    // EXPECTED_ABSENCE: page-owned access can normally throw for detached,
    // cross-origin, or hostile objects; logging it would misleadingly blame Kaboom for page behavior.
    // Ignore parent context build errors
  }
  return null
}

function pushSiblingSummaries(range, position, siblings) {
  for (const sib of range) {
    siblings.push({
      tag: sib.tagName.toLowerCase(),
      classes: Array.from(sib.classList).slice(0, 5),
      text: (sib.textContent || '').trim().slice(0, 60),
      position
    })
  }
}

function captureSiblings(el) {
  // Build siblings: up to 2 before and 2 after the target element
  const siblings = []
  try {
    const parent = el.parentElement
    if (parent) {
      const children = Array.from(parent.children)
      const idx = children.indexOf(el)
      if (idx >= 0) {
        const before = children.slice(Math.max(0, idx - 2), idx)
        const after = children.slice(idx + 1, idx + 3)
        pushSiblingSummaries(before, 'before', siblings)
        pushSiblingSummaries(after, 'after', siblings)
      }
    }
  } catch {
    // EXPECTED_ABSENCE: page-owned access can normally throw for detached,
    // cross-origin, or hostile objects; logging it would misleadingly blame Kaboom for page behavior.
    // Ignore sibling capture errors
  }
  return siblings
}

const TAILWIND_SPECIFIC_PATTERN =
  /^(p-\d|m-\d|px-\d|py-\d|mx-\d|my-\d|pt-\d|pb-\d|pl-\d|pr-\d|mt-\d|mb-\d|ml-\d|mr-\d|text-(xs|sm|base|lg|xl|2xl|3xl)|font-(thin|light|normal|medium|semibold|bold)|bg-[a-z]+-\d{2,3}|w-\d|h-\d|gap-\d|space-[xy]-\d|max-w-[\w-]+|min-w-[\w-]+|max-h-[\w-]+|min-h-[\w-]+|justify-[\w-]+|items-[\w-]+|self-[\w-]+|z-\d|opacity-[\w]+|duration-[\w]+|ease-[\w-]+|translate-[\w-]+|scale-[\w-]+|rotate-[\w-]+|skew-[\w-]+|origin-[\w-]+|delay-[\w]+)$/
const TAILWIND_GENERIC_PATTERN = /^(flex|grid|block|inline|hidden|rounded|border|shadow|overflow-|transition)$/
const BOOTSTRAP_PATTERN =
  /^(col-(xs|sm|md|lg|xl)-\d+|col-\d+|btn-[a-z]+|form-control|form-group|form-check|input-group|card|container|row|navbar|nav-[a-z]+|modal|badge|alert|dropdown|table|pagination)$/
const CSS_MODULES_PATTERN = /^[A-Z][a-zA-Z]*_[a-zA-Z]+__[a-zA-Z0-9]{5,}$/
const STYLED_COMPONENTS_PATTERN = /^(css-[a-z0-9]+|sc-[a-zA-Z]+)$/

function countMatchingClasses(classes, pattern) {
  let hits = 0
  for (const cls of classes) {
    if (pattern.test(cls)) hits++
  }
  return hits
}

function countTailwindHits(classes) {
  // Tailwind: utility class patterns (require at least 1 dash-pattern for confidence)
  let hits = 0
  let specificHits = 0
  for (const cls of classes) {
    if (TAILWIND_SPECIFIC_PATTERN.test(cls)) {
      hits++
      specificHits++
    } else if (TAILWIND_GENERIC_PATTERN.test(cls)) hits++
  }
  return { hits, specificHits }
}

/**
 * Detect CSS framework from element class names.
 * Returns framework name string or empty string if no confident match.
 */
function detectCSSFramework(el) {
  try {
    const classes = Array.from(el.classList)
    if (classes.length === 0) return ''

    const tailwind = countTailwindHits(classes)
    if (tailwind.hits >= 3 && tailwind.specificHits >= 1) return 'tailwind'

    // Bootstrap: component/grid patterns
    if (countMatchingClasses(classes, BOOTSTRAP_PATTERN) >= 2) return 'bootstrap'

    // CSS Modules: hash-suffixed classes like Component_name__hash
    if (countMatchingClasses(classes, CSS_MODULES_PATTERN) >= 1) return 'css-modules'

    // Styled-components/Emotion: css-* or sc-* prefixed classes
    if (countMatchingClasses(classes, STYLED_COMPONENTS_PATTERN) >= 2) return 'styled-components'

    return ''
  } catch {
    // EXPECTED_ABSENCE: inaccessible page-owned stylesheets are normal; logging would mislabel optional inference as failure.
    return ''
  }
}

/**
 * Run lightweight accessibility checks on an element.
 * Returns an array of flag strings describing potential issues.
 * @param {Element} el
 * @param {CSSStyleDeclaration} computed
 * @returns {string[]}
 */
