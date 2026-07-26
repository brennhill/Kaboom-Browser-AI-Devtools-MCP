/**
 * Purpose: Frame-matching probe executed in page context for targeted DOM actions.
 * Why: Must be self-contained for chrome.scripting.executeScript injection (no closures allowed).
 * Docs: docs/features/feature/interact-explore/index.md
 */

/**
 * Must stay self-contained for chrome.scripting.executeScript({ func }).
 */
export function domFrameProbe(frameTarget: string | number): { matches: boolean } {
  const isTop = window === window.top

  const getParentFrameIndex = (): number => {
    if (isTop) return -1
    try {
      const parentFrames = window.parent?.frames
      if (!parentFrames) return -1
      for (let i = 0; i < parentFrames.length; i++) {
        if (parentFrames[i] === window) return i
      }
    } catch {
      return -1
    }
    return -1
  }

  if (typeof frameTarget === 'number') {
    return { matches: getParentFrameIndex() === frameTarget }
  }

  if (frameTarget === 'all') {
    return { matches: true }
  }

  if (isTop) {
    return { matches: false }
  }

  try {
    const frameEl = window.frameElement
    if (!frameEl || typeof frameEl.matches !== 'function') {
      return { matches: false }
    }
    return { matches: frameEl.matches(frameTarget) }
  } catch {
    return { matches: false }
  }
}
