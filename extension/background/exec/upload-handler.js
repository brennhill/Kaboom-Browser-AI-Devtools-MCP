/**
 * Purpose: Handles file upload queries by fetching file data from the Go server and injecting it into DOM file inputs via DataTransfer or OS automation escalation.
 * Docs: docs/features/feature/interact-explore/index.md
 */
import { delay, fetchWithTimeout } from '../../lib/timeout-utils.js';
import { debugLog } from '../index.js';
import { getServerUrl } from '../runtime-state/settings-state.js';
import { DebugCategory } from '../debug.js';
import { errorMessage } from '../../lib/error-utils.js';
import { buildDaemonHeaders, buildDaemonJSONRequestInit } from '../../lib/daemon-http.js';
// ============================================
// Timing Constants
// ============================================
/** Wait for native file dialog to open after el.click() */
const DIALOG_OPEN_DELAY_MS = 1500;
/** Wait for dialog to close and Chrome to process file after OS automation */
const DIALOG_CLOSE_DELAY_MS = 2000;
/** Timeout for daemon fetch calls */
const DAEMON_FETCH_TIMEOUT_MS = 15000;
/** Backoff schedule for file verification — sleep BEFORE each check (~4.6s total window) */
const VERIFY_BACKOFF_MS = [300, 500, 800, 1200, 1800];
// ============================================
// Injected Functions (run in MAIN world)
// ============================================
/**
 * Self-contained function injected into the page via chrome.scripting.executeScript.
 * Sets a File on an <input type="file"> element using DataTransfer.
 * MUST NOT reference any module-level variables.
 */
function injectFileIntoInput(selector, dataBase64, fileName, mimeType) {
    const el = document.querySelector(selector);
    if (!el) {
        return { success: false, error: `element_not_found: ${selector}` };
    }
    if (!(el instanceof HTMLInputElement) || el.type !== 'file') {
        return {
            success: false,
            error: `not_file_input: <${el.tagName.toLowerCase()} type="${el.type || 'N/A'}">`
        };
    }
    try {
        const raw = atob(dataBase64);
        const bytes = new Uint8Array(raw.length);
        for (let i = 0; i < raw.length; i++) {
            bytes[i] = raw.charCodeAt(i);
        }
        const blob = new Blob([bytes], { type: mimeType });
        const file = new File([blob], fileName, { type: mimeType });
        const dt = new DataTransfer();
        dt.items.add(file);
        el.files = dt.files;
        el.dispatchEvent(new Event('change', { bubbles: true }));
        el.dispatchEvent(new Event('input', { bubbles: true }));
        return { success: true, file_name: fileName, file_size: file.size };
    }
    catch (err) {
        return { success: false, error: `inject_failed: ${errorMessage(err)}` };
    }
}
/**
 * Injected into MAIN world to check if a file is present on the input element.
 * MUST NOT reference any module-level variables.
 */
function checkFileOnInput(selector) {
    const el = document.querySelector(selector);
    if (!el || !(el instanceof HTMLInputElement)) {
        return { has_file: false };
    }
    if (el.files && el.files.length > 0) {
        return { has_file: true, file_name: el.files[0]?.name, file_size: el.files[0]?.size };
    }
    return { has_file: false };
}
/**
 * Injected into MAIN world to click a file input element to open the native file dialog.
 * MUST NOT reference any module-level variables.
 */
function clickFileInputElement(selector) {
    const el = document.querySelector(selector);
    if (!el) {
        return { clicked: false, error: `element_not_found: ${selector}` };
    }
    if (!(el instanceof HTMLInputElement) || el.type !== 'file') {
        return { clicked: false, error: 'not_file_input' };
    }
    try {
        el.click();
        return { clicked: true };
    }
    catch (err) {
        return { clicked: false, error: `click_failed: ${errorMessage(err)}` };
    }
}
// ============================================
// Exported Verification & Escalation Functions
// ============================================
/**
 * Verify whether a file is present on the input element (single check).
 */
async function verifyFileOnInputOnce(tabId, selector) {
    const results = await chrome.scripting.executeScript({
        target: { tabId, allFrames: true },
        world: 'MAIN',
        func: checkFileOnInput,
        args: [selector]
    });
    // Pick first frame that has a file
    for (const r of results) {
        const res = r.result;
        if (res?.has_file)
            return res;
    }
    return results[0]?.result ?? { has_file: false };
}
/**
 * Verify whether a file persists on the input element after Stage 1 injection.
 * Sleeps BEFORE each check so frameworks with async onChange have time to clear.
 * If the file disappears at any check, returns has_file: false immediately.
 * If it survives all checks (~4.6s window), Stage 1 is confirmed.
 */
export async function verifyFileOnInput(tabId, selector) {
    for (const delayMs of VERIFY_BACKOFF_MS) {
        await delay(delayMs);
        const result = await verifyFileOnInputOnce(tabId, selector);
        if (!result.has_file)
            return { has_file: false };
    }
    // File persisted through all checks — confirmed
    const final = await verifyFileOnInputOnce(tabId, selector);
    return final;
}
/**
 * Click a file input element to open the native file dialog.
 */
export async function clickFileInput(tabId, selector) {
    const results = await chrome.scripting.executeScript({
        target: { tabId, allFrames: true },
        world: 'MAIN',
        func: clickFileInputElement,
        args: [selector]
    });
    // Pick first frame that clicked successfully
    for (const r of results) {
        const res = r.result;
        if (res?.clicked)
            return res;
    }
    return results[0]?.result ?? { clicked: false, error: 'no_result' };
}
/** Module-level mutex to prevent concurrent Stage 4 escalations */
let escalationInProgress = false;
/**
 * Attempt to dismiss a dangling file dialog by sending Escape via OS automation.
 * Best-effort — errors are logged but not propagated.
 */
async function dismissFileDialog(serverUrl) {
    try {
        const response = await fetchWithTimeout(`${serverUrl}/api/os-automation/dismiss`, { method: 'POST', headers: buildDaemonHeaders() }, 5000);
        if (!response.ok) {
            return;
        }
    }
    catch {
        // Best-effort cleanup — ignore errors
    }
}
/**
 * Escalate to Stage 4 OS automation: click file input, call daemon, verify result.
 */
export async function escalateToStage4(tabId, selector, filePath, serverUrl) {
    // Prevent concurrent escalations
    if (escalationInProgress) {
        return {
            success: false,
            stage: 4,
            error: 'Escalation already in progress. Wait for the current upload to complete.'
        };
    }
    escalationInProgress = true;
    try {
        return await escalateToStage4Internal(tabId, selector, filePath, serverUrl);
    }
    finally {
        escalationInProgress = false;
    }
}
async function escalateToStage4Internal(tabId, selector, filePath, serverUrl) {
    // Step 1: Click file input to open native dialog
    const clickResult = await clickFileInput(tabId, selector);
    if (!clickResult.clicked) {
        return {
            success: false,
            stage: 4,
            error: `Escalation failed: could not click file input '${selector}'. Verify the element exists, is visible, and is type='file'.`
        };
    }
    // Step 2: Wait for native file dialog to open
    await delay(DIALOG_OPEN_DELAY_MS);
    // Step 3: Call daemon for OS automation with browser_pid: 0 (auto-detect)
    let daemonResponse;
    try {
        const response = await fetchWithTimeout(`${serverUrl}/api/os-automation/inject`, buildDaemonJSONRequestInit({ file_path: filePath, browser_pid: 0 }), DAEMON_FETCH_TIMEOUT_MS);
        if (!response.ok) {
            let errorMsg = `HTTP ${response.status}`;
            try {
                const body = (await response.json());
                errorMsg = body.error || errorMsg;
            }
            catch {
                /* non-JSON body */
            }
            if (response.status === 403) {
                await dismissFileDialog(serverUrl);
                return {
                    success: false,
                    stage: 4,
                    error: `Escalation failed: OS automation disabled on daemon. Restart with: kaboom-agentic-browser --daemon --enable-os-upload-automation --upload-dir=/path/to/uploads. Detail: ${errorMsg}`
                };
            }
            await dismissFileDialog(serverUrl);
            return {
                success: false,
                stage: 4,
                error: `Stage 4 OS automation failed: ${errorMsg}`
            };
        }
        daemonResponse = (await response.json());
        if (!daemonResponse.success) {
            const errorMsg = daemonResponse.error || 'unknown daemon error';
            await dismissFileDialog(serverUrl);
            return {
                success: false,
                stage: 4,
                error: `Stage 4 OS automation failed: ${errorMsg}`
            };
        }
    }
    catch (err) {
        const msg = err.name === 'AbortError'
            ? `Escalation timed out after ${DAEMON_FETCH_TIMEOUT_MS}ms waiting for daemon at ${serverUrl}/api/os-automation/inject`
            : `Escalation failed: cannot reach daemon at ${serverUrl}/api/os-automation/inject. Error: ${errorMessage(err)}`;
        await dismissFileDialog(serverUrl);
        return {
            success: false,
            stage: 4,
            error: msg
        };
    }
    // Step 4: Wait for dialog to close and file to appear
    await delay(DIALOG_CLOSE_DELAY_MS);
    // Step 5: Verify file is on input (polls up to VERIFY_MAX_ATTEMPTS times)
    const verifyResult = await verifyFileOnInput(tabId, selector);
    if (!verifyResult.has_file) {
        await dismissFileDialog(serverUrl);
        return {
            success: false,
            stage: 4,
            escalation_reason: 'stage1_file_cleared',
            error: `Stage 4 completed but file not found on input '${selector}'. The native file dialog may not have been in focus. Verify file exists: ${filePath}`
        };
    }
    return {
        success: true,
        stage: 4,
        escalation_reason: 'stage1_file_cleared',
        file_name: verifyResult.file_name
    };
}
// ============================================
// Main Upload Handler
// ============================================
export async function executeUpload(query, tabId, syncClient, sendAsyncResult, actionToast) {
    const correlationId = query.correlation_id;
    let params;
    try {
        params =
            typeof query.params === 'string'
                ? JSON.parse(query.params)
                : query.params;
    }
    catch {
        sendAsyncResult(syncClient, query.id, correlationId, 'error', null, 'invalid_params');
        return;
    }
    const { selector, file_path, file_name, mime_type } = params;
    if (!selector || !file_path) {
        sendAsyncResult(syncClient, query.id, correlationId, 'error', null, 'missing_selector_or_file_path');
        return;
    }
    actionToast(tabId, 'upload', file_name || 'file', 'trying', 10000);
    // Stage 1: Fetch file data from Go server
    let fileData;
    try {
        const response = await fetchWithTimeout(`${getServerUrl()}/api/file/read`, buildDaemonJSONRequestInit({ file_path }), DAEMON_FETCH_TIMEOUT_MS);
        if (!response.ok) {
            sendAsyncResult(syncClient, query.id, correlationId, 'error', null, `file_read_failed: HTTP ${response.status}`);
            actionToast(tabId, 'upload', `HTTP ${response.status}`, 'error');
            return;
        }
        fileData = (await response.json());
    }
    catch (err) {
        const msg = err.name === 'AbortError'
            ? `file_read_timeout: daemon did not respond within ${DAEMON_FETCH_TIMEOUT_MS}ms`
            : `file_read_failed: ${errorMessage(err)}`;
        sendAsyncResult(syncClient, query.id, correlationId, 'error', null, msg);
        actionToast(tabId, 'upload', 'fetch failed', 'error');
        return;
    }
    if (!fileData.success || !fileData.data_base64) {
        sendAsyncResult(syncClient, query.id, correlationId, 'error', null, `file_read_failed: ${fileData.error || 'no data'}`);
        actionToast(tabId, 'upload', fileData.error || 'no data', 'error');
        return;
    }
    // Stage 1: Inject file into DOM input element via DataTransfer
    const fileName = file_name || fileData.file_name || 'file';
    const mimeType = mime_type || fileData.mime_type || 'application/octet-stream';
    try {
        const results = await chrome.scripting.executeScript({
            target: { tabId, allFrames: true },
            world: 'MAIN',
            func: injectFileIntoInput,
            args: [selector, fileData.data_base64, fileName, mimeType]
        });
        let picked = null;
        for (const r of results) {
            const res = r.result;
            if (res?.success) {
                picked = res;
                break;
            }
        }
        if (!picked) {
            // Fall back to main frame result for error message
            picked = results[0]?.result || null;
        }
        if (picked?.success) {
            // Stage 1 injection succeeded — verify file persisted (polls with delay)
            debugLog(DebugCategory.CONNECTION, 'Upload injected, verifying persistence...', { selector, fileName });
            const verification = await verifyFileOnInput(tabId, selector);
            if (verification.has_file) {
                // Stage 1 success — file persisted
                debugLog(DebugCategory.CONNECTION, 'Upload Stage 1 verified', {
                    selector,
                    fileName,
                    fileSize: picked.file_size
                });
                actionToast(tabId, 'upload', fileName, 'success');
                sendAsyncResult(syncClient, query.id, correlationId, 'complete', {
                    success: true,
                    stage: 1,
                    file_name: picked.file_name,
                    file_size: picked.file_size,
                    selector
                });
            }
            else {
                // Stage 1 file was cleared by the form — escalate to Stage 4
                debugLog(DebugCategory.CONNECTION, 'Upload Stage 1 file cleared, escalating to Stage 4', { selector });
                actionToast(tabId, 'upload', 'Escalating to OS automation...', 'trying', 30000);
                const escalation = await escalateToStage4(tabId, selector, file_path, getServerUrl());
                if (escalation.success) {
                    debugLog(DebugCategory.CONNECTION, 'Upload Stage 4 succeeded', { selector, fileName: escalation.file_name });
                    actionToast(tabId, 'upload', escalation.file_name || fileName, 'success');
                    sendAsyncResult(syncClient, query.id, correlationId, 'complete', {
                        success: true,
                        stage: 4,
                        escalation_reason: escalation.escalation_reason,
                        file_name: escalation.file_name,
                        selector
                    });
                }
                else {
                    debugLog(DebugCategory.CONNECTION, 'Upload Stage 4 failed', { selector, error: escalation.error });
                    actionToast(tabId, 'upload', escalation.error || 'Stage 4 failed', 'error');
                    sendAsyncResult(syncClient, query.id, correlationId, 'error', {
                        stage: 4,
                        escalation_reason: escalation.escalation_reason
                    }, escalation.error || 'stage4_failed');
                }
            }
        }
        else {
            const error = picked?.error || 'injection_failed';
            debugLog(DebugCategory.CONNECTION, 'Upload injection failed', { selector, error });
            actionToast(tabId, 'upload', error, 'error');
            sendAsyncResult(syncClient, query.id, correlationId, 'error', null, error);
        }
    }
    catch (err) {
        const error = errorMessage(err, 'script_execution_failed');
        debugLog(DebugCategory.CONNECTION, 'Upload executeScript failed', { error });
        actionToast(tabId, 'upload', error, 'error');
        sendAsyncResult(syncClient, query.id, correlationId, 'error', null, error);
    }
}
//# sourceMappingURL=upload-handler.js.map