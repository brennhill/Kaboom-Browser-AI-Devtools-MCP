// bridge_fastpath.go -- Answers initialize/tools/list/resources directly in the
// bridge, before a daemon exists, and records what those answers cost.
// Why: fast start — a client must get a usable handshake while the daemon is
// still spawning. The telemetry counters live here rather than in their own file
// because they have exactly one producer (handleFastPath) and exist only to
// explain its behaviour; the split implied a reusable subsystem that never was.
// Docs: docs/features/feature/lazy-server-start/index.md

package bridge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	internbridge "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/bridge"
	statecfg "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// fastPathResponses maps MCP methods to their static JSON result bodies.
// Methods in this map are handled without waiting for the daemon.
var fastPathResponses = map[string]string{
	"ping":         `{}`,
	"prompts/list": `{"prompts":[]}`,
}

// sendFastResponse marshals and sends a JSON-RPC response for the fast path.
func (r *Runner) sendFastResponse(id any, result json.RawMessage, framing internbridge.StdioFraming) {
	resp := mcp.JSONRPCResponse{JSONRPC: mcp.JSONRPCVersion, ID: id, Result: result}
	// Error impossible: simple struct with no circular refs or unsupported types
	respJSON, _ := json.Marshal(resp)
	r.transport.Write(respJSON, framing)
}

func (r *Runner) sendFastError(id any, code int, message string, framing internbridge.StdioFraming) {
	resp := mcp.JSONRPCResponse{
		JSONRPC: mcp.JSONRPCVersion,
		ID:      id,
		Error:   &mcp.JSONRPCError{Code: code, Message: message},
	}
	respJSON, _ := json.Marshal(resp)
	r.transport.Write(respJSON, framing)
}

// handleFastPath handles MCP methods that don't require the daemon.
// Returns true if the method was handled.
func (r *Runner) handleFastPath(req mcp.JSONRPCRequest, toolsList []mcp.MCPTool, framing internbridge.StdioFraming) bool {
	if req.HasInvalidID() {
		r.sendBridgeError(nil, -32600, "Invalid Request: id must be string or number when present", framing)
		return true
	}

	// JSON-RPC notifications are fire-and-forget; never respond on stdio.
	if !req.HasID() {
		return true
	}

	switch req.Method {
	case "initialize":
		// Extract client capabilities for push delivery pipeline
		caps := r.protocol.ExtractCapabilities(req.Params)
		r.protocol.SetCapabilities(caps)
		r.protocol.StoreFraming(framing)

		result := map[string]any{
			"protocolVersion": r.protocol.NegotiateVersion(req.Params),
			"serverInfo":      map[string]any{"name": r.identity.ServerName, "version": r.identity.Version},
			"capabilities":    map[string]any{"tools": map[string]any{}, "resources": map[string]any{}},
			"instructions":    r.identity.ServerInstructions,
		}
		// Error impossible: map contains only primitive types and nested maps
		resultJSON, _ := json.Marshal(result)
		r.sendFastResponse(req.ID, resultJSON, framing)
		r.recordFastPathEvent(req.Method, true, 0)
		return true

	case "initialized":
		if req.HasID() {
			r.sendFastResponse(req.ID, json.RawMessage(`{}`), framing)
			r.recordFastPathEvent(req.Method, true, 0)
		}
		return true

	case "tools/list":
		result := map[string]any{"tools": toolsList}
		// Error impossible: map contains only serializable tool definitions
		resultJSON, _ := json.Marshal(result)
		r.sendFastResponse(req.ID, resultJSON, framing)
		r.recordFastPathEvent(req.Method, true, 0)
		return true

	case "resources/list":
		result := mcp.MCPResourcesListResult{Resources: r.protocol.Resources()}
		resultJSON, _ := json.Marshal(result)
		r.sendFastResponse(req.ID, resultJSON, framing)
		return true
	case "resources/templates/list":
		result := mcp.MCPResourceTemplatesListResult{ResourceTemplates: r.protocol.ResourceTemplates()}
		resultJSON, _ := json.Marshal(result)
		r.sendFastResponse(req.ID, resultJSON, framing)
		return true
	case "resources/read":
		var params struct {
			URI string `json:"uri"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			r.recordFastPathResourceRead("", false, -32602)
			r.recordFastPathEvent(req.Method, false, -32602)
			r.sendFastError(req.ID, -32602, "Invalid params: "+err.Error(), framing)
			return true
		}
		canonicalURI, text, ok := r.protocol.ResolveResource(params.URI)
		if !ok {
			r.recordFastPathResourceRead(params.URI, false, -32002)
			r.recordFastPathEvent(req.Method, false, -32002)
			r.sendFastError(req.ID, -32002, "Resource not found: "+params.URI, framing)
			return true
		}
		r.recordFastPathResourceRead(params.URI, true, 0)
		r.recordFastPathEvent(req.Method, true, 0)
		result := map[string]any{
			"contents": []map[string]any{
				{
					"uri":      canonicalURI,
					"mimeType": "text/markdown",
					"text":     text,
				},
			},
		}
		resultJSON, _ := json.Marshal(result)
		r.sendFastResponse(req.ID, resultJSON, framing)
		return true
	}

	if staticResult, ok := fastPathResponses[req.Method]; ok {
		r.sendFastResponse(req.ID, json.RawMessage(staticResult), framing)
		r.recordFastPathEvent(req.Method, true, 0)
		return true
	}

	return false
}

type bridgeFastPathResourceReadCounters struct {
	mu      sync.Mutex
	success int64
	failure int64
}

var fastPathResourceReadCounters bridgeFastPathResourceReadCounters

const fastPathTelemetryQueueCapacity = 256

type fastPathTelemetryRecord struct {
	path string
	line []byte
}

var (
	fastPathTelemetryOnce    sync.Once
	fastPathTelemetryQueue   = make(chan fastPathTelemetryRecord, fastPathTelemetryQueueCapacity)
	fastPathTelemetryPending sync.WaitGroup
)

func startFastPathTelemetryWorker() {
	util.SafeGo(func() {
		for record := range fastPathTelemetryQueue {
			appendFastPathTelemetryRecord(record)
			fastPathTelemetryPending.Done()
		}
	})
}

func enqueueFastPathTelemetry(path string, line []byte) {
	fastPathTelemetryOnce.Do(startFastPathTelemetryWorker)
	record := fastPathTelemetryRecord{path: path, line: line}
	fastPathTelemetryPending.Add(1)
	select {
	case fastPathTelemetryQueue <- record:
	default:
		fastPathTelemetryPending.Done()
	}
}

func appendFastPathTelemetryRecord(record fastPathTelemetryRecord) {
	if err := os.MkdirAll(filepath.Dir(record.path), 0o750); err != nil {
		return
	}
	// #nosec G304 -- paths are deterministic under the local state root.
	f, err := os.OpenFile(record.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	_, _ = f.Write(append(record.line, '\n'))
	_ = f.Close()
}

// FlushFastPathTelemetry waits for telemetry accepted by the bounded queue.
// Bridge shutdown calls it after the request loop, never on the request path.
func FlushFastPathTelemetry() {
	fastPathTelemetryPending.Wait()
}

// ResetFastPathResourceReadCounters resets the resource read telemetry counters.
func ResetFastPathResourceReadCounters() {
	fastPathResourceReadCounters.mu.Lock()
	defer fastPathResourceReadCounters.mu.Unlock()
	fastPathResourceReadCounters.success = 0
	fastPathResourceReadCounters.failure = 0
}

// RecordFastPathResourceRead is an exported wrapper for external callers.
func (r *Runner) RecordFastPathResourceRead(uri string, success bool, errorCode int) {
	r.recordFastPathResourceRead(uri, success, errorCode)
}

func (r *Runner) recordFastPathResourceRead(uri string, success bool, errorCode int) {
	fastPathResourceReadCounters.mu.Lock()
	if success {
		fastPathResourceReadCounters.success++
	} else {
		fastPathResourceReadCounters.failure++
	}
	successCount := fastPathResourceReadCounters.success
	failureCount := fastPathResourceReadCounters.failure
	fastPathResourceReadCounters.mu.Unlock()
	r.appendFastPathResourceReadTelemetry(uri, success, errorCode, successCount, failureCount)
}

// SnapshotFastPathResourceReadCounters returns the current success/failure counts.
func SnapshotFastPathResourceReadCounters() (success int64, failure int64) {
	fastPathResourceReadCounters.mu.Lock()
	defer fastPathResourceReadCounters.mu.Unlock()
	return fastPathResourceReadCounters.success, fastPathResourceReadCounters.failure
}

// FastPathResourceReadLogPath returns the log path for resource read telemetry.
func FastPathResourceReadLogPath() (string, error) {
	return statecfg.InRoot("logs", "bridge-fastpath-resource-read.jsonl")
}

func (r *Runner) appendFastPathResourceReadTelemetry(uri string, success bool, errorCode int, successCount int64, failureCount int64) {
	path, err := FastPathResourceReadLogPath()
	if err != nil {
		return
	}
	entry := map[string]any{
		"timestamp":      time.Now().UTC().Format(time.RFC3339Nano),
		"event":          "bridge_fastpath_resources_read",
		"uri":            uri,
		"success":        success,
		"error_code":     errorCode,
		"success_count":  successCount,
		"failure_count":  failureCount,
		"pid":            os.Getpid(),
		"bridge_version": r.identity.Version,
	}
	line, marshalErr := json.Marshal(entry)
	if marshalErr != nil {
		return
	}
	enqueueFastPathTelemetry(path, line)
}

type bridgeFastPathCounters struct {
	mu      sync.Mutex
	success int
	failure int
}

var fastPathCounters bridgeFastPathCounters

// ResetFastPathCounters resets the fast-path event counters.
func ResetFastPathCounters() {
	fastPathCounters.mu.Lock()
	defer fastPathCounters.mu.Unlock()
	fastPathCounters.success = 0
	fastPathCounters.failure = 0
}

// FastPathTelemetryLogPath returns the log path for fast-path event telemetry.
func FastPathTelemetryLogPath() (string, error) {
	return statecfg.InRoot("logs", "bridge-fastpath-events.jsonl")
}

// RecordFastPathEvent is an exported wrapper for external callers.
func (r *Runner) RecordFastPathEvent(method string, success bool, errorCode int) {
	r.recordFastPathEvent(method, success, errorCode)
}

func (r *Runner) recordFastPathEvent(method string, success bool, errorCode int) {
	successCount, failureCount := func() (int, int) {
		fastPathCounters.mu.Lock()
		defer fastPathCounters.mu.Unlock()
		if success {
			fastPathCounters.success++
		} else {
			fastPathCounters.failure++
		}
		return fastPathCounters.success, fastPathCounters.failure
	}()

	path, err := FastPathTelemetryLogPath()
	if err != nil {
		return
	}
	event := map[string]any{
		"timestamp":     time.Now().UTC().Format(time.RFC3339Nano),
		"event":         "bridge_fastpath_method",
		"method":        method,
		"success":       success,
		"error_code":    errorCode,
		"success_count": successCount,
		"failure_count": failureCount,
		"pid":           os.Getpid(),
		"version":       r.identity.Version,
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return
	}
	enqueueFastPathTelemetry(path, payload)
}
