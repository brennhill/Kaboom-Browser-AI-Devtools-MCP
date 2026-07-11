// Purpose: Defines retry-contract strategy derivation and immutable retry state types.
// Why: Keeps strategy fingerprinting separate from state storage and response decoration logic.
// Docs: docs/features/feature/interact-explore/index.md

package interacthandler

import (
	"encoding/json"
	"strings"
	"time"
)

const maxRetryAttemptsPerStep = 2

type commandRetryState struct {
	Attempt             int
	MaxAttempts         int
	Action              string
	Strategy            string
	StrategyFingerprint string
	ChangedStrategy     bool
	PolicyViolation     string
	ParentCorrelationID string
	CreatedAt           time.Time
}

type retryTerminalDecision struct {
	Terminal bool
	Cause    string
}

func parseRetryParentCorrelationID(args json.RawMessage) string {
	var params struct {
		CorrelationID string `json:"correlation_id"`
	}
	lenientUnmarshal(args, &params)
	return strings.TrimSpace(params.CorrelationID)
}

func deriveRetryStrategy(action string, args json.RawMessage) (strategy string, fingerprint string) {
	var payload map[string]any
	lenientUnmarshal(args, &payload)

	f := map[string]any{
		"action": strings.ToLower(strings.TrimSpace(action)),
	}
	for _, key := range []string{
		"selector",
		"scope_selector",
		"scope_rect",
		"annotation_rect",
		"element_id",
		"index",
		"frame",
		"world",
		"text",
		"value",
		"wait_for",
	} {
		if v, ok := payload[key]; ok {
			f[key] = v
		}
	}
	fingerprint = stableMarshalForRetry(f)

	switch {
	case payload["element_id"] != nil:
		return "element_handle", fingerprint
	case payload["scope_selector"] != nil || payload["scope_rect"] != nil || payload["annotation_rect"] != nil:
		return "scoped_selector", fingerprint
	case payload["frame"] != nil:
		return "frame_targeted", fingerprint
	case payload["selector"] != nil:
		return "selector", fingerprint
	case payload["index"] != nil:
		return "indexed", fingerprint
	case payload["world"] != nil:
		return "world_switch", fingerprint
	default:
		return "default", fingerprint
	}
}

func stableMarshalForRetry(v map[string]any) string {
	if v == nil {
		return ""
	}
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}
