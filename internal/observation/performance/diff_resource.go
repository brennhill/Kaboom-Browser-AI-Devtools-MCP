// Purpose: Computes resource-level before/after differences for navigation snapshots.
// Why: Separates resource change detection from core metric diff/rating logic.
// Docs: docs/features/feature/performance-audit/index.md

package performance

// buildResourceMap indexes resource entries by URL.
func buildResourceMap(entries []ResourceEntry) map[string]ResourceEntry {
	m := make(map[string]ResourceEntry, len(entries))
	for _, resource := range entries {
		m[resource.URL] = resource
	}
	return m
}

// isSignificantSizeChange returns true if the size delta is >= 10% OR >= 1KB.
func isSignificantSizeChange(beforeSize, delta int64) bool {
	absDelta := delta
	if absDelta < 0 {
		absDelta = -absDelta
	}
	if beforeSize == 0 {
		return absDelta >= 1024
	}
	pctChange := float64(absDelta) / float64(beforeSize) * 100
	return pctChange >= 10 || absDelta >= 1024
}

// ComputeResourceDiffForNav compares resource lists and categorizes changes.
// Resources are matched by URL. Small changes (<10% AND <1KB) are ignored.
func ComputeResourceDiffForNav(before, after []ResourceEntry) ResourceDiff {
	diff := ResourceDiff{}
	beforeMap := buildResourceMap(before)
	afterMap := buildResourceMap(after)

	for _, resource := range before {
		if _, exists := afterMap[resource.URL]; !exists {
			diff.Removed = append(diff.Removed, RemovedResource{
				URL: resource.URL, Type: resource.Type, SizeBytes: resource.TransferSize,
			})
		}
	}

	for _, resource := range after {
		if _, exists := beforeMap[resource.URL]; !exists {
			diff.Added = append(diff.Added, AddedResource{
				URL: resource.URL, Type: resource.Type, SizeBytes: resource.TransferSize,
				DurationMs: resource.Duration, RenderBlocking: resource.RenderBlocking,
			})
		}
	}

	for _, afterResource := range after {
		beforeResource, exists := beforeMap[afterResource.URL]
		if !exists {
			continue
		}
		delta := afterResource.TransferSize - beforeResource.TransferSize
		if delta == 0 || !isSignificantSizeChange(beforeResource.TransferSize, delta) {
			continue
		}
		diff.Resized = append(diff.Resized, ResizedResource{
			URL: afterResource.URL, BaselineBytes: beforeResource.TransferSize,
			CurrentBytes: afterResource.TransferSize, DeltaBytes: delta,
		})
	}

	return diff
}
