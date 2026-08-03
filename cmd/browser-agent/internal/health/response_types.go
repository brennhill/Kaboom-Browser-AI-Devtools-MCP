// response_types.go — Defines stable schemas returned by get_health.
// Why: Keeps response contracts centralized and separate from runtime collection logic.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package health

// MCPHealthResponse is the response structure for the get_health MCP tool.
// Named to distinguish from the simpler HealthResponse used by /health HTTP endpoint.
type MCPHealthResponse struct {
	Server           ServerInfo                  `json:"server"`
	Memory           MemoryInfo                  `json:"memory"`
	Buffers          BuffersInfo                 `json:"buffers"`
	RateLimiting     RateLimitingInfo            `json:"rate_limiting"`
	Audit            AuditInfo                   `json:"audit"`
	Pilot            PilotInfo                   `json:"pilot"`
	CommandExecution CommandExecutionInfo        `json:"command_execution"`
	ResourcePressure map[string]ResourcePressure `json:"resource_pressure"`
	Upgrade          *UpgradeInfo                `json:"upgrade,omitempty"`
}

// ResourcePressure is the common machine-readable budget for retained state.
type ResourcePressure struct {
	Entries            int   `json:"entries"`
	Capacity           int   `json:"capacity"`
	DroppedCount       int64 `json:"dropped_count"`
	OldestAgeMs        int64 `json:"oldest_age_ms"`
	ActiveEntries      int   `json:"active_entries"`
	RecoverableEntries int   `json:"recoverable_entries"`
	RetainedBytes      int64 `json:"retained_bytes"`
	ByteCapacity       int64 `json:"byte_capacity"`
}

// UpgradeInfo contains binary upgrade detection state.
type UpgradeInfo struct {
	Pending    bool   `json:"pending"`
	NewVersion string `json:"new_version"`
	DetectedAt string `json:"detected_at"`
}

// ServerInfo contains server identification and uptime.
type ServerInfo struct {
	Version          string  `json:"version"`
	UptimeSeconds    float64 `json:"uptime_seconds"`
	PID              int     `json:"pid"`
	Platform         string  `json:"platform"`
	GoVersion        string  `json:"go_version"`
	LaunchMode       string  `json:"launch_mode,omitempty"`
	LaunchModeReason string  `json:"launch_mode_reason,omitempty"`
	ParentProcess    string  `json:"parent_process,omitempty"`
	TerminalPort     int     `json:"terminal_port,omitempty"` // 0 = terminal server not running
}

// MemoryInfo contains memory usage statistics.
type MemoryInfo struct {
	CurrentMB       float64               `json:"current_mb"`
	AllocMB         float64               `json:"alloc_mb"`
	SysMB           float64               `json:"sys_mb"`
	BufferBreakdown BufferMemoryBreakdown `json:"buffer_breakdown"`
}

// BufferMemoryBreakdown shows memory usage per buffer type.
type BufferMemoryBreakdown struct {
	WebSocketBytes int64 `json:"websocket_bytes"`
	NetworkBytes   int64 `json:"network_bytes"`
	ActionsBytes   int64 `json:"actions_bytes"`
}

// BuffersInfo contains buffer utilization statistics.
type BuffersInfo struct {
	Console   BufferStats `json:"console"`
	Network   BufferStats `json:"network"`
	WebSocket BufferStats `json:"websocket"`
	Actions   BufferStats `json:"actions"`
}

// BufferStats contains statistics for a single buffer.
type BufferStats struct {
	Entries        int     `json:"entries"`
	Capacity       int     `json:"capacity"`
	UtilizationPct float64 `json:"utilization_pct"`
	DroppedCount   int64   `json:"dropped_count"`
}

// RateLimitingInfo contains rate limiting state.
type RateLimitingInfo struct {
	CurrentRate   int    `json:"current_rate"`
	Threshold     int    `json:"threshold"`
	CircuitOpen   bool   `json:"circuit_open"`
	CircuitReason string `json:"circuit_reason,omitempty"`
	Total429s     int    `json:"total_429s"`
}

// AuditInfo contains tool invocation statistics.
type AuditInfo struct {
	TotalCalls   int64            `json:"total_calls"`
	TotalErrors  int64            `json:"total_errors"`
	ErrorRatePct float64          `json:"error_rate_pct"`
	CallsPerTool map[string]int64 `json:"calls_per_tool"`
}

// CommandExecutionInfo summarizes async command execution reliability.
type CommandExecutionInfo struct {
	Ready                bool    `json:"ready"`
	Status               string  `json:"status"` // "pass", "warn", "fail"
	Detail               string  `json:"detail"`
	WindowSeconds        int     `json:"window_seconds"`
	QueueDepth           int     `json:"queue_depth"`
	PendingCount         int     `json:"pending_count"`
	OldestPendingAgeMs   int64   `json:"oldest_pending_age_ms,omitempty"`
	RecentSuccessCount   int     `json:"recent_success_count"`
	RecentFailedCount    int     `json:"recent_failed_count"`
	RecentExpiredCount   int     `json:"recent_expired_count"`
	RecentTimeoutCount   int     `json:"recent_timeout_count"`
	RecentErrorCount     int     `json:"recent_error_count"`
	RecentCancelledCount int     `json:"recent_cancelled_count"`
	RecentFailureRatePct float64 `json:"recent_failure_rate_pct"`
	LastSuccessAt        string  `json:"last_success_at,omitempty"`
	LastSuccessAgeMs     int64   `json:"last_success_age_ms,omitempty"`
}

// PilotInfo contains AI Web Pilot toggle state and connection status.
type PilotInfo struct {
	Enabled            bool   `json:"enabled"`
	Source             string `json:"source"` // "extension_poll", "stale", or "never_connected"
	ExtensionConnected bool   `json:"extension_connected"`
	LastPollAgo        string `json:"last_poll_ago,omitempty"`
}
