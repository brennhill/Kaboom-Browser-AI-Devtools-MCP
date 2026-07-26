// Purpose: Normalizes error messages for stable cluster fingerprinting by replacing dynamic tokens.
// Why: Separates message normalization regexes from cluster matching to keep both focused.
// Docs: docs/features/feature/error-clustering/index.md

package clustering

import "regexp"

var (
	clusterUUIDRegex      = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	clusterURLRegex       = regexp.MustCompile(`https?://[^\s"']+`)
	clusterTimestampRegex = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}[^\s]*`)
	clusterNumericIDRegex = regexp.MustCompile(`\b\d{3,}\b`)
)

func normalizeErrorMessage(msg string) string {
	result := clusterUUIDRegex.ReplaceAllString(msg, "{uuid}")
	result = clusterURLRegex.ReplaceAllString(result, "{url}")
	result = clusterTimestampRegex.ReplaceAllString(result, "{timestamp}")
	result = clusterNumericIDRegex.ReplaceAllString(result, "{id}")
	return result
}
