// Purpose: observe(what:"summarized_logs") — fingerprints, groups and ranks log entries into an aggregated view.
// Why: The handler, the fingerprint regexes, the grouping pass and the periodicity detector are one algorithm;
// they were split across four files even though every symbol in three of them served only this handler.
// Docs: docs/features/feature/observe/index.md

package logs

import (
	"encoding/json"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe/core"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// GetSummarizedLogs handles observe(what="summarized_logs").
func GetSummarizedLogs(deps core.Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	params := parseSummarizedLogsParams(args)
	if params.Scope != "current_page" && params.Scope != "all" {
		return mcp.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: mcp.StructuredErrorResponse(
				mcp.ErrInvalidParam,
				"Invalid scope: "+params.Scope,
				"Use 'current_page' (default) or 'all'",
				mcp.WithParam("scope"),
			),
		}
	}

	_, trackedTabID, trackedTabURL := deps.Capture.Extension().GetTrackingStatus()
	if params.URL == "" && params.Scope == "current_page" && trackedTabURL != "" {
		params.URL = trackedTabURL
	}
	rawEntries, _ := deps.LogEntries()
	views, noiseSuppressed := filterSummarizedLogViews(rawEntries, deps, params, trackedTabID)

	groups, anomalies := groupLogs(views, params.MinGroupSize)
	if groups == nil {
		groups = []LogGroup{}
	}
	if anomalies == nil {
		anomalies = []LogAnomaly{}
	}
	detectPeriodicity(groups)

	timeStart, timeEnd := summarizedLogsTimeRange(views)
	summary := summarizedLogsSummary(views, groups, anomalies, noiseSuppressed, timeStart, timeEnd)
	responseMeta := core.BuildResponseMetadata(deps.Capture, time.Time{})

	return mcp.Succeed(req, "Summarized logs", map[string]any{
		"groups":    cleanSummarizedLogGroups(groups),
		"anomalies": anomalies,
		"summary":   summary,
		"metadata":  responseMeta,
	})
}

type summarizedLogsParams struct {
	Limit        int
	MinLevel     string
	Level        string
	Source       string
	URL          string
	Scope        string
	MinGroupSize int
}

func parseSummarizedLogsParams(args json.RawMessage) summarizedLogsParams {
	var params summarizedLogsParams
	mcp.LenientUnmarshal(args, &params)
	if params.Scope == "" {
		params.Scope = "current_page"
	}
	params.Limit = core.ClampLimit(params.Limit, 100)
	if params.MinGroupSize <= 0 {
		params.MinGroupSize = 2
	}
	return params
}

func filterSummarizedLogViews(rawEntries []types.LogEntry, deps core.Deps, params summarizedLogsParams, trackedTabID int) ([]logEntryView, int) {
	noiseSuppressed := 0
	views := make([]logEntryView, 0, min(params.Limit, len(rawEntries)))
	count := 0

	// Scan newest first (tail to head), up to limit.
	for i := len(rawEntries) - 1; i >= 0 && count < params.Limit; i-- {
		entry := rawEntries[i]
		entryType, _ := entry["type"].(string)
		if entryType == "lifecycle" || entryType == "tracking" || entryType == "extension" {
			continue
		}
		if deps.IsConsoleNoise(entry) {
			noiseSuppressed++
			continue
		}
		if params.Scope == "current_page" && trackedTabID != 0 {
			entryTabID, _ := entry["tabId"].(float64)
			if int(entryTabID) != trackedTabID {
				continue
			}
		}
		level, _ := entry["level"].(string)
		if params.Level != "" && level != params.Level {
			continue
		}
		if params.MinLevel != "" && core.LogLevelRank(level) < core.LogLevelRank(params.MinLevel) {
			continue
		}
		if params.Source != "" {
			source, _ := entry["source"].(string)
			if source != params.Source {
				continue
			}
		}
		if params.URL != "" {
			entryURL, _ := entry["url"].(string)
			if !core.ContainsIgnoreCase(entryURL, params.URL) {
				continue
			}
		}

		message, _ := entry["message"].(string)
		source, _ := entry["source"].(string)
		url, _ := entry["url"].(string)
		ts, _ := entry["ts"].(string)
		views = append(views, logEntryView{
			Level:   level,
			Message: message,
			Source:  source,
			URL:     url,
			Line:    entry["line"],
			Column:  entry["column"],
			TS:      ts,
			TabID:   entry["tabId"],
		})
		count++
	}

	return views, noiseSuppressed
}

func cleanSummarizedLogGroups(groups []LogGroup) []map[string]any {
	cleanGroups := make([]map[string]any, len(groups))
	for i, g := range groups {
		m := map[string]any{
			"fingerprint":     g.Fingerprint,
			"sample_message":  g.SampleMessage,
			"count":           g.Count,
			"level_breakdown": g.LevelBreakdown,
			"first_seen":      g.FirstSeen,
			"last_seen":       g.LastSeen,
			"is_periodic":     g.IsPeriodic,
			"source":          g.Source,
		}
		if g.PeriodSeconds > 0 {
			m["period_seconds"] = g.PeriodSeconds
		}
		if len(g.Sources) > 1 {
			m["sources"] = g.Sources
		}
		cleanGroups[i] = m
	}
	return cleanGroups
}

func summarizedLogsSummary(views []logEntryView, groups []LogGroup, anomalies []LogAnomaly, noiseSuppressed int, timeStart, timeEnd string) map[string]any {
	totalEntries := len(views)
	compressionRatio := 0.0
	if totalEntries > 0 {
		compressionRatio = 1.0 - float64(len(groups)+len(anomalies))/float64(totalEntries)
		compressionRatio = math.Round(compressionRatio*100) / 100
	}

	return map[string]any{
		"total_entries":     totalEntries,
		"groups":            len(groups),
		"anomalies":         len(anomalies),
		"noise_suppressed":  noiseSuppressed,
		"compression_ratio": compressionRatio,
		"time_range": map[string]any{
			"start": timeStart,
			"end":   timeEnd,
		},
	}
}

func summarizedLogsTimeRange(views []logEntryView) (string, string) {
	var timeStart, timeEnd string
	for _, v := range views {
		if v.TS == "" {
			continue
		}
		if timeStart == "" || v.TS < timeStart {
			timeStart = v.TS
		}
		if timeEnd == "" || v.TS > timeEnd {
			timeEnd = v.TS
		}
	}
	return timeStart, timeEnd
}

// Pre-compiled fingerprinting regexes (initialized once at package load).
var (
	reANSI       = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	reUUID       = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	reHexHash    = regexp.MustCompile(`\b[0-9a-fA-F]{8,}\b`)
	reNumbers    = regexp.MustCompile(`\d{3,}`)
	reLongQuoted = regexp.MustCompile(`"[^"]{21,}"`)
	reURL        = regexp.MustCompile(`https?://\S+`)
	reTimestamp  = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}[^\s]*`)
	rePath       = regexp.MustCompile(`/[\w._-]+(/[\w._-]+)+`)
	reWhitespace = regexp.MustCompile(`\s+`)
	reSlugDup    = regexp.MustCompile(`_+`)
)

const maxFingerprintLen = 64

// logEntryView is a lightweight struct for grouping operations,
// extracted from the raw map[string]any log entries.
type logEntryView struct {
	Level   string
	Message string
	Source  string
	URL     string
	Line    any
	Column  any
	TS      string
	TabID   any
}

// LogGroup represents a group of repeated log entries.
type LogGroup struct {
	Fingerprint    string         `json:"fingerprint"`
	SampleMessage  string         `json:"sample_message"`
	Count          int            `json:"count"`
	LevelBreakdown map[string]int `json:"level_breakdown"`
	FirstSeen      string         `json:"first_seen"`
	LastSeen       string         `json:"last_seen"`
	IsPeriodic     bool           `json:"is_periodic"`
	PeriodSeconds  float64        `json:"period_seconds,omitempty"`
	Source         string         `json:"source"`
	Sources        []string       `json:"sources,omitempty"`
	timestamps     []string       // internal, for periodicity detection
	sourceCounts   map[string]int // internal, for primary source detection
}

// LogAnomaly represents a rare/unique log entry (the signal).
type LogAnomaly struct {
	Level   string `json:"level"`
	Message string `json:"message"`
	Source  string `json:"source,omitempty"`
	URL     string `json:"url,omitempty"`
	Line    any    `json:"line,omitempty"`
	Column  any    `json:"column,omitempty"`
	TS      string `json:"timestamp,omitempty"`
	TabID   any    `json:"tab_id,omitempty"`
}

// fingerprintMessage normalizes a log message into a stable fingerprint.
func fingerprintMessage(msg string) string {
	if msg == "" {
		return ""
	}
	s := msg
	s = reANSI.ReplaceAllString(s, "")
	s = reUUID.ReplaceAllString(s, "{uuid}")
	s = reTimestamp.ReplaceAllString(s, "{timestamp}")
	s = reURL.ReplaceAllString(s, "{url}")
	s = reHexHash.ReplaceAllString(s, "{hash}")
	s = reNumbers.ReplaceAllString(s, "{n}")
	s = reLongQuoted.ReplaceAllString(s, `"{string}"`)
	s = rePath.ReplaceAllString(s, "{path}")
	s = reWhitespace.ReplaceAllString(s, " ")
	s = strings.TrimSpace(s)
	return slugify(s)
}

// slugify converts a normalized message into a URL-safe slug.
func slugify(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return '_'
	}, s)
	s = reSlugDup.ReplaceAllString(s, "_")
	s = strings.Trim(s, "_")
	if len(s) > maxFingerprintLen {
		s = s[:maxFingerprintLen]
	}
	return s
}

// groupLogs performs single-pass grouping of log entries by fingerprint.
// Returns groups (entries with count >= minGroupSize) and anomalies (below threshold).
func groupLogs(entries []logEntryView, minGroupSize int) ([]LogGroup, []LogAnomaly) {
	if len(entries) == 0 {
		return nil, nil
	}

	type pendingSingle struct {
		entry logEntryView
	}

	groups := make(map[string]*LogGroup)
	pending := make(map[string]*pendingSingle)
	var groupOrder []string // preserve insertion order

	for _, e := range entries {
		fp := fingerprintMessage(e.Message)
		if fp == "" {
			continue
		}

		if g, ok := groups[fp]; ok {
			// Existing group — increment
			g.Count++
			g.LevelBreakdown[e.Level]++
			g.SampleMessage = e.Message // most recent
			if e.TS != "" && (g.FirstSeen == "" || e.TS < g.FirstSeen) {
				g.FirstSeen = e.TS
			}
			if e.TS != "" && e.TS > g.LastSeen {
				g.LastSeen = e.TS
			}
			if e.TS != "" {
				g.timestamps = append(g.timestamps, e.TS)
			}
			if e.Source != "" {
				g.sourceCounts[e.Source]++
			}
			continue
		}

		if p, ok := pending[fp]; ok {
			// Second occurrence — promote to group
			g := &LogGroup{
				Fingerprint:    fp,
				SampleMessage:  e.Message,
				Count:          2,
				LevelBreakdown: map[string]int{p.entry.Level: 1, e.Level: 1},
				FirstSeen:      minStr(p.entry.TS, e.TS),
				LastSeen:       maxStr(p.entry.TS, e.TS),
				sourceCounts:   make(map[string]int),
			}
			if p.entry.Level == e.Level {
				g.LevelBreakdown[e.Level] = 2
			}
			if p.entry.TS != "" {
				g.timestamps = append(g.timestamps, p.entry.TS)
			}
			if e.TS != "" {
				g.timestamps = append(g.timestamps, e.TS)
			}
			if p.entry.Source != "" {
				g.sourceCounts[p.entry.Source]++
			}
			if e.Source != "" {
				g.sourceCounts[e.Source]++
			}
			groups[fp] = g
			groupOrder = append(groupOrder, fp)
			delete(pending, fp)
			continue
		}

		// First occurrence — hold in pending
		pending[fp] = &pendingSingle{entry: e}
	}

	// Pending singles → groups (if minGroupSize <= 1) or anomalies
	var anomalies []LogAnomaly
	for fp, p := range pending {
		if minGroupSize <= 1 {
			g := &LogGroup{
				Fingerprint:    fp,
				SampleMessage:  p.entry.Message,
				Count:          1,
				LevelBreakdown: map[string]int{p.entry.Level: 1},
				FirstSeen:      p.entry.TS,
				LastSeen:       p.entry.TS,
				Source:         p.entry.Source,
				sourceCounts:   make(map[string]int),
			}
			if p.entry.TS != "" {
				g.timestamps = append(g.timestamps, p.entry.TS)
			}
			if p.entry.Source != "" {
				g.sourceCounts[p.entry.Source]++
			}
			groups[fp] = g
			groupOrder = append(groupOrder, fp)
		} else {
			anomalies = append(anomalies, logEntryToAnomaly(p.entry))
		}
	}

	// Build result: groups >= minGroupSize go to groups
	var resultGroups []LogGroup
	for _, fp := range groupOrder {
		g := groups[fp]
		if g.Count < minGroupSize {
			continue
		}
		// Resolve sources
		g.Source, g.Sources = resolveSources(g.sourceCounts)
		resultGroups = append(resultGroups, *g)
	}

	// Sort groups by count desc
	sort.Slice(resultGroups, func(i, j int) bool {
		return resultGroups[i].Count > resultGroups[j].Count
	})

	// Sort anomalies by severity desc, then timestamp desc
	sort.Slice(anomalies, func(i, j int) bool {
		ri := core.LogLevelRank(anomalies[i].Level)
		rj := core.LogLevelRank(anomalies[j].Level)
		if ri != rj {
			return ri > rj
		}
		return anomalies[i].TS > anomalies[j].TS
	})

	return resultGroups, anomalies
}

// detectPeriodicity checks each group for regular intervals.
func detectPeriodicity(groups []LogGroup) {
	for i := range groups {
		g := &groups[i]
		if len(g.timestamps) < 3 {
			continue
		}
		// Parse and sort timestamps
		times := make([]time.Time, 0, len(g.timestamps))
		for _, ts := range g.timestamps {
			t, err := time.Parse(time.RFC3339, ts)
			if err == nil {
				times = append(times, t)
			}
		}
		if len(times) < 3 {
			continue
		}
		sort.Slice(times, func(a, b int) bool { return times[a].Before(times[b]) })

		// Compute intervals
		intervals := make([]float64, 0, len(times)-1)
		for j := 1; j < len(times); j++ {
			intervals = append(intervals, times[j].Sub(times[j-1]).Seconds())
		}

		meanValue := mean(intervals)
		if meanValue <= 0 {
			continue
		}
		stddevValue := stddev(intervals, meanValue)
		if stddevValue/meanValue < 0.20 {
			g.IsPeriodic = true
			g.PeriodSeconds = math.Round(meanValue*10) / 10 // round to 1 decimal
		}
	}
}

func logEntryToAnomaly(e logEntryView) LogAnomaly {
	return LogAnomaly{
		Level:   e.Level,
		Message: e.Message,
		Source:  e.Source,
		URL:     e.URL,
		Line:    e.Line,
		Column:  e.Column,
		TS:      e.TS,
		TabID:   e.TabID,
	}
}

func resolveSources(counts map[string]int) (primary string, all []string) {
	if len(counts) == 0 {
		return "", nil
	}
	maxCount := 0
	for src, c := range counts {
		all = append(all, src)
		if c > maxCount {
			maxCount = c
			primary = src
		}
	}
	sort.Strings(all)
	if len(all) <= 1 {
		return primary, nil // omit sources array for single source
	}
	return primary, all
}

func minStr(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	if a < b {
		return a
	}
	return b
}

func maxStr(a, b string) string {
	if a > b {
		return a
	}
	return b
}

func mean(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

func stddev(vals []float64, meanValue float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sumSq := 0.0
	for _, v := range vals {
		delta := v - meanValue
		sumSq += delta * delta
	}
	return math.Sqrt(sumSq / float64(len(vals)))
}
