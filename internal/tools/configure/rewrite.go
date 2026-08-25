// Purpose: Rewrites noise_rule argument maps to normalize noise_action to the canonical action field.
// Docs: docs/features/feature/config-profiles/index.md

package configure

import "encoding/json"

func parseRawArgsMap(args json.RawMessage) (map[string]any, error) {
	var raw map[string]any
	if len(args) > 0 {
		if err := json.Unmarshal(args, &raw); err != nil {
			return nil, err
		}
	}
	if raw == nil {
		raw = make(map[string]any)
	}
	return raw, nil
}

func marshalRawArgsMap(raw map[string]any) json.RawMessage {
	// Error impossible: map contains primitive/json-compatible values from decoded input.
	rewritten, _ := json.Marshal(raw)
	return rewritten
}

func applyActionAlias(raw map[string]any, aliasField string, allowEmpty bool) {
	if sa, ok := raw[aliasField].(string); ok && (allowEmpty || sa != "") {
		raw["action"] = sa
	}
}

// RewriteNoiseRuleArgs rewrites noise_action to action in the raw argument map.
// If noise_action is empty or missing, it defaults to "list".
// Returns the rewritten JSON bytes, or an error if the input is invalid JSON.
func RewriteNoiseRuleArgs(args json.RawMessage) (json.RawMessage, error) {
	rawMap, err := parseRawArgsMap(args)
	if err != nil {
		return nil, err
	}
	rawMap["action"] = stringOrEmpty(rawMap["noise_action"])
	if rawMap["action"] == "" {
		rawMap["action"] = "list"
	}
	if action, _ := rawMap["action"].(string); action == "add" {
		maybeFlattenSingleNoiseRule(rawMap)
	}
	return marshalRawArgsMap(rawMap), nil
}

func maybeFlattenSingleNoiseRule(rawMap map[string]any) {
	if rules, ok := rawMap["rules"].([]any); ok && len(rules) > 0 {
		return
	}
	rule, ok := buildFlatNoiseRule(rawMap)
	if !ok {
		return
	}
	rawMap["rules"] = []any{rule}
}

type noiseMatchCriteria struct {
	messageRegex string
	sourceRegex  string
	urlRegex     string
	method       string
	level        string
	statusMin    any
	hasStatusMin bool
	statusMax    any
	hasStatusMax bool
}

func flatNoiseCriteria(rawMap map[string]any) noiseMatchCriteria {
	messageRegex := stringOrEmpty(rawMap["message_regex"])
	if messageRegex == "" {
		messageRegex = stringOrEmpty(rawMap["pattern"])
	}
	statusMin, hasStatusMin := rawMap["status_min"]
	statusMax, hasStatusMax := rawMap["status_max"]
	return noiseMatchCriteria{
		messageRegex: messageRegex,
		sourceRegex:  stringOrEmpty(rawMap["source_regex"]),
		urlRegex:     stringOrEmpty(rawMap["url_regex"]),
		method:       stringOrEmpty(rawMap["method"]),
		level:        stringOrEmpty(rawMap["level"]),
		statusMin:    statusMin,
		hasStatusMin: hasStatusMin,
		statusMax:    statusMax,
		hasStatusMax: hasStatusMax,
	}
}

func (c noiseMatchCriteria) empty() bool {
	return c.messageRegex == "" && c.sourceRegex == "" && c.urlRegex == "" && c.method == "" && c.level == "" && !c.hasStatusMin && !c.hasStatusMax
}

func (c noiseMatchCriteria) matchSpec() map[string]any {
	spec := map[string]any{}
	if c.messageRegex != "" {
		spec["message_regex"] = c.messageRegex
	}
	if c.sourceRegex != "" {
		spec["source_regex"] = c.sourceRegex
	}
	if c.urlRegex != "" {
		spec["url_regex"] = c.urlRegex
	}
	if c.method != "" {
		spec["method"] = c.method
	}
	if c.level != "" {
		spec["level"] = c.level
	}
	if c.hasStatusMin {
		spec["status_min"] = c.statusMin
	}
	if c.hasStatusMax {
		spec["status_max"] = c.statusMax
	}
	return spec
}

func buildFlatNoiseRule(rawMap map[string]any) (map[string]any, bool) {
	criteria := flatNoiseCriteria(rawMap)
	if criteria.empty() {
		return nil, false
	}

	category := stringOrEmpty(rawMap["category"])
	if category == "" {
		category = "console"
	}

	rule := map[string]any{
		"category":   category,
		"match_spec": criteria.matchSpec(),
	}
	if classification := stringOrEmpty(rawMap["classification"]); classification != "" {
		rule["classification"] = classification
	}
	return rule, true
}

func stringOrEmpty(v any) string {
	s, _ := v.(string)
	return s
}

// RewriteStreamingArgs rewrites streaming_action to action in the raw argument map.
// Returns the rewritten JSON bytes, or an error if the input is invalid JSON.
func RewriteStreamingArgs(args json.RawMessage) (json.RawMessage, error) {
	raw, err := parseRawArgsMap(args)
	if err != nil {
		return nil, err
	}
	applyActionAlias(raw, "streaming_action", true)
	return marshalRawArgsMap(raw), nil
}

// RewriteDiffSessionsArgs rewrites verif_session_action to action in the raw argument map.
// If the resulting action is empty or "diff_sessions", it defaults to "list".
// Returns the rewritten JSON bytes, or an error if the input is invalid JSON.
func RewriteDiffSessionsArgs(args json.RawMessage) (json.RawMessage, error) {
	raw, err := parseRawArgsMap(args)
	if err != nil {
		return nil, err
	}
	applyActionAlias(raw, "verif_session_action", false)

	// configure(action:"diff_sessions") is the tool entrypoint; default to list
	// unless a specific verif_session_action is provided.
	if action, _ := raw["action"].(string); action == "" || action == "diff_sessions" {
		raw["action"] = "list"
	}
	return marshalRawArgsMap(raw), nil
}
