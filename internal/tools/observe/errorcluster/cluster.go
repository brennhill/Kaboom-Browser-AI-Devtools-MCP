// Purpose: Groups recurring error log entries into clusters for analyze(what:"error_clusters").
// Why: A page throwing one bug 50 times should read as one cluster of 50, not 50 findings. Lives in
// its own package so the clustering rules and their normalizer stay together and carry their own
// coverage number, rather than being three helpers buried in the console-stream file.
// Docs: docs/features/feature/error-clustering/index.md

package errorcluster

import (
	"sort"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// keyCap bounds the fingerprint length so a stack-laden message cannot become a
// multi-kilobyte map key. Applied after normalization, on the placeholder form.
const keyCap = 100

// cluster is one group of error instances sharing a normalized fingerprint.
type cluster struct {
	message    string // first raw message seen; the human-readable representative
	pattern    string // normalized fingerprint these instances share
	level      string
	count      int
	firstSeen  string
	lastSeen   string
	urls       map[string]bool
	stackTrace string
}

// Analyze groups error-level entries into clusters and renders them for the MCP response.
//
// Ordering is deterministic: largest cluster first, ties broken by pattern. Both this
// list and each cluster's url list used to be produced by ranging a Go map, so identical
// input returned a different order on every call — undiffable across runs and impossible
// to pin with a golden test.
func Analyze(entries []types.LogEntry) []map[string]any {
	return toResponse(build(entries))
}

func build(entries []types.LogEntry) map[string]*cluster {
	clusters := make(map[string]*cluster)
	for _, entry := range entries {
		level, _ := entry["level"].(string)
		if level != "error" {
			continue
		}
		msg, _ := entry["message"].(string)
		if msg == "" {
			continue
		}

		// Fingerprint on the normalized form. Keying on the raw message meant any error
		// carrying an id, uuid, url or timestamp never clustered with its own siblings —
		// "…at /users/12345" and "…at /users/67890" were two separate findings, which
		// defeats the one thing clustering exists to do.
		key := normalizeErrorMessage(msg)
		if len(key) > keyCap {
			key = key[:keyCap]
		}

		timestamp, _ := entry["timestamp"].(string)
		url, _ := entry["url"].(string)
		stack, _ := entry["stackTrace"].(string)

		addToCluster(clusters, clusterEntry{key: key, msg: msg, level: level, timestamp: timestamp, url: url, stack: stack})
	}
	return clusters
}

// clusterEntry is the error-level fields of one log entry, extracted and ready
// for clustering. key is the capped normalized fingerprint.
type clusterEntry struct {
	key, msg, level, timestamp, url, stack string
}

func addToCluster(clusters map[string]*cluster, entry clusterEntry) {
	if existing, ok := clusters[entry.key]; ok {
		existing.count++
		existing.lastSeen = entry.timestamp
		if entry.url != "" {
			existing.urls[entry.url] = true
		}
		return
	}
	urls := make(map[string]bool)
	if entry.url != "" {
		urls[entry.url] = true
	}
	clusters[entry.key] = &cluster{
		message:    entry.msg,
		pattern:    entry.key,
		level:      entry.level,
		count:      1,
		firstSeen:  entry.timestamp,
		lastSeen:   entry.timestamp,
		urls:       urls,
		stackTrace: entry.stack,
	}
}

func toResponse(clusters map[string]*cluster) []map[string]any {
	ordered := make([]*cluster, 0, len(clusters))
	for _, c := range clusters {
		ordered = append(ordered, c)
	}
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].count != ordered[j].count {
			return ordered[i].count > ordered[j].count
		}
		return ordered[i].pattern < ordered[j].pattern
	})

	result := make([]map[string]any, 0, len(ordered))
	for _, c := range ordered {
		urlList := make([]string, 0, len(c.urls))
		for u := range c.urls {
			urlList = append(urlList, u)
		}
		sort.Strings(urlList)
		result = append(result, map[string]any{
			"message":     c.message,
			"count":       c.count,
			"first_seen":  c.firstSeen,
			"last_seen":   c.lastSeen,
			"urls":        urlList,
			"stack_trace": c.stackTrace,
		})
	}
	return result
}
