/**
 * Purpose: Applies the shared Kaboom flame mark inside extension popup surfaces.
 * Docs: docs/features/feature/browser-extension-enhancement/index.md
 */

/**
 * Initialize the popup logo.
 * The flame mark stays static; popup hover should not swap the asset.
 */
export function initPopupLogoMotion(): void {
  const logo = document.querySelector('.logo') as HTMLImageElement | null
  if (!logo) return

  logo.src = chrome.runtime.getURL('icons/icon.svg')
}
