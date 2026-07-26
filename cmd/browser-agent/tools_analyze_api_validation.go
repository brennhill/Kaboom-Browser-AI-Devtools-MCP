// Purpose: Implements analyze(api_validation) operations and API-validation state helpers.
// Why: Isolates API contract validation flow from other analyze behaviors.
// Docs: docs/features/feature/analyze-tool/index.md

package main

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/analysis/apicontract"
)

func (h *ToolHandler) toolValidateAPI(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Operation       string   `json:"operation"`
		URLFilter       string   `json:"url"`
		IgnoreEndpoints []string `json:"ignore_endpoints"`
	}
	if len(args) > 0 {
		if resp, stop := parseArgs(req, args, &params); stop {
			return resp
		}
	}

	switch params.Operation {
	case "analyze":
		h.processAPIValidationBodies()
		filter := h.apiValidationFilter(params.URLFilter, params.IgnoreEndpoints)
		result := h.apiContractAnalyze(filter)
		responseData := map[string]any{
			"status":                   "ok",
			"operation":                "analyze",
			"action":                   result.Action,
			"analyzed_at":              result.AnalyzedAt,
			"summary":                  result.Summary,
			"violations":               result.Violations,
			"tracked_endpoints":        result.TrackedEndpoints,
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
		return succeed(req, "API validation analyze", responseData)

	case "report":
		h.processAPIValidationBodies()
		filter := h.apiValidationFilter(params.URLFilter, params.IgnoreEndpoints)
		result := h.apiContractReport(filter)
		responseData := map[string]any{
			"status":             "ok",
			"operation":          "report",
			"action":             result.Action,
			"analyzed_at":        result.AnalyzedAt,
			"endpoints":          result.Endpoints,
			"consistency_levels": result.ConsistencyLevels,
		}
		if result.AppliedFilter != nil {
			responseData["applied_filter"] = result.AppliedFilter
		}
		return succeed(req, "API validation report", responseData)

	case "clear":
		h.clearAPIValidationState()
		clearResult := map[string]any{
			"action":    "cleared",
			"status":    "ok",
			"operation": "clear",
		}
		return succeed(req, "API validation cleared", clearResult)

	default:
		return fail(req, ErrInvalidParam, "operation parameter must be 'analyze', 'report', or 'clear'", "Use a valid value for 'operation'", withParam("operation"), withHint("analyze, report, or clear"))
	}
}

func (h *ToolHandler) processAPIValidationBodies() {
	h.apiContractMu.Lock()
	defer h.apiContractMu.Unlock()

	if h.apiContractValidator == nil {
		h.apiContractValidator = apicontract.NewAPIContractValidator()
	}

	bodies := h.capture.GetNetworkBodies()
	if h.apiContractOffset < 0 || h.apiContractOffset > len(bodies) {
		// Buffer can shrink due ring eviction; clamp to avoid replaying old data.
		h.apiContractOffset = len(bodies)
	}

	for _, body := range bodies[h.apiContractOffset:] {
		h.apiContractValidator.Validate(body)
	}
	h.apiContractOffset = len(bodies)
}

func (h *ToolHandler) apiValidationFilter(urlFilter string, ignore []string) apicontract.APIContractFilter {
	return apicontract.APIContractFilter{
		URLFilter:       urlFilter,
		IgnoreEndpoints: ignore,
	}
}

func (h *ToolHandler) apiContractAnalyze(filter apicontract.APIContractFilter) apicontract.APIContractAnalyzeResult {
	validator := h.apiContractValidatorSnapshot()
	if validator == nil {
		return apicontract.APIContractAnalyzeResult{}
	}
	return validator.Analyze(filter)
}

func (h *ToolHandler) apiContractReport(filter apicontract.APIContractFilter) apicontract.APIContractReportResult {
	validator := h.apiContractValidatorSnapshot()
	if validator == nil {
		return apicontract.APIContractReportResult{}
	}
	return validator.Report(filter)
}

func (h *ToolHandler) apiContractValidatorSnapshot() *apicontract.APIContractValidator {
	h.apiContractMu.Lock()
	defer h.apiContractMu.Unlock()
	return h.apiContractValidator
}

func (h *ToolHandler) clearAPIValidationState() {
	h.apiContractMu.Lock()
	defer h.apiContractMu.Unlock()

	h.apiContractValidator = apicontract.NewAPIContractValidator()
	h.apiContractOffset = len(h.capture.GetNetworkBodies())
}
