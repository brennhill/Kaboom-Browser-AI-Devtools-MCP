// Purpose: Implements automatic noise-rule proposal and confidence-based auto-application logic.
// Why: Continuously identifies repetitive low-value events to keep debugging streams focused.
// Docs: docs/features/feature/noise-filtering/index.md

package noise

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// AutoDetect analyzes buffers and proposes noise rules based on frequency and source analysis.
// High-confidence proposals (>= 0.9) are automatically applied.
// Note: This function holds a write lock for the entire analysis. It is designed for
// infrequent manual invocation via the MCP tool, not for hot-path usage.
func (nc *NoiseConfig) AutoDetect(consoleEntries []types.LogEntry, networkBodies []types.NetworkBody, wsEvents []types.WebSocketEvent) []NoiseProposal {
	nc.mu.Lock()
	defer nc.mu.Unlock()

	var proposals []NoiseProposal
	proposals = append(proposals, nc.detectRepetitiveMessages(consoleEntries)...)
	proposals = append(proposals, nc.detectNodeModuleSources(consoleEntries)...)
	proposals = append(proposals, nc.detectInfrastructureURLs(networkBodies)...)

	nc.autoApplyHighConfidence(proposals)
	nc.recompile()
	return proposals
}

// detectRepetitiveMessages proposes rules for console messages repeated 10+ times.
func (nc *NoiseConfig) detectRepetitiveMessages(entries []types.LogEntry) []NoiseProposal {
	if len(entries) == 0 {
		return nil
	}
	msgCounts := make(map[string]int)
	for _, entry := range entries {
		msg, _ := entry["message"].(string)
		if msg != "" {
			msgCounts[msg]++
		}
	}

	var proposals []NoiseProposal
	for msg, count := range msgCounts {
		if count < 10 || nc.isConsoleCoveredLocked(msg, entries) {
			continue
		}
		confidence := 0.7 + float64(count)/100.0
		if confidence > 0.99 {
			confidence = 0.99
		}
		proposals = append(proposals, NoiseProposal{
			Rule: NoiseRule{
				Category: "console", Classification: "repetitive", AutoDetected: true,
				MatchSpec: NoiseMatchSpec{MessageRegex: regexp.QuoteMeta(msg)},
			},
			Confidence: confidence,
			Reason:     fmt.Sprintf("message repeated %d times", count),
		})
	}
	return proposals
}

// detectNodeModuleSources proposes rules for console entries from node_modules sources.
func (nc *NoiseConfig) detectNodeModuleSources(entries []types.LogEntry) []NoiseProposal {
	if len(entries) == 0 {
		return nil
	}
	sourceCounts := make(map[string]int)
	for _, entry := range entries {
		source, _ := entry["source"].(string)
		if source != "" && strings.Contains(source, "node_modules") {
			sourceCounts[source]++
		}
	}

	var proposals []NoiseProposal
	for source, count := range sourceCounts {
		if count < 2 || nc.isSourceCoveredLocked(source) {
			continue
		}
		proposals = append(proposals, NoiseProposal{
			Rule: NoiseRule{
				Category: "console", Classification: "extension", AutoDetected: true,
				MatchSpec: NoiseMatchSpec{SourceRegex: regexp.QuoteMeta(source)},
			},
			Confidence: 0.75,
			Reason:     fmt.Sprintf("node_modules source with %d entries", count),
		})
	}
	return proposals
}

// detectInfrastructureURLs proposes rules for high-frequency infrastructure network paths.
func (nc *NoiseConfig) detectInfrastructureURLs(bodies []types.NetworkBody) []NoiseProposal {
	if len(bodies) == 0 {
		return nil
	}
	urlCounts := make(map[string]int)
	for _, body := range bodies {
		path := capture.ExtractURLPath(body.URL)
		if path != "" {
			urlCounts[path]++
		}
	}

	var proposals []NoiseProposal
	for path, count := range urlCounts {
		if count < 20 || !isInfrastructurePath(path) || nc.isURLCoveredLocked(path) {
			continue
		}
		proposals = append(proposals, NoiseProposal{
			Rule: NoiseRule{
				Category: "network", Classification: "infrastructure", AutoDetected: true,
				MatchSpec: NoiseMatchSpec{URLRegex: regexp.QuoteMeta(path)},
			},
			Confidence: 0.8,
			Reason:     fmt.Sprintf("infrastructure path hit %d times", count),
		})
	}
	return proposals
}

// isInfrastructurePath returns true if the path matches known infrastructure patterns.
func isInfrastructurePath(path string) bool {
	infraPatterns := []string{"/health", "/ping", "/ready", "/__", "/sockjs-node", "/ws"}
	for _, pat := range infraPatterns {
		if strings.Contains(path, pat) {
			return true
		}
	}
	return false
}

// autoApplyHighConfidence adds rules for proposals with confidence >= 0.9.
func (nc *NoiseConfig) autoApplyHighConfidence(proposals []NoiseProposal) {
	for i := range proposals {
		if proposals[i].Confidence < 0.9 || len(nc.rules) >= maxNoiseRules {
			continue
		}
		nc.userIDCounter++
		rule := proposals[i].Rule
		rule.ID = fmt.Sprintf("auto_%d", nc.userIDCounter)
		rule.CreatedAt = time.Now()
		nc.rules = append(nc.rules, rule)
	}
}

// isConsoleCoveredLocked checks if a message is already covered by existing rules (caller holds lock)
func (nc *NoiseConfig) isConsoleCoveredLocked(msg string, entries []types.LogEntry) bool {
	if nc.isMessageCoveredLocked(msg) {
		return true
	}
	return nc.isSourceCoveredForMessageLocked(msg, entries)
}

// isMessageCoveredLocked checks if a message matches any compiled console message regex.
func (nc *NoiseConfig) isMessageCoveredLocked(msg string) bool {
	for i := range nc.compiled {
		if nc.compiled[i].rule.Category != "console" {
			continue
		}
		if nc.compiled[i].messageRegex != nil && nc.compiled[i].messageRegex.MatchString(msg) {
			return true
		}
	}
	return false
}

// isSourceCoveredForMessageLocked checks if the source of entries with the given message is covered.
func (nc *NoiseConfig) isSourceCoveredForMessageLocked(msg string, entries []types.LogEntry) bool {
	for _, entry := range entries {
		entryMsg, _ := entry["message"].(string)
		if entryMsg != msg {
			continue
		}
		source, _ := entry["source"].(string)
		if nc.isSourceCoveredLocked(source) {
			return true
		}
	}
	return false
}

// isSourceCoveredLocked checks if a source is already covered by existing rules (caller holds lock)
func (nc *NoiseConfig) isSourceCoveredLocked(source string) bool {
	for i := range nc.compiled {
		if nc.compiled[i].rule.Category != "console" {
			continue
		}
		if nc.compiled[i].sourceRegex != nil && nc.compiled[i].sourceRegex.MatchString(source) {
			return true
		}
	}
	return false
}

// isURLCoveredLocked checks if a URL path is already covered by existing rules (caller holds lock)
func (nc *NoiseConfig) isURLCoveredLocked(path string) bool {
	for i := range nc.compiled {
		if nc.compiled[i].rule.Category != "network" {
			continue
		}
		if nc.compiled[i].urlRegex != nil && nc.compiled[i].urlRegex.MatchString(path) {
			return true
		}
	}
	return false
}
