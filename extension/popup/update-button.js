/**
 * Purpose: Popup "Update now" button — wires self-update endpoint + reload-extension prompt.
 * Why: Lets users one-click upgrade the daemon from inside the extension.
 * Docs: docs/features/feature/self-update/index.md
 */
import { DEFAULT_SERVER_URL, StorageKey } from '../lib/constants.js';
import { buildDaemonHeaders, postDaemonJSON } from '../lib/daemon-http.js';
import { getLocal } from '../lib/storage-utils.js';
// Poll /health every 2s for up to 120s after kicking off the install. A
// realistic self-update (download + checksum + binary swap + daemon respawn)
// can take 45-90s on slow connections (hotel Wi-Fi, mobile tether), so the
// 30s window this replaced routinely timed out on real users.
const VERSION_POLL_INTERVAL_MS = 2000;
const VERSION_POLL_TIMEOUT_MS = 120000;
async function getServerUrl() {
    const value = await getLocal(StorageKey.SERVER_URL);
    return value || DEFAULT_SERVER_URL;
}
async function fetchHealth(serverUrl) {
    const resp = await fetch(`${serverUrl}/health`, {
        headers: buildDaemonHeaders()
    });
    if (!resp.ok) {
        throw new Error(`health HTTP ${resp.status}`);
    }
    return (await resp.json());
}
async function fetchNonce(serverUrl) {
    const resp = await fetch(`${serverUrl}/upgrade/nonce`, {
        headers: buildDaemonHeaders()
    });
    if (!resp.ok) {
        throw new Error(`nonce HTTP ${resp.status}`);
    }
    const body = (await resp.json());
    if (!body.nonce) {
        throw new Error('nonce missing from response');
    }
    return body.nonce;
}
async function postInstall(serverUrl, nonce) {
    const resp = await postDaemonJSON(`${serverUrl}/upgrade/install`, { nonce });
    if (resp.status === 501) {
        throw new Error('Self-update is not supported on this platform — re-run the installer manually.');
    }
    if (resp.status === 429) {
        throw new Error('Update was requested recently. Wait a minute and try again.');
    }
    if (!resp.ok) {
        throw new Error(`install HTTP ${resp.status}`);
    }
}
// Update the running-state paragraph with elapsed seconds so users see the
// flow is alive during the (potentially long) upgrade window. Uses a real
// newline; CSS `white-space: pre-line` in popup.html renders it as two lines.
function setRunningText(seconds) {
    const running = document.getElementById('update-action-running');
    if (running) {
        running.textContent = `Updating… (${seconds}s)\nThe daemon will restart automatically.`;
    }
}
function openExtensionsPage() {
    const id = chrome?.runtime?.id;
    const url = id ? `chrome://extensions/?id=${id}` : 'chrome://extensions';
    chrome.tabs.create({ url });
}
// Install log path — mirrors scripts/install.sh's KABOOM_SELF_UPDATE log sink.
// Hard-coded here because the popup has no clean way to ask the daemon for its
// home dir and "~" is universally understood by macOS/Linux users.
const INSTALL_LOG_PATH = '~/.kaboom/logs/install.log';
function showState(mode, errorMessage) {
    const idle = document.getElementById('update-action-idle');
    const running = document.getElementById('update-action-running');
    const reload = document.getElementById('update-action-reload');
    const errorEl = document.getElementById('update-action-error');
    const errorTextEl = document.getElementById('update-action-error-text');
    if (idle)
        idle.style.display = mode === 'idle' ? '' : 'none';
    if (running)
        running.style.display = mode === 'running' ? '' : 'none';
    if (reload)
        reload.style.display = mode === 'reload' ? '' : 'none';
    if (errorEl) {
        errorEl.style.display = mode === 'error' ? '' : 'none';
    }
    if (errorTextEl) {
        errorTextEl.textContent = mode === 'error' && errorMessage ? errorMessage : '';
    }
}
// Latest UpdateInfo used to launch an upgrade. Held at module scope so the
// retry button can re-run the same flow without needing to re-render the
// banner or re-derive versions from /health.
let lastUpgradeInfo = null;
async function runUpgradeFlow(info) {
    lastUpgradeInfo = info;
    showState('running');
    const startTime = Date.now();
    setRunningText(0);
    try {
        const nonce = await fetchNonce(info.serverUrl);
        await postInstall(info.serverUrl, nonce);
        // Poll /health until the daemon reports the target version or we hit the
        // timeout. Loop is inline (not extracted) so each tick can update the
        // progress text directly without plumbing state through a helper.
        const deadline = startTime + VERSION_POLL_TIMEOUT_MS;
        let observed = null;
        while (Date.now() < deadline) {
            const elapsedSeconds = Math.floor((Date.now() - startTime) / 1000);
            setRunningText(elapsedSeconds);
            await new Promise((resolve) => setTimeout(resolve, VERSION_POLL_INTERVAL_MS));
            try {
                const health = await fetchHealth(info.serverUrl);
                if (health.version && health.version === info.availableVersion) {
                    observed = health.version;
                    break;
                }
            }
            catch {
                // Daemon is restarting — expected during the upgrade window.
            }
        }
        if (observed) {
            showState('reload');
        }
        else {
            showState('error', 'Daemon did not restart in time. Check the terminal or rerun the installer manually.');
        }
    }
    catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        showState('error', msg);
    }
}
/**
 * Render the update-available banner based on latest health. No-op if no
 * upgrade is offered by the daemon.
 */
export async function renderUpdateAvailableBanner(health) {
    const container = document.getElementById('update-available');
    const detail = document.getElementById('update-available-detail');
    if (!container)
        return;
    const current = health.version ?? '';
    const next = health.available_version ?? '';
    const isNewer = next && current && next !== current;
    if (!isNewer) {
        container.style.display = 'none';
        return;
    }
    const serverUrl = await getServerUrl();
    if (detail) {
        detail.textContent = `v${current} installed; v${next} available.`;
    }
    container.style.display = '';
    showState('idle');
    const btn = document.getElementById('update-now-btn');
    if (btn && !btn.dataset.wired) {
        btn.dataset.wired = '1';
        btn.addEventListener('click', () => {
            void runUpgradeFlow({ currentVersion: current, availableVersion: next, serverUrl });
        });
    }
    const reloadBtn = document.getElementById('update-reload-ext-btn');
    if (reloadBtn && !reloadBtn.dataset.wired) {
        reloadBtn.dataset.wired = '1';
        reloadBtn.addEventListener('click', openExtensionsPage);
    }
    // Retry re-runs the same upgrade flow with the UpdateInfo that was originally
    // captured. Falls back to the render-time info if nothing has run yet (e.g.
    // user-triggered retry before any prior click — unlikely but harmless).
    const retryBtn = document.getElementById('update-retry-btn');
    if (retryBtn && !retryBtn.dataset.wired) {
        retryBtn.dataset.wired = '1';
        retryBtn.addEventListener('click', () => {
            const info = lastUpgradeInfo ?? { currentVersion: current, availableVersion: next, serverUrl };
            void runUpgradeFlow(info);
        });
    }
    // Copy log path writes the (hard-coded) install log path to the clipboard so
    // non-technical users can paste it into a support request.
    const copyLogBtn = document.getElementById('update-copy-log-btn');
    if (copyLogBtn && !copyLogBtn.dataset.wired) {
        copyLogBtn.dataset.wired = '1';
        copyLogBtn.addEventListener('click', () => {
            const originalText = copyLogBtn.textContent ?? 'Copy log path';
            void (async () => {
                try {
                    await navigator.clipboard.writeText(INSTALL_LOG_PATH);
                    copyLogBtn.textContent = 'Copied!';
                }
                catch {
                    copyLogBtn.textContent = 'Copy failed';
                }
                setTimeout(() => {
                    copyLogBtn.textContent = originalText;
                }, 2000);
            })();
        });
    }
}
//# sourceMappingURL=update-button.js.map