// analyze-page-structure.ts — Structural page analysis command handler (#341).

import { registerCommand } from './registry.js'
import { errorMessage } from '../../lib/error-utils.js'

// =============================================================================
// PAGE STRUCTURE ANALYSIS (#341)
// =============================================================================

interface FrameworkInfo {
  name: string
  version: string
  evidence: string
}

interface RoutingInfo {
  type: string
  evidence: string
}

interface ScrollContainer {
  selector: string
  scroll_height: number
  client_height: number
}

interface ModalInfo {
  selector: string
  visible: boolean
  type: string
}

interface MetaInfo {
  viewport: string
  charset: string
  og_title: string
  description: string
}

interface PageStructureResult {
  frameworks: FrameworkInfo[]
  routing: RoutingInfo
  scroll_containers: ScrollContainer[]
  modals: ModalInfo[]
  shadow_roots: number
  meta: MetaInfo
}

/**
 * Combined page structure script. When useGlobals=true (MAIN world), detects
 * frameworks via window globals. When false (ISOLATED fallback), uses DOM hints only.
 * Injected via chrome.scripting func: every helper MUST stay nested inside.
 */
function pageStructureScript(useGlobals: boolean): PageStructureResult {
  const MAX_SCROLL_CONTAINERS = 20
  const MAX_MODALS = 20

  const detectReactWithGlobals = (): FrameworkInfo | null => {
    const reactRoot = document.querySelector('[data-reactroot]')
    const reactContainer = document.getElementById('root')
    const hasReactFiber = reactContainer ? '_reactRootContainer' in reactContainer : false
    if (reactRoot || hasReactFiber) {
      return { name: 'React', version: '', evidence: reactRoot ? 'data-reactroot' : '_reactRootContainer' }
    }
    return null
  }

  const detectFrameworksWithGlobals = (): FrameworkInfo[] => {
    const found: FrameworkInfo[] = []
    const win = window as unknown as Record<string, unknown>

    // Vue
    if (win.__VUE__ || win.__VUE_DEVTOOLS_GLOBAL_HOOK__) {
      const vueObj = win.__VUE__ as Record<string, unknown> | undefined
      const version = typeof vueObj?.version === 'string' ? (vueObj.version as string) : ''
      found.push({ name: 'Vue', version, evidence: 'window.__VUE__' })
    }

    const react = detectReactWithGlobals()
    if (react) found.push(react)

    // Next.js
    if (win.__NEXT_DATA__) {
      const nextData = win.__NEXT_DATA__ as { nextExport?: boolean; buildId?: string }
      found.push({ name: 'Next.js', version: nextData.buildId || '', evidence: 'window.__NEXT_DATA__' })
    }

    // Nuxt
    if (win.__NUXT__ || win.$nuxt) {
      found.push({ name: 'Nuxt', version: '', evidence: win.__NUXT__ ? 'window.__NUXT__' : 'window.$nuxt' })
    }

    // Angular
    if (
      typeof win.ng === 'object' ||
      typeof (win as Record<string, unknown>).getAllAngularRootElements === 'function'
    ) {
      found.push({ name: 'Angular', version: '', evidence: win.ng ? 'window.ng' : 'getAllAngularRootElements' })
    }

    // Svelte (DOM hint — Svelte adds class="svelte-XXXXX" hashes)
    if (document.querySelector('[class^="svelte-"], [class*=" svelte-"]')) {
      found.push({ name: 'Svelte', version: '', evidence: 'class^="svelte-"' })
    }
    return found
  }

  const detectFrameworksWithDOMHints = (): FrameworkInfo[] => {
    const found: FrameworkInfo[] = []
    // ISOLATED world: DOM hints only
    if (document.querySelector('[data-reactroot]') || document.querySelector('[data-reactid]')) {
      found.push({ name: 'React', version: '', evidence: 'data-reactroot' })
    }
    if (document.querySelector('[class^="svelte-"], [class*=" svelte-"]')) {
      found.push({ name: 'Svelte', version: '', evidence: 'class^="svelte-"' })
    }
    if (
      document.querySelector('[ng-version]') ||
      document.querySelector('[_nghost]') ||
      document.querySelector('app-root')
    ) {
      const ver = document.querySelector('[ng-version]')?.getAttribute('ng-version') || ''
      found.push({ name: 'Angular', version: ver, evidence: 'ng-version' })
    }
    if (document.querySelector('#__next') || document.querySelector('[data-nextjs-page]')) {
      found.push({ name: 'Next.js', version: '', evidence: '#__next' })
    }
    if (document.querySelector('#__nuxt') || document.querySelector('#__layout')) {
      found.push({ name: 'Nuxt', version: '', evidence: '#__nuxt' })
    }
    // Vue: data-v-XXXXX scoped attributes (CSS can't match attribute name prefix, need JS check)
    const hasVueScopedAttr =
      document.querySelector('[data-vue-meta]') ||
      Array.from(document.querySelector('#app')?.attributes || document.documentElement.attributes).some((a) =>
        a.name.startsWith('data-v-')
      )
    if (hasVueScopedAttr) {
      found.push({ name: 'Vue', version: '', evidence: 'data-v-*' })
    }
    return found
  }

  const detectRouting = (): RoutingInfo => {
    if (useGlobals) {
      const win = window as unknown as Record<string, unknown>
      if (win.__NEXT_DATA__) {
        return { type: 'next', evidence: '__NEXT_DATA__' }
      }
      if (win.__NUXT__) {
        return { type: 'nuxt', evidence: '__NUXT__' }
      }
      if (window.location.hash.length > 1) {
        return { type: 'hash', evidence: 'location.hash' }
      }
      return { type: 'unknown', evidence: '' }
    }
    if (document.querySelector('#__next')) {
      return { type: 'next', evidence: '#__next' }
    }
    if (document.querySelector('#__nuxt')) {
      return { type: 'nuxt', evidence: '#__nuxt' }
    }
    if (window.location.hash.length > 1) {
      return { type: 'hash', evidence: 'location.hash' }
    }
    return { type: 'unknown', evidence: '' }
  }

  const describeElement = (htmlEl: HTMLElement): string => {
    const tag = htmlEl.tagName.toLowerCase()
    const id = htmlEl.id ? `#${htmlEl.id}` : ''
    const cls =
      htmlEl.className && typeof htmlEl.className === 'string'
        ? '.' + htmlEl.className.trim().split(/\s+/).slice(0, 2).join('.')
        : ''
    return tag + id + cls
  }

  const collectScrollContainers = (): ScrollContainer[] => {
    const containers: ScrollContainer[] = []
    const allElements = document.querySelectorAll('*')
    // Bail out on massive DOMs to avoid expensive getComputedStyle calls (#9.7.6)
    const skipScrollDetection = allElements.length > 50000
    for (const el of Array.from(skipScrollDetection ? [] : allElements)) {
      if (containers.length >= MAX_SCROLL_CONTAINERS) break
      const htmlEl = el as HTMLElement
      if (htmlEl.scrollHeight > htmlEl.clientHeight + 50 && htmlEl.clientHeight > 0) {
        const style = getComputedStyle(htmlEl)
        if (
          style.overflow === 'auto' ||
          style.overflow === 'scroll' ||
          style.overflowY === 'auto' ||
          style.overflowY === 'scroll'
        ) {
          containers.push({
            selector: describeElement(htmlEl),
            scroll_height: htmlEl.scrollHeight,
            client_height: htmlEl.clientHeight
          })
        }
      }
    }
    return containers
  }

  const collectModals = (): ModalInfo[] => {
    const modals: ModalInfo[] = []
    const dialogEls = document.querySelectorAll(
      'dialog, [role="dialog"], [role="alertdialog"], .modal, [aria-modal="true"]'
    )
    for (const el of Array.from(dialogEls)) {
      if (modals.length >= MAX_MODALS) break
      const htmlEl = el as HTMLElement
      const tag = htmlEl.tagName.toLowerCase()
      const id = htmlEl.id ? `#${htmlEl.id}` : ''
      const role = htmlEl.getAttribute('role') || ''
      const isDialog = tag === 'dialog'
      const visible = isDialog
        ? (htmlEl as HTMLDialogElement).open
        : htmlEl.offsetParent !== null || getComputedStyle(htmlEl).display !== 'none'

      let modalType = 'unknown'
      if (tag === 'dialog') modalType = 'dialog'
      else if (role === 'dialog' || role === 'alertdialog') modalType = role
      else if (htmlEl.classList.contains('modal')) modalType = 'modal'
      else if (htmlEl.getAttribute('aria-modal') === 'true') modalType = 'aria-modal'

      modals.push({ selector: tag + id, visible, type: modalType })
    }
    return modals
  }

  const countShadowRoots = (): number => {
    // Cap iteration to avoid blocking on massive DOMs (#9.R8)
    const MAX_SHADOW_WALK = 50000
    let shadowRoots = 0
    let shadowWalked = 0
    const walker = document.createTreeWalker(document.body || document.documentElement, NodeFilter.SHOW_ELEMENT)
    while (walker.nextNode()) {
      shadowWalked++
      if (shadowWalked > MAX_SHADOW_WALK) break
      if ((walker.currentNode as Element).shadowRoot) {
        shadowRoots++
      }
    }
    return shadowRoots
  }

  const readMeta = (): MetaInfo => ({
    viewport: document.querySelector('meta[name="viewport"]')?.getAttribute('content') || '',
    charset:
      document.querySelector('meta[charset]')?.getAttribute('charset') ||
      document.querySelector('meta[http-equiv="Content-Type"]')?.getAttribute('content') ||
      '',
    og_title: document.querySelector('meta[property="og:title"]')?.getAttribute('content') || '',
    description: document.querySelector('meta[name="description"]')?.getAttribute('content') || ''
  })

  return {
    frameworks: useGlobals ? detectFrameworksWithGlobals() : detectFrameworksWithDOMHints(),
    routing: detectRouting(),
    scroll_containers: collectScrollContainers(),
    modals: collectModals(),
    shadow_roots: countShadowRoots(),
    meta: readMeta()
  }
}

registerCommand('page_structure', async (ctx) => {
  try {
    // Try MAIN world first (for framework globals)
    let results
    try {
      results = await chrome.scripting.executeScript({
        target: { tabId: ctx.tabId },
        world: 'MAIN',
        func: pageStructureScript,
        args: [true]
      })
    } catch {
      // MAIN world failed (CSP restriction), fallback to ISOLATED
      results = await chrome.scripting.executeScript({
        target: { tabId: ctx.tabId },
        world: 'ISOLATED',
        func: pageStructureScript,
        args: [false]
      })
    }

    const first = results?.[0]?.result
    const payload = first && typeof first === 'object' ? (first as unknown as Record<string, unknown>) : {}

    ctx.sendResult(payload)
  } catch (err) {
    const message = errorMessage(err, 'Page structure analysis failed')
    ctx.sendResult({
      error: 'page_structure_failed',
      message
    })
  }
})
