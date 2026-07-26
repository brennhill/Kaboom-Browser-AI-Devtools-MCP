// Purpose: Parameterizes dynamic URL path segments (UUIDs, numeric IDs, hashes) into schema placeholders.
// Why: Separates URL normalization from the main schema store to keep path canonicalization testable.
// Docs: docs/features/feature/api-schema/index.md

package apischema

import (
	"regexp"
	"strings"
)

// ============================================
// Path Parameterization
// ============================================

var (
	uuidPattern    = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	numericPattern = regexp.MustCompile(`^\d+$`)
	hexHashPattern = regexp.MustCompile(`^[0-9a-fA-F]{16,}$`)
)

// parameterizePath replaces dynamic path segments with placeholders
func parameterizePath(path string) string {
	segments := strings.Split(path, "/")
	for i, seg := range segments {
		if seg == "" {
			continue
		}
		if uuidPattern.MatchString(seg) {
			segments[i] = "{uuid}"
		} else if numericPattern.MatchString(seg) {
			segments[i] = "{id}"
		} else if hexHashPattern.MatchString(seg) {
			segments[i] = "{hash}"
		}
	}
	return strings.Join(segments, "/")
}
