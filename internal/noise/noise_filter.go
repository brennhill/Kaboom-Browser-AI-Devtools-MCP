// Purpose: Evaluates console/network/websocket events against compiled noise rules.
// Why: Keeps hot-path matching logic isolated from rule CRUD and persistence concerns.
// Docs: docs/features/feature/noise-filtering/index.md

package noise

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// IsConsoleNoise checks if a console log entry matches any noise rule.
func (nc *NoiseConfig) IsConsoleNoise(entry types.LogEntry) bool {
	nc.mu.RLock()
	defer nc.mu.RUnlock()

	message, _ := entry["message"].(string)
	source, _ := entry["source"].(string)
	level, _ := entry["level"].(string)

	for i := range nc.compiled {
		compiled := &nc.compiled[i]
		if compiled.rule.Category != "console" {
			continue
		}
		if compiled.rule.MatchSpec.Level != "" && compiled.rule.MatchSpec.Level != level {
			continue
		}
		if matchesConsoleRule(compiled, message, source) {
			nc.recordMatch(compiled.rule.ID)
			return true
		}
	}

	nc.recordSignal()
	return false
}

// matchesConsoleRule returns true if message or source matches the compiled rule (OR logic).
func matchesConsoleRule(compiled *compiledRule, message, source string) bool {
	if compiled.messageRegex != nil && compiled.messageRegex.MatchString(message) {
		return true
	}
	return compiled.sourceRegex != nil && compiled.sourceRegex.MatchString(source)
}

// IsNetworkNoise checks if a network body matches any noise rule.
// Security invariant: 401/403 responses are NEVER noise.
func (nc *NoiseConfig) IsNetworkNoise(body capture.NetworkBody) bool {
	if body.Status == 401 || body.Status == 403 {
		return false
	}

	nc.mu.RLock()
	defer nc.mu.RUnlock()

	for i := range nc.compiled {
		compiled := &nc.compiled[i]
		if compiled.rule.Category != "network" {
			continue
		}
		if !matchesNetworkFilters(compiled, body) {
			continue
		}
		if matchesNetworkRule(compiled, body.URL) {
			nc.recordMatch(compiled.rule.ID)
			return true
		}
	}

	nc.recordSignal()
	return false
}

// matchesNetworkFilters returns true if the body passes the rule's method and status filters.
func matchesNetworkFilters(compiled *compiledRule, body capture.NetworkBody) bool {
	if compiled.rule.MatchSpec.Method != "" && compiled.rule.MatchSpec.Method != body.Method {
		return false
	}
	if compiled.rule.MatchSpec.StatusMin > 0 && body.Status < compiled.rule.MatchSpec.StatusMin {
		return false
	}
	if compiled.rule.MatchSpec.StatusMax > 0 && body.Status > compiled.rule.MatchSpec.StatusMax {
		return false
	}
	return true
}

// matchesNetworkRule returns true if the URL matches the rule's regex,
// or if no URL regex is set but method/status filters matched.
func matchesNetworkRule(compiled *compiledRule, rawURL string) bool {
	if compiled.urlRegex != nil {
		return compiled.urlRegex.MatchString(rawURL)
	}
	return compiled.rule.MatchSpec.Method != "" || compiled.rule.MatchSpec.StatusMin > 0
}

// IsWebSocketNoise checks if a WebSocket event matches any noise rule.
func (nc *NoiseConfig) IsWebSocketNoise(event capture.WebSocketEvent) bool {
	nc.mu.RLock()
	defer nc.mu.RUnlock()

	for i := range nc.compiled {
		compiled := &nc.compiled[i]
		if compiled.rule.Category != "websocket" {
			continue
		}
		if compiled.urlRegex != nil && compiled.urlRegex.MatchString(event.URL) {
			nc.recordMatch(compiled.rule.ID)
			return true
		}
	}

	nc.recordSignal()
	return false
}
