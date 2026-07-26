// Purpose: Matches incoming errors to existing clusters using signal-counting heuristics.
// Why: Isolates cluster matching logic from cluster lifecycle management.
// Docs: docs/features/feature/error-clustering/index.md

package clustering

import (
	"fmt"
	"time"
)

func (cm *ClusterManager) matchesCluster(cluster *ErrorCluster, err ErrorInstance, appFr []StackFrame, normMsg string) bool {
	if err.Stack == "" && len(cluster.Instances) > 0 && cluster.Instances[0].Stack == "" && cluster.NormalizedMsg == normMsg {
		return true
	}
	return cm.clusterSignalCount(cluster, err, appFr, normMsg) >= 2
}

func (cm *ClusterManager) clusterSignalCount(cluster *ErrorCluster, err ErrorInstance, appFr []StackFrame, normMsg string) int {
	signals := 0
	if cluster.NormalizedMsg == normMsg {
		signals++
	}
	if len(appFr) > 0 && len(cluster.CommonFrames) > 0 && countSharedFrames(appFr, cluster.CommonFrames) >= 1 {
		signals++
	}
	if !cluster.LastSeen.IsZero() && err.Timestamp.Sub(cluster.LastSeen) < 2*time.Second {
		signals++
	}
	return signals
}

func (cm *ClusterManager) countSignals(existing, new ErrorInstance, newAppFr []StackFrame, newNormMsg string) int {
	signals := 0

	existingNorm := normalizeErrorMessage(existing.Message)
	if existingNorm == newNormMsg {
		signals++
	}

	existingFrames := appFrames(parseStack(existing.Stack))
	if len(existingFrames) > 0 && len(newAppFr) > 0 {
		if countSharedFrames(existingFrames, newAppFr) >= 1 {
			signals++
		}
	}

	if new.Timestamp.Sub(existing.Timestamp) < 2*time.Second {
		signals++
	}

	return signals
}

func inferRootCause(commonFrames []StackFrame, normMsg string) string {
	for _, f := range commonFrames {
		if !f.IsFramework {
			if f.Function != "<anonymous>" && f.Function != "<unknown>" {
				return fmt.Sprintf("%s (%s:%d)", f.Function, f.File, f.Line)
			}
			return fmt.Sprintf("%s:%d", f.File, f.Line)
		}
	}
	return normMsg
}
