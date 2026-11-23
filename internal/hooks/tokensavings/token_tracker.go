// token_tracker.go — Tracks token savings from output compression and reports on shutdown.

package tracking

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// CategoryStats holds compression statistics for a single category.
type CategoryStats struct {
	TokensSaved  int `json:"tokens_saved"`
	TokensBefore int `json:"tokens_before"`
	TokensAfter  int `json:"tokens_after"`
	Count        int `json:"count"`
}

// SessionStats holds aggregated session-level statistics.
type SessionStats struct {
	TotalTokensSaved  int                      `json:"total_tokens_saved"`
	TotalCompressions int                      `json:"total_compressions"`
	CompressionPct    float64                  `json:"compression_pct"`
	ByCategory        map[string]CategoryStats `json:"by_category"`
}

// LifetimeStats holds accumulated statistics across all sessions.
type LifetimeStats struct {
	TotalTokensSaved  int                      `json:"total_tokens_saved"`
	TotalSessions     int                      `json:"total_sessions"`
	TotalCompressions int                      `json:"total_compressions"`
	FirstSession      string                   `json:"first_session"`
	LastSession       string                   `json:"last_session"`
	ByCategory        map[string]CategoryStats `json:"by_category"`
}

// TokenTracker records token savings from output compression with concurrent-safe counters.
type TokenTracker struct {
	mu         sync.Mutex
	categories map[string]*CategoryStats
	nowFunc    func() time.Time // injectable clock for deterministic tests
}

// NewTokenTracker creates a new TokenTracker with initialized counters.
func NewTokenTracker() *TokenTracker {
	return &TokenTracker{
		categories: make(map[string]*CategoryStats),
		nowFunc:    time.Now,
	}
}

// Record adds a compression event for the given category.
func (t *TokenTracker) Record(category string, tokensBefore, tokensAfter int) {
	saved := tokensBefore - tokensAfter

	t.mu.Lock()
	defer t.mu.Unlock()

	cat, ok := t.categories[category]
	if !ok {
		cat = &CategoryStats{}
		t.categories[category] = cat
	}
	cat.TokensBefore += tokensBefore
	cat.TokensAfter += tokensAfter
	cat.TokensSaved += saved
	cat.Count++
}

// GetSessionStats returns structured statistics for the current session.
func (t *TokenTracker) GetSessionStats() SessionStats {
	t.mu.Lock()
	defer t.mu.Unlock()

	stats := SessionStats{
		ByCategory: make(map[string]CategoryStats, len(t.categories)),
	}

	var totalBefore int
	for name, cat := range t.categories {
		stats.ByCategory[name] = CategoryStats{
			TokensSaved:  cat.TokensSaved,
			TokensBefore: cat.TokensBefore,
			TokensAfter:  cat.TokensAfter,
			Count:        cat.Count,
		}
		stats.TotalTokensSaved += cat.TokensSaved
		stats.TotalCompressions += cat.Count
		totalBefore += cat.TokensBefore
	}

	if totalBefore > 0 {
		stats.CompressionPct = float64(stats.TotalTokensSaved) / float64(totalBefore) * 100
	}

	return stats
}

// categoryDisplayName converts internal category names to human-readable labels.
func categoryDisplayName(category string) string {
	switch category {
	case "test_output":
		return "Test output"
	case "build_output":
		return "Build output"
	case "search":
		return "Search"
	case "generic", "generic_truncation":
		return "Generic"
	default:
		return category
	}
}

// formatNumber inserts commas into an integer for human-readable display.
func formatNumber(n int) string {
	if n < 0 {
		return "-" + formatNumber(-n)
	}
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}

	var result strings.Builder
	remainder := len(s) % 3
	if remainder > 0 {
		result.WriteString(s[:remainder])
	}
	for i := remainder; i < len(s); i += 3 {
		if result.Len() > 0 {
			result.WriteByte(',')
		}
		result.WriteString(s[i : i+3])
	}
	return result.String()
}

// GetSessionSummary returns a human-readable summary of token savings for stderr output.
// Returns empty string if no savings were recorded.
func (t *TokenTracker) GetSessionSummary() string {
	stats := t.GetSessionStats()
	if stats.TotalCompressions == 0 {
		return ""
	}

	var sb strings.Builder

	fmt.Fprintf(&sb, "Kaboom saved ~%s tokens this session (%.0f%% compression across %d compressions)\n",
		formatNumber(stats.TotalTokensSaved),
		stats.CompressionPct,
		stats.TotalCompressions,
	)

	// Sort categories by tokens saved (descending) for consistent output.
	type catEntry struct {
		name  string
		stats CategoryStats
	}
	entries := make([]catEntry, 0, len(stats.ByCategory))
	for name, cat := range stats.ByCategory {
		entries = append(entries, catEntry{name: name, stats: cat})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].stats.TokensSaved > entries[j].stats.TokensSaved
	})

	// Find the longest display name for alignment.
	maxNameLen := 0
	for _, e := range entries {
		displayName := categoryDisplayName(e.name)
		if len(displayName) > maxNameLen {
			maxNameLen = len(displayName)
		}
	}

	for _, e := range entries {
		displayName := categoryDisplayName(e.name)
		var pct float64
		if e.stats.TokensBefore > 0 {
			pct = float64(e.stats.TokensSaved) / float64(e.stats.TokensBefore) * 100
		}
		padding := strings.Repeat(" ", maxNameLen-len(displayName))
		fmt.Fprintf(&sb, "  %s:%s %6s → %s (%.0f%%)\n",
			displayName,
			padding,
			formatNumber(e.stats.TokensBefore),
			formatNumber(e.stats.TokensAfter),
			pct,
		)
	}

	return sb.String()
}

// SaveLifetime merges the current session stats into a lifetime stats file at path.
func (t *TokenTracker) SaveLifetime(path string) error {
	// Ensure parent directory exists.
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("SaveLifetime: cannot create directory %s. Check permissions", dir)
	}

	// Load existing stats (if any).
	existing, err := LoadLifetime(path)
	if err != nil {
		return fmt.Errorf("SaveLifetime: cannot read existing stats. %v", err)
	}

	session := t.GetSessionStats()
	now := t.nowFunc().UTC().Format(time.RFC3339)

	// Merge session into existing.
	existing.TotalTokensSaved += session.TotalTokensSaved
	existing.TotalSessions++
	existing.TotalCompressions += session.TotalCompressions
	existing.LastSession = now
	if existing.FirstSession == "" {
		existing.FirstSession = now
	}

	if existing.ByCategory == nil {
		existing.ByCategory = make(map[string]CategoryStats)
	}
	for name, cat := range session.ByCategory {
		prev := existing.ByCategory[name]
		prev.TokensSaved += cat.TokensSaved
		prev.TokensBefore += cat.TokensBefore
		prev.TokensAfter += cat.TokensAfter
		prev.Count += cat.Count
		existing.ByCategory[name] = prev
	}

	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return fmt.Errorf("SaveLifetime: cannot marshal stats. %v", err)
	}

	// #nosec G306 -- stats file is owner-only (0600) for privacy
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("SaveLifetime: cannot write %s. Check permissions", path)
	}

	return nil
}

// LoadLifetime reads lifetime stats from path. Returns zero-value LifetimeStats if file does not exist.
func LoadLifetime(path string) (LifetimeStats, error) {
	var stats LifetimeStats

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return stats, nil
		}
		return stats, fmt.Errorf("LoadLifetime: cannot read %s. Check file exists and is readable", path)
	}

	if err := json.Unmarshal(data, &stats); err != nil {
		return stats, fmt.Errorf("LoadLifetime: invalid JSON in %s. File may be corrupted", path)
	}

	return stats, nil
}

// Reset clears all session counters.
func (t *TokenTracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.categories = make(map[string]*CategoryStats)
}
