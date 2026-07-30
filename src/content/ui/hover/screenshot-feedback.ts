/**
 * Purpose: Presentational feedback for a screenshot capture — the shutter sound and
 * the full-viewport flash.
 * Why: Both are self-contained, own their only module state (a primed AudioContext),
 * and are unrelated to the launcher's hover/panel logic. Extracted so
 * tracked-hover-launcher.ts stays within the 800-line limit.
 * Docs: docs/features/feature/terminal/index.md
 */

// Primed AudioContext — created during user gesture so it won't be blocked.
// Reused across captures; closed lazily by the browser when the page unloads.
let shutterAudioCtx: AudioContext | null = null

/**
 * Create the AudioContext while a user gesture is still on the stack.
 *
 * Chrome blocks audio created outside a gesture, and the shutter plays later (after
 * the capture round-trip), by which time the gesture is gone — so the context must
 * be primed at click time or the sound is silently dropped.
 */
export function primeShutterAudio(): void {
  if (!shutterAudioCtx || shutterAudioCtx.state === 'closed') {
    try {
      shutterAudioCtx = new AudioContext()
    } catch {
      // EXPECTED_ABSENCE: UI recipients can normally disappear during navigation
      // or teardown; logging it would misleadingly report a normal lifecycle race as failure.
      // No audio available — playShutterSound degrades to silence.
    }
  }
}

export function playShutterSound(): void {
  try {
    if (!shutterAudioCtx || shutterAudioCtx.state === 'closed') {
      shutterAudioCtx = new AudioContext()
    }
    const ctx = shutterAudioCtx
    // Resume in case the context was suspended (autoplay policy)
    if (ctx.state === 'suspended') void ctx.resume()
    const duration = 0.08
    const buffer = ctx.createBuffer(1, Math.ceil(ctx.sampleRate * duration), ctx.sampleRate)
    const data = buffer.getChannelData(0)
    for (let i = 0; i < data.length; i++) {
      const t = i / data.length
      const envelope = t < 0.1 ? t * 10 : Math.exp(-12 * (t - 0.1))
      data[i] = (Math.random() * 2 - 1) * envelope * 0.3
    }
    const source = ctx.createBufferSource()
    source.buffer = buffer
    source.connect(ctx.destination)
    source.start()
  } catch {
    // EXPECTED_ABSENCE: UI recipients can normally disappear during navigation
    // or teardown; logging it would misleadingly report a normal lifecycle race as failure.
    // Audio unavailable — silent fallback
  }
}

export function showScreenshotFlash(success: boolean): void {
  const flash = document.createElement('div')
  Object.assign(flash.style, {
    position: 'fixed',
    inset: '0',
    zIndex: '2147483647',
    background: success ? 'rgba(250,204,21,0.3)' : 'rgba(239,68,68,0.25)',
    pointerEvents: 'none',
    opacity: '1'
  })
  document.documentElement.appendChild(flash)
  // Hold the flash visible for 120ms before fading out
  setTimeout(() => {
    flash.style.transition = 'opacity 300ms ease-out'
    flash.style.opacity = '0'
  }, 120)
  setTimeout(() => flash.remove(), 450)
}
