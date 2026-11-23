// key.go — Derives privacy-safe product-usage keys from MCP tool arguments.

package toolusage

import (
	"encoding/json"
	"strings"
)

// Key returns only the product command identifier used by anonymous usage
// telemetry. Selectors, URLs, text, and all other user arguments are ignored.
func Key(arguments json.RawMessage) string {
	if len(arguments) == 0 {
		return ""
	}
	var payload struct {
		What          string `json:"what"`
		CorrelationID string `json:"correlation_id"`
	}
	if err := json.Unmarshal(arguments, &payload); err != nil {
		return ""
	}
	if payload.What != "command_result" {
		return payload.What
	}
	if payload.CorrelationID == "" {
		return "command_result"
	}
	prefix := payload.CorrelationID
	if separator := strings.IndexByte(prefix, '_'); separator > 0 {
		prefix = prefix[:separator]
	}
	return "command_result:" + prefix
}
