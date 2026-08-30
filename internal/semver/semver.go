// semver.go — Ordering for optionally v-prefixed semantic versions.
// Why: daemon takeover, binary-upgrade watching, and update prompts all need the
// same answer to "is this build newer". This lives in internal/ so both the
// composition root's packages and shared libraries can use one implementation;
// the layering contract forbids internal/** importing cmd/**, which is why it
// cannot stay in cmd/browser-agent/internal/daemonlife.

package semver

import (
	"strconv"
	"strings"
)

// Parts splits an optionally v-prefixed version into its numeric components,
// stopping at the first non-numeric segment. It returns nil when nothing numeric
// could be read, which callers treat as "not comparable".
func Parts(value string) []int {
	value = strings.TrimPrefix(value, "v")
	if value == "" {
		return nil
	}
	segments := strings.Split(value, ".")
	parts := make([]int, 0, len(segments))
	for _, segment := range segments {
		part, err := strconv.Atoi(segment)
		if err != nil || part < 0 {
			break
		}
		parts = append(parts, part)
	}
	if len(parts) == 0 {
		return nil
	}
	return parts
}

// IsNewer reports whether candidate is strictly newer than current. An
// uncomparable version on either side is never newer: an unreadable version must
// not be able to displace a running daemon.
func IsNewer(candidate, current string) bool {
	candidateParts := Parts(candidate)
	currentParts := Parts(current)
	if candidateParts == nil || currentParts == nil {
		return false
	}
	count := len(candidateParts)
	if len(currentParts) > count {
		count = len(currentParts)
	}
	for index := 0; index < count; index++ {
		candidatePart, currentPart := 0, 0
		if index < len(candidateParts) {
			candidatePart = candidateParts[index]
		}
		if index < len(currentParts) {
			currentPart = currentParts[index]
		}
		if candidatePart != currentPart {
			return candidatePart > currentPart
		}
	}
	return false
}

// Same reports whether two non-empty versions compare equal.
func Same(a, b string) bool {
	return a != "" && b != "" && !IsNewer(a, b) && !IsNewer(b, a)
}
