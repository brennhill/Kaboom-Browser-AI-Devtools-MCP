// wire_sync.go — Defines the canonical extension-daemon /sync wire contract.

package capture

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// SyncRequest is the extension heartbeat and command-lifecycle payload.
type SyncRequest struct {
	ExtSessionID         string               `json:"ext_session_id"`
	ConnectionGeneration uint64               `json:"connection_generation,omitempty"`
	ExtensionVersion     string               `json:"extension_version,omitempty"`
	Settings             *SyncSettings        `json:"settings,omitempty"`
	ExtensionLogs        []types.ExtensionLog `json:"extension_logs,omitempty"`
	LastCommandAck       string               `json:"last_command_ack,omitempty"`
	CommandResults       []SyncCommandResult  `json:"command_results,omitempty"`
	InProgress           []SyncInProgress     `json:"in_progress,omitempty"`
	FeaturesUsed         *SyncFeaturesUsed    `json:"features_used,omitempty"`
}

// SyncSettings contains extension settings sent with a heartbeat.
type SyncSettings struct {
	PilotEnabled     bool   `json:"pilot_enabled"`
	TrackingEnabled  bool   `json:"tracking_enabled"`
	TrackedTabID     int    `json:"tracked_tab_id"`
	TrackedTabURL    string `json:"tracked_tab_url"`
	TrackedTabTitle  string `json:"tracked_tab_title"`
	TabStatus        string `json:"tab_status,omitempty"`
	TrackedTabActive *bool  `json:"tracked_tab_active,omitempty"`
	CaptureLogs      bool   `json:"capture_logs"`
	CaptureNetwork   bool   `json:"capture_network"`
	CaptureWebSocket bool   `json:"capture_websocket"`
	CaptureActions   bool   `json:"capture_actions"`
	CspRestricted    bool   `json:"csp_restricted"`
	CspLevel         string `json:"csp_level"`
}

// SyncCommandResult is a terminal command outcome returned by the extension.
type SyncCommandResult struct {
	ID                   string          `json:"id"`
	CorrelationID        string          `json:"correlation_id,omitempty"`
	ConnectionGeneration uint64          `json:"connection_generation,omitempty"`
	Status               string          `json:"status"`
	Result               json.RawMessage `json:"result,omitempty"`
	Error                string          `json:"error,omitempty"`
}

// SyncInProgress is active extension command execution state.
type SyncInProgress struct {
	ID                   string   `json:"id"`
	CorrelationID        string   `json:"correlation_id,omitempty"`
	ConnectionGeneration uint64   `json:"connection_generation"`
	Type                 string   `json:"type,omitempty"`
	Status               string   `json:"status,omitempty"`
	ProgressPct          *float64 `json:"progress_pct,omitempty"`
	StartedAt            string   `json:"started_at,omitempty"`
	UpdatedAt            string   `json:"updated_at,omitempty"`
}

// SyncFeaturesUsed is the bounded UI-originated feature telemetry schema.
type SyncFeaturesUsed struct {
	Screenshot      bool `json:"screenshot,omitempty"`
	Annotations     bool `json:"annotations,omitempty"`
	Video           bool `json:"video,omitempty"`
	DOMAction       bool `json:"dom_action,omitempty"`
	ActionRecording bool `json:"action_recording,omitempty"`
}

// SyncResponse is the daemon heartbeat response and command batch.
type SyncResponse struct {
	Ack                  bool              `json:"ack"`
	ConnectionGeneration uint64            `json:"connection_generation"`
	Commands             []SyncCommand     `json:"commands"`
	NextPollMs           int               `json:"next_poll_ms"`
	ServerTime           string            `json:"server_time"`
	ServerVersion        string            `json:"server_version,omitempty"`
	InstallID            string            `json:"install_id,omitempty"`
	CaptureOverrides     map[string]string `json:"capture_overrides"`
}

// SyncCommand is one daemon command delivered to the extension.
type SyncCommand struct {
	ID                   string          `json:"id"`
	Type                 string          `json:"type"`
	Params               json.RawMessage `json:"params"`
	TabID                int             `json:"tab_id,omitempty"`
	CorrelationID        string          `json:"correlation_id,omitempty"`
	TraceID              string          `json:"trace_id,omitempty"`
	ConnectionGeneration uint64          `json:"connection_generation"`
}
