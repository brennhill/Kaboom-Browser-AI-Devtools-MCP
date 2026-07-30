// Purpose: Defines noise-rule configuration state and regex compilation scaffolding.
// Why: Keeps core noise data structures stable while behavior is split by responsibility.
// Docs: docs/features/feature/noise-filtering/index.md

package noise

import (
	"regexp"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/persistence"
)

// ============================================
// Noise Filtering
// ============================================

const maxNoiseRules = 100

// NoiseMatchSpec defines how a rule matches entries.
type NoiseMatchSpec struct {
	MessageRegex string `json:"message_regex,omitempty"`
	SourceRegex  string `json:"source_regex,omitempty"`
	URLRegex     string `json:"url_regex,omitempty"`
	Method       string `json:"method,omitempty"`
	StatusMin    int    `json:"status_min,omitempty"`
	StatusMax    int    `json:"status_max,omitempty"`
	Level        string `json:"level,omitempty"`
}

// NoiseRule represents a single noise filtering rule.
type NoiseRule struct {
	ID             string         `json:"id"`
	Category       string         `json:"category"`       // "console", "network", "websocket"
	Classification string         `json:"classification"` // "extension", "framework", "cosmetic", "analytics", "infrastructure", "repetitive", "dismissed"
	MatchSpec      NoiseMatchSpec `json:"match_spec"`
	AutoDetected   bool           `json:"auto_detected,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	Reason         string         `json:"reason,omitempty"`
}

// compiledRule holds a rule with pre-compiled regex patterns.
type compiledRule struct {
	rule         NoiseRule
	messageRegex *regexp.Regexp
	sourceRegex  *regexp.Regexp
	urlRegex     *regexp.Regexp
}

// NoiseProposal represents an auto-detected noise proposal.
type NoiseProposal struct {
	Rule       NoiseRule `json:"rule"`
	Confidence float64   `json:"confidence"`
	Reason     string    `json:"reason"`
}

// NoiseStatistics tracks filtering metrics.
type NoiseStatistics struct {
	TotalFiltered int64          `json:"total_filtered"`
	PerRule       map[string]int `json:"per_rule"`
	LastSignalAt  time.Time      `json:"last_signal_at,omitempty"`
	LastNoiseAt   time.Time      `json:"last_noise_at,omitempty"`
}

// PersistedNoiseData is the JSON schema for persisted noise rules.
type PersistedNoiseData struct {
	Version    int             `json:"version"`
	NextUserID int             `json:"next_user_id"`
	Rules      []NoiseRule     `json:"rules"`
	Statistics NoiseStatistics `json:"statistics,omitempty"`
}

// PersistenceDiagnostic describes a safe fallback taken while loading user rules.
type PersistenceDiagnostic struct {
	Kind   string
	Detail string
	Fix    string
}

// NoiseConfig manages noise filtering rules with dual-mutex concurrency control.
//
// LOCK ORDERING INVARIANT (H-5 documented):
//
//	mu -> statsMu (always acquire mu before statsMu, never the reverse)
//
// This ordering is enforced throughout the codebase:
//   - Filter methods (IsConsoleNoise, IsNetworkNoise, IsWebSocketNoise) acquire mu.RLock first,
//     then call recordMatch/recordSignal which acquire statsMu.
//   - GetStatistics acquires only statsMu (no mu held).
//   - AutoDetect acquires only mu (no statsMu held).
//
// If future code needs both locks simultaneously, it MUST acquire mu first.
// Violating this ordering will cause deadlock.
type NoiseConfig struct {
	mu                    sync.RWMutex
	rules                 []NoiseRule
	compiled              []compiledRule
	statsMu               sync.Mutex // Separate mutex for stats (written during reads).
	stats                 NoiseStatistics
	userIDCounter         int
	store                 *persistence.SessionStore // nil if no persistence
	persistenceDiagnostic *PersistenceDiagnostic
}

// PersistenceDiagnostic reports a non-fatal persisted-state recovery.
func (nc *NoiseConfig) PersistenceDiagnostic() (PersistenceDiagnostic, bool) {
	nc.mu.RLock()
	defer nc.mu.RUnlock()
	if nc.persistenceDiagnostic == nil {
		return PersistenceDiagnostic{}, false
	}
	return *nc.persistenceDiagnostic, true
}

// NewNoiseConfig creates a new NoiseConfig with built-in rules.
func NewNoiseConfig() *NoiseConfig {
	nc := &NoiseConfig{
		stats: NoiseStatistics{
			PerRule: make(map[string]int),
		},
	}

	nc.rules = builtinRules()
	nc.recompile()
	return nc
}

// NewNoiseConfigWithStore creates a new NoiseConfig with SessionStore persistence.
func NewNoiseConfigWithStore(store *persistence.SessionStore) *NoiseConfig {
	nc := &NoiseConfig{
		store: store,
		stats: NoiseStatistics{
			PerRule: make(map[string]int),
		},
	}

	nc.rules = builtinRules()
	nc.userIDCounter = 0

	if store != nil {
		nc.loadPersistedRules()
	}

	nc.recompile()
	return nc
}

// recompile compiles all regex patterns in rules. Invalid regexes result in nil (never match).
func (nc *NoiseConfig) recompile() {
	compiled := make([]compiledRule, len(nc.rules))
	for i := range nc.rules {
		rule := &nc.rules[i]
		current := compiledRule{rule: *rule}
		if rule.MatchSpec.MessageRegex != "" {
			if re, err := regexp.Compile(rule.MatchSpec.MessageRegex); err == nil {
				current.messageRegex = re
			}
		}
		if rule.MatchSpec.SourceRegex != "" {
			if re, err := regexp.Compile(rule.MatchSpec.SourceRegex); err == nil {
				current.sourceRegex = re
			}
		}
		if rule.MatchSpec.URLRegex != "" {
			if re, err := regexp.Compile(rule.MatchSpec.URLRegex); err == nil {
				current.urlRegex = re
			}
		}
		compiled[i] = current
	}
	nc.compiled = compiled
}
