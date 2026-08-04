// wire_trace.go — HTTP contracts for local Chrome performance trace streaming.

package perftrace

import "encoding/json"

type WirePerformanceTraceStartRequest struct {
	TabID int `json:"tab_id"`
}

type WirePerformanceTraceStartResponse struct {
	TraceID string `json:"trace_id"`
}

type WirePerformanceTraceChunkRequest struct {
	TraceID  string            `json:"trace_id"`
	Sequence int               `json:"sequence"`
	Events   []json.RawMessage `json:"events"`
}

type WirePerformanceTraceFinishRequest struct {
	TraceID string `json:"trace_id"`
}

type WirePerformanceTraceResult struct {
	TraceID      string `json:"trace_id"`
	ArtifactPath string `json:"artifact_path"`
	EventCount   int64  `json:"event_count"`
	ChunkCount   int    `json:"chunk_count"`
	Bytes        int64  `json:"bytes"`
}

type WirePerformanceTraceAbortRequest struct {
	TraceID string `json:"trace_id"`
}
