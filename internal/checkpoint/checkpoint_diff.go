// Purpose: Computes console/network/websocket/action diffs between a checkpoint and now, and classifies each stream.
// Why: The four computeXDiff methods and the per-stream classifiers they call are one algorithm; splitting them
// across four files hid that every classifier exists solely to serve its computeXDiff caller.
// Docs: docs/features/feature/push-alerts/index.md

package checkpoint

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	gasTypes "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

func (cm *CheckpointManager) computeConsoleDiff(cp *Checkpoint, severity string) *ConsoleDiff {
	snapshot := cm.server.GetLogSnapshot()
	newEntries := recentSlice(len(snapshot.Entries), int(snapshot.TotalAdded-cp.LogTotal))
	if newEntries < 0 {
		return &ConsoleDiff{}
	}
	entries := snapshot.Entries[len(snapshot.Entries)-newEntries:]

	classified := classifyLogEntries(entries, severity)
	return &ConsoleDiff{
		TotalNew: classified.totalNew,
		Errors:   buildConsoleEntries(classified.errorMap, classified.errorOrder),
		Warnings: buildConsoleEntries(classified.warningMap, classified.warningOrder),
	}
}

func (cm *CheckpointManager) computeNetworkDiff(cp *Checkpoint) *NetworkDiff {
	allBodies := cm.capture.GetNetworkBodies()
	count := recentSlice(len(allBodies), int(cm.capture.GetNetworkTotalAdded()-cp.NetworkTotal))
	if count < 0 {
		return &NetworkDiff{}
	}
	newBodies := allBodies[len(allBodies)-count:]

	diff := &NetworkDiff{TotalNew: len(newBodies)}
	for _, body := range newBodies {
		classifyNetworkBody(diff, body, cp.KnownEndpoints)
	}
	capNetworkDiff(diff)
	return diff
}

func (cm *CheckpointManager) computeWebSocketDiff(cp *Checkpoint, severity string) *WebSocketDiff {
	allEvents := cm.capture.GetAllWebSocketEvents()
	count := recentSlice(len(allEvents), int(cm.capture.GetWebSocketTotalAdded()-cp.WSTotal))
	if count < 0 {
		return &WebSocketDiff{}
	}
	newEvents := allEvents[len(allEvents)-count:]

	diff := &WebSocketDiff{TotalNew: len(newEvents)}
	for i := range newEvents {
		classifyWSEvent(diff, &newEvents[i], severity)
	}
	capWSDiff(diff)
	return diff
}

func (cm *CheckpointManager) computeActionsDiff(cp *Checkpoint) *ActionsDiff {
	allActions := cm.capture.GetAllEnhancedActions()
	count := recentSlice(len(allActions), int(cm.capture.GetActionTotalAdded()-cp.ActionTotal))
	if count < 0 {
		return &ActionsDiff{}
	}
	newActions := allActions[len(allActions)-count:]

	diff := &ActionsDiff{TotalNew: len(newActions)}
	for i := range newActions {
		if i >= maxDiffEntriesPerCat {
			break
		}
		diff.Actions = append(diff.Actions, ActionEntry{
			Type:      newActions[i].Type,
			URL:       newActions[i].URL,
			Timestamp: newActions[i].Timestamp,
		})
	}
	return diff
}

type fingerprintEntry struct {
	message string
	source  string
	count   int
}

type classifiedLogs struct {
	totalNew     int
	errorMap     map[string]*fingerprintEntry
	errorOrder   []string
	warningMap   map[string]*fingerprintEntry
	warningOrder []string
}

func classifyLogEntries(entries []gasTypes.LogEntry, severity string) classifiedLogs {
	cl := classifiedLogs{
		errorMap:   make(map[string]*fingerprintEntry),
		warningMap: make(map[string]*fingerprintEntry),
	}
	for _, entry := range entries {
		cl.totalNew++
		level, _ := entry["level"].(string)
		msg := extractLogMessage(entry)
		source, _ := entry["source"].(string)

		switch {
		case level == "error":
			addToFingerprintMap(cl.errorMap, &cl.errorOrder, msg, source)
		case (level == "warn" || level == "warning") && severity != "errors_only":
			addToFingerprintMap(cl.warningMap, &cl.warningOrder, msg, source)
		}
	}
	return cl
}

func extractLogMessage(entry gasTypes.LogEntry) string {
	msg, _ := entry["msg"].(string)
	if msg == "" {
		msg, _ = entry["message"].(string)
	}
	return msg
}

func addToFingerprintMap(m map[string]*fingerprintEntry, order *[]string, msg, source string) {
	fp := FingerprintMessage(msg)
	if existing, ok := m[fp]; ok {
		existing.count++
		return
	}
	m[fp] = &fingerprintEntry{message: truncateMessage(msg), source: source, count: 1}
	*order = append(*order, fp)
}

func buildConsoleEntries(m map[string]*fingerprintEntry, order []string) []ConsoleEntry {
	var entries []ConsoleEntry
	for i, fp := range order {
		if i >= maxDiffEntriesPerCat {
			break
		}
		e := m[fp]
		entries = append(entries, ConsoleEntry{
			Message: e.message,
			Source:  e.source,
			Count:   e.count,
		})
	}
	return entries
}

func classifyNetworkBody(diff *NetworkDiff, body capture.NetworkBody, known map[string]endpointState) {
	path := capture.ExtractURLPath(body.URL)

	if body.Status >= 400 {
		classifyFailedRequest(diff, path, body.Status, known)
		return
	}
	classifySuccessfulRequest(diff, path, body.Duration, known)
}

func classifyFailedRequest(diff *NetworkDiff, path string, status int, known map[string]endpointState) {
	if prev, ok := known[path]; ok && prev.Status < 400 {
		diff.Failures = append(diff.Failures, NetworkFailure{
			Path:           path,
			Status:         status,
			PreviousStatus: prev.Status,
		})
	} else if !ok {
		appendUniqueEndpoint(diff, path)
	}
}

func classifySuccessfulRequest(diff *NetworkDiff, path string, duration int, known map[string]endpointState) {
	if _, ok := known[path]; !ok {
		appendUniqueEndpoint(diff, path)
	}
	if duration <= 0 {
		return
	}
	if prev, ok := known[path]; ok && prev.Duration > 0 && duration > prev.Duration*degradedLatencyFactor {
		diff.Degraded = append(diff.Degraded, NetworkDegraded{
			Path:     path,
			Duration: duration,
			Baseline: prev.Duration,
		})
	}
}

func appendUniqueEndpoint(diff *NetworkDiff, path string) {
	if !containsString(diff.NewEndpoints, path) {
		diff.NewEndpoints = append(diff.NewEndpoints, path)
	}
}

func capNetworkDiff(diff *NetworkDiff) {
	if len(diff.Failures) > maxDiffEntriesPerCat {
		diff.Failures = diff.Failures[:maxDiffEntriesPerCat]
	}
	if len(diff.NewEndpoints) > maxDiffEntriesPerCat {
		diff.NewEndpoints = diff.NewEndpoints[:maxDiffEntriesPerCat]
	}
	if len(diff.Degraded) > maxDiffEntriesPerCat {
		diff.Degraded = diff.Degraded[:maxDiffEntriesPerCat]
	}
}

func (cm *CheckpointManager) buildKnownEndpoints(existing map[string]endpointState) map[string]endpointState {
	result := make(map[string]endpointState)

	for k, v := range existing {
		result[k] = v
	}

	for _, body := range cm.capture.GetNetworkBodies() {
		path := capture.ExtractURLPath(body.URL)
		result[path] = endpointState{
			Status:   body.Status,
			Duration: body.Duration,
		}
	}

	return result
}

func classifyWSEvent(diff *WebSocketDiff, evt *capture.WebSocketEvent, severity string) {
	switch evt.Event {
	case "close":
		if severity != "errors_only" {
			diff.Disconnections = append(diff.Disconnections, WSDisconnection{
				URL:         evt.URL,
				CloseCode:   evt.CloseCode,
				CloseReason: evt.CloseReason,
			})
		}
	case "open":
		diff.Connections = append(diff.Connections, WSConn{URL: evt.URL, ID: evt.ID})
	case "error":
		diff.Errors = append(diff.Errors, WSError{URL: evt.URL, Message: evt.Data})
	}
}

func capWSDiff(diff *WebSocketDiff) {
	if len(diff.Disconnections) > maxDiffEntriesPerCat {
		diff.Disconnections = diff.Disconnections[:maxDiffEntriesPerCat]
	}
	if len(diff.Connections) > maxDiffEntriesPerCat {
		diff.Connections = diff.Connections[:maxDiffEntriesPerCat]
	}
	if len(diff.Errors) > maxDiffEntriesPerCat {
		diff.Errors = diff.Errors[:maxDiffEntriesPerCat]
	}
}
