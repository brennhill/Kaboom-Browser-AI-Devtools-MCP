// Purpose: Checkpoint lifecycle — create/resolve checkpoints, compose the diff response, and summarize it.
// Why: Keeps the whole GetChangesSince path (resolution -> diff assembly -> severity/summary) in one place
// so the lock-holding sequence in GetChangesSince is readable end to end.
// Docs: docs/features/feature/push-alerts/index.md

package checkpoint

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/server"
)

func NewCheckpointManager(serverReader server.LogReader, capture *capture.Store) *CheckpointManager {
	return &CheckpointManager{
		namedCheckpoints: make(map[string]*Checkpoint),
		namedOrder:       make([]string, 0),
		server:           serverReader,
		capture:          capture,
	}
}

func (cm *CheckpointManager) CreateCheckpoint(name string, clientID string) error {
	if name == "" {
		return fmt.Errorf("checkpoint name cannot be empty")
	}
	if len(name) > maxCheckpointNameLen {
		return fmt.Errorf("checkpoint name exceeds %d characters", maxCheckpointNameLen)
	}

	storedName := name
	if clientID != "" {
		storedName = clientID + ":" + name
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	cp := cm.snapshotNow()
	cp.Name = name
	cp.AlertDelivery = cm.alertDelivery

	if _, exists := cm.namedCheckpoints[storedName]; !exists {
		cm.namedOrder = append(cm.namedOrder, storedName)
	}
	cm.namedCheckpoints[storedName] = cp

	for len(cm.namedCheckpoints) > maxNamedCheckpoints {
		oldest := cm.namedOrder[0]
		newOrder := make([]string, len(cm.namedOrder)-1)
		copy(newOrder, cm.namedOrder[1:])
		cm.namedOrder = newOrder
		delete(cm.namedCheckpoints, oldest)
	}

	return nil
}

func (cm *CheckpointManager) GetNamedCheckpointCount() int {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return len(cm.namedCheckpoints)
}

func (cm *CheckpointManager) GetChangesSince(params GetChangesSinceParams, clientID string) DiffResponse {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	now := time.Now()
	cp, isNamedQuery := cm.resolveCheckpoint(params.Checkpoint, clientID, now)

	resp := cm.computeDiffs(cp, params, now)
	cm.applySeverityFilter(&resp, params.Severity)
	cm.pruneEmptyDiffs(&resp)
	cm.attachAlerts(&resp, cp.AlertDelivery)

	jsonBytes, _ := json.Marshal(resp)
	resp.TokenCount = len(jsonBytes) / 4

	if !isNamedQuery {
		cm.markAlertsDelivered()
		cm.autoCheckpoint = cm.snapshotNow()
		cm.autoCheckpoint.KnownEndpoints = cm.buildKnownEndpoints(cp.KnownEndpoints)
		cm.autoCheckpoint.AlertDelivery = cm.alertDelivery
	}

	return resp
}

func (cm *CheckpointManager) computeDiffs(cp *Checkpoint, params GetChangesSinceParams, now time.Time) DiffResponse {
	resp := DiffResponse{
		From:       cp.CreatedAt,
		To:         now,
		DurationMs: now.Sub(cp.CreatedAt).Milliseconds(),
	}
	if cm.shouldInclude(params.Include, "console") {
		resp.Console = cm.computeConsoleDiff(cp, params.Severity)
	}
	if cm.shouldInclude(params.Include, "network") {
		resp.Network = cm.computeNetworkDiff(cp)
	}
	if cm.shouldInclude(params.Include, "websocket") {
		resp.WebSocket = cm.computeWebSocketDiff(cp, params.Severity)
	}
	if cm.shouldInclude(params.Include, "actions") {
		resp.Actions = cm.computeActionsDiff(cp)
	}
	return resp
}

func (cm *CheckpointManager) applySeverityFilter(resp *DiffResponse, severity string) {
	if severity == "errors_only" {
		if resp.Console != nil {
			resp.Console.Warnings = nil
		}
		if resp.WebSocket != nil && len(resp.WebSocket.Errors) == 0 {
			resp.WebSocket = nil
		}
	}
	resp.Severity = cm.determineSeverity(*resp)
	resp.Summary = cm.buildSummary(*resp)
}

func (cm *CheckpointManager) pruneEmptyDiffs(resp *DiffResponse) {
	if resp.Console != nil && resp.Console.TotalNew == 0 {
		resp.Console = nil
	}
	if resp.Network != nil && resp.Network.TotalNew == 0 && len(resp.Network.Failures) == 0 && len(resp.Network.NewEndpoints) == 0 && len(resp.Network.Degraded) == 0 {
		resp.Network = nil
	}
	if resp.WebSocket != nil && resp.WebSocket.TotalNew == 0 {
		resp.WebSocket = nil
	}
	if resp.Actions != nil && resp.Actions.TotalNew == 0 {
		resp.Actions = nil
	}
}

func (cm *CheckpointManager) attachAlerts(resp *DiffResponse, checkpointDelivery int64) {
	alerts := cm.getPendingAlerts(checkpointDelivery)
	if len(alerts) > 0 {
		resp.PerformanceAlerts = alerts
	}
}

func (cm *CheckpointManager) shouldInclude(include []string, category string) bool {
	if len(include) == 0 {
		return true
	}
	for _, inc := range include {
		if inc == category {
			return true
		}
	}
	return false
}

func (cm *CheckpointManager) resolveCheckpoint(name, clientID string, now time.Time) (*Checkpoint, bool) {
	if name == "" {
		return cm.resolveAutoCheckpoint(now), false
	}

	namespacedName := name
	if clientID != "" {
		namespacedName = clientID + ":" + name
	}

	if named, ok := cm.namedCheckpoints[namespacedName]; ok {
		return named, true
	}
	if named, ok := cm.namedCheckpoints[name]; ok {
		return named, true
	}
	if cp := cm.resolveTimestampCheckpoint(name); cp != nil {
		return cp, true
	}
	return &Checkpoint{CreatedAt: now, KnownEndpoints: make(map[string]endpointState)}, true
}

func (cm *CheckpointManager) resolveAutoCheckpoint(now time.Time) *Checkpoint {
	if cm.autoCheckpoint != nil {
		return cm.autoCheckpoint
	}
	return &Checkpoint{
		CreatedAt:      now,
		KnownEndpoints: make(map[string]endpointState),
	}
}

func (cm *CheckpointManager) snapshotNow() *Checkpoint {
	logTotal := cm.server.GetLogTotalAdded()
	netTotal := cm.capture.GetNetworkTotalAdded()
	wsTotal := cm.capture.GetWebSocketTotalAdded()
	actTotal := cm.capture.GetActionTotalAdded()

	return &Checkpoint{
		CreatedAt:      time.Now(),
		LogTotal:       logTotal,
		NetworkTotal:   netTotal,
		WSTotal:        wsTotal,
		ActionTotal:    actTotal,
		KnownEndpoints: make(map[string]endpointState),
	}
}

func (cm *CheckpointManager) resolveTimestampCheckpoint(tsStr string) *Checkpoint {
	t, err := time.Parse(time.RFC3339Nano, tsStr)
	if err != nil {
		t, err = time.Parse(time.RFC3339, tsStr)
		if err != nil {
			return nil
		}
	}

	logTotal := cm.findPositionAtTime(cm.server.GetLogTimestamps(), cm.server.GetLogTotalAdded(), t)
	netTotal := cm.findPositionAtTime(cm.capture.GetNetworkTimestamps(), cm.capture.GetNetworkTotalAdded(), t)
	wsTotal := cm.findPositionAtTime(cm.capture.GetWebSocketTimestamps(), cm.capture.GetWebSocketTotalAdded(), t)
	actTotal := cm.findPositionAtTime(cm.capture.GetActionTimestamps(), cm.capture.GetActionTotalAdded(), t)

	return &Checkpoint{
		CreatedAt:      t,
		LogTotal:       logTotal,
		NetworkTotal:   netTotal,
		WSTotal:        wsTotal,
		ActionTotal:    actTotal,
		KnownEndpoints: make(map[string]endpointState),
	}
}

func (cm *CheckpointManager) findPositionAtTime(addedAt []time.Time, currentTotal int64, t time.Time) int64 {
	if len(addedAt) == 0 {
		return currentTotal
	}

	idx := sort.Search(len(addedAt), func(i int) bool {
		return addedAt[i].After(t)
	})

	entriesAfter := int64(len(addedAt) - idx)
	pos := currentTotal - entriesAfter
	if pos < 0 {
		pos = 0
	}
	return pos
}

func (cm *CheckpointManager) determineSeverity(resp DiffResponse) string {
	if hasConsoleErrors(resp) || hasNetworkFailures(resp) {
		return "error"
	}
	if hasConsoleWarnings(resp) || hasWSDisconnections(resp) {
		return "warning"
	}
	return "clean"
}

func hasConsoleErrors(resp DiffResponse) bool {
	return resp.Console != nil && len(resp.Console.Errors) > 0
}

func hasNetworkFailures(resp DiffResponse) bool {
	return resp.Network != nil && len(resp.Network.Failures) > 0
}

func hasConsoleWarnings(resp DiffResponse) bool {
	return resp.Console != nil && len(resp.Console.Warnings) > 0
}

func hasWSDisconnections(resp DiffResponse) bool {
	return resp.WebSocket != nil && len(resp.WebSocket.Disconnections) > 0
}

func (cm *CheckpointManager) buildSummary(resp DiffResponse) string {
	if resp.Severity == "clean" {
		return "No significant changes."
	}
	parts := collectSummaryParts(resp)
	if len(parts) == 0 {
		return "No significant changes."
	}
	return strings.Join(parts, ", ")
}

func collectSummaryParts(resp DiffResponse) []string {
	var parts []string
	if hasConsoleErrors(resp) {
		parts = append(parts, fmt.Sprintf("%d new console error(s)", sumConsoleCounts(resp.Console.Errors)))
	}
	if hasNetworkFailures(resp) {
		parts = append(parts, fmt.Sprintf("%d network failure(s)", len(resp.Network.Failures)))
	}
	if hasConsoleWarnings(resp) {
		parts = append(parts, fmt.Sprintf("%d new console warning(s)", sumConsoleCounts(resp.Console.Warnings)))
	}
	if hasWSDisconnections(resp) {
		parts = append(parts, fmt.Sprintf("%d websocket disconnection(s)", len(resp.WebSocket.Disconnections)))
	}
	return parts
}

func sumConsoleCounts(entries []ConsoleEntry) int {
	total := 0
	for _, e := range entries {
		total += e.Count
	}
	return total
}
