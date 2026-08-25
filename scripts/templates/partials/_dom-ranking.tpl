  // --- PARTIAL: Ambiguous Target Ranking ---

  function selectorRankingLabel(selectorText: string): string {
    // Extract the text portion from semantic selectors (text=Post → "Post")
    if (selectorText.startsWith('text=')) return selectorText.slice(5)
    if (selectorText.startsWith('aria-label=')) return selectorText.slice(11)
    if (selectorText.startsWith('label=')) return selectorText.slice(6)
    if (selectorText.startsWith('placeholder=')) return selectorText.slice(12)
    return ''
  }

  function clickActionScore(el: Element, tag: string, role: string): number {
    const isButtonLike =
      tag === 'button' ||
      role === 'button' ||
      (tag === 'input' && ((el as HTMLInputElement).type === 'submit' || (el as HTMLInputElement).type === 'button'))
    if (isButtonLike) return 100
    if (tag === 'a' || role === 'link') return 40
    return 0
  }

  function typeActionScore(el: Element, tag: string, role: string): number {
    const isFieldLike =
      tag === 'input' ||
      tag === 'textarea' ||
      tag === 'select' ||
      el.getAttribute('contenteditable') === 'true' ||
      role === 'textbox'
    if (isFieldLike) return 100
    if (tag === 'button' || role === 'button') return 10
    return 0
  }

  function elementTypeMatchScore(el: Element, tag: string, role: string, action: string): number {
    const clickLikeActions = new Set(['click', 'key_press', 'focus', 'scroll_to', 'set_attribute', 'paste'])
    const typeLikeActions = new Set(['type', 'select', 'check'])
    if (clickLikeActions.has(action)) return clickActionScore(el, tag, role)
    if (typeLikeActions.has(action)) return typeActionScore(el, tag, role)
    return 0
  }

  function textMatchScore(el: Element, selectorLabel: string): number {
    if (!selectorLabel) return 0
    const trimmedLabel = extractElementLabel(el).trim()
    if (trimmedLabel === selectorLabel) {
      return 80 // exact match
    }
    if (trimmedLabel.startsWith(selectorLabel) && trimmedLabel.length <= selectorLabel.length + 5) {
      return 60 // tight prefix
    }
    return 0
  }

  function primaryButtonScore(el: Element, tag: string, role: string): number {
    if (tag !== 'button' && role !== 'button') return 0
    const htmlEl = el as HTMLElement
    const cls = (typeof htmlEl.className === 'string' ? htmlEl.className : '').toLowerCase()
    const type = el.getAttribute('type') || ''
    if (type === 'submit') return 60
    if (/\bprimary\b|\bbtn-primary\b|\bcta\b/.test(cls)) return 60
    const style = typeof getComputedStyle === 'function' ? getComputedStyle(htmlEl) : null
    if (!style) return 0
    const bg = style.backgroundColor || ''
    // Colored background (not transparent, not white, not gray-ish)
    if (bg && !/transparent|rgba\(0,\s*0,\s*0,\s*0\)|rgb\(255,\s*255,\s*255\)|rgb\(2[45]\d,\s*2[45]\d,\s*2[45]\d\)/.test(bg)) {
      return 30
    }
    return 0
  }

  function rankAmbiguousCandidates(
    candidates: Element[],
    action: string,
    selectorText: string
  ): { winner: Element | null; gap: number; ranked: { element: Element; score: number }[] } {
    const dialogs = collectDialogs()
    const topDialog = dialogs.length > 0 ? pickTopDialog(dialogs) : null
    const selectorLabel = selectorRankingLabel(selectorText)

    const scored = candidates.map((el) => {
      const tag = el.tagName.toLowerCase()
      const role = el.getAttribute('role') || ''
      let score = 0

      // Modal scoping: element inside the top open dialog
      if (topDialog && typeof topDialog.contains === 'function' && topDialog.contains(el)) {
        score += 200
      }

      // Element type match
      score += elementTypeMatchScore(el, tag, role, action)

      // Text matching (only when selector provides text)
      score += textMatchScore(el, selectorLabel)

      // Primary button heuristic
      score += primaryButtonScore(el, tag, role)

      // z-index (0–50)
      score += Math.min(50, Math.max(0, elementZIndexScore(el)))

      // Area (0–30)
      score += areaScore(el, 30)

      return { element: el, score }
    })

    scored.sort((a, b) => b.score - a.score)

    const topScore = scored[0]?.score ?? 0
    const secondScore = scored[1]?.score ?? 0
    const gap = topScore - secondScore
    const winner = gap >= 50 ? (scored[0]?.element ?? null) : null

    return { winner, gap, ranked: scored }
  }
