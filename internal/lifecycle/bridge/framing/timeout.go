// Purpose: Assigns per-request timeouts by MCP method and tool name (fast/slow/blocking categories).
// Docs: docs/features/feature/bridge-restart/index.md

package bridge

import (
	"encoding/json"
	"time"
)

// Timeout constants for different tool categories.
const (
	FastTimeout  = 10 * time.Second
	SlowTimeout  = 35 * time.Second
	BlockingPoll = 65 * time.Second
)

// ToolCallTimeout returns the per-request timeout based on the MCP method and tool name.
// Fast tools (observe, generate, most configure actions, resources/read) get 10s;
// slow tools (analyze, interact, long-running configure actions) get 35s.
// Annotation observe (observe command_result for ann_*) gets 65s for blocking poll.
//
// method is the JSON-RPC method (e.g. "tools/call", "resources/read").
// params is the raw JSON of the request params.
func ToolCallTimeout(method string, params json.RawMessage) time.Duration {
	if method == "resources/read" {
		return FastTimeout
	}
	if method != "tools/call" {
		return FastTimeout
	}

	var p struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if json.Unmarshal(params, &p) != nil {
		return FastTimeout
	}

	switch p.Name {
	case "analyze", "interact":
		return SlowTimeout
	case "configure":
		return configureTimeout(p.Arguments)
	case "observe":
		return observeTimeout(p.Arguments)
	default:
		return FastTimeout
	}
}

func configureTimeout(args json.RawMessage) time.Duration {
	var a struct {
		Action string `json:"action"`
		What   string `json:"what"`
	}
	if json.Unmarshal(args, &a) != nil {
		return FastTimeout
	}
	action := a.Action
	if action == "" {
		action = a.What
	}
	switch action {
	case "replay_sequence", "playback":
		return SlowTimeout
	}
	return FastTimeout
}

func observeTimeout(args json.RawMessage) time.Duration {
	var a struct {
		What          string `json:"what"`
		CorrelationID string `json:"correlation_id"`
	}
	if json.Unmarshal(args, &a) != nil {
		return FastTimeout
	}
	if a.What == "command_result" && isAnnotationCorrelationID(a.CorrelationID) {
		return BlockingPoll
	}
	if a.What == "screenshot" {
		return SlowTimeout
	}
	return FastTimeout
}

func isAnnotationCorrelationID(correlationID string) bool {
	return len(correlationID) > 4 && correlationID[:4] == "ann_"
}

// ExtractToolAction extracts the tool name and action parameter from a tools/call request.
// Returns empty strings for non-tools/call methods or if parsing fails.
func ExtractToolAction(method string, params json.RawMessage) (toolName, action string) {
	if method != "tools/call" {
		return "", ""
	}
	var p struct {
		Name string          `json:"name"`
		Args json.RawMessage `json:"arguments"`
	}
	if json.Unmarshal(params, &p) != nil {
		return "", ""
	}
	var a struct {
		What   string `json:"what"`
		Action string `json:"action"`
	}
	_ = json.Unmarshal(p.Args, &a)
	act := a.What
	if act == "" {
		act = a.Action
	}
	return p.Name, act
}
