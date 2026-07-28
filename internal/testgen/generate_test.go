// Purpose: Tests for Playwright test script generation.
// Docs: docs/features/feature/test-generation/index.md

// generate_test.go — Tests for test generation functions using mock DataProvider.
package testgen

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"strings"
	"testing"
	"time"
)

// mockDataProvider implements DataProvider for testing.
type mockDataProvider struct {
	logEntries []map[string]any
	actions    []types.EnhancedAction
	bodies     []types.NetworkBody
}

func (m *mockDataProvider) GetLogEntries() []map[string]any               { return m.logEntries }
func (m *mockDataProvider) GetAllEnhancedActions() []types.EnhancedAction { return m.actions }
func (m *mockDataProvider) GetNetworkBodies() []types.NetworkBody         { return m.bodies }

// ============================================
// Tests for FindTargetError
// ============================================

func TestFindTargetError_EmptyEntries(t *testing.T) {
	t.Parallel()
	dp := &mockDataProvider{}

	entry, id, ts := FindTargetError(dp, "")
	if entry != nil {
		t.Fatalf("expected nil entry, got %v", entry)
	}
	if id != "" || ts != 0 {
		t.Fatalf("expected empty results, got id=%q ts=%d", id, ts)
	}
}

func TestFindTargetError_NoErrorLevel(t *testing.T) {
	t.Parallel()
	dp := &mockDataProvider{
		logEntries: []map[string]any{
			{"level": "info", "message": "just info"},
			{"level": "warn", "message": "a warning"},
		},
	}

	entry, _, _ := FindTargetError(dp, "")
	if entry != nil {
		t.Fatalf("expected nil entry for no-error entries, got %v", entry)
	}
}

func TestFindTargetError_ReturnsLastError(t *testing.T) {
	t.Parallel()
	dp := &mockDataProvider{
		logEntries: []map[string]any{
			{"level": "error", "message": "first error", "error_id": "e1", "ts": "2024-01-01T00:00:01Z"},
			{"level": "info", "message": "info between"},
			{"level": "error", "message": "second error", "error_id": "e2", "ts": "2024-01-01T00:00:02Z"},
		},
	}

	entry, id, _ := FindTargetError(dp, "")
	if entry == nil {
		t.Fatal("expected non-nil entry")
	}
	msg, _ := entry["message"].(string)
	if msg != "second error" {
		t.Fatalf("message = %q, want %q", msg, "second error")
	}
	if id != "e2" {
		t.Fatalf("id = %q, want %q", id, "e2")
	}
}

func TestFindTargetError_SpecificErrorID(t *testing.T) {
	t.Parallel()
	dp := &mockDataProvider{
		logEntries: []map[string]any{
			{"level": "error", "message": "first error", "error_id": "e1", "ts": "2024-01-01T00:00:01Z"},
			{"level": "error", "message": "second error", "error_id": "e2", "ts": "2024-01-01T00:00:02Z"},
		},
	}

	entry, id, _ := FindTargetError(dp, "e1")
	if entry == nil {
		t.Fatal("expected non-nil entry for specific errorID")
	}
	msg, _ := entry["message"].(string)
	if msg != "first error" {
		t.Fatalf("message = %q, want %q", msg, "first error")
	}
	if id != "e1" {
		t.Fatalf("id = %q, want %q", id, "e1")
	}
}

func TestFindTargetError_SpecificErrorIDNotFound(t *testing.T) {
	t.Parallel()
	dp := &mockDataProvider{
		logEntries: []map[string]any{
			{"level": "error", "message": "an error", "error_id": "e1", "ts": "2024-01-01T00:00:01Z"},
		},
	}

	entry, _, _ := FindTargetError(dp, "nonexistent")
	if entry != nil {
		t.Fatalf("expected nil for nonexistent errorID, got %v", entry)
	}
}

// ============================================
// Tests for GetActionsInTimeWindow
// ============================================

func TestGetActionsInTimeWindow_NoActions(t *testing.T) {
	t.Parallel()
	dp := &mockDataProvider{}

	_, err := GetActionsInTimeWindow(dp, 1000, 500)
	if err == nil || !strings.Contains(err.Error(), ErrNoActionsCaptured) {
		t.Fatalf("error = %v, want %q", err, ErrNoActionsCaptured)
	}
}

func TestGetActionsInTimeWindow_NoneInWindow(t *testing.T) {
	t.Parallel()
	dp := &mockDataProvider{
		actions: []types.EnhancedAction{
			{Type: "click", Timestamp: 1000},
			{Type: "input", Timestamp: 2000},
		},
	}

	_, err := GetActionsInTimeWindow(dp, 10000, 500)
	if err == nil || !strings.Contains(err.Error(), ErrNoActionsCaptured) {
		t.Fatalf("error = %v, want %q", err, ErrNoActionsCaptured)
	}
}

func TestGetActionsInTimeWindow_AllInWindow(t *testing.T) {
	t.Parallel()
	dp := &mockDataProvider{
		actions: []types.EnhancedAction{
			{Type: "click", Timestamp: 900},
			{Type: "input", Timestamp: 1000},
			{Type: "navigate", Timestamp: 1100},
		},
	}

	result, err := GetActionsInTimeWindow(dp, 1000, 500)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if len(result) != 3 {
		t.Fatalf("got %d actions, want 3", len(result))
	}
}

func TestGetActionsInTimeWindow_PartialMatch(t *testing.T) {
	t.Parallel()
	dp := &mockDataProvider{
		actions: []types.EnhancedAction{
			{Type: "click", Timestamp: 100},
			{Type: "input", Timestamp: 900},
			{Type: "navigate", Timestamp: 1000},
			{Type: "scroll", Timestamp: 1400},
			{Type: "wait", Timestamp: 2000},
		},
	}

	result, err := GetActionsInTimeWindow(dp, 1000, 500)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if len(result) != 3 {
		t.Fatalf("got %d actions, want 3", len(result))
	}
}

func TestGetActionsInTimeWindow_ExactBoundary(t *testing.T) {
	t.Parallel()
	dp := &mockDataProvider{
		actions: []types.EnhancedAction{
			{Type: "click", Timestamp: 500},
			{Type: "input", Timestamp: 1500},
		},
	}

	result, err := GetActionsInTimeWindow(dp, 1000, 500)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("got %d actions, want 2", len(result))
	}
}

// ============================================
// Tests for CountNetworkAssertions
// ============================================

func TestCountNetworkAssertions_Empty(t *testing.T) {
	t.Parallel()
	dp := &mockDataProvider{}

	if count := CountNetworkAssertions(dp); count != 0 {
		t.Fatalf("CountNetworkAssertions(empty) = %d, want 0", count)
	}
}

func TestCountNetworkAssertions_WithBodies(t *testing.T) {
	t.Parallel()
	dp := &mockDataProvider{
		bodies: []types.NetworkBody{
			{Method: "GET", URL: "/api/users", Status: 200},
			{Method: "POST", URL: "/api/login", Status: 201},
			{Method: "GET", URL: "/api/data", Status: 0},
		},
	}

	if count := CountNetworkAssertions(dp); count != 2 {
		t.Fatalf("CountNetworkAssertions = %d, want 2", count)
	}
}

func TestCountNetworkAssertions_NegativeStatusIgnored(t *testing.T) {
	t.Parallel()
	dp := &mockDataProvider{
		bodies: []types.NetworkBody{{Method: "GET", URL: "/api/a", Status: -1}},
	}

	if count := CountNetworkAssertions(dp); count != 0 {
		t.Fatalf("CountNetworkAssertions(negative) = %d, want 0", count)
	}
}

// ============================================
// Tests for CollectErrorMessages
// ============================================

func TestCollectErrorMessages_NoEntries(t *testing.T) {
	t.Parallel()
	dp := &mockDataProvider{}

	if msgs := CollectErrorMessages(dp, 5); len(msgs) != 0 {
		t.Fatalf("len = %d, want 0", len(msgs))
	}
}

func TestCollectErrorMessages_MixedLevels(t *testing.T) {
	t.Parallel()
	dp := &mockDataProvider{
		logEntries: []map[string]any{
			{"level": "error", "message": "error one"},
			{"level": "info", "message": "info msg"},
			{"level": "error", "message": "error two"},
			{"level": "error", "message": "error three"},
		},
	}

	msgs := CollectErrorMessages(dp, 5)
	if len(msgs) != 3 {
		t.Fatalf("len = %d, want 3", len(msgs))
	}
}

func TestCollectErrorMessages_RespectsLimit(t *testing.T) {
	t.Parallel()
	dp := &mockDataProvider{
		logEntries: []map[string]any{
			{"level": "error", "message": "err-1"},
			{"level": "error", "message": "err-2"},
			{"level": "error", "message": "err-3"},
			{"level": "error", "message": "err-4"},
		},
	}

	msgs := CollectErrorMessages(dp, 3)
	if len(msgs) != 3 {
		t.Fatalf("len = %d, want 3", len(msgs))
	}
}

func TestCollectErrorMessages_SkipsEmptyMessages(t *testing.T) {
	t.Parallel()
	dp := &mockDataProvider{
		logEntries: []map[string]any{
			{"level": "error", "message": ""},
			{"level": "error", "message": "real error"},
			{"level": "error"},
		},
	}

	msgs := CollectErrorMessages(dp, 5)
	if len(msgs) != 1 {
		t.Fatalf("len = %d, want 1", len(msgs))
	}
}

func TestCollectErrorMessages_LimitZero(t *testing.T) {
	t.Parallel()
	dp := &mockDataProvider{
		logEntries: []map[string]any{{"level": "error", "message": "should not appear"}},
	}

	if msgs := CollectErrorMessages(dp, 0); len(msgs) != 0 {
		t.Fatalf("len = %d, want 0", len(msgs))
	}
}

// ============================================
// Tests for GenerateTestFromError
// ============================================

func TestGenerateTestFromError_NoErrors(t *testing.T) {
	t.Parallel()
	dp := &mockDataProvider{}

	_, err := GenerateTestFromError(dp, TestFromContextRequest{Framework: "playwright"})
	if err == nil || !strings.Contains(err.Error(), ErrNoErrorContext) {
		t.Fatalf("error = %v, want %q", err, ErrNoErrorContext)
	}
}

func TestGenerateTestFromError_ErrorButNoActions(t *testing.T) {
	t.Parallel()
	dp := &mockDataProvider{
		logEntries: []map[string]any{
			{"level": "error", "message": "test error", "error_id": "e1", "ts": "2024-01-01T00:00:01Z"},
		},
	}

	_, err := GenerateTestFromError(dp, TestFromContextRequest{Framework: "playwright"})
	if err == nil || !strings.Contains(err.Error(), ErrNoActionsCaptured) {
		t.Fatalf("error = %v, want %q", err, ErrNoActionsCaptured)
	}
}

func TestGenerateTestFromError_Success(t *testing.T) {
	t.Parallel()
	errorTime := time.Date(2024, 1, 1, 0, 0, 1, 0, time.UTC)
	dp := &mockDataProvider{
		logEntries: []map[string]any{
			{"level": "error", "message": "click failed", "error_id": "e1", "ts": errorTime.Format(time.RFC3339)},
		},
		actions: []types.EnhancedAction{
			{
				Type: "click", Selectors: map[string]any{"target": "#submit-btn"},
				URL: "https://app.example.com", Timestamp: errorTime.Add(-2 * time.Second).UnixMilli(),
			},
			{
				Type: "input", Selectors: map[string]any{"target": "#email"},
				Value: "user@test.com", URL: "https://app.example.com",
				Timestamp: errorTime.Add(-1 * time.Second).UnixMilli(),
			},
		},
	}

	result, err := GenerateTestFromError(dp, TestFromContextRequest{
		Framework: "playwright",
		BaseURL:   "https://app.example.com",
	})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if result.Framework != "playwright" {
		t.Fatalf("Framework = %q, want playwright", result.Framework)
	}
	if result.Metadata.SourceError != "e1" {
		t.Fatalf("SourceError = %q, want e1", result.Metadata.SourceError)
	}
	if !result.Coverage.ErrorReproduced {
		t.Fatal("expected ErrorReproduced to be true")
	}
}

// ============================================
// Tests for GenerateTestFromInteraction
// ============================================

func TestGenerateTestFromInteraction_NoActions(t *testing.T) {
	t.Parallel()
	dp := &mockDataProvider{}

	_, err := GenerateTestFromInteraction(dp, TestFromContextRequest{Framework: "playwright"})
	if err == nil || !strings.Contains(err.Error(), ErrNoActionsCaptured) {
		t.Fatalf("error = %v, want %q", err, ErrNoActionsCaptured)
	}
}

func TestGenerateTestFromInteraction_Basic(t *testing.T) {
	t.Parallel()
	dp := &mockDataProvider{
		actions: []types.EnhancedAction{
			{Type: "click", Selectors: map[string]any{"target": "#btn", "testId": "submit"}, URL: "https://app.example.com"},
			{Type: "input", Selectors: map[string]any{"target": "#email"}, Value: "user@test.com"},
		},
	}

	result, err := GenerateTestFromInteraction(dp, TestFromContextRequest{
		Framework: "playwright",
		BaseURL:   "https://app.example.com",
	})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if result.Coverage.ErrorReproduced {
		t.Fatal("ErrorReproduced should be false for interaction test")
	}
	if !result.Coverage.StateCaptured {
		t.Fatal("StateCaptured should be true")
	}
	if !strings.Contains(result.Content, "getByTestId('submit').click()") {
		t.Fatalf("script should contain click action; got:\n%s", result.Content)
	}
	if len(result.Metadata.ContextUsed) != 1 || result.Metadata.ContextUsed[0] != "actions" {
		t.Fatalf("ContextUsed = %v, want [actions]", result.Metadata.ContextUsed)
	}
}

func TestGenerateTestFromInteraction_WithMocks(t *testing.T) {
	t.Parallel()
	dp := &mockDataProvider{
		actions: []types.EnhancedAction{
			{Type: "click", Selectors: map[string]any{"target": "#btn"}},
		},
		bodies: []types.NetworkBody{
			{Method: "GET", URL: "/api/data", Status: 200},
			{Method: "POST", URL: "/api/submit", Status: 201},
			{Method: "GET", URL: "/api/noop", Status: 0},
		},
	}

	result, err := GenerateTestFromInteraction(dp, TestFromContextRequest{
		Framework:    "playwright",
		IncludeMocks: true,
	})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if !result.Coverage.NetworkMocked {
		t.Fatal("NetworkMocked should be true when IncludeMocks is true")
	}
	if result.Assertions < 2 {
		t.Fatalf("Assertions = %d, want >= 2", result.Assertions)
	}
}

// ============================================
// Tests for GenerateTestFromRegression
// ============================================

func TestGenerateTestFromRegression_NoActions(t *testing.T) {
	t.Parallel()
	dp := &mockDataProvider{}

	_, err := GenerateTestFromRegression(dp, TestFromContextRequest{Framework: "playwright"})
	if err == nil || !strings.Contains(err.Error(), ErrNoActionsCaptured) {
		t.Fatalf("error = %v, want %q", err, ErrNoActionsCaptured)
	}
}

func TestGenerateTestFromRegression_CleanBaseline(t *testing.T) {
	t.Parallel()
	dp := &mockDataProvider{
		actions: []types.EnhancedAction{
			{Type: "click", Selectors: map[string]any{"target": "#submit"}},
			{Type: "navigate", ToURL: "https://app.example.com/dashboard"},
		},
	}

	result, err := GenerateTestFromRegression(dp, TestFromContextRequest{
		Framework: "playwright",
		BaseURL:   "https://app.example.com",
	})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if result.Filename != "regression-test.spec.ts" {
		t.Fatalf("Filename = %q, want regression-test.spec.ts", result.Filename)
	}
	if result.Assertions < 1 {
		t.Fatalf("Assertions = %d, want >= 1", result.Assertions)
	}
	if !strings.Contains(result.Content, "expect(consoleErrors).toHaveLength(0)") {
		t.Fatalf("expected clean baseline assertion")
	}
}

func TestGenerateTestFromRegression_WithErrorsAndNetwork(t *testing.T) {
	t.Parallel()
	dp := &mockDataProvider{
		actions: []types.EnhancedAction{
			{Type: "click", Selectors: map[string]any{"target": "#btn"}},
		},
		logEntries: []map[string]any{
			{"level": "error", "message": "TypeError: undefined is not a function"},
			{"level": "error", "message": "ReferenceError: x is not defined"},
		},
		bodies: []types.NetworkBody{
			{Method: "GET", URL: "/api/users", Status: 200},
		},
	}

	result, err := GenerateTestFromRegression(dp, TestFromContextRequest{Framework: "playwright"})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if !strings.Contains(result.Content, "Baseline had 2 console errors") {
		t.Fatal("expected baseline error comment")
	}
	if !strings.Contains(result.Content, "Assert GET /api/users returns 200") {
		t.Fatal("expected network assertion")
	}
}
