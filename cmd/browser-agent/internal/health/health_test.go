// health_test.go — Unit tests for the health sub-package exported API.

package health

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Metrics
// ---------------------------------------------------------------------------

func TestNewMetrics(t *testing.T) {
	m := NewMetrics()
	if m == nil {
		t.Fatal("NewMetrics returned nil")
	}
	if m.GetTotalRequests() != 0 {
		t.Errorf("expected 0 total requests, got %d", m.GetTotalRequests())
	}
	if m.GetTotalErrors() != 0 {
		t.Errorf("expected 0 total errors, got %d", m.GetTotalErrors())
	}
}

func TestMetrics_IncrementRequest(t *testing.T) {
	m := NewMetrics()
	m.IncrementRequest("observe")
	m.IncrementRequest("observe")
	m.IncrementRequest("interact")

	if got := m.GetRequestCount("observe"); got != 2 {
		t.Errorf("observe request count: want 2, got %d", got)
	}
	if got := m.GetRequestCount("interact"); got != 1 {
		t.Errorf("interact request count: want 1, got %d", got)
	}
	if got := m.GetRequestCount("unknown"); got != 0 {
		t.Errorf("unknown tool request count: want 0, got %d", got)
	}
	if got := m.GetTotalRequests(); got != 3 {
		t.Errorf("total requests: want 3, got %d", got)
	}
}

func TestMetrics_IncrementError(t *testing.T) {
	m := NewMetrics()
	m.IncrementError("observe")
	m.IncrementError("observe")
	m.IncrementError("generate")

	if got := m.GetErrorCount("observe"); got != 2 {
		t.Errorf("observe error count: want 2, got %d", got)
	}
	if got := m.GetTotalErrors(); got != 3 {
		t.Errorf("total errors: want 3, got %d", got)
	}
}

func TestMetrics_GetUptime(t *testing.T) {
	m := NewMetrics()
	time.Sleep(5 * time.Millisecond)
	uptime := m.GetUptime()
	if uptime < 5*time.Millisecond {
		t.Errorf("uptime too short: %v", uptime)
	}
}

func TestMetrics_ConcurrentAccess(t *testing.T) {
	m := NewMetrics()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			m.IncrementRequest("tool")
		}()
		go func() {
			defer wg.Done()
			m.IncrementError("tool")
		}()
	}
	wg.Wait()
	if got := m.GetTotalRequests(); got != 100 {
		t.Errorf("concurrent requests: want 100, got %d", got)
	}
	if got := m.GetTotalErrors(); got != 100 {
		t.Errorf("concurrent errors: want 100, got %d", got)
	}
}

func TestMetrics_BuildAuditInfo(t *testing.T) {
	m := NewMetrics()
	m.IncrementRequest("observe")
	m.IncrementRequest("observe")
	m.IncrementRequest("interact")
	m.IncrementError("observe")

	info := m.BuildAuditInfo()
	if info.TotalCalls != 3 {
		t.Errorf("TotalCalls: want 3, got %d", info.TotalCalls)
	}
	if info.TotalErrors != 1 {
		t.Errorf("TotalErrors: want 1, got %d", info.TotalErrors)
	}
	// 1 error / 3 calls = ~33.33%
	if info.ErrorRatePct < 33.0 || info.ErrorRatePct > 34.0 {
		t.Errorf("ErrorRatePct: want ~33.33, got %f", info.ErrorRatePct)
	}
	if info.CallsPerTool["observe"] != 2 {
		t.Errorf("CallsPerTool[observe]: want 2, got %d", info.CallsPerTool["observe"])
	}
}

func TestMetrics_BuildAuditInfo_ZeroCalls(t *testing.T) {
	m := NewMetrics()
	info := m.BuildAuditInfo()
	if info.ErrorRatePct != 0 {
		t.Errorf("ErrorRatePct with zero calls should be 0, got %f", info.ErrorRatePct)
	}
}

// ---------------------------------------------------------------------------
// CalcUtilization
// ---------------------------------------------------------------------------

func TestCalcUtilization(t *testing.T) {
	tests := []struct {
		name     string
		entries  int
		capacity int
		want     float64
	}{
		{"empty", 0, 100, 0},
		{"half", 50, 100, 50},
		{"full", 100, 100, 100},
		{"over", 150, 100, 150},
		{"zero capacity", 10, 0, 0},
		{"negative capacity", 10, -1, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalcUtilization(tt.entries, tt.capacity)
			if got != tt.want {
				t.Errorf("CalcUtilization(%d, %d) = %f, want %f", tt.entries, tt.capacity, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// BuildUpgradeInfo
// ---------------------------------------------------------------------------

type fakeUpgradeProvider struct {
	pending    bool
	newVer     string
	detectedAt time.Time
}

func (f *fakeUpgradeProvider) UpgradeInfo() (bool, string, time.Time) {
	return f.pending, f.newVer, f.detectedAt
}

func TestBuildUpgradeInfo_Nil(t *testing.T) {
	if got := BuildUpgradeInfo(nil); got != nil {
		t.Errorf("expected nil for nil provider, got %+v", got)
	}
}

func TestBuildUpgradeInfo_NotPending(t *testing.T) {
	p := &fakeUpgradeProvider{pending: false}
	if got := BuildUpgradeInfo(p); got != nil {
		t.Errorf("expected nil when not pending, got %+v", got)
	}
}

func TestBuildUpgradeInfo_Pending(t *testing.T) {
	now := time.Date(2026, 3, 29, 12, 0, 0, 0, time.UTC)
	p := &fakeUpgradeProvider{pending: true, newVer: "0.9.0", detectedAt: now}
	got := BuildUpgradeInfo(p)
	if got == nil {
		t.Fatal("expected non-nil UpgradeInfo")
	}
	if !got.Pending {
		t.Error("expected Pending to be true")
	}
	if got.NewVersion != "0.9.0" {
		t.Errorf("NewVersion: want 0.9.0, got %s", got.NewVersion)
	}
	if got.DetectedAt != "2026-03-29T12:00:00Z" {
		t.Errorf("DetectedAt: want 2026-03-29T12:00:00Z, got %s", got.DetectedAt)
	}
}

// ---------------------------------------------------------------------------
// DoctorCheck type construction
// ---------------------------------------------------------------------------

func TestDoctorCheck_Construction(t *testing.T) {
	check := DoctorCheck{
		Name:   "test_check",
		Status: "pass",
		Detail: "Everything is fine",
		Fix:    "",
	}
	if check.Name != "test_check" {
		t.Errorf("Name: want test_check, got %s", check.Name)
	}
	if check.Status != "pass" {
		t.Errorf("Status: want pass, got %s", check.Status)
	}
}

// ---------------------------------------------------------------------------
// EvaluateFastPathFailureThreshold
// ---------------------------------------------------------------------------

func TestEvaluateFastPathFailureThreshold(t *testing.T) {
	tests := []struct {
		name            string
		summary         FastPathTelemetrySummary
		minSamples      int
		maxFailureRatio float64
		wantErr         bool
	}{
		{
			name:            "negative ratio skips check",
			summary:         FastPathTelemetrySummary{Total: 100, Failure: 50},
			minSamples:      10,
			maxFailureRatio: -1,
			wantErr:         false,
		},
		{
			name:            "ratio > 1 is invalid",
			summary:         FastPathTelemetrySummary{Total: 100},
			minSamples:      10,
			maxFailureRatio: 1.5,
			wantErr:         true,
		},
		{
			name:            "minSamples < 1 is invalid",
			summary:         FastPathTelemetrySummary{Total: 100},
			minSamples:      0,
			maxFailureRatio: 0.5,
			wantErr:         true,
		},
		{
			name:            "insufficient samples",
			summary:         FastPathTelemetrySummary{Total: 5},
			minSamples:      10,
			maxFailureRatio: 0.5,
			wantErr:         true,
		},
		{
			name:            "within threshold",
			summary:         FastPathTelemetrySummary{Total: 100, Failure: 5},
			minSamples:      10,
			maxFailureRatio: 0.1,
			wantErr:         false,
		},
		{
			name:            "exceeds threshold",
			summary:         FastPathTelemetrySummary{Total: 100, Failure: 20},
			minSamples:      10,
			maxFailureRatio: 0.1,
			wantErr:         true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := EvaluateFastPathFailureThreshold(tt.summary, tt.minSamples, tt.maxFailureRatio)
			if (err != nil) != tt.wantErr {
				t.Errorf("wantErr=%v, got err=%v", tt.wantErr, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// SummarizeFastPathTelemetryLog
// ---------------------------------------------------------------------------

func TestSummarizeFastPathTelemetryLog_ZeroMaxLines(t *testing.T) {
	s := SummarizeFastPathTelemetryLog("/nonexistent", 0)
	if s.Total != 0 {
		t.Errorf("expected 0 total for maxLines=0, got %d", s.Total)
	}
}

func TestSummarizeFastPathTelemetryLog_NonexistentFile(t *testing.T) {
	s := SummarizeFastPathTelemetryLog("/tmp/nonexistent-telemetry-log-12345", 100)
	if s.Total != 0 {
		t.Errorf("expected 0 total for missing file, got %d", s.Total)
	}
}

func TestSummarizeFastPathTelemetryLog_ValidFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "telemetry.jsonl")
	content := `{"event":"bridge_fastpath_method","success":true,"method":"GET"}
{"event":"bridge_fastpath_method","success":false,"method":"POST","error_code":500}
{"event":"bridge_fastpath_method","success":true,"method":"GET"}
{"event":"other_event","success":true}
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	s := SummarizeFastPathTelemetryLog(path, 100)
	if s.Total != 3 {
		t.Errorf("Total: want 3, got %d", s.Total)
	}
	if s.Success != 2 {
		t.Errorf("Success: want 2, got %d", s.Success)
	}
	if s.Failure != 1 {
		t.Errorf("Failure: want 1, got %d", s.Failure)
	}
	if s.Methods["GET"] != 2 {
		t.Errorf("Methods[GET]: want 2, got %d", s.Methods["GET"])
	}
	if s.ErrorCodes[500] != 1 {
		t.Errorf("ErrorCodes[500]: want 1, got %d", s.ErrorCodes[500])
	}
}

// ---------------------------------------------------------------------------
// BuildMemoryInfo (nil capture)
// ---------------------------------------------------------------------------

func TestBuildMemoryInfo_NilCapture(t *testing.T) {
	info := BuildMemoryInfo(nil)
	// Should still return valid info from runtime.MemStats
	if info.SysMB <= 0 {
		t.Error("expected positive SysMB from runtime stats")
	}
}

// ---------------------------------------------------------------------------
// Response type construction
// ---------------------------------------------------------------------------

func TestMCPHealthResponse_Construction(t *testing.T) {
	resp := MCPHealthResponse{
		Server: ServerInfo{Version: "0.8.5", PID: 1234},
		Memory: MemoryInfo{CurrentMB: 10.5},
	}
	if resp.Server.Version != "0.8.5" {
		t.Errorf("Version: want 0.8.5, got %s", resp.Server.Version)
	}
	if resp.Upgrade != nil {
		t.Error("expected nil Upgrade by default")
	}
}

func TestCommandExecutionInfo_Defaults(t *testing.T) {
	info := CommandExecutionInfo{Ready: true, Status: "pass"}
	if !info.Ready {
		t.Error("expected Ready=true")
	}
	if info.QueueDepth != 0 {
		t.Error("expected QueueDepth=0 by default")
	}
}
