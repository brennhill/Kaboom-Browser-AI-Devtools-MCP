// Purpose: Implements cluster mutation operations and cluster creation details.
// Why: Keeps cluster lifecycle state changes separate from manager entry points.
// Docs: docs/features/feature/error-clustering/index.md

package clustering

import (
	"fmt"
	"time"
)

// removeUnclustered removes an entry by index from the unclustered slice.
func (cm *ClusterManager) removeUnclustered(i int) {
	newUnclustered := make([]ErrorInstance, len(cm.unclustered)-1)
	copy(newUnclustered, cm.unclustered[:i])
	copy(newUnclustered[i:], cm.unclustered[i+1:])
	cm.unclustered = newUnclustered
}

// enforceClusterCap removes the oldest cluster if the cap is exceeded.
func (cm *ClusterManager) enforceClusterCap() {
	if len(cm.clusters) > 50 {
		newClusters := make([]*ErrorCluster, len(cm.clusters)-1)
		copy(newClusters, cm.clusters[1:])
		cm.clusters = newClusters
	}
}

// addUnclustered adds an error to the unclustered list with FIFO eviction.
func (cm *ClusterManager) addUnclustered(err ErrorInstance) {
	cm.unclustered = append(cm.unclustered, err)
	if len(cm.unclustered) > 100 {
		newUnclustered := make([]ErrorInstance, 100)
		copy(newUnclustered, cm.unclustered[len(cm.unclustered)-100:])
		cm.unclustered = newUnclustered
	}
}

// addToCluster adds an error to an existing cluster.
func (cm *ClusterManager) addToCluster(cluster *ErrorCluster, err ErrorInstance) {
	cluster.InstanceCount++
	cluster.LastSeen = err.Timestamp

	if len(cluster.Instances) < 20 {
		cluster.Instances = append(cluster.Instances, err)
	}

	frames := appFrames(parseStack(err.Stack))
	for _, frame := range frames {
		found := false
		for _, existing := range cluster.AffectedFiles {
			if existing == frame.File {
				found = true
				break
			}
		}
		if !found {
			cluster.AffectedFiles = append(cluster.AffectedFiles, frame.File)
		}
	}

	if cluster.InstanceCount == 3 && !cluster.alertedAt3 {
		cluster.alertedAt3 = true
		cm.pendingAlert = &Alert{
			Severity:  "error",
			Category:  "error_cluster",
			Title:     fmt.Sprintf("Error cluster: %d related errors from %s", cluster.InstanceCount, cluster.RootCause),
			Detail:    fmt.Sprintf("Cluster %s: %s", cluster.ID, cluster.Representative.Message),
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Source:    "clustering",
		}
	}
}

// createCluster creates a new cluster from two matching errors.
func (cm *ClusterManager) createCluster(first, second ErrorInstance, normMsg string) *ErrorCluster {
	cm.nextID++
	id := fmt.Sprintf("cluster_%d", cm.nextID)

	frames1 := appFrames(parseStack(first.Stack))
	frames2 := appFrames(parseStack(second.Stack))
	common := findCommonFrames(frames1, frames2)
	rootCause := inferRootCause(common, normMsg)
	files := collectAffectedFiles(frames1, frames2)

	severity := "error"
	if first.Severity == "warning" && second.Severity == "warning" {
		severity = "warning"
	}

	return &ErrorCluster{
		ID:             id,
		Representative: first,
		NormalizedMsg:  normMsg,
		CommonFrames:   common,
		RootCause:      rootCause,
		Instances:      []ErrorInstance{first, second},
		InstanceCount:  2,
		FirstSeen:      first.Timestamp,
		LastSeen:       second.Timestamp,
		AffectedFiles:  files,
		Severity:       severity,
	}
}
