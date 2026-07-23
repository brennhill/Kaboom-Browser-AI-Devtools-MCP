/**
 * @fileoverview Draw Mode Export — Annotated screenshot compositing.
 * Composites annotation rectangles + text + number badges onto a page screenshot.
 * Called when draw mode exits to produce a single annotated PNG.
 */

const ANNOTATION_COLOR = '#ef4444'
const BADGE_RADIUS = 12

/**
 * Composite annotations onto a page screenshot.
 * @param {string} screenshotDataUrl - Base64 data URL from chrome.tabs.captureVisibleTab
 * @param {Array} annotations - Array of annotation objects with rect, text, id
 * @returns {Promise<string>} Annotated screenshot as data URL (PNG)
 */
export async function compositeAnnotations(screenshotDataUrl, annotations) {
  if (!annotations || annotations.length === 0) {
    return screenshotDataUrl // Nothing to composite
  }

  // Load the screenshot image
  const img = await loadImage(screenshotDataUrl)

  // Create offscreen canvas matching screenshot dimensions
  const canvas = document.createElement('canvas')
  canvas.width = img.width
  canvas.height = img.height
  const ctx = canvas.getContext('2d')
  if (!ctx) {
    return screenshotDataUrl
  }

  // Calculate scale factor (screenshot may be higher resolution than viewport)
  const scaleX = img.width / window.innerWidth
  const scaleY = img.height / window.innerHeight

  // Draw original screenshot
  ctx.drawImage(img, 0, 0)

  // Draw each annotation
  for (let i = 0; i < annotations.length; i++) {
    const ann = annotations[i]
    const r = ann.rect

    // Scale rect coordinates to screenshot resolution
    const sx = r.x * scaleX
    const sy = r.y * scaleY
    const sw = r.width * scaleX
    const sh = r.height * scaleY

    // Semi-transparent red fill
    ctx.fillStyle = 'rgba(239, 68, 68, 0.15)'
    ctx.fillRect(sx, sy, sw, sh)

    // Red stroke
    ctx.strokeStyle = ANNOTATION_COLOR
    ctx.lineWidth = 2 * Math.max(scaleX, scaleY)
    ctx.setLineDash([])
    ctx.strokeRect(sx, sy, sw, sh)

    // Number badge (top-left corner of rect)
    const badgeR = BADGE_RADIUS * Math.max(scaleX, scaleY)
    ctx.fillStyle = ANNOTATION_COLOR
    ctx.beginPath()
    ctx.arc(sx, sy, badgeR, 0, Math.PI * 2)
    ctx.fill()

    ctx.fillStyle = '#ffffff'
    ctx.font = `bold ${Math.round(12 * Math.max(scaleX, scaleY))}px -apple-system, sans-serif`
    ctx.textAlign = 'center'
    ctx.textBaseline = 'middle'
    ctx.fillText(String(i + 1), sx, sy)

    // Text label below rectangle
    if (ann.text) {
      const fontSize = Math.round(13 * Math.max(scaleX, scaleY))
      ctx.font = `${fontSize}px -apple-system, sans-serif`
      const labelY = sy + sh + fontSize + 4
      const textWidth = ctx.measureText(ann.text).width
      const padding = 6 * Math.max(scaleX, scaleY)

      // Background pill
      ctx.fillStyle = 'rgba(0, 0, 0, 0.8)'
      ctx.fillRect(sx - padding, labelY - fontSize, textWidth + padding * 2, fontSize + padding)

      // Text
      ctx.fillStyle = '#ffffff'
      ctx.textAlign = 'left'
      ctx.textBaseline = 'middle'
      ctx.fillText(ann.text, sx, labelY - fontSize / 2 + padding / 2)
    }
  }

  return canvas.toDataURL('image/png')
}

/**
 * Load an image from a data URL.
 * @param {string} dataUrl
 * @returns {Promise<HTMLImageElement>}
 */
function loadImage(dataUrl) {
  return new Promise((resolve, reject) => {
    const img = new Image()
    img.onload = () => resolve(img)
    img.onerror = (e) => reject(new Error('Failed to load screenshot image'))
    img.src = dataUrl
  })
}
