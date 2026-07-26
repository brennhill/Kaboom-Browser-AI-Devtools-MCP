// Purpose: Tests for streaming alert event generation and throttling.
// Docs: docs/features/feature/context-streaming/index.md

// alertbuf_test.go — Unit tests for AlertBuffer methods and pure alert processing functions.
package alertbuf

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

func TestDeduplicateCorrelateAndSortAlerts(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 2, 11, 8, 0, 0, 0, time.UTC)
	raw := []types.Alert{
		{
			Severity:  "error",
			Category:  "regression",
			Title:     "Request failed",
			Detail:    "500 on /api",
			Timestamp: base.Format(time.RFC3339),
			Source:    "network",
		},
		{
			Severity:  "warning",
			Category:  "anomaly",
			Title:     "Spike",
			Detail:    "error burst",
			Timestamp: base.Add(2 * time.Second).Format(time.RFC3339),
			Source:    "anomaly_detector",
		},
		{
			Severity:  "info",
			Category:  "ci",
			Title:     "CI failure",
			Detail:    "pipeline failed",
			Timestamp: base.Add(1 * time.Second).Format(time.RFC3339),
			Source:    "ci_webhook",
		},
		{
			Severity:  "info",
			Category:  "ci",
			Title:     "CI failure",
			Detail:    "pipeline failed again",
			Timestamp: base.Add(3 * time.Second).Format(time.RFC3339),
			Source:    "ci_webhook",
		},
	}

	deduped := DeduplicateAlerts(raw)
	if len(deduped) != 3 {
		t.Fatalf("DeduplicateAlerts len = %d, want 3", len(deduped))
	}

	var ci types.Alert
	for _, a := range deduped {
		if a.Category == "ci" {
			ci = a
			break
		}
	}
	if ci.Count != 2 {
		t.Fatalf("deduped ci count = %d, want 2", ci.Count)
	}

	correlated := CorrelateAlerts(deduped)
	if len(correlated) != 2 {
		t.Fatalf("CorrelateAlerts len = %d, want 2", len(correlated))
	}

	SortAlertsByPriority(correlated)
	if correlated[0].Category != "regression" {
		t.Fatalf("highest priority category = %q, want regression", correlated[0].Category)
	}
	if !strings.Contains(correlated[0].Title, "Correlated:") {
		t.Fatalf("expected merged correlation title, got %q", correlated[0].Title)
	}
}

func TestCanCorrelateAndMergeAlerts(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 2, 11, 8, 5, 0, 0, time.UTC)
	reg := types.Alert{
		Severity:  "warning",
		Category:  "regression",
		Title:     "R",
		Detail:    "d1",
		Timestamp: base.Format(time.RFC3339),
		Source:    "src1",
	}
	anom := types.Alert{
		Severity:  "error",
		Category:  "anomaly",
		Title:     "A",
		Detail:    "d2",
		Timestamp: base.Add(3 * time.Second).Format(time.RFC3339),
		Source:    "src2",
	}

	if !CanCorrelate(reg, anom) {
		t.Fatal("expected regression+anomaly within window to correlate")
	}

	merged := MergeAlerts(reg, anom)
	if merged.Severity != "error" {
		t.Fatalf("merged severity = %q, want error", merged.Severity)
	}
	if !strings.Contains(merged.Title, "Correlated:") {
		t.Fatalf("merged title = %q, want correlated prefix", merged.Title)
	}
	if merged.Source != reg.Source {
		t.Fatalf("merged source = %q, want %q", merged.Source, reg.Source)
	}

	other := types.Alert{Category: "ci", Timestamp: base.Format(time.RFC3339)}
	if CanCorrelate(reg, other) {
		t.Fatal("regression+ci should not correlate")
	}

	badTS := types.Alert{Category: "anomaly", Timestamp: "not-a-time"}
	if CanCorrelate(reg, badTS) {
		t.Fatal("invalid timestamp should not correlate")
	}
}

func TestBuildAlertSummaryAndFormatBlock(t *testing.T) {
	t.Parallel()

	alerts := []types.Alert{
		{Category: "regression"},
		{Category: "anomaly"},
		{Category: "ci"},
		{Category: "ci"},
	}

	summary := BuildAlertSummary(alerts)
	if !strings.Contains(summary, "4 alerts:") {
		t.Fatalf("summary = %q, want count prefix", summary)
	}
	if !strings.Contains(summary, "1 regression") ||
		!strings.Contains(summary, "1 anomaly") ||
		!strings.Contains(summary, "2 ci") {
		t.Fatalf("summary categories missing: %q", summary)
	}

	block := FormatAlertsBlock(alerts)
	if !strings.Contains(block, "--- ALERTS (4) ---") {
		t.Fatalf("alerts block missing header: %q", block)
	}
	if !strings.Contains(block, "4 alerts:") {
		t.Fatalf("alerts block should include summary for >=4 alerts: %q", block)
	}

	shortBlock := FormatAlertsBlock(alerts[:2])
	if strings.Contains(shortBlock, "alerts:") {
		t.Fatalf("alerts block should not include summary for <=3 alerts: %q", shortBlock)
	}
}

func TestAddAlertAndDrainAlerts(t *testing.T) {
	base := time.Date(2026, 2, 11, 8, 10, 0, 0, time.UTC)
	ab := NewAlertBuffer()

	for i := 0; i < AlertBufferCap+1; i++ {
		ab.AddAlert(types.Alert{
			Severity:  "info",
			Category:  "ci",
			Title:     "alert-" + string(rune('a'+(i%26))),
			Timestamp: base.Add(time.Duration(i) * time.Second).Format(time.RFC3339),
			Source:    "test",
		})
	}

	ab.Mu.Lock()
	if len(ab.Alerts) != AlertBufferCap {
		ab.Mu.Unlock()
		t.Fatalf("alerts len after eviction = %d, want %d", len(ab.Alerts), AlertBufferCap)
	}
	ab.Mu.Unlock()

	// Replace with deterministic set to exercise dedupe/correlation.
	ab.Mu.Lock()
	ab.Alerts = []types.Alert{
		{
			Severity:  "error",
			Category:  "regression",
			Title:     "R1",
			Detail:    "d1",
			Timestamp: base.Format(time.RFC3339),
			Source:    "network",
		},
		{
			Severity:  "warning",
			Category:  "anomaly",
			Title:     "A1",
			Detail:    "d2",
			Timestamp: base.Add(1 * time.Second).Format(time.RFC3339),
			Source:    "anomaly",
		},
		{
			Severity:  "info",
			Category:  "ci",
			Title:     "CI",
			Detail:    "pipeline [abc123]",
			Timestamp: base.Add(2 * time.Second).Format(time.RFC3339),
			Source:    "ci_webhook",
		},
		{
			Severity:  "info",
			Category:  "ci",
			Title:     "CI",
			Detail:    "pipeline [abc123]",
			Timestamp: base.Add(3 * time.Second).Format(time.RFC3339),
			Source:    "ci_webhook",
		},
	}
	ab.Mu.Unlock()

	drained := ab.DrainAlerts()
	if len(drained) != 2 {
		t.Fatalf("DrainAlerts len = %d, want 2", len(drained))
	}
	if drained[0].Category != "regression" {
		t.Fatalf("drained[0].Category = %q, want regression", drained[0].Category)
	}
	if drained[1].Category != "ci" || drained[1].Count != 2 {
		t.Fatalf("expected merged CI alert with count=2, got %+v", drained[1])
	}

	if second := ab.DrainAlerts(); second != nil {
		t.Fatalf("second drain should be nil after clear, got %+v", second)
	}
}

func TestBuildCIAlert(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 2, 11, 8, 15, 0, 0, time.UTC)
	alert := BuildCIAlert(types.CIResult{
		Status:     "failure",
		Source:     "github-actions",
		Commit:     "abc123",
		Summary:    "2 tests failed",
		ReceivedAt: now,
		Failures: []types.CIFailure{
			{Name: "auth flow"},
			{Name: "checkout flow"},
		},
	})

	if alert.Severity != "error" {
		t.Fatalf("failure status should map to error severity, got %q", alert.Severity)
	}
	if alert.Category != "ci" || alert.Source != "ci_webhook" {
		t.Fatalf("unexpected CI alert fields: %+v", alert)
	}
	if !strings.Contains(alert.Detail, "auth flow") || !strings.Contains(alert.Detail, "[abc123]") {
		t.Fatalf("CI alert detail missing failures/commit: %q", alert.Detail)
	}
	if alert.Timestamp != now.Format(time.RFC3339) {
		t.Fatalf("CI alert timestamp = %q, want %q", alert.Timestamp, now.Format(time.RFC3339))
	}
}

func TestRecordErrorForAnomaly(t *testing.T) {
	ab := NewAlertBuffer()
	base := time.Date(2026, 2, 11, 8, 20, 0, 0, time.UTC)

	// Build a burst where the second sample creates the first anomaly,
	// then ensure additional samples inside the bucket do not duplicate alerts.
	ab.RecordErrorForAnomaly(base)
	ab.RecordErrorForAnomaly(base.Add(1 * time.Second))
	ab.RecordErrorForAnomaly(base.Add(2 * time.Second))
	ab.RecordErrorForAnomaly(base.Add(3 * time.Second))

	ab.Mu.Lock()
	anomalyCount := 0
	for _, a := range ab.Alerts {
		if a.Category == "anomaly" && a.Source == "anomaly_detector" {
			anomalyCount++
		}
	}
	ab.Mu.Unlock()

	if anomalyCount != 1 {
		t.Fatalf("expected exactly one anomaly alert in the same bucket, got %d", anomalyCount)
	}

	// Additional sample inside the 10s dedupe bucket should not add a new anomaly.
	ab.RecordErrorForAnomaly(base.Add(9 * time.Second))

	ab.Mu.Lock()
	defer ab.Mu.Unlock()
	finalAnomalyCount := 0
	for _, a := range ab.Alerts {
		if a.Category == "anomaly" && a.Source == "anomaly_detector" {
			finalAnomalyCount++
		}
	}
	if finalAnomalyCount != 1 {
		t.Fatalf("expected one anomaly alert total, got %d", finalAnomalyCount)
	}
}

func TestFormatAlertsBlockProducesJSONPayload(t *testing.T) {
	t.Parallel()

	alerts := []types.Alert{
		{
			Severity:  "error",
			Category:  "regression",
			Title:     "boom",
			Timestamp: time.Date(2026, 2, 11, 8, 30, 0, 0, time.UTC).Format(time.RFC3339),
			Source:    "test",
		},
	}
	block := FormatAlertsBlock(alerts)

	lines := strings.Split(block, "\n")
	if len(lines) < 2 {
		t.Fatalf("FormatAlertsBlock returned too few lines: %q", block)
	}
	var parsed []types.Alert
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &parsed); err != nil {
		t.Fatalf("alerts JSON line not parseable: %v; block=%q", err, block)
	}
	if len(parsed) != 1 || parsed[0].Title != "boom" {
		t.Fatalf("unexpected parsed alerts JSON: %+v", parsed)
	}
}

func TestProcessCIResult(t *testing.T) {
	t.Parallel()

	ab := NewAlertBuffer()
	now := time.Date(2026, 2, 11, 8, 25, 0, 0, time.UTC)

	// First CI result creates an alert.
	result := types.CIResult{
		Status:     "failure",
		Source:     "gha",
		Commit:     "abc123",
		Summary:    "tests failed",
		ReceivedAt: now,
	}
	alert := ab.ProcessCIResult(result)
	if alert == nil {
		t.Fatal("first ProcessCIResult should return an alert")
	}
	if alert.Category != "ci" {
		t.Fatalf("expected ci category, got %q", alert.Category)
	}

	ab.Mu.Lock()
	if len(ab.CIResults) != 1 || len(ab.Alerts) != 1 {
		ab.Mu.Unlock()
		t.Fatalf("expected 1 CI result and 1 alert, got %d/%d", len(ab.CIResults), len(ab.Alerts))
	}
	ab.Mu.Unlock()

	// Idempotent update: same commit+status returns nil.
	updated := types.CIResult{
		Status:     "failure",
		Source:     "gha",
		Commit:     "abc123",
		Summary:    "updated summary",
		ReceivedAt: now.Add(time.Second),
	}
	alert2 := ab.ProcessCIResult(updated)
	if alert2 != nil {
		t.Fatal("idempotent update should return nil")
	}

	ab.Mu.Lock()
	if len(ab.CIResults) != 1 {
		ab.Mu.Unlock()
		t.Fatalf("should still have 1 CI result, got %d", len(ab.CIResults))
	}
	if ab.CIResults[0].Summary != "updated summary" {
		ab.Mu.Unlock()
		t.Fatalf("CI result not updated: %q", ab.CIResults[0].Summary)
	}
	ab.Mu.Unlock()
}
