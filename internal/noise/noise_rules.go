// Purpose: Implements manual noise rule lifecycle operations.
// Why: Separates CRUD/validation paths from runtime matching for modularity and testability.
// Docs: docs/features/feature/noise-filtering/index.md

package noise

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statediag"
)

// ListRules returns a copy of all current rules.
func (nc *NoiseConfig) ListRules() []NoiseRule {
	nc.mu.RLock()
	defer nc.mu.RUnlock()

	result := make([]NoiseRule, len(nc.rules))
	copy(result, nc.rules)
	return result
}

// validateRegexPattern checks if a regex pattern is safe to compile.
// Rejects patterns with excessive length or nested quantifiers that could
// cause significant performance degradation.
// Returns nil if the pattern is safe (even if it has invalid syntax - those are caught during compilation).
func validateRegexPattern(pattern string) error {
	const maxPatternLength = 512

	if len(pattern) > maxPatternLength {
		return fmt.Errorf("regex pattern exceeds maximum length of %d characters", maxPatternLength)
	}

	nestedQuantifierPatterns := []string{
		`\+\s*\)?\s*[\+\*\?]`,
		`\*\s*\)?\s*[\+\*\?]`,
		`\?\s*\)?\s*[\+\*\?]`,
		`\}\s*\)?\s*[\+\*\?]`,
	}

	for _, nestedPattern := range nestedQuantifierPatterns {
		if matched, _ := regexp.MatchString(nestedPattern, pattern); matched {
			return fmt.Errorf("regex pattern contains nested quantifiers which can cause performance issues")
		}
	}

	// Invalid syntax is intentionally not rejected here.
	// recompile() skips regexes that fail compilation to preserve backward compatibility.
	return nil
}

// validateAllRulePatterns validates regex patterns in all rules before any are added.
func validateAllRulePatterns(rules []NoiseRule) error {
	fieldNames := []struct {
		label string
		get   func(*NoiseMatchSpec) string
	}{
		{"MessageRegex", func(spec *NoiseMatchSpec) string { return spec.MessageRegex }},
		{"SourceRegex", func(spec *NoiseMatchSpec) string { return spec.SourceRegex }},
		{"URLRegex", func(spec *NoiseMatchSpec) string { return spec.URLRegex }},
	}

	for i := range rules {
		for _, field := range fieldNames {
			pattern := field.get(&rules[i].MatchSpec)
			if pattern == "" {
				continue
			}
			if err := validateRegexPattern(pattern); err != nil {
				return fmt.Errorf("invalid %s in rule: %w", field.label, err)
			}
		}
	}
	return nil
}

// AddRules adds user rules to the config. Rules exceeding max are silently dropped.
func (nc *NoiseConfig) AddRules(rules []NoiseRule) error {
	nc.mu.Lock()
	defer nc.mu.Unlock()

	if err := validateAllRulePatterns(rules); err != nil {
		return err
	}

	for i := range rules {
		if len(nc.rules) >= maxNoiseRules {
			break
		}
		nc.userIDCounter++
		rules[i].ID = fmt.Sprintf("user_%d", nc.userIDCounter)
		rules[i].CreatedAt = time.Now()
		nc.rules = append(nc.rules, rules[i])
	}

	nc.recompile()
	nc.persistRulesLocked()
	return nil
}

// RemoveRule removes a rule by ID. Built-in rules cannot be removed.
func (nc *NoiseConfig) RemoveRule(id string) error {
	nc.mu.Lock()
	defer nc.mu.Unlock()

	if strings.HasPrefix(id, "builtin_") {
		return fmt.Errorf("cannot remove built-in rule: %s", id)
	}

	for i := range nc.rules {
		if nc.rules[i].ID == id {
			nc.rules = append(nc.rules[:i], nc.rules[i+1:]...)
			nc.recompile()
			nc.persistRulesLocked()
			return nil
		}
	}
	return fmt.Errorf("rule not found: %s", id)
}

// Reset removes all user/auto rules, reverting to only built-ins.
func (nc *NoiseConfig) Reset() {
	nc.mu.Lock()
	defer nc.mu.Unlock()

	nc.rules = builtinRules()
	nc.userIDCounter = 0
	nc.recompile()
	nc.stats = NoiseStatistics{
		PerRule: make(map[string]int),
	}
	nc.persistRulesLocked()
}

// DismissNoise is a convenience method that creates a "dismissed" rule from a pattern.
// If category is empty, defaults to "console".
func (nc *NoiseConfig) DismissNoise(pattern string, category string, reason string) {
	if category == "" {
		category = "console"
	}

	nc.mu.Lock()
	defer nc.mu.Unlock()

	if len(nc.rules) >= maxNoiseRules {
		return
	}

	nc.userIDCounter++
	rule := NoiseRule{
		ID:             fmt.Sprintf("dismiss_%d", nc.userIDCounter),
		Category:       category,
		Classification: "dismissed",
		CreatedAt:      time.Now(),
		Reason:         reason,
	}

	switch category {
	case "network", "websocket":
		rule.MatchSpec.URLRegex = pattern
	default:
		rule.MatchSpec.MessageRegex = pattern
	}

	nc.rules = append(nc.rules, rule)
	nc.recompile()
	nc.persistRulesLocked()
}

func (nc *NoiseConfig) recordMatch(ruleID string) {
	nc.statsMu.Lock()
	defer nc.statsMu.Unlock()
	nc.stats.TotalFiltered++
	nc.stats.PerRule[ruleID]++
	nc.stats.LastNoiseAt = time.Now()
}

func (nc *NoiseConfig) recordSignal() {
	nc.statsMu.Lock()
	defer nc.statsMu.Unlock()
	nc.stats.LastSignalAt = time.Now()
}

// GetStatistics returns a detached snapshot of noise statistics.
func (nc *NoiseConfig) GetStatistics() NoiseStatistics {
	nc.statsMu.Lock()
	defer nc.statsMu.Unlock()
	perRule := make(map[string]int, len(nc.stats.PerRule))
	for key, value := range nc.stats.PerRule {
		perRule[key] = value
	}
	return NoiseStatistics{
		TotalFiltered: nc.stats.TotalFiltered,
		PerRule:       perRule,
		LastSignalAt:  nc.stats.LastSignalAt,
		LastNoiseAt:   nc.stats.LastNoiseAt,
	}
}

func (nc *NoiseConfig) loadPersistedRules() {
	if nc.store == nil {
		return
	}
	persisted, ok := nc.readPersistedData()
	if !ok {
		return
	}
	validRules := nc.validatePersistedRules(persisted.Rules)
	nc.restoreUserIDCounter(persisted.NextUserID, validRules)
	maxUserRules := maxNoiseRules - len(nc.rules)
	if len(validRules) > maxUserRules {
		fmt.Fprintf(os.Stderr, "noise: truncating %d rules to fit max of %d\n", len(validRules), maxUserRules)
		validRules = validRules[:maxUserRules]
	}
	nc.rules = append(nc.rules, validRules...)
	nc.restoreStatistics(persisted.Statistics)
}

func (nc *NoiseConfig) readPersistedData() (PersistedNoiseData, bool) {
	data, err := nc.store.Load("noise", "rules")
	if err != nil || data == nil {
		if err != nil && !errors.Is(err, statediag.ErrAbsent) {
			nc.reportPersistenceRecovery(
				"Persisted noise rules could not be read; built-in defaults are active.",
				"Check session-store permissions, then save the noise rules again.",
			)
		}
		return PersistedNoiseData{}, false
	}
	var persisted PersistedNoiseData
	if err := json.Unmarshal(data, &persisted); err != nil {
		fmt.Fprintf(os.Stderr, "noise: corrupted persisted rules: %v\n", err)
		nc.reportPersistenceRecovery(
			"Persisted noise rules were malformed; built-in defaults are active.",
			"Reset noise rules from System Doctor or configure(what='noise_rule', noise_action='reset').",
		)
		return PersistedNoiseData{}, false
	}
	if persisted.Version != 1 {
		fmt.Fprintf(os.Stderr, "noise: unsupported persistence version: %d\n", persisted.Version)
		nc.reportPersistenceRecovery(
			fmt.Sprintf("Persisted noise rules use unsupported version %d; built-in defaults are active.", persisted.Version),
			"Reset noise rules to rewrite them in the current format.",
		)
		return PersistedNoiseData{}, false
	}
	statediag.Resolve(nc.diagnostics, "noise_rule_state")
	return persisted, true
}

func (nc *NoiseConfig) validatePersistedRules(rules []NoiseRule) []NoiseRule {
	valid := []NoiseRule{}
	for _, rule := range rules {
		if strings.HasPrefix(rule.ID, "builtin_") {
			continue
		}
		if !isRuleRegexValid(rule) {
			fmt.Fprintf(os.Stderr, "noise: skipping rule %s: invalid regex\n", rule.ID)
			continue
		}
		valid = append(valid, rule)
	}
	return valid
}

func isRuleRegexValid(rule NoiseRule) bool {
	patterns := []string{rule.MatchSpec.MessageRegex, rule.MatchSpec.SourceRegex, rule.MatchSpec.URLRegex}
	for _, pattern := range patterns {
		if pattern == "" {
			continue
		}
		if _, err := regexp.Compile(pattern); err != nil {
			return false
		}
	}
	return true
}

func (nc *NoiseConfig) restoreUserIDCounter(nextUserID int, validRules []NoiseRule) {
	nc.userIDCounter = nextUserID - 1
	maxID := nextUserID - 1
	for _, rule := range validRules {
		if strings.HasPrefix(rule.ID, "user_") {
			id, err := strconv.Atoi(strings.TrimPrefix(rule.ID, "user_"))
			if err == nil && id > maxID {
				maxID = id
			}
		}
	}
	if maxID > nc.userIDCounter {
		nc.userIDCounter = maxID
	}
}

func (nc *NoiseConfig) restoreStatistics(stats NoiseStatistics) {
	nc.statsMu.Lock()
	defer nc.statsMu.Unlock()
	if stats.PerRule != nil {
		nc.stats.PerRule = stats.PerRule
	}
	nc.stats.TotalFiltered = stats.TotalFiltered
	nc.stats.LastSignalAt = stats.LastSignalAt
	nc.stats.LastNoiseAt = stats.LastNoiseAt
}

func (nc *NoiseConfig) persistRulesLocked() {
	if nc.store == nil {
		return
	}
	userRules := nc.filterUserRulesLocked()
	stats := nc.GetStatistics()
	persisted := PersistedNoiseData{
		Version:    1,
		NextUserID: nc.userIDCounter + 1,
		Rules:      userRules,
		Statistics: stats,
	}
	data, err := json.Marshal(persisted)
	if err != nil {
		fmt.Fprintf(os.Stderr, "noise: failed to marshal rules: %v\n", err)
		return
	}
	if err := nc.store.Save("noise", "rules", data); err != nil {
		fmt.Fprintln(os.Stderr, "noise: persisted rule write failed; see System Doctor")
		nc.reportPersistenceRecovery(
			"Noise rule changes could not be persisted; they remain active only for this process.",
			"Check session-store space and permissions, then save the noise rules again.",
		)
		return
	}
	statediag.Resolve(nc.diagnostics, "noise_rule_state")
}

func (nc *NoiseConfig) reportPersistenceRecovery(detail, fix string) {
	if nc == nil || nc.diagnostics == nil {
		return
	}
	nc.diagnostics.Report(statediag.Diagnostic{Name: "noise_rule_state", Detail: detail, Fix: fix})
}

func (nc *NoiseConfig) filterUserRulesLocked() []NoiseRule {
	var userRules []NoiseRule
	for _, rule := range nc.rules {
		if !strings.HasPrefix(rule.ID, "builtin_") {
			userRules = append(userRules, rule)
		}
	}
	return userRules
}
