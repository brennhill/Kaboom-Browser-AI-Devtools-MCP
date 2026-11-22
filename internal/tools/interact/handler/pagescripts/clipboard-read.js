/*
 * clipboard-read.js — Bounded, self-classifying MAIN-world clipboard read.
 *
 * Why: `await navigator.clipboard.readText()` can hang forever. Chrome raises a
 * modal permission prompt no agent can answer, and the promise stays pending
 * until the injected executor gives up with a generic `execution_timeout` — a
 * verdict that says nothing about permissions, focus, or navigation, and leaves
 * the prompt on screen to strand every later action. This script decides from
 * the permission state before it ever touches the clipboard API, bounds the
 * granted path, and names the exact outcome.
 *
 * Contract: evaluates to a promise for either `{ text, permission_state }` or
 * `{ error, message, permission_state, ... }`. Must stay a single parenthesized
 * expression — the injected executor compiles it with `return (<script>)`.
 * Failure payloads never carry clipboard contents.
 */
(async () => {
  const DEADLINE_MS = 2000
  const MAX_DETAIL_CHARS = 200

  const permissionState = await (async () => {
    try {
      const status = await navigator.permissions.query({ name: 'clipboard-read' })
      return typeof status?.state === 'string' ? status.state : 'unknown'
    } catch {
      // EXPECTED_ABSENCE: browsers that do not describe clipboard-read through the
      // Permissions API are normal; the read below is still attempted and bounded,
      // so reporting this probe as a failure would be misleading.
      return 'unknown'
    }
  })()

  if (permissionState === 'denied') {
    return {
      error: 'clipboard_permission_denied',
      message: 'Clipboard read is denied for this origin. Grant it in the site permissions, then retry.',
      permission_state: 'denied'
    }
  }
  if (permissionState === 'prompt') {
    return {
      error: 'clipboard_permission_prompt_required',
      message: 'Clipboard read needs a permission the browser can only grant through a user prompt.',
      permission_state: 'prompt'
    }
  }

  const classify = (failure) => {
    const name = typeof failure?.name === 'string' ? failure.name : 'Error'
    const detail = String(failure?.message || '').slice(0, MAX_DETAIL_CHARS)
    if (/destroy|invalidated/i.test(detail)) {
      return {
        error: 'clipboard_read_context_destroyed',
        message: 'The page context was destroyed before the clipboard read completed.',
        permission_state: permissionState
      }
    }
    if (/focus/i.test(detail)) {
      return {
        error: 'clipboard_document_not_focused',
        message: 'The browser refused the clipboard read because the page is not focused.',
        permission_state: permissionState
      }
    }
    if (name === 'NotAllowedError') {
      return {
        error: 'clipboard_permission_denied',
        message: 'The browser refused the clipboard read for this origin.',
        permission_state: permissionState
      }
    }
    return {
      error: 'clipboard_read_failed',
      message: 'The clipboard read failed.',
      permission_state: permissionState,
      reason: name,
      detail
    }
  }

  let settle
  const outcome = new Promise((resolve) => {
    settle = resolve
  })
  const onPageHide = () =>
    settle({
      error: 'clipboard_read_navigation_cancelled',
      message: 'The page navigated away before the clipboard read completed.',
      permission_state: permissionState
    })
  const deadline = setTimeout(
    () =>
      settle({
        error: 'clipboard_read_timeout',
        message: `The clipboard read did not resolve within ${DEADLINE_MS}ms.`,
        permission_state: permissionState
      }),
    DEADLINE_MS
  )

  window.addEventListener('pagehide', onPageHide, { once: true })
  try {
    navigator.clipboard.readText().then(
      (text) => settle({ text, permission_state: permissionState }),
      (failure) => settle(classify(failure))
    )
    return await outcome
  } catch (failure) {
    return classify(failure)
  } finally {
    clearTimeout(deadline)
    window.removeEventListener('pagehide', onPageHide)
  }
})()
