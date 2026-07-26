// version_compare.go — Parses and compares semver version strings for upgrade detection and version mismatch checks.
// Why: Supports the takeover policy, version check, and binary watcher features without depending on external semver libraries.

package daemonlife

import (
	"strconv"
	"strings"
)

// ParseVersionParts splits a version string like "0.7.5" or "v0.7.5" into integer parts.
// Returns nil if the version string is empty or contains no valid numeric parts.
func ParseVersionParts(v string) []int {
	v = strings.TrimPrefix(v, "v")
	if v == "" {
		return nil
	}
	segments := strings.Split(v, ".")
	parts := make([]int, 0, len(segments))
	for _, seg := range segments {
		n, err := strconv.Atoi(seg)
		if err != nil {
			break
		}
		parts = append(parts, n)
	}
	if len(parts) == 0 {
		return nil
	}
	return parts
}

// sameNonEmptyVersion reports whether a and b are both non-empty and parse to the
// same semver (neither strictly newer). Used to gate the install-epoch takeover
// tiebreaker to genuinely equal versions — a blank/unparseable version is never
// treated as "same", so the epoch rule can't fire on unknown data.
func sameNonEmptyVersion(a, b string) bool {
	return a != "" && b != "" && !IsNewerVersion(a, b) && !IsNewerVersion(b, a)
}

// IsNewerVersion returns true if candidate is strictly newer than current.
// Both strings are parsed as semver (with optional "v" prefix).
// Returns false for equal versions, malformed input, or empty strings.
func IsNewerVersion(candidate, current string) bool {
	cParts := ParseVersionParts(candidate)
	rParts := ParseVersionParts(current)
	if cParts == nil || rParts == nil {
		return false
	}

	// Compare element-by-element, zero-padding the shorter slice.
	maxLen := len(cParts)
	if len(rParts) > maxLen {
		maxLen = len(rParts)
	}
	for i := 0; i < maxLen; i++ {
		c, r := 0, 0
		if i < len(cParts) {
			c = cParts[i]
		}
		if i < len(rParts) {
			r = rParts[i]
		}
		if c > r {
			return true
		}
		if c < r {
			return false
		}
	}
	return false // equal
}
