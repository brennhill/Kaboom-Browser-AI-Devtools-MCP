"use strict";
(() => {
  // extension/lib/timeouts.js
  function readTestScale() {
    const globalScale = typeof globalThis !== "undefined" && typeof globalThis.KABOOM_TEST_TIMEOUT_SCALE === "number" ? globalThis.KABOOM_TEST_TIMEOUT_SCALE : null;
    if (globalScale !== null)
      return globalScale;
    if (typeof process !== "undefined" && process.env) {
      const raw = process.env.KABOOM_TEST_TIMEOUT_SCALE || process.env.KABOOM_TEST_TIME_SCALE;
      if (raw) {
        const parsed = Number(raw);
        if (Number.isFinite(parsed))
          return parsed;
      }
    }
    return 1;
  }
  function scaleTimeout(ms) {
    const scale = readTestScale();
    if (!Number.isFinite(scale) || scale <= 0 || scale === 1) {
      return ms;
    }
    return Math.max(5, Math.round(ms * scale));
  }

  // extension/lib/constants.js
  var DEFAULT_SERVER_URL = "http://localhost:7890";
  var ASYNC_COMMAND_TIMEOUT_MS = scaleTimeout(6e4);
  var AI_CONTEXT_PIPELINE_TIMEOUT_MS = scaleTimeout(3e3);
  var SettingName = {
    NETWORK_WATERFALL: "set_network_waterfall_enabled",
    PERFORMANCE_MARKS: "set_performance_marks_enabled",
    ACTION_REPLAY: "set_action_replay_enabled",
    WEBSOCKET_CAPTURE: "set_web_socket_capture_enabled",
    WEBSOCKET_CAPTURE_MODE: "set_web_socket_capture_mode",
    PERFORMANCE_SNAPSHOT: "set_performance_snapshot_enabled",
    DEFERRAL: "set_deferral_enabled",
    NETWORK_BODY_CAPTURE: "set_network_body_capture_enabled",
    ACTION_TOASTS: "set_action_toasts_enabled",
    SUBTITLES: "set_subtitles_enabled",
    SERVER_URL: "set_server_url"
  };
  var VALID_SETTING_NAMES = new Set(Object.values(SettingName));
  var RuntimeMessageName = {
    SHOW_TRACKED_HOVER_LAUNCHER: "kaboom_show_tracked_hover_launcher"
  };
  var INJECT_FORWARDED_SETTINGS = /* @__PURE__ */ new Set([
    SettingName.NETWORK_WATERFALL,
    SettingName.PERFORMANCE_MARKS,
    SettingName.ACTION_REPLAY,
    SettingName.WEBSOCKET_CAPTURE,
    SettingName.WEBSOCKET_CAPTURE_MODE,
    SettingName.PERFORMANCE_SNAPSHOT,
    SettingName.DEFERRAL,
    SettingName.NETWORK_BODY_CAPTURE,
    SettingName.SERVER_URL
  ]);
  var StorageKey = {
    TRACKED_TAB_ID: "trackedTabId",
    TRACKED_TAB_URL: "trackedTabUrl",
    TRACKED_TAB_TITLE: "trackedTabTitle",
    AI_WEB_PILOT_ENABLED: "aiWebPilotEnabled",
    DEBUG_MODE: "debugMode",
    SERVER_URL: "serverUrl",
    SCREENSHOT_ON_ERROR: "screenshotOnError",
    SOURCE_MAP_ENABLED: "sourceMapEnabled",
    LOG_LEVEL: "logLevel",
    THEME: "theme",
    DEFERRAL_ENABLED: "deferralEnabled",
    WEBSOCKET_CAPTURE_ENABLED: "webSocketCaptureEnabled",
    WEBSOCKET_CAPTURE_MODE: "webSocketCaptureMode",
    NETWORK_WATERFALL_ENABLED: "networkWaterfallEnabled",
    PERFORMANCE_MARKS_ENABLED: "performanceMarksEnabled",
    ACTION_REPLAY_ENABLED: "actionReplayEnabled",
    NETWORK_BODY_CAPTURE_ENABLED: "networkBodyCaptureEnabled",
    ACTION_TOASTS_ENABLED: "actionToastsEnabled",
    SUBTITLES_ENABLED: "subtitlesEnabled",
    ACTION_RECORDING: "kaboom_action_recording",
    RECORDING: "kaboom_recording",
    TRACKED_HOVER_LAUNCHER_HIDDEN: "kaboom_tracked_hover_launcher_hidden",
    // Per-session token the tracked-hover launcher publishes so the draw-mode
    // content script can prove a kaboom-annotations-ready event is extension-origin
    // (chrome.storage is invisible to page scripts, so a page cannot read/forge it).
    ANNOTATION_CHANNEL_NONCE: "kaboom_annotation_channel_nonce",
    PENDING_RECORDING: "kaboom_pending_recording",
    PENDING_MIC_RECORDING: "kaboom_pending_mic_recording",
    MIC_GRANTED: "kaboom_mic_granted",
    RECORD_AUDIO_PREF: "kaboom_record_audio_pref",
    TERMINAL_CONFIG: "kaboom_terminal_config",
    TERMINAL_AI_COMMAND: "kaboom_terminal_ai_command",
    TERMINAL_DEV_ROOT: "kaboom_terminal_dev_root",
    POPUP_LAST_STATUS: "kaboom_popup_last_status",
    TERMINAL_SESSION: "kaboom_terminal_session",
    TERMINAL_UI_STATE: "kaboom_terminal_ui_state",
    TERMINAL_WORKSPACE_GROUP_ID: "kaboom_terminal_workspace_group_id",
    TERMINAL_WORKSPACE_MAIN_TAB_ID: "kaboom_terminal_workspace_main_tab_id",
    CLOAKED_DOMAINS: "kaboom_cloaked_domains",
    ERROR_GROUPS: "kaboom_error_groups"
  };

  // extension/lib/brand.js
  var KABOOM_LOG_PREFIX = "[KaBOOM!]";
  var KABOOM_RECORDING_LOG_PREFIX = "[KaBOOM! REC]";

  // extension/lib/storage/io.js
  function isPromiseLike(value) {
    return typeof value === "object" && value !== null && typeof value.then === "function";
  }
  function storageLastError() {
    if (typeof chrome === "undefined" || !chrome.runtime)
      return null;
    const error = chrome.runtime.lastError;
    return error ? error.message ?? "unknown chrome.storage error" : null;
  }
  function persist(write, context) {
    void write.catch((error) => {
      console.warn(`${KABOOM_LOG_PREFIX} storage write failed (${context}):`, error);
    });
  }
  function readStorage(method, keys) {
    return new Promise((resolve, reject) => {
      let settled = false;
      const finish = (result = {}) => {
        if (settled)
          return;
        settled = true;
        resolve(result);
      };
      try {
        const maybePromise = method(keys, finish);
        if (isPromiseLike(maybePromise)) {
          maybePromise.then((result) => finish(result ?? {})).catch(reject);
        }
      } catch (error) {
        reject(error);
      }
    });
  }
  function runStorageWrite(label, invoke) {
    return new Promise((resolve, reject) => {
      let settled = false;
      const finish = () => {
        if (settled)
          return;
        settled = true;
        const errorMessage2 = storageLastError();
        if (errorMessage2)
          reject(new Error(`chrome.storage ${label} failed: ${errorMessage2}`));
        else
          resolve();
      };
      try {
        const maybePromise = invoke(finish);
        if (isPromiseLike(maybePromise)) {
          maybePromise.then(() => finish()).catch(reject);
        }
      } catch (error) {
        reject(error);
      }
    });
  }
  function writeStorage(method, items) {
    return runStorageWrite("write", (finish) => method(items, finish));
  }
  function removeFromStorage(method, keys) {
    return runStorageWrite("remove", (finish) => method(keys, finish));
  }

  // extension/lib/storage/local.js
  async function getLocal(key) {
    if (typeof chrome === "undefined" || !chrome.storage)
      return void 0;
    const result = await readStorage(chrome.storage.local.get.bind(chrome.storage.local), key);
    return result[key];
  }
  async function getLocals(keys) {
    if (typeof chrome === "undefined" || !chrome.storage)
      return {};
    return await readStorage(chrome.storage.local.get.bind(chrome.storage.local), keys);
  }
  async function setLocal(key, value) {
    if (typeof chrome === "undefined" || !chrome.storage)
      return;
    await writeStorage(chrome.storage.local.set.bind(chrome.storage.local), { [key]: value });
  }
  async function setLocals(items) {
    if (typeof chrome === "undefined" || !chrome.storage)
      return;
    await writeStorage(chrome.storage.local.set.bind(chrome.storage.local), items);
  }
  async function removeLocal(key) {
    if (typeof chrome === "undefined" || !chrome.storage)
      return;
    await removeFromStorage(chrome.storage.local.remove.bind(chrome.storage.local), [key]);
  }
  async function removeLocals(keys) {
    if (typeof chrome === "undefined" || !chrome.storage)
      return;
    await removeFromStorage(chrome.storage.local.remove.bind(chrome.storage.local), keys);
  }

  // extension/lib/storage/session.js
  function getStorageWithSession() {
    if (typeof chrome === "undefined" || !chrome.storage)
      return null;
    return chrome.storage;
  }
  async function getSession(key) {
    const storage = getStorageWithSession();
    if (!storage?.session)
      return void 0;
    const result = await readStorage(storage.session.get.bind(storage.session), key);
    return result[key];
  }
  async function setSession(key, value) {
    const storage = getStorageWithSession();
    if (!storage?.session)
      return;
    await writeStorage(storage.session.set.bind(storage.session), { [key]: value });
  }

  // extension/lib/tabs/internal-url.js
  function isInternalUrl(url) {
    if (!url)
      return true;
    const internalPrefixes = ["chrome://", "chrome-extension://", "about:", "edge://", "brave://", "devtools://"];
    return internalPrefixes.some((prefix) => url.startsWith(prefix));
  }

  // extension/popup/shell/ui-utils.js
  function formatFileSize(bytes) {
    if (bytes === 0)
      return "0 B";
    const units = ["B", "KB", "MB", "GB"];
    const i = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
    const value = bytes / Math.pow(1024, i);
    return `${value < 10 ? value.toFixed(1) : Math.round(value)} ${units[i]}`;
  }

  // extension/popup/shell/status-display.js
  var DEFAULT_MAX_ENTRIES = 1e3;
  function updateConnectionStatus(status) {
    const statusEl = document.getElementById("status");
    const entriesEl = document.getElementById("entries-count");
    const errorEl = document.getElementById("error-message");
    const serverUrlEl = document.getElementById("server-url");
    const logFileEl = document.getElementById("log-file-path");
    const errorCountEl = document.getElementById("error-count");
    const troubleshootingEl = document.getElementById("troubleshooting");
    if (status.connected) {
      if (statusEl) {
        statusEl.textContent = "Connected";
        statusEl.classList.remove("disconnected");
        statusEl.classList.add("connected");
      }
      const entries = status.entries || 0;
      const maxEntries = status.maxEntries || DEFAULT_MAX_ENTRIES;
      if (entriesEl) {
        entriesEl.textContent = `${entries} / ${maxEntries}`;
      }
      if (errorEl) {
        errorEl.textContent = "";
      }
      if (troubleshootingEl) {
        troubleshootingEl.style.display = "none";
      }
    } else {
      if (statusEl) {
        statusEl.textContent = "Offline";
        statusEl.classList.remove("connected");
        statusEl.classList.add("disconnected");
      }
      if (errorEl && status.error) {
        errorEl.textContent = status.error;
      }
      if (troubleshootingEl) {
        troubleshootingEl.style.display = "block";
      }
    }
    const versionWarningEl = document.getElementById("version-mismatch");
    if (versionWarningEl) {
      if (status.versionMismatch && status.serverVersion && status.extensionVersion) {
        versionWarningEl.style.display = "block";
        const versionDetail = versionWarningEl.querySelector(".version-detail");
        if (versionDetail) {
          versionDetail.textContent = `Server: v${status.serverVersion} / Extension: v${status.extensionVersion}`;
        }
      } else {
        versionWarningEl.style.display = "none";
      }
    }
    const securityWarningEl = document.getElementById("security-mode-warning");
    const securityDetailEl = document.getElementById("security-mode-detail");
    if (securityWarningEl) {
      if (status.securityMode === "insecure_proxy") {
        securityWarningEl.style.display = "block";
        if (securityDetailEl) {
          const rewrites = status.insecureRewritesApplied && status.insecureRewritesApplied.length > 0 ? status.insecureRewritesApplied.join(", ") : "csp_headers";
          securityDetailEl.textContent = `INSECURE DEBUG MODE active. production_parity=${status.productionParity === false ? "false" : "true"}; rewrites=${rewrites}`;
        }
      } else {
        securityWarningEl.style.display = "none";
        if (securityDetailEl) {
          securityDetailEl.textContent = "";
        }
      }
    }
    if (serverUrlEl && status.serverUrl) {
      serverUrlEl.textContent = status.serverUrl;
    }
    if (logFileEl && status.logFile) {
      logFileEl.textContent = status.logFile;
    }
    if (errorCountEl && status.errorCount !== void 0) {
      errorCountEl.textContent = String(status.errorCount);
    }
    const fileSizeEl = document.getElementById("log-file-size");
    if (fileSizeEl && status.logFileSize !== void 0) {
      fileSizeEl.textContent = formatFileSize(status.logFileSize);
    }
    const healthSection = document.getElementById("health-indicators");
    const cbEl = document.getElementById("health-circuit-breaker");
    const mpEl = document.getElementById("health-memory-pressure");
    if (healthSection && cbEl && mpEl) {
      const cbState = status.circuitBreakerState || "closed";
      const mpState = status.memoryPressure?.memoryPressureLevel || "normal";
      cbEl.classList.remove("health-error", "health-warning");
      if (!status.connected || cbState === "closed") {
        cbEl.style.display = "none";
        cbEl.textContent = "";
      } else if (cbState === "open") {
        cbEl.style.display = "";
        cbEl.classList.add("health-error");
        cbEl.textContent = "Server: paused (recovering from errors)";
      } else if (cbState === "half-open") {
        cbEl.style.display = "";
        cbEl.classList.add("health-warning");
        cbEl.textContent = "Server: recovering";
      }
      mpEl.classList.remove("health-error", "health-warning");
      if (!status.connected || mpState === "normal") {
        mpEl.style.display = "none";
        mpEl.textContent = "";
      } else if (mpState === "soft") {
        mpEl.style.display = "";
        mpEl.classList.add("health-warning");
        mpEl.textContent = "Memory: elevated (some features limited)";
      } else if (mpState === "hard") {
        mpEl.style.display = "";
        mpEl.classList.add("health-error");
        mpEl.textContent = "Memory: critical (network capture disabled)";
      }
      const cbVisible = status.connected && cbState !== "closed";
      const mpVisible = status.connected && mpState !== "normal";
      healthSection.style.display = cbVisible || mpVisible ? "" : "none";
    }
    const contextWarningEl = document.getElementById("context-warning");
    const contextWarningTextEl = document.getElementById("context-warning-text");
    if (contextWarningEl) {
      if (status.connected && status.contextWarning) {
        contextWarningEl.style.display = "block";
        if (contextWarningTextEl) {
          contextWarningTextEl.textContent = `${status.contextWarning.count} recent entries have context annotations averaging ${status.contextWarning.sizeKB}KB. This may consume significant AI context window space.`;
        }
      } else {
        contextWarningEl.style.display = "none";
        if (contextWarningTextEl) {
          contextWarningTextEl.textContent = "";
        }
      }
    }
  }

  // extension/popup/system-doctor.js
  function doctorElements() {
    return {
      host: document.getElementById("system-doctor"),
      overall: document.getElementById("system-doctor-overall"),
      checks: document.getElementById("system-doctor-checks")
    };
  }
  function renderMessage(label, detail, status) {
    const elements = doctorElements();
    if (!elements.overall || !elements.checks)
      return;
    elements.overall.textContent = label;
    elements.overall.className = `doctor-overall doctor-${status}`;
    const row = document.createElement("div");
    row.className = `doctor-check doctor-${status}`;
    row.textContent = detail;
    elements.checks.replaceChildren(row);
  }
  function renderReport(report) {
    const elements = doctorElements();
    if (!elements.overall || !elements.checks)
      return;
    elements.overall.textContent = report.ready_for_interaction ? "Ready" : "Needs attention";
    elements.overall.className = `doctor-overall doctor-${report.ready_for_interaction ? "pass" : "warn"}`;
    elements.checks.replaceChildren(...report.checks.map((check) => {
      const row = document.createElement("div");
      row.className = `doctor-check doctor-${check.status}`;
      const detail = document.createElement("div");
      detail.className = "doctor-check-detail";
      detail.textContent = check.detail;
      row.appendChild(detail);
      if (check.fix) {
        const fix = document.createElement("div");
        fix.className = "doctor-check-fix";
        fix.textContent = `Fix: ${check.fix}`;
        row.appendChild(fix);
      }
      return row;
    }));
  }
  function isDoctorReport(value) {
    if (!value || typeof value !== "object")
      return false;
    const candidate = value;
    return typeof candidate.ready_for_interaction === "boolean" && typeof candidate.version === "string" && Array.isArray(candidate.checks);
  }
  async function refreshSystemDoctor(status, fetchImpl = fetch) {
    if (!status.connected) {
      renderMessage("Daemon offline", "Start the daemon to run System Doctor checks.", "warn");
      return;
    }
    try {
      const response = await fetchImpl(`${status.serverUrl ?? "http://127.0.0.1:7890"}/doctor`);
      if (!response.ok) {
        renderMessage("Check failed", `System Doctor returned HTTP ${response.status}. Retry after restarting the daemon.`, "fail");
        return;
      }
      const report = await response.json();
      if (!isDoctorReport(report)) {
        renderMessage("Check failed", "System Doctor returned an invalid report. Update the daemon and extension.", "fail");
        return;
      }
      renderReport(report);
    } catch {
      renderMessage("Check failed", "System Doctor could not reach the daemon. Retry after restarting it.", "fail");
    }
  }

  // extension/lib/storage/changes.js
  function onStorageChanged(listener) {
    if (typeof chrome === "undefined" || !chrome.storage)
      return () => {
      };
    chrome.storage.onChanged.addListener(listener);
    return () => chrome.storage.onChanged.removeListener(listener);
  }

  // extension/lib/error-utils.js
  function errorMessage(err, fallback = "Unknown error") {
    if (err instanceof Error && err.message)
      return err.message;
    if (typeof err === "string" && err)
      return err;
    return fallback;
  }

  // extension/popup/recording/recording-io.js
  var LOG = `${KABOOM_RECORDING_LOG_PREFIX} Popup:`;
  function sendRecordingGestureDecision(type) {
    chrome.runtime.sendMessage({ type }, () => {
      void chrome.runtime.lastError;
    });
  }
  function showMicPermissionPrompt(saveInfoEl, audioMode) {
    chrome.tabs.query({ active: true, currentWindow: true }, (activeTabs) => {
      persist(setLocal(StorageKey.PENDING_MIC_RECORDING, { audioMode, returnTabId: activeTabs[0]?.id }), "pending-mic-recording");
    });
    saveInfoEl.innerHTML = 'Microphone access needed. <a href="#" id="grant-mic-link" style="color: #58a6ff; text-decoration: underline; cursor: pointer">Grant access</a>';
    saveInfoEl.style.display = "block";
    saveInfoEl.style.background = "rgba(248, 81, 73, 0.1)";
    saveInfoEl.style.color = "#f85149";
    const link = document.getElementById("grant-mic-link");
    if (link) {
      link.addEventListener("click", (e) => {
        e.preventDefault();
        chrome.tabs.create({ url: chrome.runtime.getURL("mic-permission.html") });
      });
    }
  }
  function sendRecordStart(els, state, audioMode, showRecording3, showIdle3, showStartError2) {
    console.log(LOG, "sendStart() called, sending screen_recording_start with audio:", audioMode);
    chrome.runtime.sendMessage({ type: "screen_recording_start", audio: audioMode }, (resp) => {
      console.log(LOG, "screen_recording_start response:", resp);
      if (chrome.runtime.lastError) {
        console.error(LOG, "screen_recording_start lastError:", chrome.runtime.lastError.message);
      }
      if (resp?.status === "recording" && resp.name) {
        showRecording3(els, state, resp.name, resp.startTime ?? Date.now());
      } else {
        showIdle3(els, state);
        if (resp?.error)
          showStartError2(els.saveInfoEl, resp.error);
      }
    });
  }
  function tryMicPermissionThenStart(els, state, audioMode, showRecording3, showIdle3, showStartError2) {
    console.log(LOG, "trying getUserMedia from popup...");
    navigator.mediaDevices.getUserMedia({ audio: true }).then((micStream) => {
      console.log(LOG, "getUserMedia succeeded from popup");
      micStream.getTracks().forEach((t) => t.stop());
      persist(setLocal(StorageKey.MIC_GRANTED, true), "mic-granted");
      sendRecordStart(els, state, audioMode, showRecording3, showIdle3, showStartError2);
    }).catch((err) => {
      console.log(LOG, "getUserMedia FAILED:", err.name, errorMessage(err));
      persist(removeLocal(StorageKey.MIC_GRANTED), "mic-granted-clear");
      showIdle3(els, state);
      if (els.saveInfoEl)
        showMicPermissionPrompt(els.saveInfoEl, audioMode);
    });
  }
  function handleStartClick(els, state, showRecording3, showIdle3, showStartError2) {
    const audioSelect = document.getElementById("record-audio-mode");
    const audioMode = audioSelect?.value ?? "";
    persist(setLocal(StorageKey.RECORD_AUDIO_PREF, audioMode), "record-audio-pref");
    if (els.optionsEl)
      els.optionsEl.style.display = "none";
    if (els.saveInfoEl)
      els.saveInfoEl.style.display = "none";
    els.label.textContent = "Starting...";
    if (audioMode === "mic" || audioMode === "both") {
      console.log(LOG, "mic/both mode \u2014 checking stored mic approval");
      tryMicPermissionThenStart(els, state, audioMode, showRecording3, showIdle3, showStartError2);
    } else {
      sendRecordStart(els, state, audioMode, showRecording3, showIdle3, showStartError2);
    }
  }
  function handleStopClick(els, state, showIdle3, showSaveResult2) {
    els.row.classList.remove("is-recording");
    els.label.textContent = "Saving...";
    console.log(LOG, "sending screen_recording_stop");
    chrome.runtime.sendMessage({ type: "screen_recording_stop" }, (resp) => {
      console.log(LOG, "screen_recording_stop response:", resp);
      if (chrome.runtime.lastError) {
        console.error(LOG, "screen_recording_stop lastError:", chrome.runtime.lastError.message);
      }
      showIdle3(els, state);
      showSaveResult2(els.saveInfoEl, resp);
    });
  }

  // extension/popup/recording/recording.js
  var START_LABEL = "Record screen";
  var STOP_LABEL = "Stop recording";
  var HIGHLIGHT_LABEL = "\u25CF \xAB Click here to record";
  var RECENT_RECORDING_START_MS = 8e3;
  var TOP_NOTICE_DURATION_MS = 4e3;
  var LOG2 = `${KABOOM_RECORDING_LOG_PREFIX} Popup:`;
  var AUDIO_LABELS = {
    "": "Video only",
    tab: "Video + tab audio",
    mic: "Video + microphone",
    both: "Video + tab + mic"
  };
  var topNoticeTimer = null;
  function getRecordSection(els) {
    const closest = els.row.closest;
    if (typeof closest !== "function")
      return null;
    return closest.call(els.row, ".section");
  }
  function applyRecordHighlight(els) {
    const section = getRecordSection(els);
    if (section)
      section.classList.add("record-highlight");
    els.label.textContent = HIGHLIGHT_LABEL;
  }
  function removeRecordHighlight(els) {
    const section = getRecordSection(els);
    if (section)
      section.classList.remove("record-highlight");
    if (els.label.textContent === HIGHLIGHT_LABEL) {
      els.label.textContent = START_LABEL;
    }
  }
  function showRecording(els, state, name, startTime) {
    const wasRecording = state.isRecording;
    removeRecordHighlight(els);
    state.isRecording = true;
    els.row.classList.add("is-recording");
    els.label.textContent = STOP_LABEL;
    els.statusEl.textContent = "";
    if (els.optionsEl)
      els.optionsEl.style.display = "none";
    if (state.timerInterval)
      clearInterval(state.timerInterval);
    state.timerInterval = setInterval(() => {
      const elapsed = Math.round((Date.now() - startTime) / 1e3);
      const mins = Math.floor(elapsed / 60);
      const secs = elapsed % 60;
      els.statusEl.textContent = `${mins}:${secs.toString().padStart(2, "0")}`;
    }, 1e3);
    if (!wasRecording && Date.now() - startTime <= RECENT_RECORDING_START_MS) {
      showTopNotice(els, "Recording started");
    }
  }
  function showIdle(els, state) {
    state.isRecording = false;
    removeRecordHighlight(els);
    els.row.classList.remove("is-recording");
    els.label.textContent = START_LABEL;
    els.statusEl.textContent = "";
    if (els.optionsEl)
      els.optionsEl.style.display = "block";
    if (state.timerInterval) {
      clearInterval(state.timerInterval);
      state.timerInterval = null;
    }
  }
  function describePendingRecording(pending) {
    const parts = [];
    if (pending.name)
      parts.push(`Name: ${pending.name}`);
    if (typeof pending.fps === "number")
      parts.push(`FPS: ${pending.fps}`);
    const audioLabel = AUDIO_LABELS[pending.audio ?? ""] ?? AUDIO_LABELS[""];
    parts.push(`Mode: ${audioLabel}`);
    return parts.join(" \xB7 ");
  }
  function setApprovalPendingState(els, approvalEls, state, pending) {
    const setRowAriaDisabled = (value) => {
      const setAttr = els.row.setAttribute;
      const removeAttr = els.row.removeAttribute;
      if (value !== null) {
        if (typeof setAttr === "function")
          setAttr.call(els.row, "aria-disabled", value);
        return;
      }
      if (typeof removeAttr === "function")
        removeAttr.call(els.row, "aria-disabled");
    };
    const approvalPending = Boolean(pending && !pending.highlight && !state.isRecording);
    if (approvalPending) {
      if (approvalEls.detail && pending)
        approvalEls.detail.textContent = describePendingRecording(pending);
      if (approvalEls.card)
        approvalEls.card.style.display = "block";
      els.row.classList.add("is-disabled");
      setRowAriaDisabled("true");
      if (els.optionsEl)
        els.optionsEl.style.display = "none";
      return;
    }
    if (approvalEls.detail)
      approvalEls.detail.textContent = "";
    if (approvalEls.card)
      approvalEls.card.style.display = "none";
    els.row.classList.remove("is-disabled");
    setRowAriaDisabled(null);
    if (!state.isRecording && els.optionsEl)
      els.optionsEl.style.display = "block";
  }
  function showTopNotice(els, text) {
    const notice = els.topNoticeEl;
    if (!notice)
      return;
    notice.textContent = text;
    notice.style.display = "block";
    if (topNoticeTimer)
      clearTimeout(topNoticeTimer);
    topNoticeTimer = setTimeout(() => {
      notice.style.display = "none";
    }, TOP_NOTICE_DURATION_MS);
  }
  function showSavedLink(saveInfoEl, displayName, filePath) {
    saveInfoEl.textContent = "Saved: ";
    const link = document.createElement("a");
    link.href = "#";
    link.id = "reveal-recording";
    link.textContent = displayName;
    link.style.color = "#58a6ff";
    link.style.textDecoration = "underline";
    link.style.cursor = "pointer";
    saveInfoEl.appendChild(link);
    const linkEl = document.getElementById("reveal-recording");
    if (linkEl) {
      linkEl.addEventListener("click", (e) => {
        e.preventDefault();
        chrome.runtime.sendMessage({ type: "reveal_file", path: filePath }, (result) => {
          if (result?.error) {
            saveInfoEl.textContent = `Could not open folder: ${result.error}`;
            saveInfoEl.style.color = "#f85149";
            setTimeout(() => {
              saveInfoEl.style.display = "none";
            }, 5e3);
          }
        });
      });
    }
  }
  function showSaveResult(saveInfoEl, resp) {
    if (resp?.status !== "saved" || !resp.name || !saveInfoEl)
      return;
    const displayName = resp.name.replace(/--\d{4}-\d{2}-\d{2}-\d{4}(-\d+)?$/, "");
    if (resp.path) {
      showSavedLink(saveInfoEl, displayName, resp.path);
    } else {
      saveInfoEl.textContent = `Saved: ${displayName}`;
    }
    saveInfoEl.style.display = "block";
    setTimeout(() => {
      saveInfoEl.style.display = "none";
    }, 12e3);
  }
  function showStartError(saveInfoEl, errorText) {
    if (!saveInfoEl)
      return;
    saveInfoEl.textContent = errorText;
    saveInfoEl.style.display = "block";
    saveInfoEl.style.background = "rgba(248, 81, 73, 0.1)";
    saveInfoEl.style.color = "#f85149";
    setTimeout(() => {
      saveInfoEl.style.display = "none";
      saveInfoEl.style.background = "rgba(63, 185, 80, 0.1)";
      saveInfoEl.style.color = "#3fb950";
    }, 5e3);
  }
  function setupRecordingUI() {
    const row = document.getElementById("record-row");
    const label = document.getElementById("record-label");
    const statusEl = document.getElementById("recording-status");
    if (!row || !label || !statusEl)
      return;
    const els = {
      row,
      label,
      statusEl,
      optionsEl: document.getElementById("record-options"),
      saveInfoEl: document.getElementById("record-save-info"),
      topNoticeEl: document.getElementById("record-top-notice")
    };
    const approvalEls = {
      card: document.getElementById("record-approval-card"),
      detail: document.getElementById("record-approval-detail"),
      approveBtn: document.getElementById("record-approve-btn"),
      denyBtn: document.getElementById("record-deny-btn")
    };
    const state = { isRecording: false, timerInterval: null };
    let pendingRecordingIntent = null;
    const updatePendingRecording = (pendingValue) => {
      const pending = pendingValue;
      if (pending?.highlight && !state.isRecording) {
        applyRecordHighlight(els);
        pendingRecordingIntent = null;
        setApprovalPendingState(els, approvalEls, state, null);
        persist(removeLocal(StorageKey.PENDING_RECORDING), "pending-recording-clear");
        return;
      }
      pendingRecordingIntent = pending && !pending.highlight ? pending : null;
      if (!pendingRecordingIntent && !state.isRecording)
        removeRecordHighlight(els);
      setApprovalPendingState(els, approvalEls, state, pendingRecordingIntent);
    };
    const clearPendingRecordingIntent = () => {
      pendingRecordingIntent = null;
      setApprovalPendingState(els, approvalEls, state, null);
      persist(removeLocal(StorageKey.PENDING_RECORDING), "pending-recording-clear");
    };
    void getLocal(StorageKey.RECORDING).then(async (value) => {
      const rec = value;
      console.log(LOG2, "recording state from storage:", rec);
      if (rec?.active && rec.name && rec.startTime) {
        console.log(LOG2, "resuming recording UI for", rec.name);
        showRecording(els, state, rec.name, rec.startTime);
      }
      const pendingValue = await getLocal(StorageKey.PENDING_RECORDING);
      updatePendingRecording(pendingValue);
    });
    onStorageChanged((changes, areaName) => {
      if (areaName === "local" && changes[StorageKey.RECORDING]) {
        const rec = changes[StorageKey.RECORDING].newValue;
        console.log(LOG2, "recording state changed:", rec);
        if (rec?.active && rec.name && rec.startTime) {
          showRecording(els, state, rec.name, rec.startTime);
        } else {
          showIdle(els, state);
        }
        setApprovalPendingState(els, approvalEls, state, pendingRecordingIntent);
        return;
      }
      if (areaName === "local" && changes[StorageKey.PENDING_RECORDING]) {
        updatePendingRecording(changes[StorageKey.PENDING_RECORDING].newValue);
      }
    });
    approvalEls.approveBtn?.addEventListener("click", (event) => {
      event.preventDefault();
      sendRecordingGestureDecision("recording_gesture_granted");
      clearPendingRecordingIntent();
    });
    approvalEls.denyBtn?.addEventListener("click", (event) => {
      event.preventDefault();
      sendRecordingGestureDecision("recording_gesture_denied");
      clearPendingRecordingIntent();
    });
    void getLocal(StorageKey.PENDING_MIC_RECORDING).then(async (value) => {
      const intent = value;
      console.log(LOG2, "pending mic recording intent:", intent);
      if (!intent?.audioMode)
        return;
      console.log(LOG2, "consuming mic intent, pre-selecting audio mode:", intent.audioMode);
      await removeLocal(StorageKey.PENDING_MIC_RECORDING);
      chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
        if (tabs[0]?.id) {
          chrome.tabs.sendMessage(tabs[0].id, {
            type: "kaboom_action_toast",
            text: "",
            detail: "",
            state: "success",
            duration_ms: 1
          }).catch(() => {
          });
        }
      });
      const audioSelect = document.getElementById("record-audio-mode");
      if (audioSelect)
        audioSelect.value = intent.audioMode;
    });
    void getLocal(StorageKey.RECORD_AUDIO_PREF).then((value) => {
      const saved = value;
      if (saved) {
        const audioSelect = document.getElementById("record-audio-mode");
        if (audioSelect)
          audioSelect.value = saved;
      }
    });
    row.addEventListener("click", () => {
      console.log(LOG2, "record row clicked, isRecording:", state.isRecording);
      if (pendingRecordingIntent && !state.isRecording) {
        console.log(LOG2, "record row click ignored while approval is pending");
        return;
      }
      removeRecordHighlight(els);
      if (state.isRecording) {
        handleStopClick(els, state, showIdle, showSaveResult);
      } else {
        handleStartClick(els, state, showRecording, showIdle, showStartError);
      }
    });
  }

  // extension/popup/draw-mode.js
  function showDrawModeError(label, message) {
    label.textContent = message;
    label.style.color = "#f85149";
    setTimeout(() => {
      label.textContent = "Draw";
      label.style.color = "";
    }, 3e3);
  }
  function setupDrawModeButton() {
    const row = document.getElementById("draw-mode-row");
    const label = document.getElementById("draw-mode-label");
    if (!row || !label)
      return;
    const statusEl = document.getElementById("draw-mode-status");
    if (statusEl) {
      const hasNavigator = typeof navigator !== "undefined";
      const isMac = hasNavigator && (navigator.platform?.toUpperCase().includes("MAC") || navigator.userAgentData?.platform === "macOS");
      statusEl.textContent = isMac ? "\u2325\u21E7D" : "Alt+Shift+D";
    }
    row.addEventListener("click", () => {
      chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
        const tab = tabs[0];
        if (!tab?.id) {
          showDrawModeError(label, "No active tab");
          return;
        }
        if (tab.url?.startsWith("chrome://") || tab.url?.startsWith("about:") || tab.url?.startsWith("chrome-extension://")) {
          showDrawModeError(label, "Cannot draw on internal pages");
          return;
        }
        const trackMsg = { type: "track_ui_feature", feature: "annotations" };
        chrome.runtime.sendMessage(trackMsg).catch(() => {
        });
        label.textContent = "Starting...";
        chrome.tabs.sendMessage(tab.id, { type: "kaboom_draw_mode_start", started_by: "user" }, (resp) => {
          if (chrome.runtime.lastError) {
            showDrawModeError(label, "Content script not loaded \u2014 try refreshing the page");
            return;
          }
          if (resp?.error) {
            showDrawModeError(label, resp.message || "Draw mode failed");
            return;
          }
          window.close();
        });
      });
    });
  }

  // extension/lib/daemon-http.js
  var DEFAULT_CLIENT_NAME = "kaboom-extension";
  function buildDaemonHeaders(options = {}) {
    const { clientName = DEFAULT_CLIENT_NAME, extensionVersion, contentType = "application/json", additionalHeaders = {} } = options;
    const normalizedVersion = typeof extensionVersion === "string" && extensionVersion.trim().length > 0 ? extensionVersion.trim() : "";
    const headers = {
      "X-Kaboom-Client": normalizedVersion ? `${clientName}/${normalizedVersion}` : clientName
    };
    if (contentType !== null) {
      headers["Content-Type"] = contentType;
    }
    if (normalizedVersion) {
      headers["X-Kaboom-Extension-Version"] = normalizedVersion;
    }
    return {
      ...headers,
      ...additionalHeaders
    };
  }
  function buildDaemonJSONRequestInit(payload, options = {}) {
    const { method = "POST", signal, ...headerOptions } = options;
    return {
      method,
      headers: buildDaemonHeaders(headerOptions),
      body: JSON.stringify(payload),
      ...signal ? { signal } : {}
    };
  }
  async function postDaemonJSON(url, payload, options = {}) {
    const { timeoutMs, signal, ...requestOptions } = options;
    const effectiveSignal = signal || (typeof timeoutMs === "number" && timeoutMs > 0 && typeof AbortSignal.timeout === "function" ? AbortSignal.timeout(timeoutMs) : void 0);
    return fetch(url, buildDaemonJSONRequestInit(payload, { ...requestOptions, signal: effectiveSignal }));
  }

  // extension/popup/recording/action-recording.js
  var START_LABEL2 = "Record action workflow";
  var STOP_LABEL2 = "Stop recording";
  function showRecording2(els, state) {
    state.isRecording = true;
    els.row.classList.add("is-recording");
    els.label.textContent = STOP_LABEL2;
    els.statusEl.textContent = "";
    if (state.timerInterval)
      clearInterval(state.timerInterval);
    const start = state.startTime ?? Date.now();
    state.timerInterval = setInterval(() => {
      const elapsed = Math.round((Date.now() - start) / 1e3);
      const mins = Math.floor(elapsed / 60);
      const secs = elapsed % 60;
      els.statusEl.textContent = `${mins}:${secs.toString().padStart(2, "0")}`;
    }, 1e3);
  }
  function showIdle2(els, state) {
    state.isRecording = false;
    state.recordingId = null;
    state.startTime = null;
    els.row.classList.remove("is-recording");
    els.label.textContent = START_LABEL2;
    els.statusEl.textContent = "";
    if (state.timerInterval) {
      clearInterval(state.timerInterval);
      state.timerInterval = null;
    }
  }
  function showError(els, message) {
    els.statusEl.textContent = message;
    els.statusEl.style.color = "#f85149";
    setTimeout(() => {
      els.statusEl.textContent = "";
      els.statusEl.style.color = "";
    }, 5e3);
  }
  async function getServerUrl() {
    const value = await getLocal(StorageKey.SERVER_URL);
    return value || DEFAULT_SERVER_URL;
  }
  function getConfigureError(data) {
    const message = data.error?.message;
    return typeof message === "string" && message.length > 0 ? message : null;
  }
  function extractRecordingID(data) {
    const text = data.result?.content?.[0]?.text ?? "";
    const idMatch = text.match(/"recording_id"\s*:\s*"([^"]+)"/);
    return idMatch?.[1] ?? null;
  }
  async function fetchDaemonPid() {
    try {
      const data = await callConfigureFromPopup({ what: "health" });
      const text = data.result?.content?.[0]?.text ?? "";
      const match = text.match(/"pid"\s*:\s*([0-9]+)/);
      if (!match)
        return null;
      const pid = parseInt(match[1] ?? "", 10);
      return Number.isFinite(pid) ? pid : null;
    } catch {
      return null;
    }
  }
  async function isActionRecordingStillLive(startedPid) {
    if (startedPid == null)
      return true;
    const currentPid = await fetchDaemonPid();
    if (currentPid == null)
      return true;
    return currentPid === startedPid;
  }
  async function callConfigureFromPopup(argumentsPayload) {
    const serverUrl = await getServerUrl();
    const resp = await postDaemonJSON(`${serverUrl}/mcp`, {
      jsonrpc: "2.0",
      id: Date.now(),
      method: "tools/call",
      params: {
        name: "configure",
        arguments: argumentsPayload
      }
    });
    if (!resp.ok) {
      throw new Error(`Server error: HTTP ${resp.status}`);
    }
    return await resp.json();
  }
  async function startActionRecording(els, state) {
    els.label.textContent = "Starting...";
    try {
      const data = await callConfigureFromPopup({
        what: "event_recording_start",
        name: `workflow-${Date.now()}`
      });
      const configureError = getConfigureError(data);
      if (configureError) {
        showIdle2(els, state);
        showError(els, configureError);
        return;
      }
      state.recordingId = extractRecordingID(data);
      state.startTime = Date.now();
      const daemonPid = await fetchDaemonPid();
      persist(setLocal(StorageKey.ACTION_RECORDING, {
        active: true,
        recordingId: state.recordingId,
        startTime: state.startTime,
        daemonPid
      }), "action-recording");
      showRecording2(els, state);
    } catch (err) {
      showIdle2(els, state);
      showError(els, `Connection failed: ${err instanceof Error ? err.message : String(err)}`);
    }
  }
  async function stopActionRecording(els, state) {
    els.label.textContent = "Stopping...";
    try {
      const data = await callConfigureFromPopup({
        what: "event_recording_stop",
        recording_id: state.recordingId ?? ""
      });
      const configureError = getConfigureError(data);
      if (configureError) {
        showError(els, configureError);
      }
      persist(removeLocal(StorageKey.ACTION_RECORDING), "action-recording-clear");
      showIdle2(els, state);
    } catch (err) {
      showIdle2(els, state);
      showError(els, `Connection failed: ${err instanceof Error ? err.message : String(err)}`);
    }
  }
  function setupActionRecordingUI() {
    const row = document.getElementById("action-record-row");
    const label = document.getElementById("action-record-label");
    const statusEl = document.getElementById("action-recording-status");
    if (!row || !label || !statusEl)
      return;
    const els = { row, label, statusEl };
    const state = {
      isRecording: false,
      recordingId: null,
      timerInterval: null,
      startTime: null
    };
    void getLocal(StorageKey.ACTION_RECORDING).then(async (value) => {
      const saved = value;
      if (!saved?.active || !saved.recordingId)
        return;
      if (!await isActionRecordingStillLive(saved.daemonPid)) {
        persist(removeLocal(StorageKey.ACTION_RECORDING), "action-recording-stale");
        return;
      }
      state.recordingId = saved.recordingId;
      state.startTime = saved.startTime ?? Date.now();
      showRecording2(els, state);
    });
    row.addEventListener("click", () => {
      if (state.isRecording) {
        void stopActionRecording(els, state);
      } else {
        void startActionRecording(els, state);
      }
    });
  }

  // extension/popup/feature-toggles.js
  var FEATURE_TOGGLES = [
    {
      id: "toggle-websocket",
      storageKey: StorageKey.WEBSOCKET_CAPTURE_ENABLED,
      messageType: SettingName.WEBSOCKET_CAPTURE,
      default: true
    },
    {
      id: "toggle-network-waterfall",
      storageKey: StorageKey.NETWORK_WATERFALL_ENABLED,
      messageType: SettingName.NETWORK_WATERFALL,
      default: true
    },
    {
      id: "toggle-performance-marks",
      storageKey: StorageKey.PERFORMANCE_MARKS_ENABLED,
      messageType: SettingName.PERFORMANCE_MARKS,
      default: true
    },
    {
      id: "toggle-action-replay",
      storageKey: StorageKey.ACTION_REPLAY_ENABLED,
      messageType: SettingName.ACTION_REPLAY,
      default: true
    },
    {
      id: "toggle-screenshot",
      storageKey: StorageKey.SCREENSHOT_ON_ERROR,
      messageType: "set_screenshot_on_error",
      default: true
    },
    {
      id: "toggle-source-maps",
      storageKey: StorageKey.SOURCE_MAP_ENABLED,
      messageType: "set_source_map_enabled",
      default: true
    },
    {
      id: "toggle-network-body-capture",
      storageKey: StorageKey.NETWORK_BODY_CAPTURE_ENABLED,
      messageType: SettingName.NETWORK_BODY_CAPTURE,
      default: true
    },
    {
      id: "toggle-action-toasts",
      storageKey: StorageKey.ACTION_TOASTS_ENABLED,
      messageType: SettingName.ACTION_TOASTS,
      default: true
    },
    {
      id: "toggle-subtitles",
      storageKey: StorageKey.SUBTITLES_ENABLED,
      messageType: SettingName.SUBTITLES,
      default: true
    }
  ];
  function handleFeatureToggle(storageKey, messageType, enabled) {
    chrome.runtime.sendMessage({ type: messageType, enabled }, (response) => {
      if (chrome.runtime.lastError) {
        console.error(`[KaBOOM!] Message error for ${messageType}:`, chrome.runtime.lastError.message);
      } else if (response?.success) {
        console.log(`[KaBOOM!] ${messageType} acknowledged by background`);
      } else {
        console.warn(`[KaBOOM!] ${messageType} - no response from background`);
      }
    });
  }
  function applyFeatureToggles(result) {
    for (const toggle of FEATURE_TOGGLES) {
      const checkbox = document.getElementById(toggle.id);
      if (checkbox) {
        const savedValue = result[toggle.storageKey];
        checkbox.checked = savedValue !== void 0 ? savedValue : toggle.default;
        checkbox.addEventListener("change", () => {
          handleFeatureToggle(toggle.storageKey, toggle.messageType, checkbox.checked);
        });
      }
    }
  }

  // extension/lib/tabs/tracked-tab-storage.js
  var TRACKED_TAB_STORAGE_KEYS = [
    StorageKey.TRACKED_TAB_ID,
    StorageKey.TRACKED_TAB_URL,
    StorageKey.TRACKED_TAB_TITLE
  ];
  async function readTrackedTab() {
    const stored = await getLocals(TRACKED_TAB_STORAGE_KEYS);
    return {
      id: stored[StorageKey.TRACKED_TAB_ID],
      url: stored[StorageKey.TRACKED_TAB_URL],
      title: stored[StorageKey.TRACKED_TAB_TITLE]
    };
  }
  async function setTrackedTab(tab) {
    if (!tab.id)
      return;
    await setLocals({
      [StorageKey.TRACKED_TAB_ID]: tab.id,
      [StorageKey.TRACKED_TAB_URL]: tab.url ?? "",
      [StorageKey.TRACKED_TAB_TITLE]: tab.title ?? ""
    });
  }
  async function clearTrackedTab() {
    await removeLocals(TRACKED_TAB_STORAGE_KEYS);
  }

  // extension/lib/tabs/cloaked-domains.js
  var BUILTIN_CLOAKED = ["cloudflare.com", "dash.cloudflare.com"];
  function matchesDomain(hostname, domain) {
    return hostname === domain || hostname.endsWith("." + domain);
  }
  async function isDomainCloaked(hostname) {
    const host = hostname || (typeof location !== "undefined" ? location.hostname : "");
    if (!host)
      return false;
    for (const domain of BUILTIN_CLOAKED) {
      if (matchesDomain(host, domain))
        return true;
    }
    try {
      const userDomains = await getLocal(StorageKey.CLOAKED_DOMAINS);
      if (userDomains && Array.isArray(userDomains)) {
        for (const domain of userDomains) {
          if (matchesDomain(host, domain))
            return true;
        }
      }
    } catch {
    }
    return false;
  }

  // extension/lib/tabs/tab-tracking-core.js
  function hostnameOf(url) {
    try {
      return url ? new URL(url).hostname : "";
    } catch {
      return "";
    }
  }
  function notifyTrackingState(tabId, isTracked) {
    chrome.tabs.sendMessage(tabId, { type: "tracking_state_changed", state: { isTracked, aiPilotEnabled: false } }).catch(() => {
    });
  }
  function ensureContentScript(tabId) {
    chrome.tabs.sendMessage(tabId, { type: "kaboom_ping" }, (response) => {
      if (chrome.runtime.lastError || !response?.status) {
        chrome.tabs.reload(tabId);
      } else {
        notifyTrackingState(tabId, true);
      }
    });
  }
  async function trackTab(tab) {
    if (isInternalUrl(tab.url))
      return "internal_page";
    if (await isDomainCloaked(hostnameOf(tab.url)))
      return "cloaked";
    await setTrackedTab(tab);
    if (tab.id)
      ensureContentScript(tab.id);
    return "tracked";
  }
  async function untrackTab(prevTabId, onStopped) {
    await clearTrackedTab();
    if (onStopped)
      await onStopped();
    if (typeof prevTabId === "number")
      notifyTrackingState(prevTabId, false);
  }

  // extension/lib/tabs/tab-focus.js
  async function focusTabAndWindow(tabId) {
    const updated = await chrome.tabs.update(tabId, { active: true });
    const tab = updated ?? await chrome.tabs.get(tabId);
    if (tab?.windowId != null && chrome.windows?.update) {
      await chrome.windows.update(tab.windowId, { focused: true });
    }
    return tab;
  }

  // extension/lib/tabs/request-audit.js
  async function requestAudit(pageUrl, tabId) {
    try {
      await chrome.runtime.sendMessage({ type: "open_terminal_panel", tab_id: tabId });
    } catch {
    }
    await chrome.runtime.sendMessage({ type: "qa_scan_requested", page_url: pageUrl });
  }

  // extension/popup/tabs/tab-tracking-api.js
  async function handleAuditClick(pageUrl, tabId) {
    await requestAudit(pageUrl, tabId);
  }
  async function handleStopTracking(showIdleState2) {
    const prevTabId = (await readTrackedTab()).id;
    if (!prevTabId)
      return;
    await untrackTab(prevTabId, () => {
      chrome.runtime.sendMessage({ type: "screen_recording_stop" }, () => {
        if (chrome.runtime.lastError) {
        }
      });
    });
    const btn = document.getElementById("track-page-btn");
    if (btn)
      showIdleState2(btn);
    console.log(KABOOM_LOG_PREFIX, "Stopped tracking via bar stop button");
  }
  async function handleUrlClick(tabId) {
    if (!tabId)
      return;
    try {
      await focusTabAndWindow(tabId);
      console.log(KABOOM_LOG_PREFIX, "Switched to tracked tab:", tabId);
    } catch (err) {
      console.error(KABOOM_LOG_PREFIX, "Failed to switch to tracked tab:", err);
      persist(clearTrackedTab(), "tracked-tab-clear");
    }
  }
  async function handleTrackPageClick(showInternalPageState2, showCloakedState2, showTrackingState2, showIdleState2) {
    const btn = document.getElementById("track-page-btn");
    const trackedTabId = (await readTrackedTab()).id;
    if (trackedTabId) {
      try {
        await chrome.tabs.get(trackedTabId);
        await handleStopTracking(showIdleState2);
        return;
      } catch {
      }
    }
    const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
    if (!tab)
      return;
    const outcome = await trackTab(tab);
    if (outcome === "internal_page") {
      if (btn)
        showInternalPageState2(btn);
      return;
    }
    if (outcome === "cloaked") {
      if (btn)
        showCloakedState2(btn);
      return;
    }
    if (btn)
      showTrackingState2(btn, tab.title, tab.url, tab.id);
    console.log(KABOOM_LOG_PREFIX, "Now tracking tab:", tab.id, tab.url);
  }

  // extension/popup/tabs/tab-tracking.js
  var trackingStorageSyncInstalled = false;
  var AUDIT_BUTTON_ENABLED = false;
  function hideAuditButton() {
    const trackingBarAudit = document.getElementById("tracking-bar-audit");
    if (!trackingBarAudit)
      return;
    trackingBarAudit.style.display = "none";
    trackingBarAudit.onclick = null;
  }
  function showInternalPageState(btn) {
    const trackingBar = document.getElementById("tracking-bar");
    if (trackingBar)
      trackingBar.style.display = "none";
    hideAuditButton();
    btn.disabled = true;
    btn.textContent = "Cannot Track Internal Pages";
    btn.title = "Chrome blocks extensions on internal pages like chrome:// and about:";
    Object.assign(btn.style, { opacity: "0.5", background: "#252525", color: "#888", borderColor: "#333" });
  }
  function showCloakedState(btn) {
    const trackingBar = document.getElementById("tracking-bar");
    if (trackingBar)
      trackingBar.style.display = "none";
    hideAuditButton();
    btn.disabled = true;
    btn.textContent = "Tracking Disabled on This Site";
    btn.title = "This domain is in the cloaked domains list. KaBOOM! is disabled here to prevent interference.";
    Object.assign(btn.style, { opacity: "0.5", background: "#252525", color: "#888", borderColor: "#333" });
  }
  function showTrackingState(btn, trackedTabTitle, trackedTabUrl, trackedTabId) {
    const heroEl = document.getElementById("track-hero");
    if (heroEl)
      heroEl.style.display = "none";
    const noTrackEl = document.getElementById("no-tracking-warning");
    if (noTrackEl)
      noTrackEl.style.display = "none";
    const trackingBar = document.getElementById("tracking-bar");
    const trackingBarTitle = document.getElementById("tracking-bar-title");
    const trackingBarUrl = document.getElementById("tracking-bar-url");
    const trackingBarAudit = document.getElementById("tracking-bar-audit");
    const trackingBarStop = document.getElementById("tracking-bar-stop");
    if (trackingBar)
      trackingBar.style.display = "flex";
    if (trackingBarTitle)
      trackingBarTitle.textContent = trackedTabTitle || "Tracked tab";
    if (trackingBarUrl && trackedTabUrl) {
      trackingBarUrl.textContent = trackedTabUrl;
      trackingBarUrl.onclick = () => {
        void handleUrlClick(trackedTabId);
      };
    }
    if (trackingBarAudit) {
      if (AUDIT_BUTTON_ENABLED) {
        trackingBarAudit.textContent = "Audit";
        trackingBarAudit.style.display = "inline-flex";
        trackingBarAudit.onclick = () => {
          void handleAuditClick(trackedTabUrl, trackedTabId);
        };
      } else {
        trackingBarAudit.style.display = "none";
        trackingBarAudit.onclick = null;
      }
    }
    if (trackingBarStop) {
      trackingBarStop.onclick = (e) => {
        e.stopPropagation();
        void handleStopTracking(showIdleState);
      };
    }
  }
  function showStaleState(btn, trackedTabTitle, trackedTabUrl) {
    showIdleState(btn);
    btn.textContent = "Track Current Tab";
    btn.title = "The previously tracked tab is gone. Track the current tab instead.";
    const warning = document.getElementById("no-tracking-warning");
    if (warning)
      warning.textContent = "The previously tracked tab is no longer available.";
    const identity = document.getElementById("stale-tracking-identity");
    if (identity) {
      identity.textContent = [trackedTabTitle, trackedTabUrl].filter(Boolean).join(" \u2014 ");
      identity.style.display = identity.textContent ? "block" : "none";
    }
  }
  function showIdleState(btn) {
    const heroEl = document.getElementById("track-hero");
    if (heroEl)
      heroEl.style.display = "";
    btn.textContent = "Track This Tab";
    Object.assign(btn.style, {
      background: "#1a3a5c",
      color: "#58a6ff",
      borderColor: "#58a6ff",
      fontSize: "16px",
      fontWeight: "600",
      padding: "14px 16px",
      borderWidth: "2px"
    });
    const heroDesc = document.getElementById("track-hero-desc");
    if (heroDesc)
      heroDesc.style.display = "";
    const trackingBar = document.getElementById("tracking-bar");
    if (trackingBar)
      trackingBar.style.display = "none";
    hideAuditButton();
    const noTrackEl = document.getElementById("no-tracking-warning");
    if (noTrackEl) {
      noTrackEl.style.display = "block";
      noTrackEl.textContent = "No tab tracked \u2014 data capture disabled";
    }
    const staleIdentity = document.getElementById("stale-tracking-identity");
    if (staleIdentity) {
      staleIdentity.textContent = "";
      staleIdentity.style.display = "none";
    }
  }
  function syncTrackButtonState(btn) {
    void readTrackedTab().then(({ id: trackedTabId, url: trackedTabUrl, title: trackedTabTitle }) => {
      chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
        const currentUrl = tabs?.[0]?.url;
        if (trackedTabId) {
          void chrome.tabs.get(trackedTabId).then(() => showTrackingState(btn, trackedTabTitle, trackedTabUrl, trackedTabId), () => showStaleState(btn, trackedTabTitle, trackedTabUrl));
        } else if (isInternalUrl(currentUrl)) {
          showInternalPageState(btn);
        } else {
          let hostname = "";
          try {
            hostname = currentUrl ? new URL(currentUrl).hostname : "";
          } catch {
          }
          isDomainCloaked(hostname).then((cloaked) => {
            if (cloaked) {
              showCloakedState(btn);
            } else {
              showIdleState(btn);
            }
          }).catch(() => showIdleState(btn));
        }
      });
    });
  }
  function installTrackingStorageSync(btn) {
    if (trackingStorageSyncInstalled)
      return;
    trackingStorageSyncInstalled = true;
    onStorageChanged((changes, areaName) => {
      if (areaName !== "local")
        return;
      if (!changes[StorageKey.TRACKED_TAB_ID] && !changes[StorageKey.TRACKED_TAB_URL] && !changes[StorageKey.TRACKED_TAB_TITLE])
        return;
      syncTrackButtonState(btn);
    });
  }
  function initTrackPageButton() {
    const btn = document.getElementById("track-page-btn");
    if (!btn)
      return;
    syncTrackButtonState(btn);
    installTrackingStorageSync(btn);
    btn.addEventListener("click", () => {
      void handleTrackPageClick(showInternalPageState, showCloakedState, showTrackingState, showIdleState);
    });
  }
  async function handleTrackPageClick2() {
    return handleTrackPageClick(showInternalPageState, showCloakedState, showTrackingState, showIdleState);
  }

  // extension/popup/ai-web-pilot.js
  function applyAiWebPilotToggle(value) {
    const toggle = document.getElementById("aiWebPilotEnabled");
    if (!toggle)
      return;
    toggle.checked = value !== false;
    toggle.addEventListener("change", () => {
      handleAiWebPilotToggle(toggle.checked);
    });
  }
  async function handleAiWebPilotToggle(enabled) {
    chrome.runtime.sendMessage({ type: "set_ai_web_pilot_enabled", enabled }, (response) => {
      if (!response || !response.success) {
        console.error("[KaBOOM!] Failed to set AI Web Pilot toggle in background");
        const toggle = document.getElementById("aiWebPilotEnabled");
        if (toggle) {
          toggle.checked = !enabled;
        }
      }
    });
  }

  // extension/popup/shell/logo-motion.js
  function initPopupLogoMotion() {
    const logo = document.querySelector(".logo");
    if (!logo)
      return;
    logo.src = chrome.runtime.getURL("icons/icon.svg");
  }

  // extension/popup/settings.js
  function handleWebSocketModeChange(mode) {
    persist(setLocal(StorageKey.WEBSOCKET_CAPTURE_MODE, mode), "websocket-capture-mode");
    chrome.runtime.sendMessage({ type: SettingName.WEBSOCKET_CAPTURE_MODE, mode });
  }
  function applyWebSocketMode(value) {
    const modeSelect = document.getElementById("ws-mode");
    if (!modeSelect)
      return;
    modeSelect.value = value || "medium";
  }
  var clearConfirmPending = false;
  var clearConfirmTimer = null;
  function resetClearConfirm() {
    clearConfirmPending = false;
    if (clearConfirmTimer) {
      clearTimeout(clearConfirmTimer);
      clearConfirmTimer = null;
    }
  }
  async function handleClearLogs() {
    const clearBtn = document.getElementById("clear-btn");
    const entriesEl = document.getElementById("entries-count");
    if (clearBtn && !clearConfirmPending) {
      clearConfirmPending = true;
      clearBtn.textContent = "Confirm Clear?";
      clearConfirmTimer = setTimeout(() => {
        clearConfirmPending = false;
        if (clearBtn)
          clearBtn.textContent = "Clear Logs";
      }, 3e3);
      return Promise.resolve(null);
    }
    clearConfirmPending = false;
    if (clearConfirmTimer) {
      clearTimeout(clearConfirmTimer);
      clearConfirmTimer = null;
    }
    if (clearBtn) {
      clearBtn.disabled = true;
      clearBtn.textContent = "Clearing...";
    }
    return new Promise((resolve) => {
      chrome.runtime.sendMessage({ type: "clear_logs" }, (response) => {
        if (clearBtn) {
          clearBtn.disabled = false;
          clearBtn.textContent = "Clear Logs";
        }
        if (response?.success) {
          if (entriesEl) {
            entriesEl.textContent = "0 / 1000";
          }
        } else if (response?.error) {
          const errorEl = document.getElementById("error-message");
          if (errorEl) {
            errorEl.textContent = response.error;
          }
        }
        resolve(response || null);
      });
    });
  }

  // extension/popup.js
  void getLocal("theme").then((value) => {
    if (value === "light")
      document.body.classList.add("light-theme");
  });
  var DEFAULT_MAX_ENTRIES2 = 1e3;
  var RESHOW_TRACKED_HOVER_LAUNCHER_MESSAGE = {
    type: RuntimeMessageName.SHOW_TRACKED_HOVER_LAUNCHER
  };
  var terminalTargetTabId;
  function handleOpenTerminalClick() {
    if (typeof terminalTargetTabId !== "number")
      return;
    void chrome.runtime.sendMessage({ type: "open_terminal_panel", tab_id: terminalTargetTabId });
  }
  function bindToggleVisibility(toggle, target, isVisible) {
    target.style.display = isVisible() ? "block" : "none";
    toggle.addEventListener("change", () => {
      target.style.display = isVisible() ? "block" : "none";
    });
  }
  function setupWebSocketUI() {
    const wsToggle = document.getElementById("toggle-websocket");
    const wsModeContainer = document.getElementById("ws-mode-container");
    if (wsToggle && wsModeContainer) {
      bindToggleVisibility(wsToggle, wsModeContainer, () => wsToggle.checked);
    }
    const wsModeSelect = document.getElementById("ws-mode");
    if (wsModeSelect) {
      wsModeSelect.addEventListener("change", (e) => {
        handleWebSocketModeChange(e.target.value);
      });
    }
    const wsMessagesWarning = document.getElementById("ws-messages-warning");
    if (wsModeSelect && wsMessagesWarning) {
      bindToggleVisibility(wsModeSelect, wsMessagesWarning, () => wsModeSelect.value === "all");
    }
  }
  function setupToggleWarnings() {
    const toggleWarnings = [
      { toggleId: "toggle-screenshot", warningId: "screenshot-warning" },
      { toggleId: "toggle-network-waterfall", warningId: "waterfall-warning" },
      { toggleId: "toggle-performance-marks", warningId: "perfmarks-warning" }
    ];
    for (const { toggleId, warningId } of toggleWarnings) {
      const toggle = document.getElementById(toggleId);
      const warning = document.getElementById(warningId);
      if (toggle && warning) {
        warning.style.display = toggle.checked ? "block" : "none";
        toggle.addEventListener("change", () => {
          warning.style.display = toggle.checked ? "block" : "none";
        });
      }
    }
  }
  function requestTrackedHoverLauncherReshow() {
    if (!chrome.tabs?.query || !chrome.tabs?.sendMessage)
      return;
    chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
      const tabId = tabs[0]?.id;
      if (!tabId)
        return;
      chrome.tabs.sendMessage(tabId, RESHOW_TRACKED_HOVER_LAUNCHER_MESSAGE, () => {
        void chrome.runtime.lastError;
      });
    });
  }
  function cacheStatus(status) {
    persist(setSession(StorageKey.POPUP_LAST_STATUS, status), "popup-last-status");
  }
  function renderPopupStatus(status, shouldCache = true) {
    updateConnectionStatus(status);
    if (shouldCache)
      cacheStatus(status);
    void refreshSystemDoctor(status);
  }
  function initPopup() {
    setupRecordingUI();
    setupActionRecordingUI();
    initTrackPageButton();
    setupWebSocketUI();
    setupToggleWarnings();
    const clearBtn = document.getElementById("clear-btn");
    if (clearBtn)
      clearBtn.addEventListener("click", handleClearLogs);
    const openTerminalBtn = document.getElementById("open-terminal-btn");
    if (openTerminalBtn) {
      openTerminalBtn.addEventListener("click", handleOpenTerminalClick);
      chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
        terminalTargetTabId = tabs[0]?.id;
      });
    }
    chrome.runtime.onMessage.addListener((message) => {
      if (message.type === "status_update" && message.status) {
        renderPopupStatus(message.status);
      }
    });
    void getSession(StorageKey.POPUP_LAST_STATUS).then((value) => {
      const cached = value;
      if (cached)
        renderPopupStatus(cached, false);
    });
    try {
      chrome.runtime.sendMessage({ type: "get_status" }, (status) => {
        if (chrome.runtime.lastError) {
          renderPopupStatus({
            connected: false,
            entries: 0,
            maxEntries: DEFAULT_MAX_ENTRIES2,
            errorCount: 0,
            logFile: "",
            error: "Extension restarting \u2014 please wait a moment and reopen popup"
          });
          return;
        }
        if (status) {
          renderPopupStatus(status);
        }
      });
    } catch {
      renderPopupStatus({
        connected: false,
        entries: 0,
        maxEntries: DEFAULT_MAX_ENTRIES2,
        errorCount: 0,
        logFile: "",
        error: "Extension error \u2014 try reloading the extension"
      });
    }
    const toggleKeys = FEATURE_TOGGLES.map((t) => t.storageKey);
    const allKeys = [...toggleKeys, StorageKey.WEBSOCKET_CAPTURE_MODE, StorageKey.AI_WEB_PILOT_ENABLED];
    void getLocals(allKeys).then((result) => {
      applyFeatureToggles(result);
      applyWebSocketMode(result[StorageKey.WEBSOCKET_CAPTURE_MODE]);
      applyAiWebPilotToggle(result[StorageKey.AI_WEB_PILOT_ENABLED]);
    });
    const deferredInit = () => {
      initPopupLogoMotion();
      setupDrawModeButton();
      requestTrackedHoverLauncherReshow();
    };
    if (typeof requestAnimationFrame === "function") {
      requestAnimationFrame(deferredInit);
    } else {
      deferredInit();
    }
  }
  if (typeof document !== "undefined" && typeof globalThis.process === "undefined") {
    if (document.readyState === "loading") {
      document.addEventListener("DOMContentLoaded", initPopup);
    } else {
      initPopup();
    }
  }
})();
//# sourceMappingURL=popup.bundled.js.map
