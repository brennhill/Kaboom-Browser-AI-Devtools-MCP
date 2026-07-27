// noise_actions.go — Handles configure noise-rule CRUD and auto-detection operations.
// Why: Keeps noise filtering behavior isolated from session diff and audit-log actions.
// Docs: docs/features/feature/noise-filtering/index.md

package toolconfigure

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/noise"
)

// NoiseRuleArgs holds the parsed parameters for noise configuration.
type NoiseRuleArgs struct {
	Action string `json:"action"`
	Rules  []struct {
		Category       string `json:"category"`
		Classification string `json:"classification"`
		MatchSpec      struct {
			MessageRegex string `json:"message_regex"`
			SourceRegex  string `json:"source_regex"`
			URLRegex     string `json:"url_regex"`
			Method       string `json:"method"`
			StatusMin    int    `json:"status_min"`
			StatusMax    int    `json:"status_max"`
			Level        string `json:"level"`
		} `json:"match_spec"`
	} `json:"rules"`
	RuleID string `json:"rule_id"`
}

// HandleNoise handles configure(what="noise_rule") after arg rewriting.
func HandleNoise(d Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var arguments NoiseRuleArgs
	mcp.LenientUnmarshal(args, &arguments)

	nc := d.NoiseConfig()
	if nc == nil {
		return mcp.Fail(req, mcp.ErrNotInitialized, "Noise configuration not initialized", "Internal error — do not retry")
	}

	responseData, errResp := dispatchNoiseAction(d, nc, req, arguments)
	if errResp != nil {
		return *errResp
	}

	return mcp.Succeed(req, "Noise configuration updated", responseData)
}

func dispatchNoiseAction(d Deps, nc *noise.NoiseConfig, req mcp.JSONRPCRequest, args NoiseRuleArgs) (any, *mcp.JSONRPCResponse) {
	switch args.Action {
	case "add":
		return noiseActionAdd(nc, req, args)
	case "remove":
		return noiseActionRemove(nc, req, args)
	case "list":
		return noiseActionList(nc), nil
	case "reset":
		return noiseActionReset(nc), nil
	case "auto_detect":
		return noiseActionAutoDetect(d, nc), nil
	default:
		resp := mcp.Fail(req, mcp.ErrUnknownMode, "Unknown noise action: "+args.Action, "Use a valid action: add, remove, list, reset, auto_detect", mcp.WithParam("noise_action"))
		return nil, &resp
	}
}

func noiseActionAdd(nc *noise.NoiseConfig, req mcp.JSONRPCRequest, args NoiseRuleArgs) (any, *mcp.JSONRPCResponse) {
	rules := make([]noise.NoiseRule, len(args.Rules))
	for i, r := range args.Rules {
		rules[i] = noise.NoiseRule{
			Category:       r.Category,
			Classification: r.Classification,
			MatchSpec: noise.NoiseMatchSpec{
				MessageRegex: r.MatchSpec.MessageRegex,
				SourceRegex:  r.MatchSpec.SourceRegex,
				URLRegex:     r.MatchSpec.URLRegex,
				Method:       r.MatchSpec.Method,
				StatusMin:    r.MatchSpec.StatusMin,
				StatusMax:    r.MatchSpec.StatusMax,
				Level:        r.MatchSpec.Level,
			},
		}
	}
	if err := nc.AddRules(rules); err != nil {
		resp := mcp.Fail(req, mcp.ErrInvalidParam, err.Error(), "Fix the rule pattern and try again")
		return nil, &resp
	}
	return map[string]any{
		"status":      "ok",
		"rules_added": len(args.Rules),
		"total_rules": len(nc.ListRules()),
	}, nil
}

func noiseActionRemove(nc *noise.NoiseConfig, req mcp.JSONRPCRequest, args NoiseRuleArgs) (any, *mcp.JSONRPCResponse) {
	if args.RuleID == "" {
		resp := mcp.Fail(req, mcp.ErrMissingParam, "Missing required parameter: rule_id", "Add the 'rule_id' parameter", mcp.WithParam("rule_id"))
		return nil, &resp
	}
	if err := nc.RemoveRule(args.RuleID); err != nil {
		resp := mcp.Fail(req, mcp.ErrInvalidParam, err.Error(), "Use a valid rule ID from list action")
		return nil, &resp
	}
	return map[string]any{"status": "ok", "removed": args.RuleID}, nil
}

func noiseActionList(nc *noise.NoiseConfig) any {
	rules := nc.ListRules()
	stats := nc.GetStatistics()
	return map[string]any{
		"rules": rules,
		"statistics": map[string]any{
			"total_filtered": stats.TotalFiltered,
			"per_rule":       stats.PerRule,
			"last_signal_at": stats.LastSignalAt,
			"last_noise_at":  stats.LastNoiseAt,
		},
	}
}

func noiseActionReset(nc *noise.NoiseConfig) any {
	nc.Reset()
	return map[string]any{
		"status":      "ok",
		"total_rules": len(nc.ListRules()),
		"message":     "Reset to built-in rules only",
	}
}

func noiseActionAutoDetect(d Deps, nc *noise.NoiseConfig) any {
	consoleEntries := d.ConsoleEntries()
	networkBodies := d.NetworkBodies()
	wsEvents := d.AllWebSocketEvents()

	proposals := nc.AutoDetect(consoleEntries, networkBodies, wsEvents)
	return map[string]any{
		"proposals":       proposals,
		"total_rules":     len(nc.ListRules()),
		"proposals_count": len(proposals),
		"message":         "High-confidence proposals (>= 0.9) were auto-applied",
	}
}
