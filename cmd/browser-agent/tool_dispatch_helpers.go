// tool_dispatch_helpers.go — Shared alias-resolution, mode-list helpers, and generic dispatch for tool routing.

package main

import (
	"encoding/json"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// ModeHandler is the unified function signature for all tool mode handlers.
// All five tools (observe, analyze, configure, generate, interact) use this signature.
type ModeHandler func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse

// toolRegistry bundles the handler map, alias definitions, and metadata for a tool.
type toolRegistry struct {
	Handlers   map[string]ModeHandler
	AliasDefs  []modeAlias
	Resolution modeResolution
	// PreDispatch is called after mode resolution but before handler dispatch.
	// Returns modified args and optional response (non-nil short-circuits dispatch).
	PreDispatch func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage, what string) (json.RawMessage, *JSONRPCResponse)
	// PostDispatch is called after the handler returns, before alias warning.
	PostDispatch func(h *ToolHandler, req JSONRPCRequest, resp JSONRPCResponse, what string) JSONRPCResponse
}

// dispatchTool resolves the mode, looks up the handler, and dispatches.
// Handles the resolve→lookup→not-found→call→alias-warning pattern shared by all 4 registry tools.
func (h *ToolHandler) dispatchTool(req JSONRPCRequest, args json.RawMessage, reg toolRegistry) JSONRPCResponse {
	what, usedAliasParam, errResp := resolveToolMode(req, args, reg.AliasDefs, reg.Resolution)
	if errResp != nil {
		return *errResp
	}

	deprecatedIn, removeIn := findAliasParamDeprecation(usedAliasParam, reg.AliasDefs)

	handler, ok := reg.Handlers[what]
	if !ok {
		validModes := reg.Resolution.ValidModes
		resp := mcp.Fail(req, ErrUnknownMode, "Unknown "+reg.Resolution.ToolName+" mode: "+what,
			"Use a valid mode from the 'what' enum", withParam("what"), withHint("Valid values: "+validModes), describeCapabilitiesRecovery(reg.Resolution.ToolName))
		return appendCanonicalWhatAliasWarning(resp, usedAliasParam, what, deprecatedIn, removeIn)
	}

	if reg.PreDispatch != nil {
		var preResp *JSONRPCResponse
		args, preResp = reg.PreDispatch(h, req, args, what)
		if preResp != nil {
			return appendCanonicalWhatAliasWarning(*preResp, usedAliasParam, what, deprecatedIn, removeIn)
		}
	}

	resp := handler(h, req, args)

	if reg.PostDispatch != nil {
		resp = reg.PostDispatch(h, req, resp, what)
	}

	return appendCanonicalWhatAliasWarning(resp, usedAliasParam, what, deprecatedIn, removeIn)
}

// method adapts a ToolHandler method (that takes req, args) into a ModeHandler.
// This eliminates the one-line closure boilerplate in registries:
//
//	Before: "dom": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolQueryDOM(req, args) },
//	After:  "dom": method((*ToolHandler).toolQueryDOM),
func method(fn func(*ToolHandler, JSONRPCRequest, json.RawMessage) JSONRPCResponse) ModeHandler {
	return fn
}

// defaultModeActionAliases defines the standard deprecated alias parameters ("mode", "action")
// shared by observe and analyze tools. Reference this from tool registries instead of duplicating.
var defaultModeActionAliases = []modeAlias{
	{JSONField: "mode", DeprecatedIn: "0.7.0", RemoveIn: "0.9.0"},
	{JSONField: "action", DeprecatedIn: "0.7.0", RemoveIn: "0.9.0"},
}

// modeAlias defines a deprecated parameter that can substitute for the canonical 'what' param.
//
// ConflictFn gates the conflict check: when set, a conflict is only raised if ConflictFn returns true.
// This supports tools where a param like "action" doubles as both a mode selector and a sub-action
// field — conflicts are only flagged when the alias value is a known top-level mode.
//
// FallbackFn gates the fallback: when set, the alias value is only used as a mode selector when
// FallbackFn returns true. When nil, any non-empty alias value is accepted as a fallback.
type modeAlias struct {
	JSONField    string            // JSON field name in args (e.g. "action", "mode", "format")
	ConflictFn   func(string) bool // Optional: only raise conflict when this returns true
	FallbackFn   func(string) bool // Optional: only use as fallback mode when this returns true
	DeprecatedIn string            // Semver when deprecated (e.g. "0.7.0"); empty = not tracked
	RemoveIn     string            // Semver when removal is planned (e.g. "0.9.0"); empty = not tracked
}

// modeValueAlias maps a shorthand mode value to its canonical name with deprecation tracking.
type modeValueAlias struct {
	Canonical    string // Canonical mode name (e.g. "network_waterfall")
	DeprecatedIn string // Semver when deprecated (e.g. "0.7.0")
	RemoveIn     string // Semver when removal is planned (e.g. "0.9.0")
}

// modeResolution bundles context needed for mode resolution error messages.
type modeResolution struct {
	ToolName     string                    // For error messages (e.g. "observe", "analyze")
	ValidModes   string                    // Sorted comma-separated list for hints
	Aliases      map[string]string         // Mode aliases (e.g. "network" -> "network_waterfall") — legacy, used when ValueAliases is nil
	ValueAliases map[string]modeValueAlias // Mode aliases with deprecation metadata — preferred over Aliases
}

// resolveToolMode extracts and resolves the 'what' parameter from args, checking alias params
// for fallback values. Returns the resolved mode, which alias param was used (empty if canonical),
// and an error response if resolution fails.
//
// Resolution order:
//  1. Parse 'what' and all alias params from args.
//  2. Detect conflicts: if 'what' is set and an alias has a different value, return conflict error.
//  3. Fall back to aliases in order if 'what' is empty.
//  4. Return missing-param error if no mode found.
//  5. Apply mode aliases (e.g. "network" -> "network_waterfall").
func resolveToolMode(
	req JSONRPCRequest,
	args json.RawMessage,
	aliasDefs []modeAlias,
	res modeResolution,
) (what string, usedAliasParam string, errResp *JSONRPCResponse) {

	// Parse all potential mode fields into a map.
	fields := make(map[string]string, len(aliasDefs)+1)
	if len(args) > 0 {
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(args, &raw); err != nil {
			resp := mcp.Fail(req, ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")
			return "", "", &resp
		}
		for _, key := range append([]string{"what"}, aliasFieldNames(aliasDefs)...) {
			if v, ok := raw[key]; ok {
				var s string
				if json.Unmarshal(v, &s) == nil {
					fields[key] = s
				}
			}
		}
	}

	what = fields["what"]

	// Check for conflicts: what is set and an alias has a different value.
	for _, ad := range aliasDefs {
		aliasVal := fields[ad.JSONField]
		if aliasVal == "" || aliasVal == what || what == "" {
			continue
		}
		if ad.ConflictFn != nil && !ad.ConflictFn(aliasVal) {
			continue
		}
		resp := whatAliasConflictResponse(req, ad.JSONField, what, aliasVal, res.ValidModes)
		return "", "", &resp
	}

	// Fall back to alias params in order.
	if what == "" {
		for _, ad := range aliasDefs {
			aliasVal := fields[ad.JSONField]
			if aliasVal == "" {
				continue
			}
			if ad.FallbackFn != nil && !ad.FallbackFn(aliasVal) {
				continue
			}
			what = aliasVal
			usedAliasParam = ad.JSONField
			break
		}
	}

	// Missing mode.
	if what == "" {
		resp := mcp.Fail(req, ErrMissingParam,
			"Required parameter 'what' is missing",
			"Add the 'what' parameter and call again",
			withParam("what"),
			withHint("Valid values: "+res.ValidModes))
		return "", usedAliasParam, &resp
	}

	// Apply mode aliases (e.g. "network" -> "network_waterfall").
	if res.ValueAliases != nil {
		if va, ok := res.ValueAliases[what]; ok {
			what = va.Canonical
		}
	} else if res.Aliases != nil {
		if canonical, ok := res.Aliases[what]; ok {
			what = canonical
		}
	}

	return what, usedAliasParam, nil
}

// findAliasParamDeprecation returns the deprecation metadata for a used alias param.
func findAliasParamDeprecation(usedAliasParam string, aliasDefs []modeAlias) (deprecatedIn, removeIn string) {
	if usedAliasParam == "" {
		return "", ""
	}
	for _, ad := range aliasDefs {
		if ad.JSONField == usedAliasParam {
			return ad.DeprecatedIn, ad.RemoveIn
		}
	}
	return "", ""
}

// aliasFieldNames extracts JSON field names from alias definitions.
func aliasFieldNames(defs []modeAlias) []string {
	names := make([]string, len(defs))
	for i, d := range defs {
		names[i] = d.JSONField
	}
	return names
}
