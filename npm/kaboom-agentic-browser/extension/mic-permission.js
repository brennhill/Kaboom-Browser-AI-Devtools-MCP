// mic-permission.js — Requests microphone access from a full extension page.
// Chrome MV3 CSP blocks inline scripts, so this must be a separate file.
// After granting, tells the background to close this tab and show a toast
// guiding the user to click the Kaboom icon (which auto-starts recording).

const btn = document.getElementById('grant-btn')
const statusEl = document.getElementById('status')

/** After mic is granted, tell background to close this tab and show guidance toast. */
function onPermissionGranted() {
  chrome.storage.local.set({ gasoline_mic_granted: true })
  statusEl.className = 'success'
  statusEl.textContent = 'Granted! Closing...'
  btn.textContent = 'Granted'
  btn.disabled = true
  chrome.runtime.sendMessage({ type: 'mic_granted_close_tab' })
}

// Check current permission state on load
async function checkPermission() {
  try {
    const result = await navigator.permissions.query({ name: 'microphone' })
    if (result.state === 'granted') {
      onPermissionGranted()
    } else if (result.state === 'denied') {
      statusEl.className = 'error'
      statusEl.innerHTML =
        'Microphone is blocked for this extension.<br><br>' +
        'To fix: click the lock icon in the address bar above, or go to<br>' +
        '<code style="background:#333;padding:2px 6px;border-radius:3px">chrome://settings/content/microphone</code><br>' +
        'and remove the block for this extension.'
    }
  } catch {
    // permissions.query not supported for microphone in this browser
  }
}

checkPermission()

btn.addEventListener('click', async () => {
  btn.disabled = true
  btn.textContent = 'Requesting...'
  statusEl.textContent = ''
  statusEl.className = ''

  try {
    const stream = await navigator.mediaDevices.getUserMedia({ audio: true })
    stream.getTracks().forEach((t) => t.stop())
    onPermissionGranted()
  } catch (err) {
    statusEl.className = 'error'
    if (err.name === 'NotAllowedError') {
      statusEl.innerHTML =
        'Permission was not granted.<br><br>' +
        'If no dialog appeared, Chrome may have auto-blocked it. To fix:<br>' +
        '1. Click the <strong>lock icon</strong> in the address bar above<br>' +
        '2. Set <strong>Microphone</strong> to <strong>Allow</strong><br>' +
        '3. Click the button again'
    } else if (err.name === 'NotFoundError') {
      statusEl.textContent = 'No microphone found. Please connect a microphone and try again.'
    } else {
      statusEl.textContent = 'Error: ' + err.message
    }
    btn.textContent = 'Try Again'
    btn.disabled = false
  }
})
