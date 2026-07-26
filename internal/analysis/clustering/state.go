// Purpose: Exposes cluster snapshot/query APIs and periodic cleanup behavior.
// Why: Separates read/report lifecycle methods from clustering mutation logic.
// Docs: docs/features/feature/error-clustering/index.md

package clustering

import (
	"fmt"
	"time"
)

// GetClusters returns all active clusters.
func (cm *ClusterManager) GetClusters() []*ErrorCluster {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	result := make([]*ErrorCluster, 0, len(cm.clusters))
	for _, cluster := range cm.clusters {
		clone := *cluster
		clone.Instances = append([]ErrorInstance(nil), cluster.Instances...)
		clone.CommonFrames = append([]StackFrame(nil), cluster.CommonFrames...)
		clone.AffectedFiles = append([]string(nil), cluster.AffectedFiles...)
		result = append(result, &clone)
	}
	return result
}

// UnclusteredCount returns the number of errors not in any cluster.
func (cm *ClusterManager) UnclusteredCount() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return len(cm.unclustered)
}

// DrainAlert returns and clears the pending alert.
func (cm *ClusterManager) DrainAlert() *Alert {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	alert := cm.pendingAlert
	cm.pendingAlert = nil
	return alert
}

// Cleanup removes expired clusters.
func (cm *ClusterManager) Cleanup() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	now := time.Now()
	active := make([]*ErrorCluster, 0, len(cm.clusters))
	for _, cluster := range cm.clusters {
		if now.Sub(cluster.LastSeen) < cm.expiryDuration {
			active = append(active, cluster)
		}
	}
	cm.clusters = active
}

// GetAnalysisResponse builds the response for analyze(target: "errors").
func (cm *ClusterManager) GetAnalysisResponse() ClusterAnalysisResponse {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	clusters := make([]ClusterSummary, 0, len(cm.clusters))
	for _, cluster := range cm.clusters {
		clusters = append(clusters, ClusterSummary{
			ID:                cluster.ID,
			RepresentativeMsg: cluster.Representative.Message,
			RootCause:         cluster.RootCause,
			InstanceCount:     cluster.InstanceCount,
			FirstSeen:         cluster.FirstSeen.UTC().Format(time.RFC3339),
			LastSeen:          cluster.LastSeen.UTC().Format(time.RFC3339),
			AffectedFiles:     append([]string(nil), cluster.AffectedFiles...),
			Severity:          cluster.Severity,
		})
	}

	summary := fmt.Sprintf("%d error clusters identified. %d unclustered errors. %d total errors.",
		len(clusters), len(cm.unclustered), cm.totalErrors)
	if len(cm.clusters) > 0 {
		top := cm.clusters[0]
		summary = fmt.Sprintf("%d error clusters. Primary: %s (%d instances). %d unclustered.",
			len(clusters), top.RootCause, top.InstanceCount, len(cm.unclustered))
	}

	return ClusterAnalysisResponse{
		Clusters:          clusters,
		UnclusteredErrors: len(cm.unclustered),
		TotalErrors:       cm.totalErrors,
		Summary:           summary,
	}
}
