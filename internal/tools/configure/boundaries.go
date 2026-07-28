// Purpose: Parses and validates test_boundary_start/end parameters for scoped test isolation.
// Docs: docs/features/feature/config-profiles/index.md

package configure

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// BoundaryHandler owns active test-boundary lifecycle state.
type BoundaryHandler struct {
	mu     sync.Mutex
	active map[string]time.Time
}

// NewBoundaryHandler returns an initialized boundary lifecycle owner.
func NewBoundaryHandler() *BoundaryHandler {
	return &BoundaryHandler{active: make(map[string]time.Time)}
}

// Start validates and activates a test boundary.
func (h *BoundaryHandler) Start(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	result, errResp := ParseTestBoundaryStart(req.ID, args)
	if errResp != nil {
		return *errResp
	}

	h.mu.Lock()
	h.active[result.TestID] = time.Now()
	h.mu.Unlock()
	return BuildTestBoundaryStartResponse(req.ID, result)
}

// End validates and removes an active test boundary.
func (h *BoundaryHandler) End(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	result, errResp := ParseTestBoundaryEnd(req.ID, args)
	if errResp != nil {
		return *errResp
	}

	h.mu.Lock()
	_, wasActive := h.active[result.TestID]
	if wasActive {
		delete(h.active, result.TestID)
	}
	h.mu.Unlock()
	if !wasActive {
		return mcp.Fail(req, mcp.ErrInvalidParam,
			"No active test boundary for test_id '"+result.TestID+"'",
			"Call configure({what: 'test_boundary_start', test_id: '"+result.TestID+"'}) first",
			mcp.WithParam("test_id"))
	}
	return BuildTestBoundaryEndResponse(req.ID, result, true)
}

func unmarshalBoundaryArgs(reqID any, args json.RawMessage, target any) *mcp.JSONRPCResponse {
	if len(args) == 0 {
		return nil
	}
	if err := json.Unmarshal(args, target); err != nil {
		resp := mcp.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      reqID,
			Result: mcp.StructuredErrorResponse(
				mcp.ErrInvalidJSON,
				"Invalid JSON arguments: "+err.Error(),
				"Fix JSON syntax and call again",
			),
		}
		return &resp
	}
	return nil
}

func missingTestIDResponse(reqID any) *mcp.JSONRPCResponse {
	resp := mcp.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      reqID,
		Result: mcp.StructuredErrorResponse(
			mcp.ErrMissingParam,
			"Required parameter 'test_id' is missing",
			"Add the 'test_id' parameter",
			mcp.WithParam("test_id"),
		),
	}
	return &resp
}

// TestBoundaryStartResult holds the validated parameters for a test_boundary_start request.
type TestBoundaryStartResult struct {
	TestID string
	Label  string
}

// ParseTestBoundaryStart validates test_boundary_start arguments and returns
// the result or an MCP error response. If the error response is non-nil,
// the caller should return it directly.
func ParseTestBoundaryStart(reqID any, args json.RawMessage) (*TestBoundaryStartResult, *mcp.JSONRPCResponse) {
	var params struct {
		TestID string `json:"test_id"`
		Label  string `json:"label"`
	}
	if resp := unmarshalBoundaryArgs(reqID, args, &params); resp != nil {
		return nil, resp
	}

	if params.TestID == "" {
		return nil, missingTestIDResponse(reqID)
	}

	label := params.Label
	if label == "" {
		label = "Test: " + params.TestID
	}

	return &TestBoundaryStartResult{TestID: params.TestID, Label: label}, nil
}

// BuildTestBoundaryStartResponse builds the MCP response for a validated test_boundary_start.
func BuildTestBoundaryStartResponse(reqID any, r *TestBoundaryStartResult) mcp.JSONRPCResponse {
	return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: reqID, Result: mcp.JSONResponse("Test boundary started", map[string]any{
		"status":  "ok",
		"test_id": r.TestID,
		"label":   r.Label,
		"message": "Test boundary started",
	})}
}

// TestBoundaryEndResult holds the validated parameters for a test_boundary_end request.
type TestBoundaryEndResult struct {
	TestID string
}

// ParseTestBoundaryEnd validates test_boundary_end arguments and returns
// the result or an MCP error response.
func ParseTestBoundaryEnd(reqID any, args json.RawMessage) (*TestBoundaryEndResult, *mcp.JSONRPCResponse) {
	var params struct {
		TestID string `json:"test_id"`
	}
	if resp := unmarshalBoundaryArgs(reqID, args, &params); resp != nil {
		return nil, resp
	}

	if params.TestID == "" {
		return nil, missingTestIDResponse(reqID)
	}

	return &TestBoundaryEndResult{TestID: params.TestID}, nil
}

// BuildTestBoundaryEndResponse builds the MCP response for a validated test_boundary_end.
func BuildTestBoundaryEndResponse(reqID any, r *TestBoundaryEndResult, wasActive bool) mcp.JSONRPCResponse {
	return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: reqID, Result: mcp.JSONResponse("Test boundary ended", map[string]any{
		"status":     "ok",
		"test_id":    r.TestID,
		"was_active": wasActive,
		"message":    "Test boundary ended",
	})}
}
