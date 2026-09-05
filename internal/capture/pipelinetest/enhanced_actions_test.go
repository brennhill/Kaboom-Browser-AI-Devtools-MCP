// Purpose: Tests for enhanced action capture with mutation tracking.
// Docs: docs/features/feature/backend-log-streaming/index.md

// enhanced_actions_test.go — Tests for enhanced action buffering, enrichment, and ring buffer eviction.
package pipelinetest

import (
	. "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"testing"
	"time"
)

func actionCapacity(c *Capture) int { return c.Telemetry().Actions().Stats().Capacity }

// ============================================
// AddEnhancedActions Tests
// ============================================

func TestNewAddEnhancedActions_SingleAction(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	t.Cleanup(c.Close)

	action := types.EnhancedAction{
		Type:      "click",
		Timestamp: time.Now().UnixMilli(),
		URL:       "https://example.com/page",
		Selectors: map[string]any{"css": "#submit-btn"},
		TabID:     1,
		Source:    "human",
	}

	c.Telemetry().AddEnhancedActions([]types.EnhancedAction{action})

	if got := len(c.Telemetry().Actions().Snapshot().Actions); got != 1 {
		t.Fatalf("GetEnhancedActionCount() = %d, want 1", got)
	}

	actions := c.Telemetry().Actions().Snapshot().Actions
	if len(actions) != 1 {
		t.Fatalf("len(GetAllEnhancedActions()) = %d, want 1", len(actions))
	}

	stored := actions[0]
	if stored.Type != "click" {
		t.Errorf("Type = %q, want %q", stored.Type, "click")
	}
	if stored.URL != "https://example.com/page" {
		t.Errorf("URL = %q, want %q", stored.URL, "https://example.com/page")
	}
	if stored.TabID != 1 {
		t.Errorf("TabID = %d, want 1", stored.TabID)
	}
	if stored.Source != "human" {
		t.Errorf("Source = %q, want %q", stored.Source, "human")
	}
	cssVal, ok := stored.Selectors["css"]
	if !ok || cssVal != "#submit-btn" {
		t.Errorf("Selectors[css] = %v, want #submit-btn", cssVal)
	}
}

func TestNewAddEnhancedActions_MultipleBatch(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	t.Cleanup(c.Close)

	actions := []types.EnhancedAction{
		{Type: "click", URL: "https://example.com"},
		{Type: "type", Value: "hello", InputType: "text"},
		{Type: "navigate", FromURL: "https://a.com", ToURL: "https://b.com"},
	}

	c.Telemetry().AddEnhancedActions(actions)

	if got := len(c.Telemetry().Actions().Snapshot().Actions); got != 3 {
		t.Fatalf("GetEnhancedActionCount() = %d, want 3", got)
	}

	stored := c.Telemetry().Actions().Snapshot().Actions
	if stored[0].Type != "click" {
		t.Errorf("stored[0].Type = %q, want click", stored[0].Type)
	}
	if stored[1].Type != "type" {
		t.Errorf("stored[1].Type = %q, want type", stored[1].Type)
	}
	if stored[1].Value != "hello" {
		t.Errorf("stored[1].Value = %q, want hello", stored[1].Value)
	}
	if stored[1].InputType != "text" {
		t.Errorf("stored[1].InputType = %q, want text", stored[1].InputType)
	}
	if stored[2].Type != "navigate" {
		t.Errorf("stored[2].Type = %q, want navigate", stored[2].Type)
	}
	if stored[2].FromURL != "https://a.com" {
		t.Errorf("stored[2].FromURL = %q, want https://a.com", stored[2].FromURL)
	}
	if stored[2].ToURL != "https://b.com" {
		t.Errorf("stored[2].ToURL = %q, want https://b.com", stored[2].ToURL)
	}
}

func TestNewAddEnhancedActions_TestIDTagging(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	t.Cleanup(c.Close)

	// Set active test IDs through the canonical runtime boundary.
	c.Extension().SetTestBoundaryStart("test-alpha")
	c.Extension().SetTestBoundaryStart("test-beta")

	added := []types.EnhancedAction{
		{Type: "click"},
		{Type: "type", Value: "text"},
	}
	c.Telemetry().AddEnhancedActions(added)

	actions := c.Telemetry().Actions().Snapshot().Actions
	// Observed-input signal: without this the per-action assertions below would
	// all hold vacuously if the store had ingested nothing.
	if len(actions) != len(added) {
		t.Fatalf("stored %d actions, want %d — nothing was ingested to tag", len(actions), len(added))
	}
	for i, action := range actions {
		if len(action.TestIDs) != 2 {
			t.Fatalf("action[%d].TestIDs len = %d, want 2", i, len(action.TestIDs))
		}
		// Check both test IDs are present (order may vary)
		testIDSet := make(map[string]bool)
		for _, id := range action.TestIDs {
			testIDSet[id] = true
		}
		if !testIDSet["test-alpha"] {
			t.Errorf("action[%d] missing test-alpha in TestIDs", i)
		}
		if !testIDSet["test-beta"] {
			t.Errorf("action[%d] missing test-beta in TestIDs", i)
		}
	}
}

func TestNewAddEnhancedActions_NoActiveTestIDs(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	t.Cleanup(c.Close)

	c.Telemetry().AddEnhancedActions([]types.EnhancedAction{{Type: "click"}})

	actions := c.Telemetry().Actions().Snapshot().Actions
	if len(actions[0].TestIDs) != 0 {
		t.Errorf("TestIDs = %v, want empty when no active tests", actions[0].TestIDs)
	}
}

func TestNewAddEnhancedActions_IncrementsTotalAdded(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	t.Cleanup(c.Close)

	c.Telemetry().AddEnhancedActions([]types.EnhancedAction{{Type: "click"}, {Type: "type"}})
	c.Telemetry().AddEnhancedActions([]types.EnhancedAction{{Type: "navigate"}})

	total := c.Telemetry().Actions().Stats().TotalAdded

	if total != 3 {
		t.Errorf("actionTotalAdded = %d, want 3", total)
	}
}

func TestNewAddEnhancedActions_EmptyBatch(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	t.Cleanup(c.Close)

	c.Telemetry().AddEnhancedActions([]types.EnhancedAction{})

	if got := len(c.Telemetry().Actions().Snapshot().Actions); got != 0 {
		t.Errorf("GetEnhancedActionCount() after empty batch = %d, want 0", got)
	}
}

// ============================================
// Ring Buffer Eviction Tests
// ============================================

func TestNewAddEnhancedActions_RingBufferEviction(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	t.Cleanup(c.Close)

	// Fill beyond actionCapacity(c)
	overflow := actionCapacity(c) + 50
	batch := make([]types.EnhancedAction, overflow)
	for i := 0; i < overflow; i++ {
		batch[i] = types.EnhancedAction{
			Type:      "click",
			Timestamp: int64(i),
		}
	}

	c.Telemetry().AddEnhancedActions(batch)

	if got := len(c.Telemetry().Actions().Snapshot().Actions); got != actionCapacity(c) {
		t.Fatalf("GetEnhancedActionCount() = %d, want %d (max)", got, actionCapacity(c))
	}

	// The oldest entries should be evicted; newest should remain
	actions := c.Telemetry().Actions().Snapshot().Actions
	first := actions[0]
	last := actions[len(actions)-1]

	// First element should be from index 50 (the 50 oldest were evicted)
	if first.Timestamp != 50 {
		t.Errorf("first action timestamp = %d, want 50 (oldest evicted)", first.Timestamp)
	}
	if last.Timestamp != int64(overflow-1) {
		t.Errorf("last action timestamp = %d, want %d", last.Timestamp, overflow-1)
	}
}

func TestNewAddEnhancedActions_ExactCapacity(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	t.Cleanup(c.Close)

	batch := make([]types.EnhancedAction, actionCapacity(c))
	for i := range batch {
		batch[i] = types.EnhancedAction{Type: "click", Timestamp: int64(i)}
	}

	c.Telemetry().AddEnhancedActions(batch)

	if got := len(c.Telemetry().Actions().Snapshot().Actions); got != actionCapacity(c) {
		t.Fatalf("GetEnhancedActionCount() at exact capacity = %d, want %d", got, actionCapacity(c))
	}
}

func TestNewAddEnhancedActions_IncrementalOverflow(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	t.Cleanup(c.Close)

	// Fill to capacity
	batch := make([]types.EnhancedAction, actionCapacity(c))
	for i := range batch {
		batch[i] = types.EnhancedAction{Type: "click", Timestamp: int64(i)}
	}
	c.Telemetry().AddEnhancedActions(batch)

	// Add 5 more
	extra := make([]types.EnhancedAction, 5)
	for i := range extra {
		extra[i] = types.EnhancedAction{Type: "type", Timestamp: int64(actionCapacity(c) + i)}
	}
	c.Telemetry().AddEnhancedActions(extra)

	if got := len(c.Telemetry().Actions().Snapshot().Actions); got != actionCapacity(c) {
		t.Fatalf("GetEnhancedActionCount() after incremental overflow = %d, want %d", got, actionCapacity(c))
	}

	// Last 5 actions should be "type"
	actions := c.Telemetry().Actions().Snapshot().Actions
	for i := actionCapacity(c) - 5; i < actionCapacity(c); i++ {
		if actions[i].Type != "type" {
			t.Errorf("actions[%d].Type = %q, want type (newly added)", i, actions[i].Type)
		}
	}
}

// ============================================
// Parallel Array Mismatch Recovery Tests
// ============================================

func TestNewAddEnhancedActions_AppendAfterDirectBufferSet(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	t.Cleanup(c.Close)

	// Pre-populate through the canonical owner without extension enrichment.
	now := time.Now()
	c.Telemetry().Actions().Add([]types.EnhancedAction{{Type: "click"}, {Type: "type"}, {Type: "navigate"}}, now)

	// Adding appends to existing entries
	c.Telemetry().AddEnhancedActions([]types.EnhancedAction{{Type: "scroll"}})

	// 3 existing + 1 new = 4
	if got := len(c.Telemetry().Actions().Snapshot().Actions); got != 4 {
		t.Fatalf("GetEnhancedActionCount() after append = %d, want 4", got)
	}
}

// ============================================
// GetEnhancedActionCount Tests
// ============================================

func TestNewGetEnhancedActionCount_Empty(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	t.Cleanup(c.Close)

	if got := len(c.Telemetry().Actions().Snapshot().Actions); got != 0 {
		t.Errorf("GetEnhancedActionCount() on fresh capture = %d, want 0", got)
	}
}

func TestNewGetEnhancedActionCount_AfterAdds(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	t.Cleanup(c.Close)

	c.Telemetry().AddEnhancedActions([]types.EnhancedAction{{Type: "click"}})
	if got := len(c.Telemetry().Actions().Snapshot().Actions); got != 1 {
		t.Errorf("GetEnhancedActionCount() after 1 add = %d, want 1", got)
	}

	c.Telemetry().AddEnhancedActions([]types.EnhancedAction{{Type: "type"}, {Type: "navigate"}})
	if got := len(c.Telemetry().Actions().Snapshot().Actions); got != 3 {
		t.Errorf("GetEnhancedActionCount() after 2 adds = %d, want 3", got)
	}
}

// ============================================
// All Action Fields Tests
// ============================================

func TestNewAddEnhancedActions_AllFieldsPreserved(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	t.Cleanup(c.Close)

	action := types.EnhancedAction{
		Type:          "click",
		Timestamp:     1700000000000,
		URL:           "https://example.com/page",
		Selectors:     map[string]any{"css": "#btn", "xpath": "//button"},
		Value:         "Submit",
		InputType:     "button",
		Key:           "Enter",
		FromURL:       "https://from.com",
		ToURL:         "https://to.com",
		SelectedValue: "option-1",
		SelectedText:  "Option 1",
		ScrollY:       500,
		TabID:         42,
		Source:        "ai",
	}

	c.Telemetry().AddEnhancedActions([]types.EnhancedAction{action})
	stored := c.Telemetry().Actions().Snapshot().Actions[0]

	if stored.Type != "click" {
		t.Errorf("Type = %q, want click", stored.Type)
	}
	if stored.Timestamp != 1700000000000 {
		t.Errorf("Timestamp = %d, want 1700000000000", stored.Timestamp)
	}
	if stored.URL != "https://example.com/page" {
		t.Errorf("URL = %q, want https://example.com/page", stored.URL)
	}
	if stored.Value != "Submit" {
		t.Errorf("Value = %q, want Submit", stored.Value)
	}
	if stored.InputType != "button" {
		t.Errorf("InputType = %q, want button", stored.InputType)
	}
	if stored.Key != "Enter" {
		t.Errorf("Key = %q, want Enter", stored.Key)
	}
	if stored.FromURL != "https://from.com" {
		t.Errorf("FromURL = %q, want https://from.com", stored.FromURL)
	}
	if stored.ToURL != "https://to.com" {
		t.Errorf("ToURL = %q, want https://to.com", stored.ToURL)
	}
	if stored.SelectedValue != "option-1" {
		t.Errorf("SelectedValue = %q, want option-1", stored.SelectedValue)
	}
	if stored.SelectedText != "Option 1" {
		t.Errorf("SelectedText = %q, want Option 1", stored.SelectedText)
	}
	if stored.ScrollY != 500 {
		t.Errorf("ScrollY = %d, want 500", stored.ScrollY)
	}
	if stored.TabID != 42 {
		t.Errorf("TabID = %d, want 42", stored.TabID)
	}
	if stored.Source != "ai" {
		t.Errorf("Source = %q, want ai", stored.Source)
	}
	if len(stored.Selectors) != 2 {
		t.Errorf("Selectors len = %d, want 2", len(stored.Selectors))
	}
}
