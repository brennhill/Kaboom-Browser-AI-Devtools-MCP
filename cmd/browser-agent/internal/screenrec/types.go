// Purpose: Shared video-recording constants and data types used by recording handlers and file endpoints.
// Why: Keeps durable contracts (state names, metadata schema, command timeouts) centralized.
// Docs: docs/features/feature/tab-recording/index.md

package screenrec

import "time"

var MaxUploadSizeBytes int64 = 1 << 30 // 1 GiB

const (
	recordingStateIdle            = "idle"
	recordingStateAwaitingGesture = "awaiting_user_gesture"
	recordingStateRecording       = "recording"
	recordingStateStopping        = "stopping"

	recordStartCommandTimeout = 2 * time.Minute
	recordStopCommandTimeout  = 90 * time.Second
)

// State tracks interact(screen_recording_start/screen_recording_stop) lifecycle.
type State struct {
	State              string
	StartCorrelationID string
	StopCorrelationID  string
	UpdatedAt          time.Time
}

// Metadata is the sidecar JSON written next to each .webm file.
type Metadata struct {
	Name            string `json:"name"`
	DisplayName     string `json:"display_name"`
	CreatedAt       string `json:"created_at"`
	DurationSeconds int    `json:"duration_seconds"`
	SizeBytes       int64  `json:"size_bytes"`
	URL             string `json:"url"`
	TabID           int    `json:"tab_id"`
	Resolution      string `json:"resolution"`
	Format          string `json:"format"`
	FPS             int    `json:"fps"`
	HasAudio        bool   `json:"has_audio,omitempty"`
	AudioMode       string `json:"audio_mode,omitempty"`
	Truncated       bool   `json:"truncated,omitempty"`
}
