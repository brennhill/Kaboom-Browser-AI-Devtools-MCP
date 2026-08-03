"use strict";
(() => {
  // extension/lib/error-utils.js
  function errorMessage(err, fallback = "Unknown error") {
    if (err instanceof Error && err.message)
      return err.message;
    if (typeof err === "string" && err)
      return err;
    return fallback;
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

  // extension/lib/brand.js
  var KABOOM_RECORDING_LOG_PREFIX = "[KaBOOM! REC]";

  // extension/offscreen/recording-worker.js
  var MAX_RECORDING_BYTES = 1024 * 1024 * 1024;
  var defaultState = {
    active: false,
    name: "",
    startTime: 0,
    serverUrl: "",
    fps: 15,
    audioMode: "",
    tabId: 0,
    url: "",
    recorder: null,
    stream: null,
    chunks: [],
    totalBytes: 0
  };
  var state = { ...defaultState };
  var LOG = `${KABOOM_RECORDING_LOG_PREFIX} offscreen`;
  function reportBackgroundDeliveryFailure(transition) {
    console.error(LOG, `Background did not receive recording transition: ${transition}`);
  }
  async function handleStartRecording(msg) {
    console.log(LOG, "handleStartRecording", {
      name: msg.name,
      audioMode: msg.audioMode,
      fps: msg.fps,
      tabId: msg.tabId,
      streamId: msg.streamId?.substring(0, 20) + "...",
      currentlyActive: state.active
    });
    if (state.active) {
      console.warn(LOG, "START BLOCKED: already recording");
      chrome.runtime.sendMessage({
        target: "background",
        type: "offscreen_recording_started",
        success: false,
        connection_generation: msg.connection_generation,
        error: "RECORD_START: Already recording in offscreen document."
      }).catch(() => reportBackgroundDeliveryFailure("start_already_active"));
      return;
    }
    state.active = true;
    const acquiredStreams = [];
    try {
      const fps = Math.max(5, Math.min(60, msg.fps));
      const hasTabAudio = msg.audioMode === "tab" || msg.audioMode === "both";
      const hasMicAudio = msg.audioMode === "mic" || msg.audioMode === "both";
      const hasAnyAudio = hasTabAudio || hasMicAudio;
      const tabConstraints = {
        video: {
          // @ts-expect-error -- Chrome-specific mandatory constraints for tab capture
          mandatory: {
            chromeMediaSource: "tab",
            chromeMediaSourceId: msg.streamId,
            minWidth: 1280,
            minHeight: 720,
            maxWidth: 1920,
            maxHeight: 1080,
            maxFrameRate: fps
          }
        },
        audio: hasTabAudio ? {
          // @ts-expect-error -- Chrome-specific mandatory constraints for tab audio
          mandatory: {
            chromeMediaSource: "tab",
            chromeMediaSourceId: msg.streamId
          }
        } : false
      };
      console.log(LOG, "Calling getUserMedia for tab stream", { hasTabAudio, hasMicAudio });
      const tabStream = await navigator.mediaDevices.getUserMedia(tabConstraints);
      acquiredStreams.push(tabStream);
      console.log(LOG, "Got tab stream", {
        videoTracks: tabStream.getVideoTracks().length,
        audioTracks: tabStream.getAudioTracks().length
      });
      let stream;
      if (hasMicAudio && hasTabAudio) {
        const micStream = await navigator.mediaDevices.getUserMedia({
          audio: { echoCancellation: false, noiseSuppression: true, autoGainControl: true }
        });
        acquiredStreams.push(micStream);
        const audioCtx = new AudioContext();
        const tabSource = audioCtx.createMediaStreamSource(new MediaStream(tabStream.getAudioTracks()));
        const micSource = audioCtx.createMediaStreamSource(micStream);
        const dest = audioCtx.createMediaStreamDestination();
        tabSource.connect(dest);
        micSource.connect(dest);
        stream = new MediaStream([...tabStream.getVideoTracks(), ...dest.stream.getAudioTracks()]);
      } else if (hasMicAudio) {
        const micStream = await navigator.mediaDevices.getUserMedia({
          audio: { echoCancellation: true, noiseSuppression: true, autoGainControl: true }
        });
        acquiredStreams.push(micStream);
        stream = new MediaStream([...tabStream.getVideoTracks(), ...micStream.getAudioTracks()]);
      } else {
        stream = tabStream;
      }
      const bitrate = Math.round(fps / 15 * 5e5);
      const mimeType = hasAnyAudio ? "video/webm;codecs=vp8,opus" : "video/webm;codecs=vp8";
      const recorderOptions = {
        mimeType,
        videoBitsPerSecond: bitrate
      };
      if (hasAnyAudio) {
        recorderOptions.audioBitsPerSecond = 128e3;
      }
      const recorder = new MediaRecorder(stream, recorderOptions);
      const chunks = [];
      let totalBytes = 0;
      let autoStopping = false;
      recorder.ondataavailable = (e) => {
        if (e.data.size > 0) {
          chunks.push(e.data);
          totalBytes += e.data.size;
          state.totalBytes = totalBytes;
          if (totalBytes >= MAX_RECORDING_BYTES && !autoStopping) {
            autoStopping = true;
            handleStopRecording(true);
          }
        }
      };
      const videoTrack = stream.getVideoTracks()[0];
      if (videoTrack) {
        videoTrack.onended = () => {
          if (state.active && !autoStopping) {
            autoStopping = true;
            handleStopRecording(true);
          }
        };
      }
      console.log(LOG, "Starting MediaRecorder", { mimeType, videoBps: bitrate, audioBps: hasAnyAudio ? 128e3 : 0 });
      recorder.start(1e3);
      state = {
        active: true,
        name: msg.name,
        startTime: Date.now(),
        serverUrl: msg.serverUrl,
        fps,
        audioMode: msg.audioMode,
        tabId: msg.tabId,
        url: msg.url,
        recorder,
        stream,
        chunks,
        totalBytes: 0,
        connectionGeneration: msg.connection_generation
      };
      console.log(LOG, "Recording STARTED, sending confirmation to background");
      chrome.runtime.sendMessage({
        target: "background",
        type: "offscreen_recording_started",
        success: true,
        connection_generation: msg.connection_generation
      }).catch(() => reportBackgroundDeliveryFailure("start_confirmed"));
    } catch (err) {
      console.error(LOG, "START EXCEPTION:", errorMessage(err), err.stack);
      for (const s of acquiredStreams) {
        console.log(LOG, "Cleaning up leaked stream, stopping", s.getTracks().length, "tracks");
        s.getTracks().forEach((t) => t.stop());
      }
      state = { ...defaultState };
      chrome.runtime.sendMessage({
        target: "background",
        type: "offscreen_recording_started",
        success: false,
        connection_generation: msg.connection_generation,
        error: `RECORD_START: ${errorMessage(err, "Failed to start recording in offscreen document.")}`
      }).catch(() => reportBackgroundDeliveryFailure("start_failed"));
    }
  }
  function handleStopRecording(truncated = false, connectionGeneration = state.connectionGeneration) {
    console.log(LOG, "handleStopRecording", {
      active: state.active,
      name: state.name,
      truncated,
      chunks: state.chunks.length,
      totalBytes: state.totalBytes,
      recorderState: state.recorder?.state
    });
    if (!state.active) {
      console.warn(LOG, "STOP: not active");
      chrome.runtime.sendMessage({
        target: "background",
        type: "offscreen_recording_stopped",
        status: "error",
        connection_generation: connectionGeneration,
        name: "",
        error: "RECORD_STOP: No active recording in offscreen document."
      }).catch(() => reportBackgroundDeliveryFailure("stop_not_active"));
      return;
    }
    const { name, startTime, recorder, stream, chunks, serverUrl } = state;
    state.active = false;
    if (!recorder || recorder.state === "inactive") {
      console.warn(LOG, "STOP: recorder null or inactive", { recorder: !!recorder, state: recorder?.state });
      if (stream) {
        stream.getTracks().forEach((t) => t.stop());
      }
      state = { ...defaultState };
      chrome.runtime.sendMessage({
        target: "background",
        type: "offscreen_recording_stopped",
        status: "error",
        connection_generation: connectionGeneration,
        name: "",
        error: "RECORD_STOP: Recorder already inactive."
      }).catch(() => reportBackgroundDeliveryFailure("stop_recorder_inactive"));
      return;
    }
    console.log(LOG, "Stopping recorder, waiting for onstop callback");
    recorder.onstop = async () => {
      try {
        const blob = new Blob(chunks, { type: "video/webm" });
        const duration = Math.round((Date.now() - startTime) / 1e3);
        console.log(LOG, "Recorder stopped, assembling blob", { chunks: chunks.length, size: blob.size, duration });
        if (stream) {
          stream.getTracks().forEach((t) => t.stop());
        }
        const displayName = name.replace(/--\d{4}-\d{2}-\d{2}-\d{4}(-\d+)?$/, "").replace(/-/g, " ");
        const hasAudio = state.audioMode === "tab" || state.audioMode === "both" || state.audioMode === "mic";
        const format = hasAudio ? "video/webm;codecs=vp8,opus" : "video/webm;codecs=vp8";
        const formData = new FormData();
        formData.append("video", blob, `${name}.webm`);
        formData.append("metadata", JSON.stringify({
          name,
          display_name: displayName,
          created_at: new Date(startTime).toISOString(),
          duration_seconds: duration,
          size_bytes: blob.size,
          url: state.url,
          tab_id: state.tabId,
          resolution: "1920x1080",
          format,
          fps: state.fps,
          has_audio: hasAudio,
          audio_mode: state.audioMode || void 0,
          truncated
        }));
        console.log(LOG, "POSTing to", `${serverUrl}/recordings/save`, { size: blob.size, hasAudio });
        const response = await fetch(`${serverUrl}/recordings/save`, {
          method: "POST",
          headers: buildDaemonHeaders({
            clientName: "kaboom-extension-offscreen",
            contentType: null
          }),
          body: formData
        });
        console.log(LOG, "Server response:", response.status);
        state = { ...defaultState };
        if (!response.ok) {
          console.error(LOG, "Server returned error:", response.status);
          chrome.runtime.sendMessage({
            target: "background",
            type: "offscreen_recording_stopped",
            status: "error",
            connection_generation: connectionGeneration,
            name,
            error: `RECORD_STOP: Server returned ${response.status}.`
          }).catch(() => reportBackgroundDeliveryFailure("save_http_failed"));
          return;
        }
        let savePath;
        try {
          const body = await response.json();
          savePath = body.path;
        } catch {
        }
        console.log(LOG, "Recording SAVED", { name, duration, size: blob.size, path: savePath });
        chrome.runtime.sendMessage({
          target: "background",
          type: "offscreen_recording_stopped",
          status: "saved",
          connection_generation: connectionGeneration,
          name,
          duration_seconds: duration,
          size_bytes: blob.size,
          truncated: truncated || void 0,
          path: savePath
        }).catch(() => reportBackgroundDeliveryFailure("save_confirmed"));
      } catch (err) {
        console.error(LOG, "SAVE EXCEPTION:", errorMessage(err), err.stack);
        state = { ...defaultState };
        chrome.runtime.sendMessage({
          target: "background",
          type: "offscreen_recording_stopped",
          status: "error",
          connection_generation: connectionGeneration,
          name,
          error: `RECORD_STOP: ${errorMessage(err, "Save failed.")}`
        }).catch(() => reportBackgroundDeliveryFailure("save_failed"));
      }
    };
    recorder.stop();
  }
  console.log(LOG, "Offscreen recording worker loaded");
  chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
    if (sender.id !== chrome.runtime.id)
      return;
    if (message.target !== "offscreen")
      return;
    console.log(LOG, "Received message:", message.type);
    if (message.type === "offscreen_start_recording") {
      handleStartRecording(message);
    } else if (message.type === "offscreen_stop_recording") {
      handleStopRecording(false, message.connection_generation);
    } else if (message.type === "offscreen_get_recording_state") {
      sendResponse({
        active: state.active,
        name: state.name,
        startTime: state.startTime,
        fps: state.fps,
        audioMode: state.audioMode,
        tabId: state.tabId,
        url: state.url
      });
    }
  });
})();
//# sourceMappingURL=offscreen.bundled.js.map
