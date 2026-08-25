  // --- PARTIAL: Core Action Handlers ---
  // Purpose: click, type, select, check, get_text, get_value, get_attribute, set_attribute, focus, scroll_to, wait_for.
  // Why: Separated from main template to keep each partial under 500 LOC.

  function linkForNewTab(clickTarget: HTMLElement): Element | null {
    const tag = clickTarget.tagName.toLowerCase()
    if (tag === 'a') return clickTarget as Element
    if (typeof clickTarget.closest === 'function') {
      return clickTarget.closest('a[href]')
    }
    return null
  }

  function openInNewTab(clickTarget: HTMLElement, linkNode: Element | null, href: string): void {
    let opened = false
    try {
      if (typeof window !== 'undefined' && typeof window.open === 'function') {
        window.open(href, '_blank', 'noopener,noreferrer')
        opened = true
      }
    } catch {
      // EXPECTED_ABSENCE: blocked window.open is normal under page popup
      // policy; logging it would misleadingly report the valid link-click fallback as failure.
    }

    if (!opened && linkNode instanceof Element) {
      const previousTarget = linkNode.getAttribute('target')
      linkNode.setAttribute('target', '_blank')
      ;(linkNode as HTMLElement).click()
      if (previousTarget == null) {
        linkNode.removeAttribute('target')
      } else {
        linkNode.setAttribute('target', previousTarget)
      }
    }
  }

  function structuredTextSections(node: HTMLElement): Array<{header?: string; content: string; expanded?: boolean; tag: string}> {
    // Structured extraction: preserve hierarchy for accordions, lists, etc.
    const sections: Array<{header?: string; content: string; expanded?: boolean; tag: string}> = []
    const children = node.children
    for (let i = 0; i < children.length && sections.length < 50; i++) {
      const child = children[i] as HTMLElement
      if (!child.tagName) continue
      const tag = child.tagName.toLowerCase()
      // Detect accordion/collapsible patterns
      const heading = child.querySelector('h1, h2, h3, h4, h5, h6, [role="heading"], summary, button[aria-expanded]')
      if (heading) {
        const headerText = (heading as HTMLElement).innerText?.trim() || ''
        const ariaExpanded = heading.getAttribute('aria-expanded')
        const expanded = ariaExpanded !== null ? ariaExpanded === 'true' : undefined
        // Get content from sibling/next panel or remaining text
        const contentParts: string[] = []
        const contentNodes = child.querySelectorAll('p, li, span, div, td, pre, code')
        contentNodes.forEach((cn) => {
          if (cn !== heading && !heading.contains(cn)) {
            const t = (cn as HTMLElement).innerText?.trim()
            if (t && t.length > 0) contentParts.push(t)
          }
        })
        sections.push({
          header: headerText,
          content: contentParts.join('\n') || (child.innerText?.replace(headerText, '').trim() || ''),
          expanded,
          tag,
        })
      } else {
        // Non-accordion child: just capture its text
        const t = child.innerText?.trim()
        if (t && t.length > 0) {
          sections.push({ content: t, tag })
        }
      }
    }
    return sections
  }

  function buildActionHandlers(node: Element): Record<string, ActionHandler> {
    return {
      click: () =>
        withMutationTracking(() => {
          if (!(node instanceof HTMLElement)) return domError('not_interactive', `Element is not an HTMLElement: ${node.tagName}`)

          // #332: Bubble up to nearest interactive ancestor if the matched element is a wrapper
          const interactiveAncestor = findInteractiveAncestor(node)
          const clickTarget = (interactiveAncestor instanceof HTMLElement ? interactiveAncestor : node) as HTMLElement

          // Check if element is behind a modal overlay before clicking
          const overlayErr = blockedByOverlayError(node)
          if (overlayErr) return overlayErr
          if (options.new_tab) {
            const linkNode = linkForNewTab(clickTarget)

            const href = linkNode
              ? (linkNode.getAttribute('href') || (linkNode as HTMLAnchorElement).href || '')
              : ''
            if (!href) {
              return domError('new_tab_requires_link', 'new_tab=true requires a link target with href')
            }

            openInNewTab(clickTarget, linkNode, href)

            return mutatingSuccess(clickTarget, { value: href, reason: 'opened_new_tab' })
          }


          // #336: Auto-scroll off-screen elements into view before clicking
          const didScroll = autoScrollIfNeeded(clickTarget)
          clickTarget.click()
          return mutatingSuccess(clickTarget, didScroll ? { auto_scrolled: true } : undefined)
        }),

      type: () =>
        withMutationTracking(() => {
          const overlayErr = blockedByOverlayError(node)
          if (overlayErr) return overlayErr

          // Normalize literal \n sequences to actual newlines (MCP parameter encoding)
          const text = (options.text || '').replace(/\\n/g, '\n')

          // Contenteditable elements (Gmail compose body, rich text editors)
          if (node instanceof HTMLElement && node.isContentEditable) {
            node.focus()
            if (options.clear) {
              const selection = document.getSelection()
              if (selection) {
                selection.selectAllChildren(node)
                selection.deleteFromDocument()
              }
            }

            // Detect rich editor framework
            const editor = detectRichEditor(node)
            let strategy: string

            if (editor) {
              // Native DOM insertion — bypasses CSP, works with Quill/ProseMirror/etc
              insertViaRichEditor(editor.type, editor.target, text, !!options.clear)
              strategy = editor.type + '_native'
            } else {
              // Per-character keyboard event simulation for all generic contenteditable
              insertViaKeyboardSim(node, text)
              strategy = 'keyboard_simulation'
            }

            return mutatingSuccess(node, { value: node.innerText, insertion_strategy: strategy })
          }

          if (!(node instanceof HTMLInputElement) && !(node instanceof HTMLTextAreaElement)) {
            return domError('not_typeable', `Element is not an input, textarea, or contenteditable: ${node.tagName}`)
          }

          // Dispatch per-character keyboard events so React/Vue onChange handlers fire
          node.focus()
          for (const char of text) {
            dispatchKeySequence(node, char, false)
          }

          // Set the value via native setter (needed to bypass React's synthetic event system)
          const proto = node instanceof HTMLTextAreaElement ? HTMLTextAreaElement : HTMLInputElement
          const nativeSetter = Object.getOwnPropertyDescriptor(proto.prototype, 'value')?.set
          if (nativeSetter) {
            const newValue = options.clear ? text : node.value + text
            nativeSetter.call(node, newValue)
          } else {
            node.value = options.clear ? text : node.value + text
          }
          node.dispatchEvent(new InputEvent('input', { bubbles: true, data: text, inputType: 'insertText' }))
          node.dispatchEvent(new Event('change', { bubbles: true }))
          return mutatingSuccess(node, { value: node.value, insertion_strategy: 'native_setter' })
        }),

      select: () =>
        withMutationTracking(() => {
          const overlayErr = blockedByOverlayError(node)
          if (overlayErr) return overlayErr

          if (!(node instanceof HTMLSelectElement)) return domError('not_select', `Element is not a <select>: ${node.tagName}`) // nosemgrep: html-in-template-string
          const nativeSelectSetter = Object.getOwnPropertyDescriptor(HTMLSelectElement.prototype, 'value')?.set
          if (nativeSelectSetter) {
            nativeSelectSetter.call(node, options.value || '')
          } else {
            node.value = options.value || ''
          }
          node.dispatchEvent(new Event('change', { bubbles: true }))
          return mutatingSuccess(node, { value: node.value })
        }),

      check: () =>
        withMutationTracking(() => {
          const overlayErr = blockedByOverlayError(node)
          if (overlayErr) return overlayErr

          if (!(node instanceof HTMLInputElement) || (node.type !== 'checkbox' && node.type !== 'radio')) {
            return domError('not_checkable', `Element is not a checkbox or radio: ${node.tagName} type=${(node as HTMLInputElement).type || 'N/A'}`)
          }
          const desired = options.checked !== undefined ? options.checked : true
          if (node.checked !== desired) {
            node.click()
          }
          return mutatingSuccess(node, { value: node.checked })
        }),

      get_text: () => {
        if (options.structured && node instanceof HTMLElement) {
          const sections = structuredTextSections(node)
          return { success: true, action, selector, sections, section_count: sections.length }
        }
        const text = node instanceof HTMLElement ? node.innerText : node.textContent
        if (text === null || text === undefined) {
          return {
            success: true,
            action,
            selector,
            value: text,
            reason: 'no_text_content',
            message: 'Resolved text content is null'
          }
        }
        return { success: true, action, selector, value: text }
      },

      get_value: () => {
        if (!('value' in node)) return domError('no_value_property', `Element has no value property: ${node.tagName}`)
        const value = (node as HTMLInputElement).value
        if (value === null || value === undefined) {
          return {
            success: true,
            action,
            selector,
            value,
            reason: 'no_value',
            message: 'Element value is null'
          }
        }
        return { success: true, action, selector, value }
      },

      get_attribute: () => {
        const attrName = options.name || ''
        const value = node.getAttribute(attrName)
        if (value === null) {
          return {
            success: true,
            action,
            selector,
            value,
            reason: 'attribute_not_found',
            message: `Attribute "${attrName}" not found`
          }
        }
        return { success: true, action, selector, value }
      },

      set_attribute: () =>
        withMutationTracking(() => {
          node.setAttribute(options.name || '', options.value || '')
          return mutatingSuccess(node, { value: node.getAttribute(options.name || '') })
        }),

      focus: () => {
        const overlayErr = blockedByOverlayError(node)
        if (overlayErr) return overlayErr

        if (!(node instanceof HTMLElement)) return domError('not_focusable', `Element is not an HTMLElement: ${node.tagName}`)
        node.focus()
        return mutatingSuccess(node)
      },

      scroll_to: () => {
        // #387: Container-aware scroll_to — find scrollable ancestor and support directional scrolling
        function findScrollableContainer(el: Element): HTMLElement | null {
          let current: Element | null = el
          while (current && current !== document.documentElement) {
            if (current instanceof HTMLElement && current.scrollHeight > current.clientHeight + 10) {
              const style = typeof getComputedStyle === 'function' ? getComputedStyle(current) : null
              if (style) {
                const ov = style.overflow || ''
                const ovY = style.overflowY || ''
                if (ov === 'auto' || ov === 'scroll' || ovY === 'auto' || ovY === 'scroll') {
                  return current
                }
              }
            }
            current = current.parentElement
          }
          return null
        }

        function scrollToY(container: HTMLElement, top: number): void {
          if (typeof (container as { scrollTo?: unknown }).scrollTo === 'function') {
            container.scrollTo({ top, behavior: 'smooth' })
            return
          }
          ;(container as { scrollTop?: number }).scrollTop = top
        }

        function scrollByY(container: HTMLElement, deltaY: number): void {
          if (typeof (container as { scrollBy?: unknown }).scrollBy === 'function') {
            container.scrollBy({ top: deltaY, behavior: 'smooth' })
            return
          }
          const currentTop = typeof (container as { scrollTop?: unknown }).scrollTop === 'number'
            ? Number((container as { scrollTop?: unknown }).scrollTop)
            : 0
          ;(container as { scrollTop?: number }).scrollTop = currentTop + deltaY
        }

        // Accept both `direction` (preferred) and legacy `value` for backward compatibility.
        const direction = (options.direction || '').toLowerCase()
        const tag = node.tagName.toLowerCase()

        // Check if the target itself is a scrollable container
        const isContainer = node instanceof HTMLElement &&
          node.scrollHeight > node.clientHeight + 10 && (() => {
            const s = typeof getComputedStyle === 'function' ? getComputedStyle(node) : null
            if (!s) return false
            const ov = s.overflow || ''
            const ovY = s.overflowY || ''
            return ov === 'auto' || ov === 'scroll' || ovY === 'auto' || ovY === 'scroll'
          })()

        // Directional scrolling within the resolved container (target, ancestor, or page root)
        const directionalContainer = (() => {
          if (isContainer) return node as HTMLElement
          const ancestor = findScrollableContainer(node)
          if (ancestor) return ancestor
          if (typeof document !== 'undefined' && document.scrollingElement instanceof HTMLElement) {
            return document.scrollingElement
          }
          if (tag === 'body' || tag === 'html') return document.documentElement as HTMLElement
          return document.documentElement as HTMLElement
        })()

        if (direction && directionalContainer) {
          const container = directionalContainer
          switch (direction) {
            case 'top':
              scrollToY(container, 0)
              return mutatingSuccess(node, { reason: 'scrolled_container_top' })
            case 'bottom':
              scrollToY(container, container.scrollHeight)
              return mutatingSuccess(node, { reason: 'scrolled_container_bottom' })
            case 'up':
              scrollByY(container, -container.clientHeight * 0.8)
              return mutatingSuccess(node, { reason: 'scrolled_container_up' })
            case 'down':
              scrollByY(container, container.clientHeight * 0.8)
              return mutatingSuccess(node, { reason: 'scrolled_container_down' })
          }
        }

        // #333: For body/html targets, find the actual scrollable container in SPA layouts
        if (tag === 'body' || tag === 'html') {
          const scrollable = findScrollableContainer(document.body)
          if (scrollable) {
            scrollable.scrollIntoView({ behavior: 'smooth', block: 'center' })
            return mutatingSuccess(node, { reason: 'scrolled_nested_container' })
          }
        }

        // If element is inside a scrollable container, scroll it into view within that container
        const parentContainer = findScrollableContainer(node)
        if (parentContainer && parentContainer !== document.documentElement) {
          node.scrollIntoView({ behavior: 'smooth', block: 'center' })
          return mutatingSuccess(node, { reason: 'scrolled_within_container' })
        }

        node.scrollIntoView({ behavior: 'smooth', block: 'center' })
        return mutatingSuccess(node)
      },

      wait_for: () => ({ success: true, action, selector, value: node.tagName.toLowerCase() }),

      wait_for_text: () => {
        const searchText = options.text || ''
        if (!searchText) {
          return { success: false, action, selector: '', error: 'empty_text', message: 'text parameter is required for wait_for_text' } as DOMResult
        }
        const bodyText = document.body?.innerText ?? ''
        if (bodyText.includes(searchText)) {
          return { success: true, action, selector: '', matched_text: searchText } as DOMResult
        }
        return { success: false, action, selector: '', error: 'text_not_found' } as DOMResult
      },

      wait_for_absent: () => {
        if (!selector) {
          return { success: false, action, selector: '', error: 'missing_selector', message: 'selector is required for wait_for_absent' } as DOMResult
        }
        const el = resolveElement(selector)
        if (!el) {
          return { success: true, action, selector, absent: true } as DOMResult
        }
        return { success: false, action, selector, error: 'element_still_present' } as DOMResult
      },
