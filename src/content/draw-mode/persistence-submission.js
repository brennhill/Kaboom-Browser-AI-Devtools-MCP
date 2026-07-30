// persistence-submission.js — Annotation persistence, cancellation, submission, and result delivery.
/* eslint-disable no-unused-vars, no-undef */
function reportDrawStateRecovery(detail) {
  try {
    void chrome.runtime
      .sendMessage({
        type: 'report_state_recovery',
        lifecycle: 'active',
        diagnostic: {
          name: 'annotation_state',
          detail,
          fix: 'Start annotation mode again to create fresh annotation state.'
        }
      })
      .catch(() => {
        console.warn('[KaBOOM!] annotation recovery diagnostic was not delivered')
      })
  } catch {
    // The draw UI still falls back safely when the extension context is gone.
  }
}

function resolveDrawStateRecovery() {
  try {
    void chrome.runtime
      .sendMessage({
        type: 'report_state_recovery',
        lifecycle: 'recovered',
        diagnostic: { name: 'annotation_state', detail: '', fix: '' }
      })
      .catch(() => {
        console.warn('[KaBOOM!] annotation recovery resolution was not delivered')
      })
  } catch {
    // A later page load will verify state again when this context is unavailable.
  }
}

function persistAnnotations() {
  if (saveTimeout) clearTimeout(saveTimeout)
  saveTimeout = setTimeout(() => {
    if (!storageAvailable) return
    try {
      const key = 'gasoline_draw_annotations'
      const toStore =
        annotations.length > MAX_PERSISTED_ANNOTATIONS ? annotations.slice(-MAX_PERSISTED_ANNOTATIONS) : annotations
      chrome.storage.session.set(
        {
          [key]: {
            annotations: toStore,
            page_url: window.location.href,
            timestamp: Date.now()
          }
        },
        () => {
          if (chrome.runtime?.lastError) {
            storageAvailable = false
            reportDrawStateRecovery('Annotation state could not be saved; the current canvas remains active.')
          }
        }
      )
    } catch {
      storageAvailable = false
    }
  }, 500) // Debounce 500ms
}

function clearPersistedAnnotations() {
  if (!storageAvailable) return
  try {
    chrome.storage.session.remove('gasoline_draw_annotations', () => {
      if (chrome.runtime?.lastError) {
        storageAvailable = false
        reportDrawStateRecovery('Saved annotation state could not be cleared; the current canvas was still closed.')
      }
    })
  } catch {
    storageAvailable = false
  }
}

function loadAnnotations() {
  if (!storageAvailable) return
  try {
    const key = 'gasoline_draw_annotations'
    chrome.storage.session.get([key], (result) => {
      if (chrome.runtime?.lastError) {
        storageAvailable = false
        reportDrawStateRecovery('Saved annotation state could not be read; an empty canvas is active.')
        return
      }
      const data = result?.[key]
      const valid =
        data === undefined ||
        (typeof data === 'object' &&
          data !== null &&
          Array.isArray(data.annotations) &&
          typeof data.page_url === 'string' &&
          (data.timestamp === undefined || typeof data.timestamp === 'number'))
      if (!valid) {
        reportDrawStateRecovery('Saved annotation state was malformed; an empty canvas is active.')
        return
      }
      resolveDrawStateRecovery()
      if (data?.annotations && data.page_url === window.location.href) {
        annotations = data.annotations.map(normalizeLoadedAnnotation)
        renderAnnotations()
        updateActionBar()
      }
    })
  } catch {
    storageAvailable = false
  }
}

// ============================================================================
// DEACTIVATION + RESULT DELIVERY
// ============================================================================

/**
 * Called when the user submits with Enter, from popup toggle, or from
 * KABOOM_DRAW_MODE_STOP message.
 * Captures screenshot WHILE overlay is still visible, then deactivates and sends results.
 * Protected by a re-entry guard to prevent repeated-submit races.
 */
export function deactivateAndSendResults() {
  if (!active || isDeactivating) return
  isDeactivating = true

  // Shortcut/popup stop while an editor is open should behave like submit, not cancel.
  if (textInput) {
    if (!submitActiveTextInputBeforeExit()) {
      isDeactivating = false
      return {
        status: 'validation_error',
        message: 'Annotation text is required before exiting draw mode.'
      }
    }
  }

  const pageUrl = window.location.href
  const currentSessionName = sessionName // capture before deactivate clears it
  const currentCorrelationId = sessionCorrelationId // capture before deactivate clears it

  /**
   * Complete the deactivation: fade out overlay, show toast, tear down,
   * send results to background, and clear persisted storage.
   */
  const finishDeactivation = (screenshotDataUrl) => {
    // Fade out the overlay before tearing it down
    if (overlay) {
      overlay.style.opacity = '0'
    }

    // Show success toast via extension messaging
    try {
      if (typeof chrome !== 'undefined' && chrome.runtime) {
        chrome.runtime.sendMessage({
          type: 'kaboom_action_toast',
          text: 'Annotations submitted',
          state: 'success',
          duration_ms: 2000
        })
      }
    } catch {
      // Extension context may be invalidated
    }

    // Delay teardown to let fade complete
    setTimeout(() => {
      isDeactivating = false
      const result = deactivateDrawMode()
      // Clear persisted annotations from storage after successful deactivation
      clearPersistedAnnotations()
      try {
        if (typeof chrome !== 'undefined' && chrome.runtime) {
          const msg = {
            type: 'draw_mode_completed',
            annotations: result.annotations,
            elementDetails: result.elementDetails,
            page_url: pageUrl,
            screenshot_data_url: screenshotDataUrl,
            correlation_id: currentCorrelationId
          }
          if (currentSessionName) {
            msg.annot_session_name = currentSessionName
          }
          chrome.runtime.sendMessage(msg)
        }
      } catch {
        // Extension context may be invalidated
      }

      // Dispatch CustomEvent so content-script peers (e.g. terminal launcher)
      // can auto-send annotation summaries without round-tripping through background.
      // Stamp it with the per-session channel token the launcher published to
      // extension-only storage (StorageKey.ANNOTATION_CHANNEL_NONCE) so the
      // receiver can tell an extension-origin event from a page-forged one — the
      // `window` event target is shared with the page.
      const emitAnnotationsReady = (nonce) => {
        try {
          window.dispatchEvent(
            new CustomEvent('kaboom-annotations-ready', {
              detail: {
                annotations: result.annotations,
                page_url: pageUrl,
                nonce: nonce || ''
              }
            })
          )
        } catch {
          // CustomEvent dispatch failed — non-critical
        }
      }
      try {
        // Key literal must match StorageKey.ANNOTATION_CHANNEL_NONCE in lib/constants.ts.
        if (typeof chrome !== 'undefined' && chrome.storage && chrome.storage.local) {
          chrome.storage.local.get('kaboom_annotation_channel_nonce', (res) => {
            if (chrome.runtime?.lastError) {
              reportDrawStateRecovery('Saved annotation channel identity could not be read; page notification was suppressed.')
              emitAnnotationsReady('')
              return
            }
            const nonce = res && res['kaboom_annotation_channel_nonce']
            if (nonce !== undefined && typeof nonce !== 'string') {
              reportDrawStateRecovery('Saved annotation channel identity was malformed; page notification was suppressed.')
              emitAnnotationsReady('')
              return
            }
            emitAnnotationsReady(nonce)
          })
        } else {
          emitAnnotationsReady('')
        }
      } catch {
        // storage unavailable — dispatch without a token (receiver will ignore it)
        emitAnnotationsReady('')
      }
    }, 300)
  }

  // Re-capture DOM data for all annotations so element details match the screenshot moment.
  // Annotations may have been drawn minutes ago; the DOM may have changed since then.
  refreshElementDetails()

  // Request screenshot capture from background BEFORE deactivating,
  // so the overlay with annotation drawings is included in the screenshot.
  if (typeof chrome !== 'undefined' && chrome.runtime) {
    let screenshotHandled = false
    // Timeout fallback: if screenshot callback never fires (extension context
    // invalidated, background unresponsive), proceed without screenshot after 1s.
    const fallbackTimer = setTimeout(() => {
      if (!screenshotHandled) {
        screenshotHandled = true
        finishDeactivation('')
      }
    }, 1000)

    try {
      chrome.runtime.sendMessage({ type: 'kaboom_capture_screenshot' }, (screenshotResponse) => {
        if (screenshotHandled) return // Timeout already fired
        screenshotHandled = true
        clearTimeout(fallbackTimer)
        finishDeactivation(screenshotResponse?.dataUrl || '')
      })
    } catch {
      // Fallback: deactivate without screenshot
      if (!screenshotHandled) {
        screenshotHandled = true
        clearTimeout(fallbackTimer)
        finishDeactivation('')
      }
    }
  } else {
    deactivateDrawMode()
    isDeactivating = false
  }
}

/**
 * Exit draw mode without delivering annotations.
 * Escape always follows this path, including while the annotation editor is open.
 */
function cancelDrawMode() {
  if (!active) return
  isDeactivating = false
  deactivateDrawMode()
  clearPersistedAnnotations()
  try {
    if (typeof chrome !== 'undefined' && chrome.runtime) {
      chrome.runtime.sendMessage({
        type: 'kaboom_action_toast',
        text: 'Annotations cancelled',
        state: 'info',
        duration_ms: 1600
      })
    }
  } catch {
    // Extension context may be invalidated
  }
}

function submitActiveTextInputBeforeExit() {
  if (!textInput) return true
  const text = textInput.value.trim()
  if (!text) {
    try {
      if (typeof chrome !== 'undefined' && chrome.runtime) {
        chrome.runtime.sendMessage({
          type: 'kaboom_action_toast',
          text: 'Annotation text required',
          detail: 'Type feedback, then press the shortcut again to submit.',
          state: 'error',
          duration_ms: 2500
        })
      }
    } catch {
      // Extension context may be invalidated
    }
    textInput.focus()
    return false
  }

  // Reuse Enter-submit path so payload matches explicit submit behavior.
  confirmTextInput()
  return true
}

// ============================================================================
// UTILITY
// ============================================================================
