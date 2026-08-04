// Purpose: Compares network requests between two snapshots by endpoint key to find new, missing, and status-changed entries.
// Docs: docs/features/feature/request-session-correlation/index.md

// network.go — Network diff computation.
package snapdiff

import (
	"fmt"
	"sort"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// endpointKey uniquely identifies a network endpoint by method and path.
type endpointKey struct {
	Method string
	Path   string
}

// buildEndpointMap indexes network requests by (method, path).
func buildEndpointMap(requests []types.SnapshotNetworkRequest) map[endpointKey]types.SnapshotNetworkRequest {
	m := make(map[endpointKey]types.SnapshotNetworkRequest, len(requests))
	for _, req := range requests {
		key := endpointKey{Method: req.Method, Path: util.ExtractURLPath(req.URL)}
		m[key] = req
	}
	return m
}

// formatDurationChange returns a formatted duration delta string, or "" if not applicable.
func formatDurationChange(beforeDur, afterDur int) string {
	if beforeDur <= 0 || afterDur <= 0 {
		return ""
	}
	delta := afterDur - beforeDur
	if delta >= 0 {
		return fmt.Sprintf("+%dms", delta)
	}
	return fmt.Sprintf("%dms", delta)
}

// Network compares network requests between two snapshots.
// Requests are matched by (method, URL path) — query params are ignored.
func Network(a, b *types.NamedSnapshot) NetworkDiff {
	diff := NetworkDiff{
		NewErrors:         make([]types.SnapshotNetworkRequest, 0),
		StatusChanges:     make([]NetworkChange, 0),
		NewEndpoints:      make([]types.SnapshotNetworkRequest, 0),
		MissingEndpoints:  make([]types.SnapshotNetworkRequest, 0),
		DuplicateRequests: make([]DuplicateRequestChange, 0),
	}

	aEndpoints := buildEndpointMap(a.NetworkRequests)
	bEndpoints := buildEndpointMap(b.NetworkRequests)
	aCounts := endpointCounts(a.NetworkRequests)
	bCounts := endpointCounts(b.NetworkRequests)
	countKeys := make(map[endpointKey]struct{}, len(aCounts)+len(bCounts))
	for key := range aCounts {
		countKeys[key] = struct{}{}
	}
	for key := range bCounts {
		countKeys[key] = struct{}{}
	}
	for key := range countKeys {
		after := bCounts[key]
		before := aCounts[key]
		if (after > 1 || before > 1) && after != before {
			url := aEndpoints[key].URL
			if request, exists := bEndpoints[key]; exists {
				url = request.URL
			}
			diff.DuplicateRequests = append(diff.DuplicateRequests, DuplicateRequestChange{
				Method: key.Method, URL: url, Before: before, After: after,
			})
		}
	}

	// New endpoints = in B but not in A
	for key, req := range bEndpoints {
		if _, found := aEndpoints[key]; !found {
			diff.NewEndpoints = append(diff.NewEndpoints, req)
			if req.Status >= 400 {
				diff.NewErrors = append(diff.NewErrors, req)
			}
		}
	}

	// Missing endpoints = in A but not in B
	for key, req := range aEndpoints {
		if _, found := bEndpoints[key]; !found {
			diff.MissingEndpoints = append(diff.MissingEndpoints, req)
		}
	}

	// Status changes = same endpoint, different status
	for key, aReq := range aEndpoints {
		bReq, found := bEndpoints[key]
		if !found || aReq.Status == bReq.Status {
			continue
		}
		change := NetworkChange{
			Method:         key.Method,
			URL:            aReq.URL,
			BeforeStatus:   aReq.Status,
			AfterStatus:    bReq.Status,
			DurationChange: formatDurationChange(aReq.Duration, bReq.Duration),
		}
		diff.StatusChanges = append(diff.StatusChanges, change)
		if bReq.Status >= 400 && aReq.Status < 400 {
			diff.NewErrors = append(diff.NewErrors, bReq)
		}
	}

	sortNetworkDiff(&diff)
	return diff
}

func endpointCounts(requests []types.SnapshotNetworkRequest) map[endpointKey]int {
	counts := make(map[endpointKey]int)
	for _, request := range requests {
		counts[endpointKey{Method: request.Method, Path: util.ExtractURLPath(request.URL)}]++
	}
	return counts
}

func sortNetworkDiff(diff *NetworkDiff) {
	sort.Slice(diff.NewErrors, func(i, j int) bool { return requestSortKey(diff.NewErrors[i]) < requestSortKey(diff.NewErrors[j]) })
	sort.Slice(diff.NewEndpoints, func(i, j int) bool {
		return requestSortKey(diff.NewEndpoints[i]) < requestSortKey(diff.NewEndpoints[j])
	})
	sort.Slice(diff.MissingEndpoints, func(i, j int) bool {
		return requestSortKey(diff.MissingEndpoints[i]) < requestSortKey(diff.MissingEndpoints[j])
	})
	sort.Slice(diff.StatusChanges, func(i, j int) bool {
		return diff.StatusChanges[i].Method+diff.StatusChanges[i].URL < diff.StatusChanges[j].Method+diff.StatusChanges[j].URL
	})
	sort.Slice(diff.DuplicateRequests, func(i, j int) bool {
		return diff.DuplicateRequests[i].Method+diff.DuplicateRequests[i].URL < diff.DuplicateRequests[j].Method+diff.DuplicateRequests[j].URL
	})
}

func requestSortKey(request types.SnapshotNetworkRequest) string { return request.Method + request.URL }
