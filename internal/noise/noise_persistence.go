// Purpose: Handles persistence/load of noise rules and statistics to/from session storage.
// Why: Separates storage concerns from runtime matching/filtering logic for clearer module boundaries.
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

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statediag"
)

// loadPersistedRules loads user rules from SessionStore (called during init).
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

	// Enforce max rules limit.
	maxUserRules := maxNoiseRules - len(nc.rules)
	if len(validRules) > maxUserRules {
		fmt.Fprintf(os.Stderr, "noise: truncating %d rules to fit max of %d\n", len(validRules), maxUserRules)
		validRules = validRules[:maxUserRules]
	}

	nc.rules = append(nc.rules, validRules...)
	nc.restoreStatistics(persisted.Statistics)
}

// readPersistedData loads and unmarshals persisted noise data from the store.
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

// validatePersistedRules filters rules, skipping built-ins and invalid regexes.
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

// isRuleRegexValid checks that all regex patterns in a rule compile.
func isRuleRegexValid(rule NoiseRule) bool {
	patterns := []string{rule.MatchSpec.MessageRegex, rule.MatchSpec.SourceRegex, rule.MatchSpec.URLRegex}
	for _, p := range patterns {
		if p == "" {
			continue
		}
		if _, err := regexp.Compile(p); err != nil {
			return false
		}
	}
	return true
}

// restoreUserIDCounter sets the user ID counter from persisted state, handling desync.
func (nc *NoiseConfig) restoreUserIDCounter(nextUserID int, validRules []NoiseRule) {
	nc.userIDCounter = nextUserID - 1
	maxID := nextUserID - 1
	for _, rule := range validRules {
		if strings.HasPrefix(rule.ID, "user_") {
			idStr := strings.TrimPrefix(rule.ID, "user_")
			if id, err := strconv.Atoi(idStr); err == nil && id > maxID {
				maxID = id
			}
		}
	}
	if maxID > nc.userIDCounter {
		nc.userIDCounter = maxID
	}
}

// restoreStatistics restores noise statistics from persisted data.
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

// persistRulesLocked saves user rules to SessionStore (assumes mu is held).
func (nc *NoiseConfig) persistRulesLocked() {
	if nc.store == nil {
		return
	}

	// Filter to only user rules (exclude built-ins).
	userRules := nc.filterUserRulesLocked()

	// Build persisted data.
	statsSnapshot := func() NoiseStatistics {
		nc.statsMu.Lock()
		defer nc.statsMu.Unlock()
		perRule := make(map[string]int, len(nc.stats.PerRule))
		for ruleID, count := range nc.stats.PerRule {
			perRule[ruleID] = count
		}
		return NoiseStatistics{
			TotalFiltered: nc.stats.TotalFiltered,
			PerRule:       perRule,
			LastSignalAt:  nc.stats.LastSignalAt,
			LastNoiseAt:   nc.stats.LastNoiseAt,
		}
	}()
	persisted := PersistedNoiseData{
		Version:    1,
		NextUserID: nc.userIDCounter + 1,
		Rules:      userRules,
		Statistics: statsSnapshot,
	}

	// Marshal and save.
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

// filterUserRulesLocked extracts non-builtin rules (assumes mu is held).
func (nc *NoiseConfig) filterUserRulesLocked() []NoiseRule {
	var userRules []NoiseRule
	for _, rule := range nc.rules {
		if !strings.HasPrefix(rule.ID, "builtin_") {
			userRules = append(userRules, rule)
		}
	}
	return userRules
}
