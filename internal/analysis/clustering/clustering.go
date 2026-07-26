// Purpose: Implements clustering of recurring error instances into root-cause groups and summaries.
// Why: Reduces noisy error streams into actionable clusters for faster debugging triage.
// Docs: docs/features/feature/error-clustering/index.md

// Package clustering groups recurring error instances into root-cause clusters.
//
// Errors are fingerprinted by normalized message (IDs, UUIDs, URLs and timestamps
// replaced with placeholders) plus parsed application stack frames; an incoming
// error joins a cluster when at least two of {message, shared frame, temporal
// proximity} agree. ClusterManager owns the whole lifecycle: matching, capping,
// alerting at the third instance, and inactivity expiry.
package clustering

import (
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// Alert is an alias for types.Alert to avoid qualifying everywhere.
type Alert = types.Alert

// ErrorInstance represents a single error received from the extension.
type ErrorInstance struct {
	Message   string
	Stack     string
	Source    string // file:line extracted from first app frame
	Timestamp time.Time
	ErrorType string // TypeError, ReferenceError, etc.
	Severity  string // error, warning
}

// ErrorCluster groups related errors by root cause.
type ErrorCluster struct {
	ID             string
	Representative ErrorInstance
	NormalizedMsg  string
	CommonFrames   []StackFrame
	RootCause      string
	Instances      []ErrorInstance
	InstanceCount  int
	FirstSeen      time.Time
	LastSeen       time.Time
	AffectedFiles  []string
	Severity       string
	alertedAt3     bool // track if we already alerted for this cluster
}

// ClusterManager manages error clustering with session-scoped lifecycle.
type ClusterManager struct {
	mu             sync.RWMutex
	clusters       []*ErrorCluster
	unclustered    []ErrorInstance
	nextID         int
	expiryDuration time.Duration
	pendingAlert   *Alert
	totalErrors    int
}

// ClusterAnalysisResponse is the response for analyze(target: "errors").
type ClusterAnalysisResponse struct {
	Clusters          []ClusterSummary `json:"clusters"`
	UnclusteredErrors int              `json:"unclustered_errors"`
	TotalErrors       int              `json:"total_errors"`
	Summary           string           `json:"summary"`
}

// ClusterSummary is a single cluster in the analysis response.
type ClusterSummary struct {
	ID                string   `json:"id"`
	RepresentativeMsg string   `json:"representative_error"`
	RootCause         string   `json:"root_cause"`
	InstanceCount     int      `json:"instance_count"`
	FirstSeen         string   `json:"first_seen"`
	LastSeen          string   `json:"last_seen"`
	AffectedFiles     []string `json:"affected_components"`
	Severity          string   `json:"severity"`
}

// NewClusterManager creates an empty cluster manager.
func NewClusterManager() *ClusterManager {
	return &ClusterManager{
		clusters:       make([]*ErrorCluster, 0),
		unclustered:    make([]ErrorInstance, 0),
		expiryDuration: 5 * time.Minute,
	}
}

// AddError adds an error to a cluster or creates a new one.
func (cm *ClusterManager) AddError(err ErrorInstance) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.totalErrors++

	frames := parseStack(err.Stack)
	appFrames := appFrames(frames)
	normMsg := normalizeErrorMessage(err.Message)

	if cm.tryMatchExistingCluster(err, appFrames, normMsg) {
		return
	}
	if cm.tryFormNewCluster(err, appFrames, normMsg) {
		return
	}
	cm.addUnclustered(err)
}

// tryMatchExistingCluster attempts to add the error to an existing cluster.
func (cm *ClusterManager) tryMatchExistingCluster(err ErrorInstance, appFrames []StackFrame, normMsg string) bool {
	for _, cluster := range cm.clusters {
		if cm.matchesCluster(cluster, err, appFrames, normMsg) {
			cm.addToCluster(cluster, err)
			return true
		}
	}
	return false
}

// tryFormNewCluster attempts to form a new cluster from an unclustered error.
func (cm *ClusterManager) tryFormNewCluster(err ErrorInstance, appFrames []StackFrame, normMsg string) bool {
	for i, unc := range cm.unclustered {
		signals := cm.countSignals(unc, err, appFrames, normMsg)
		if signals < 2 && (signals < 1 || unc.Stack != "" || err.Stack != "") {
			continue
		}
		cluster := cm.createCluster(unc, err, normMsg)
		cm.clusters = append(cm.clusters, cluster)
		cm.removeUnclustered(i)
		cm.enforceClusterCap()
		return true
	}
	return false
}
