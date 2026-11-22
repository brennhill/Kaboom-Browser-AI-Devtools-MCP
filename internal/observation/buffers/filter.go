// filter.go — Generic reverse-iterate-filter-limit for newest-first queries.

package buffers

// ReverseFilterLimit iterates a slice from end to start, applies a filter
// predicate, and collects up to limit matching elements. Returns results
// in newest-first order. A limit of 0 or negative means no limit.
func ReverseFilterLimit[T any](slice []T, filter func(T) bool, limit int) []T {
	if limit <= 0 {
		limit = len(slice)
	}
	results := make([]T, 0, min(limit, len(slice)))
	for i := len(slice) - 1; i >= 0 && len(results) < limit; i-- {
		if filter(slice[i]) {
			results = append(results, slice[i])
		}
	}
	return results
}
