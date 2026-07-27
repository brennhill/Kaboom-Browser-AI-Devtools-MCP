// runtime_handler.go — Owns incremental API-validation state and MCP operations.
// Docs: docs/features/feature/analyze-tool/index.md

package apicontract

import (
	"encoding/json"
	"sync"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

type Runtime struct {
	mu        sync.Mutex
	validator *APIContractValidator
	offset    int
}

func NewRuntime() *Runtime {
	return &Runtime{validator: NewAPIContractValidator()}
}

func (r *Runtime) Handle(req mcp.JSONRPCRequest, args json.RawMessage, bodies []capture.NetworkBody) mcp.JSONRPCResponse {
	var params struct {
		Operation       string   `json:"operation"`
		URLFilter       string   `json:"url"`
		IgnoreEndpoints []string `json:"ignore_endpoints"`
	}
	if len(args) > 0 {
		if response, stop := mcp.ParseArgs(req, args, &params); stop {
			return response
		}
	}
	switch params.Operation {
	case "analyze":
		validator := r.process(bodies)
		result := validator.Analyze(runtimeFilter(params.URLFilter, params.IgnoreEndpoints))
		responseData := map[string]any{
			"status": "ok", "operation": "analyze", "action": result.Action,
			"analyzed_at": result.AnalyzedAt, "summary": result.Summary,
			"violations": result.Violations, "tracked_endpoints": result.TrackedEndpoints,
			"total_requests_analyzed":  result.TotalRequestsAnalyzed,
			"clean_endpoints":          result.CleanEndpoints,
			"possible_violation_types": result.PossibleViolationTypes,
		}
		if result.DataWindowStartedAt != "" {
			responseData["data_window_started_at"] = result.DataWindowStartedAt
		}
		if result.AppliedFilter != nil {
			responseData["applied_filter"] = result.AppliedFilter
		}
		if result.Hint != "" {
			responseData["hint"] = result.Hint
		}
		return mcp.Succeed(req, "API validation analyze", responseData)
	case "report":
		validator := r.process(bodies)
		result := validator.Report(runtimeFilter(params.URLFilter, params.IgnoreEndpoints))
		responseData := map[string]any{
			"status": "ok", "operation": "report", "action": result.Action,
			"analyzed_at": result.AnalyzedAt, "endpoints": result.Endpoints,
			"consistency_levels": result.ConsistencyLevels,
		}
		if result.AppliedFilter != nil {
			responseData["applied_filter"] = result.AppliedFilter
		}
		return mcp.Succeed(req, "API validation report", responseData)
	case "clear":
		r.clear(len(bodies))
		return mcp.Succeed(req, "API validation cleared", map[string]any{
			"action": "cleared", "status": "ok", "operation": "clear",
		})
	default:
		return mcp.Fail(req, mcp.ErrInvalidParam,
			"operation parameter must be 'analyze', 'report', or 'clear'",
			"Use a valid value for 'operation'", mcp.WithParam("operation"),
			mcp.WithHint("analyze, report, or clear"))
	}
}

func (r *Runtime) process(bodies []capture.NetworkBody) *APIContractValidator {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.validator == nil {
		r.validator = NewAPIContractValidator()
	}
	if r.offset < 0 || r.offset > len(bodies) {
		r.offset = len(bodies)
	}
	for _, body := range bodies[r.offset:] {
		r.validator.Validate(body)
	}
	r.offset = len(bodies)
	return r.validator
}

func (r *Runtime) clear(bodyCount int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.validator = NewAPIContractValidator()
	r.offset = bodyCount
}

func runtimeFilter(urlFilter string, ignore []string) APIContractFilter {
	return APIContractFilter{URLFilter: urlFilter, IgnoreEndpoints: ignore}
}
